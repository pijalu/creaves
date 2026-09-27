package actions

import (
	"fmt"
	"strings"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// Fulfillment writers (§6.2): turn one occurrence into the real care /
// treatment record plus the linking care_plan_applications row. The
// applications UNIQUE (source_type, source_id, animal_id, due_at) makes the
// whole operation idempotent — a double submit fails the insert and is
// reported as "already applied", never duplicated (§6.2 step 1).
//
// The created records carry click time (now) as their date (§10.1); due_at
// lives only on the application row.

// planFulfillmentNone marks skipped/deferred applications: they create no
// fulfillment record (§4.5) but the model requires a non-empty id.
const planFulfillmentNone = "none"

// PlanApplyInput carries the §6.2 per-kind request fields plus the
// skip/defer fields (§10-A2/CP4).
type PlanApplyInput struct {
	Status string // applied | skipped | deferred (§4.5)

	// Skip/defer: mandatory reason (§10-CP4), stored in applications.note.
	Note string
	// Defer: snooze target, clamped before the next occurrence (§10-CP4).
	DeferredUntil *time.Time

	// Weighing: required (§10-L1).
	Weight string
	// Observation: required answer (§10-L1).
	Answer string
	// AnswerIsAlert: the answer equals the payload's alert_on outcome
	// (§10.1-6) — computed by the handler from the source payload.
	AnswerIsAlert bool
}

// planPayload is the merged read view of the §4.2 payload documents (the
// authoritative per-kind validation runs at save time — here we only read).
type planPayload struct {
	Instructions       string  `json:"instructions"`
	Note               string  `json:"note"`
	CaretypeID         string  `json:"caretype_id"`
	Drug               string  `json:"drug"`
	Dosage             string  `json:"dosage"`
	DosageFromTable    *bool   `json:"dosage_from_dosages_table"`
	Remarks            string  `json:"remarks"`
	Prompt             string  `json:"prompt"`
	AlertOn            *string `json:"alert_on"`
	AlertFollowUpHours *int    `json:"alert_follow_up_hours"`
}

func parsePlanPayload(src careplan.PlanSource) planPayload {
	var p planPayload
	_ = jsonUnmarshalStrictish(src.Payload(), &p)
	return p
}

// writePlanApplication fulfills one occurrence (§6.2): creates the
// fulfillment record for applied status, then the application row.
// Returns the stored application; the UNIQUE key surfaces duplicate
// submits as a MySQL 1062 error wrapped by pop.
func writePlanApplication(tx *pop.Connection, src careplan.PlanSource, animalID int, dueAt, now time.Time, userID uuid.UUID, in PlanApplyInput) (*models.CarePlanApplication, error) {
	if err := reverifyPlanSource(tx, src, animalID); err != nil {
		return nil, err
	}

	payload := parsePlanPayload(src)
	fType, fID := models.ApplicationFulfillmentCare, planFulfillmentNone

	if in.Status == models.ApplicationStatusApplied {
		var err error
		switch src.ActionKind() {
		case careplan.KindFeeding, careplan.KindCare:
			fID, err = writeCareFulfillment(tx, payload.CaretypeID, animalID, now, in, false)
		case careplan.KindCleanup:
			fID, err = writeCleanupFulfillment(tx, animalID, now, in)
		case careplan.KindWeighing:
			if strings.TrimSpace(in.Weight) == "" {
				return nil, fmt.Errorf("weight is required for weighing (§10-L1)")
			}
			fID, err = writeWeighingFulfillment(tx, animalID, now, in)
		case careplan.KindObservation:
			fID, err = writeObservationFulfillment(tx, src, payload, animalID, now, userID, in)
		case careplan.KindMedication:
			fType = models.ApplicationFulfillmentTreatment
			fID, err = writeMedicationFulfillment(tx, payload, animalID, dueAt, now, in)
		default:
			return nil, fmt.Errorf("unknown action kind %q", src.ActionKind())
		}
		if err != nil {
			return nil, err
		}
	}

	// §10-M3: closing the alert loop — a non-alert answer on a follow-up
	// observation writes the Réponse alerte care (reset_warning=1), which
	// clears the animal's red landing row.
	if in.Status == models.ApplicationStatusApplied &&
		src.ActionKind() == careplan.KindObservation &&
		src.SourceType() == careplan.SourceAnimal &&
		strings.HasPrefix(src.Name(), followUpPlanPrefix) &&
		!in.AnswerIsAlert {
		typeID, err := resetWarningCareType(tx)
		if err != nil {
			return nil, err
		}
		if err := tx.Create(&models.Care{Date: now, AnimalID: animalID, TypeID: typeID, Note: nulls.NewString(in.Answer)}); err != nil {
			return nil, err
		}
	}

	app := &models.CarePlanApplication{
		SourceType:      string(src.SourceType()),
		SourceID:        sourceIDUUID(src),
		SourceSnapshot:  sourceSnapshot(src),
		AnimalID:        animalID,
		DueAt:           dueAt,
		AppliedAt:       now,
		UserID:          userID,
		FulfillmentType: fType,
		FulfillmentID:   fID,
		Status:          in.Status,
		DeferredUntil:   in.DeferredUntil,
		Note:            in.Note,
	}
	if err := tx.Create(app); err != nil {
		return nil, err // UNIQUE violation ⇒ already applied (idempotent)
	}

	// §10-CP3: alert follow-up plans self-deactivate once closed.
	if in.Status != models.ApplicationStatusDeferred &&
		src.SourceType() == careplan.SourceAnimal &&
		src.ActionKind() == careplan.KindObservation &&
		strings.HasPrefix(src.Name(), followUpPlanPrefix) {
		if id, err := uuid.FromString(src.SourceID()); err == nil {
			tx.RawQuery("UPDATE care_animal_plans SET active = ? WHERE id = ?", false, id).Exec()
		}
	}
	return app, nil
}

// reverifyPlanSource re-checks that the source still produces the
// occurrence (§6.2 step 1): the source row must still exist and be active
// (matcher drift re-check happens on the day-plan reload; the UNIQUE key
// remains the concurrency backstop).
func reverifyPlanSource(tx *pop.Connection, src careplan.PlanSource, animalID int) error {
	switch src.SourceType() {
	case careplan.SourceRule:
		exists, err := tx.Where("id = ? AND active = ?", sourceIDUUID(src), true).Exists("care_rules")
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("rule %s is no longer active", src.SourceID())
		}
	case careplan.SourceAnimal:
		exists, err := tx.Where("id = ? AND active = ?", sourceIDUUID(src), true).Exists("care_animal_plans")
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("plan %s is no longer active", src.SourceID())
		}
	default:
		return fmt.Errorf("unknown plan source type %q", src.SourceType())
	}
	return nil
}

