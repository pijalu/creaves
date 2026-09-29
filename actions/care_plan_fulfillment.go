package actions

import (
	"fmt"
	"sort"
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
	// Dosage: manual dosage override for medication (§10-B6) — mandatory
	// when automatic resolution warns (no weight, no dosages-table row…).
	Dosage string
}

// planPayload is the merged read view of the §4.2 payload documents (the
// authoritative per-kind validation runs at save time — here we only read).
type planPayload struct {
	Instructions       string  `json:"instructions"`
	Note               string  `json:"note"`
	CaretypeID         string  `json:"caretype_id"`
	Food               string  `json:"food"`
	ForceFeed          bool    `json:"force_feed"`
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
	// bugs.md R5-3b (U25/D-e): pre-generated application ID for medication
	// — the per-time entries reference it before the application insert.
	// Everything runs in one request transaction: a UNIQUE violation on the
	// application (double submit) rolls the entries back.
	var medicationAppID *uuid.UUID

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
			appID := uuid.Must(uuid.NewV4())
			fID, err = writeMedicationFulfillment(tx, src, payload, animalID, dueAt, now, userID, appID, in)
			if err == nil {
				medicationAppID = &appID
			}
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
	if in.Status == models.ApplicationStatusApplied {
		if err := maybeWriteResetWarningCare(tx, src, animalID, now, in); err != nil {
			return nil, err
		}
	}

	app := &models.CarePlanApplication{
		ID:              applicationIDOrDefault(medicationAppID),
		SourceType:      string(src.SourceType()),
		SourceID:        sourceIDUUID(src),
		SourceSnapshot:  sourceSnapshotWithDosage(src, in),
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
	if in.Status != models.ApplicationStatusDeferred {
		if err := maybeDeactivateFollowUpPlan(tx, src, in); err != nil {
			return nil, err
		}
	}
	return app, nil
}

// maybeWriteResetWarningCare implements §10-M3: a non-alert answer on a
// follow-up observation plan writes the Réponse alerte care
// (reset_warning=1), which clears the animal's red landing row.
func maybeWriteResetWarningCare(tx *pop.Connection, src careplan.PlanSource, animalID int, now time.Time, in PlanApplyInput) error {
	if src.ActionKind() != careplan.KindObservation ||
		src.SourceType() != careplan.SourceAnimal ||
		!strings.HasPrefix(src.Name(), followUpPlanPrefix) ||
		in.AnswerIsAlert {
		return nil
	}
	typeID, err := resetWarningCareType(tx)
	if err != nil {
		return err
	}
	return tx.Create(&models.Care{Date: now, AnimalID: animalID, TypeID: typeID, Note: nulls.NewString(in.Answer)})
}

// maybeDeactivateFollowUpPlan implements §10-CP3: alert follow-up plans
// self-deactivate once their occurrence is applied or skipped.
func maybeDeactivateFollowUpPlan(tx *pop.Connection, src careplan.PlanSource, in PlanApplyInput) error {
	if src.SourceType() != careplan.SourceAnimal ||
		src.ActionKind() != careplan.KindObservation ||
		!strings.HasPrefix(src.Name(), followUpPlanPrefix) {
		return nil
	}
	id, err := uuid.FromString(src.SourceID())
	if err != nil {
		return nil // not a uuid source id — nothing to deactivate
	}
	return tx.RawQuery("UPDATE care_animal_plans SET active = ? WHERE id = ?", false, id).Exec()
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
// Medication (§6.2, §10-M1, bugs.md U25 R5-3b): per-time treatment entries
// ---------------------------------------------------------------------------

// treatmentBucketBit maps the occurrence's due time to the legacy treatment
// slot bitmap (§10-M1): <11:00 → morning(1), 11:00–15:00 → noon(2),
// >15:00 → evening(4). Since R5-3 the bitmap only feeds the legacy display
// columns — the done state lives in treatment_time_entries (D-e dormant).
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

// applicationIDOrDefault returns the pre-generated medication application
// ID, or a fresh nil UUID sentinel when absent (non-medication paths pass
// nil — the applications row generates its own ID in that case).
func applicationIDOrDefault(pre *uuid.UUID) uuid.UUID {
	if pre != nil {
		return *pre
	}
	return uuid.Nil
}

// medicationDosage decides the dosage for one medication apply (§10-B6): a
// manual dosage wins verbatim; otherwise the automatic path must resolve —
// a warning without manual dosage becomes the 422 dosage_required flow.
func medicationDosage(tx *pop.Connection, payload planPayload, animalID int, in PlanApplyInput) (string, error) {
	if strings.TrimSpace(in.Dosage) != "" {
		return strings.TrimSpace(in.Dosage), nil
	}
	dosage, warn, err := resolvePlanDosage(tx, payload, animalID)
	if err != nil {
		return "", err
	}
	if warn != nil {
		return "", &DosageRequiredError{Warning: warn}
	}
	return dosage, nil
}

// scheduleTimeLabels returns the ascending HH:MM labels of the source
// schedule (bugs.md R5-3b): the day's expected times, always including the
// occurrence's own label. Falls back to the single occurrence label when
// the source carries no schedule times.
func scheduleTimeLabels(src careplan.PlanSource, dueLabel string) []string {
	set := map[string]bool{dueLabel: true}
	if src != nil {
		for _, t := range src.Schedule().Times {
			set[fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)] = true
		}
	}
	labels := make([]string, 0, len(set))
	for l := range set {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	return labels
}

// entryDueAt resolves the day+HH:MM label to the entry's due timestamp.
func entryDueAt(day time.Time, label string) time.Time {
	t, err := time.Parse("15:04", label)
	if err != nil {
		return day
	}
	return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, day.Location())
}

// applyEntryTo stamps one entry done for this application (bugs.md R5-3b):
// click time, user, application link and the optional caretaker note.
func applyEntryTo(e *models.TreatmentTimeEntry, now time.Time, userID, applicationID uuid.UUID, in PlanApplyInput) {
	e.MarkDone(now, userID)
	e.ApplicationID = nulls.NewUUID(applicationID)
	if note := strings.TrimSpace(in.Note); note != "" {
		e.Note = nulls.NewString(note)
	}
}

// loadTreatmentEntries reads the per-time entries of one treatment ordered
// by due time.
func loadTreatmentEntries(tx *pop.Connection, treatmentID uuid.UUID) (models.TreatmentTimeEntries, error) {
	entries := models.TreatmentTimeEntries{}
	err := tx.Where("treatment_id = ?", treatmentID).Order("due_at asc").All(&entries)
	return entries, err
}

// writeMedicationFulfillment realizes one medication occurrence as a
// per-time entry (bugs.md R5-3b, U25/D-e — the legacy Timedonebitmap is
// dormant). Two shapes:
//
//   - no same-day row for this animal+drug yet → create the treatments row
//     and seed ONE ENTRY PER EXPECTED TIME of the source schedule; the due
//     entry is done (click time, user, application), its siblings stay
//     pending — the treatment page then shows the whole day (E2E-3 step 1).
//   - a row exists → complete the pending entry matching the occurrence's
//     time label; when none is pending (same-slot second administration,
//     legacy rows), append a NEW done entry — per-time storage keeps every
//     administration distinct (legacy appended "; HH:MM (note)" remarks).
//
// bugs.md U11 still applies verbatim: a manual dosage overrides the row
// dosage. The row's Timebitmap keeps the legacy bucket for display
// compatibility; Timedonebitmap is never written (dormant, D-e).
func writeMedicationFulfillment(tx *pop.Connection, src careplan.PlanSource, payload planPayload, animalID int, dueAt, now time.Time, userID, applicationID uuid.UUID, in PlanApplyInput) (string, error) {
	dosage, err := medicationDosage(tx, payload, animalID, in)
	if err != nil {
		return "", err
	}
	timeLabel := dueAt.Format("15:04")
	dayStart := time.Date(dueAt.Year(), dueAt.Month(), dueAt.Day(), 0, 0, 0, 0, dueAt.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	var treatments []models.Treatment
	if err := tx.Where("animal_id = ? AND drug = ? AND date >= ? AND date < ?",
		animalID, payload.Drug, dayStart, dayEnd).All(&treatments); err != nil {
		return "", err
	}

	if len(treatments) == 0 {
		treatment, err := createTreatmentWithEntries(tx, src, payload, dosage, animalID, dueAt, dayStart, timeLabel, now, userID, applicationID, in)
		if err != nil {
			return "", err
		}
		return treatment.ID.String(), nil
	}
	return completeTreatmentEntry(tx, treatments, animalID, dayStart, timeLabel, now, userID, applicationID, in)
}

// createTreatmentWithEntries creates the day's treatments row (§10-M1) and
// seeds ONE ENTRY PER EXPECTED TIME of the source schedule; the due entry
// is done (click time, user, application), its siblings stay pending — the
// treatment page then shows the whole day (E2E-3 step 1).
func createTreatmentWithEntries(tx *pop.Connection, src careplan.PlanSource, payload planPayload, dosage string, animalID int, dueAt, dayStart time.Time, timeLabel string, now time.Time, userID, applicationID uuid.UUID, in PlanApplyInput) (*models.Treatment, error) {
	remarks := payload.Remarks
	if payload.Instructions != "" {
		if remarks != "" {
			remarks += " — "
		}
		remarks += payload.Instructions
	}
	treatment := &models.Treatment{
		Date:       now,
		AnimalID:   animalID,
		Drug:       payload.Drug,
		Dosage:     dosage,
		Remarks:    nulls.NewString(remarks),
		Timebitmap: treatmentBucketBit(dueAt),
	}
	if err := tx.Create(treatment); err != nil {
		return nil, err
	}
	for _, label := range scheduleTimeLabels(src, timeLabel) {
		entry := &models.TreatmentTimeEntry{
			TreatmentID: treatment.ID,
			AnimalID:    animalID,
			DueAt:       entryDueAt(dayStart, label),
			TimeLabel:   label,
			Status:      models.TreatmentEntryStatusPending,
			Source:      models.TreatmentEntrySourceProtocol,
		}
		if label == timeLabel {
			applyEntryTo(entry, now, userID, applicationID, in)
		}
		if err := tx.Create(entry); err != nil {
			return nil, err
		}
	}
	return treatment, nil
}

// completeTreatmentEntry completes the pending entry matching the
// occurrence's time label on the first row that carries one; when none is
// pending (second same-slot administration or a legacy row without
// entries) it appends a NEW done entry — per-time storage keeps every
// administration distinct (legacy appended "; HH:MM (note)" remarks).
// bugs.md U11: a manual dosage wins verbatim on the touched row.
func completeTreatmentEntry(tx *pop.Connection, treatments []models.Treatment, animalID int, dayStart time.Time, timeLabel string, now time.Time, userID, applicationID uuid.UUID, in PlanApplyInput) (string, error) {
	for i := range treatments {
		t := &treatments[i]
		entries, err := loadTreatmentEntries(tx, t.ID)
		if err != nil {
			return "", err
		}
		if e := entries.FindByLabel(timeLabel); e != nil && e.Status != models.TreatmentEntryStatusDone {
			if err := overrideTreatmentDosage(tx, t, in.Dosage); err != nil {
				return "", err
			}
			applyEntryTo(e, now, userID, applicationID, in)
			if err := tx.Update(e); err != nil {
				return "", err
			}
			return t.ID.String(), nil
		}
	}

	first := &treatments[0]
	if err := overrideTreatmentDosage(tx, first, in.Dosage); err != nil {
		return "", err
	}
	entry := &models.TreatmentTimeEntry{
		TreatmentID: first.ID,
		AnimalID:    animalID,
		DueAt:       entryDueAt(dayStart, timeLabel),
		TimeLabel:   timeLabel,
		Status:      models.TreatmentEntryStatusDone,
		Source:      models.TreatmentEntrySourceProtocol,
	}
	applyEntryTo(entry, now, userID, applicationID, in)
	if err := tx.Create(entry); err != nil {
		return "", err
	}
	return first.ID.String(), nil
}

// overrideTreatmentDosage applies the bugs.md U11 verbatim-manual-dosage
// rule: the caretaker's explicit dosage replaces the row's stale one.
func overrideTreatmentDosage(tx *pop.Connection, t *models.Treatment, manualDosage string) error {
	if manual := strings.TrimSpace(manualDosage); manual != "" {
		t.Dosage = manual
		return tx.Update(t)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Dosage resolution (§4.2, §10-B6, §10-L2)
// ---------------------------------------------------------------------------

// DosageWarning reports a §10-B6 dosage-resolution failure. It never blocks
// the apply: the caretaker resubmits with a manual dosage. §10-L2: the
// warning carries the animal's last recorded weight and its date.
type DosageWarning struct {
	Reason       string `json:"reason"` // machine key: no_dosage_path | drug_not_found | no_dosage_row | dosage_no_per_grams | no_weight
	Detail       string `json:"detail"` // human explanation
	LastWeight   string `json:"last_weight,omitempty"`
	LastWeightAt string `json:"last_weight_at,omitempty"` // RFC3339
}

// DosageRequiredError is returned by the medication fulfillment when the
// automatic dosage resolution warned and no manual dosage was supplied.
// The HTTP layer renders it as the structured 422 dosage_required body.
type DosageRequiredError struct {
	Warning *DosageWarning
}

func (e *DosageRequiredError) Error() string {
	return "dosage_required: " + e.Warning.Detail
}

// resolvePlanDosage resolves the §4.2 dosage path: literal dosage, or the
// dosages-table path (drugs × animaltype × last weight, §10-B6). Resolution
// failures are warnings (never block apply, §10-B6); err is reserved for
// unexpected DB failures.
func resolvePlanDosage(tx *pop.Connection, payload planPayload, animalID int) (string, *DosageWarning, error) {
	if payload.Dosage != "" {
		return payload.Dosage, nil, nil
	}
	lw := lastWeight(tx, animalID)
	warn := func(reason, detail string) (string, *DosageWarning, error) {
		w := &DosageWarning{Reason: reason, Detail: detail}
		if lw.ok {
			w.LastWeight = lw.raw
			w.LastWeightAt = lw.at.Format(time.RFC3339)
		}
		return "", w, nil
	}
	if payload.DosageFromTable == nil || !*payload.DosageFromTable {
		return warn("no_dosage_path", "medication payload has no dosage path")
	}

	var drug models.Drug
	if err := tx.Where("name = ?", payload.Drug).First(&drug); err != nil {
		return warn("drug_not_found", fmt.Sprintf("drug %q not found in the drugs table", payload.Drug))
	}
	animal := &models.Animal{}
	if err := tx.Find(animal, animalID); err != nil {
		return "", nil, err // unexpected: the animal of an occurrence must exist
	}
	var dosage models.Dosage
	if err := tx.Where("drug_id = ? AND animaltype_id = ? AND enabled = ?",
		drug.ID, animal.AnimaltypeID, true).First(&dosage); err != nil {
		return warn("no_dosage_row", fmt.Sprintf("no dosage row for drug %q and this animal type", payload.Drug))
	}
	perKilo := dosage.PerKilo()
	if !perKilo.Valid {
		return warn("dosage_no_per_grams", fmt.Sprintf("dosage row for drug %q carries no per-grams value", payload.Drug))
	}
	if !lw.ok || lw.grams <= 0 {
		return warn("no_weight", "no weight on record — cannot compute the table dosage")
	}
	dose := perKilo.Float64 * lw.grams / 1000.0
	unit := dosage.DosagePerGramsUnit.String
	if unit == "" {
		unit = "ml"
	}
	return fmt.Sprintf("%.2f %s", dose, unit), nil, nil
}

// lastWeightInfo is the animal's latest recorded care weight (§10-L2).
type lastWeightInfo struct {
	raw   string
	grams float64
	at    time.Time
	ok    bool
}

// lastWeight reads the latest care weight of an animal (raw string, grams
// when parseable, and the recording date).
func lastWeight(tx *pop.Connection, animalID int) lastWeightInfo {
	var rows []struct {
		Weight nulls.String `db:"weight"`
		Date   time.Time    `db:"date"`
	}
	q := `SELECT weight, date FROM cares WHERE animal_id = ? AND weight IS NOT NULL AND weight <> ''
	      ORDER BY date DESC LIMIT 1`
	if err := tx.RawQuery(q, animalID).All(&rows); err != nil || len(rows) == 0 {
		return lastWeightInfo{}
	}
	info := lastWeightInfo{raw: rows[0].Weight.String, at: rows[0].Date, ok: true}
	_, _ = fmtSscan(rows[0].Weight.String, &info.grams)
	return info
}