// resolveCaretypeUUID resolves a caretype id string, tolerating "" only via
// the fallback resolvers below.
func resolveCaretypeUUID(raw string) (uuid.UUID, error) {
	return uuid.FromString(strings.TrimSpace(raw))
}

// defaultCareType is the fallback caretype for plan-generated cares without
// a dedicated reference (cleanup/weighing/observation answers): the seeded
// default "Care" type, else any caretype.
func defaultCareType(tx *pop.Connection) (uuid.UUID, error) {
	var ct models.Caretype
	if err := tx.Where("def = ?", true).First(&ct); err == nil {
		return ct.ID, nil
	}
	if err := tx.First(&ct); err != nil {
		return uuid.Nil, fmt.Errorf("no caretype available: %w", err)
	}
	return ct.ID, nil
}

// warningCareType resolves the caretypes.warning=1 row ("Alerte", §10.1-6).
func warningCareType(tx *pop.Connection) (uuid.UUID, error) {
	var ct models.Caretype
	if err := tx.Where("warning = ?", true).First(&ct); err != nil {
		return uuid.Nil, fmt.Errorf("no warning caretype: %w", err)
	}
	return ct.ID, nil
}

// resetWarningCareType resolves the caretypes.reset_warning=1 row
// ("Réponse alerte", §10-M3).
func resetWarningCareType(tx *pop.Connection) (uuid.UUID, error) {
	var ct models.Caretype
	if err := tx.Where("reset_warning = ?", true).First(&ct); err != nil {
		return uuid.Nil, fmt.Errorf("no warning-response caretype: %w", err)
	}
	return ct.ID, nil
}

func writeCareFulfillment(tx *pop.Connection, caretypeID string, animalID int, now time.Time, in PlanApplyInput, clean bool) (string, error) {
	typeID, err := resolveCaretypeUUID(caretypeID)
	if err != nil {
		return "", fmt.Errorf("caretype_id missing or invalid in action payload")
	}
	care := &models.Care{
		Date:     now,
		AnimalID: animalID,
		TypeID:   typeID,
		Note:     nulls.NewString(in.Note),
	}
	if clean {
		care.Clean = nulls.NewBool(true)
	}
	if err := tx.Create(care); err != nil {
		return "", err
	}
	return care.ID.String(), nil
}

func writeCleanupFulfillment(tx *pop.Connection, animalID int, now time.Time, in PlanApplyInput) (string, error) {
	typeID, err := defaultCareType(tx)
	if err != nil {
		return "", err
	}
	care := &models.Care{
		Date:     now,
		AnimalID: animalID,
		TypeID:   typeID,
		Clean:    nulls.NewBool(true),
		Note:     nulls.NewString(in.Note),
	}
	if err := tx.Create(care); err != nil {
		return "", err
	}
	return care.ID.String(), nil
}

func writeWeighingFulfillment(tx *pop.Connection, animalID int, now time.Time, in PlanApplyInput) (string, error) {
	typeID, err := defaultCareType(tx)
	if err != nil {
		return "", err
	}
	care := &models.Care{
		Date:     now,
		AnimalID: animalID,
		TypeID:   typeID,
		Weight:   nulls.NewString(strings.TrimSpace(in.Weight)),
		Note:     nulls.NewString(in.Note),
	}
	if err := tx.Create(care); err != nil {
		return "", err
	}
	return care.ID.String(), nil
}

// ---------------------------------------------------------------------------
// Observation (§6.2, §10.1-6, §10-CP3, §10-M3): answer care + alert loop
// ---------------------------------------------------------------------------

// writeObservationFulfillment records the answer as a care-row note. When
// the answer is the alert outcome it also writes a Warning caretype care
// (red row until a Réponse alerte clears it) and auto-creates the single
// follow-up observation animal-plan so the alert loop always closes.
func writeObservationFulfillment(tx *pop.Connection, src careplan.PlanSource, payload planPayload, animalID int, now time.Time, userID uuid.UUID, in PlanApplyInput) (string, error) {
	if strings.TrimSpace(in.Answer) == "" {
		return "", fmt.Errorf("answer is required for observation (§10-L1)")
	}
	typeID, err := defaultCareType(tx)
	if err != nil {
		return "", err
	}
	note := in.Answer
	if in.Note != "" {
		note = in.Answer + " — " + in.Note
	}
	care := &models.Care{
		Date:     now,
		AnimalID: animalID,
		TypeID:   typeID,
		Note:     nulls.NewString(note),
	}
	if err := tx.Create(care); err != nil {
		return "", err
	}

	if in.AnswerIsAlert {
		if err := writeWarningCare(tx, animalID, now); err != nil {
			return "", err
		}
		if err := createAlertFollowUpPlan(tx, src, payload, animalID, now, userID); err != nil {
			return "", err
		}
	}
	return care.ID.String(), nil
}

// writeWarningCare writes the caretypes.warning=1 care ("Alerte") — the
// animal's landing row turns red until a Réponse alerte care (§10.1-6).
func writeWarningCare(tx *pop.Connection, animalID int, now time.Time) error {
	typeID, err := warningCareType(tx)
	if err != nil {
		return err
	}
	return tx.Create(&models.Care{Date: now, AnimalID: animalID, TypeID: typeID})
}

// createAlertFollowUpPlan auto-creates the single-occurrence follow-up
// observation plan (§10-CP3): name «Vérifier alerte: <prompt>», one
// occurrence at now + alert_follow_up_hours, self-deactivating after its
// own apply/skip (done in writePlanApplication).
func createAlertFollowUpPlan(tx *pop.Connection, src careplan.PlanSource, payload planPayload, animalID int, now time.Time, userID uuid.UUID) error {
	hours := defaultFollowUpHours
	if payload.AlertFollowUpHours != nil && *payload.AlertFollowUpHours >= 1 {
		hours = *payload.AlertFollowUpHours
	}
	due := now.Add(time.Duration(hours) * time.Hour)

	schedule := map[string]interface{}{
		"times":       []string{due.Format("15:04")},
		"anchor":      careplan.AnchorFixed,
		"anchor_date": due.Format("2006-01-02"),
		// generous windows: the follow-up must not expire before seen
		"grace_minutes":     12 * 60,
		"miss_after_hours":  48,
		"lookahead_minutes": 4 * 60,
	}
	schedJSON, err := jsonMarshal(schedule)
	if err != nil {
		return err
	}
	payloadJSON, err := jsonMarshal(planPayload{
		Instructions:       payload.Instructions,
		Prompt:             payload.Prompt,
		AlertOn:            payload.AlertOn, // chain on alert (§10-M3)
		AlertFollowUpHours: payload.AlertFollowUpHours,
	})
	if err != nil {
		return err
	}

	// replace an identical still-active follow-up instead of stacking
	tx.RawQuery("DELETE FROM care_animal_plans WHERE name = ? AND animal_id = ? AND active = ?",
		followUpPlanPrefix+payload.Prompt, animalID, true).Exec()

	plan := &models.CareAnimalPlan{
		AnimalID:      animalID,
		Name:          followUpPlanPrefix + payload.Prompt,
		ActionKind:    careplan.KindObservation,
		ActionPayload: payloadJSON,
		Schedule:      schedJSON,
		Active:        true,
		CreatedBy:     nulls.NewUUID(userID),
	}
	return tx.Create(plan)
}

// ---------------------------------------------------------------------------
// Medication (§6.2, §10-M1): treatments row + slot bucket mapping
// ---------------------------------------------------------------------------

// treatmentBucketBit maps the occurrence's due time to the legacy treatment
// slot bitmap (§10-M1): <11:00 → morning(1), 11:00–15:00 → noon(2),
// >15:00 → evening(4).
func treatmentBucketBit(due time.Time) int {
	h := due.Hour()
	switch {
	case h < 11:
		return models.Treatement_MORNING
	case h <= 15:
		return models.Treatement_NOON
	default:
		return models.Treatement_EVENING
	}
}

// writeMedicationFulfillment creates (or completes) the treatments row for
// the bucket of dueAt (§10-M1). Same-bucket collision: the bit is set by
// the first apply; later same-bucket applies append to the row's remarks —
// both application rows are still recorded.
func writeMedicationFulfillment(tx *pop.Connection, payload planPayload, animalID int, dueAt, now time.Time, in PlanApplyInput) (string, error) {
	dosage, err := resolvePlanDosage(tx, payload, animalID)
	if err != nil {
		return "", err
	}
	bit := treatmentBucketBit(dueAt)
	dayStart := time.Date(dueAt.Year(), dueAt.Month(), dueAt.Day(), 0, 0, 0, 0, dueAt.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	var treatments []models.Treatment
	if err := tx.Where("animal_id = ? AND drug = ? AND date >= ? AND date < ?",
		animalID, payload.Drug, dayStart, dayEnd).All(&treatments); err != nil {
		return "", err
	}
	for i := range treatments {
		t := &treatments[i]
		if t.Timebitmap&bit == 0 {
			continue
		}
		if t.Timedonebitmap&bit == 0 {
			t.Timedonebitmap |= bit
			if err := tx.Update(t); err != nil {
				return "", err
			}
			return t.ID.String(), nil
		}
		// same-bucket collision (§10-M1): append, keep both applications
		appendum := fmt.Sprintf("; %s", now.Format("15:04"))
		if in.Note != "" {
			appendum = fmt.Sprintf(" (%s)", in.Note)
		}
		t.Remarks = nulls.NewString(t.Remarks.String + appendum)
		if err := tx.Update(t); err != nil {
			return "", err
		}
		return t.ID.String(), nil
	}

	remarks := payload.Remarks
	if payload.Instructions != "" {
		if remarks != "" {
			remarks += " — "
		}
		remarks += payload.Instructions
	}
	treatment := &models.Treatment{
		Date:           now,
		AnimalID:       animalID,
		Drug:           payload.Drug,
		Dosage:         dosage,
		Remarks:        nulls.NewString(remarks),
		Timebitmap:     bit,
		Timedonebitmap: bit,
	}
	if err := tx.Create(treatment); err != nil {
		return "", err
	}
	return treatment.ID.String(), nil
}

// resolvePlanDosage resolves the §4.2 dosage path: literal dosage, or the
// dosages-table path (drugs × animaltype × last weight, §10-B6).
func resolvePlanDosage(tx *pop.Connection, payload planPayload, animalID int) (string, error) {
	if payload.Dosage != "" {
		return payload.Dosage, nil
	}
	if payload.DosageFromTable == nil || !*payload.DosageFromTable {
		return "", fmt.Errorf("medication payload has no dosage path")
	}

	var drug models.Drug
	if err := tx.Where("name = ?", payload.Drug).First(&drug); err != nil {
		return "", fmt.Errorf("drug %q not found", payload.Drug)
	}
	animal := &models.Animal{}
	if err := tx.Find(animal, animalID); err != nil {
		return "", err
	}
	var dosage models.Dosage
	if err := tx.Where("drug_id = ? AND animaltype_id = ? AND enabled = ?",
		drug.ID, animal.AnimaltypeID, true).First(&dosage); err != nil {
		return "", fmt.Errorf("no dosage for drug %q and animal type", payload.Drug)
	}
	perKilo := dosage.PerKilo()
	if !perKilo.Valid {
		return "", fmt.Errorf("dosage for drug %q carries no per-grams value", payload.Drug)
	}
	weight := lastWeightGrams(tx, animalID)
	if weight <= 0 {
		return "", fmt.Errorf("no weight on record — cannot resolve table dosage for animal %d", animalID)
	}
	dose := perKilo.Float64 * weight / 1000.0
	unit := dosage.DosagePerGramsUnit.String
	if unit == "" {
		unit = "ml"
	}
	return fmt.Sprintf("%.2f %s", dose, unit), nil
}

// lastWeightGrams reads the latest care weight (grams) of an animal.
func lastWeightGrams(tx *pop.Connection, animalID int) float64 {
	var rows []struct {
		Weight nulls.String `db:"weight"`
	}
	q := `SELECT weight FROM cares WHERE animal_id = ? AND weight IS NOT NULL AND weight <> ''
	      ORDER BY date DESC LIMIT 1`
	if err := tx.RawQuery(q, animalID).All(&rows); err != nil || len(rows) == 0 {
		return 0
	}
	var g float64
	if _, err := fmtSscan(rows[0].Weight.String, &g); err != nil {
		return 0
	}
	return g
}
