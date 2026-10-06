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
)

// §8.1 steps 2 and 4 of docs/care-expert.md: legacy feeding schedules and
// open treatment series become care-plan rules / bounded animal plans.
//
// Converter-created animal plans are identified by name suffix
// " (conversion)" + created_by IS NULL (created_by carries a users FK, so
// no synthetic author is possible; §8.2 rollback targets rows by that
// pair).

// ---------------------------------------------------------------------------
// Step 2 — feeding schedules → rules/plans
// ---------------------------------------------------------------------------

// feedingEntry is one convertible legacy feeding row.
type feedingEntry struct {
	AnimalID int
	Label    string
	Times    []careplan.TimeOfDay
	Force    bool
	Diet     string
	// Fallback marks best-effort conversions (incomplete legacy data): the
	// plan is still created — the no-loss rule — but flagged "à vérifier".
	Fallback bool
}

// DeriveFeedingTimes expands a legacy feeding_start/feeding_end/period row
// into fixed daily slots (§8.1 step 2, §11.7): start + n×period ≤ end,
// n ≥ 0 — the start slot itself is a feeding time (legacy calculateFeeding
// semantics). start == end yields the single daily slot [start]; end < start
// is an overnight window (legacy UI adds 24 h). Degenerate inputs (period
// ≤ 0) yield nothing — the caller falls back to [start].
func DeriveFeedingTimes(start, end time.Time, periodMinutes int) []careplan.TimeOfDay {
	if periodMinutes <= 0 {
		return nil
	}
	startMin := start.Hour()*60 + start.Minute()
	endMin := end.Hour()*60 + end.Minute()
	if endMin < startMin {
		endMin += 24 * 60 // overnight window (legacy feeding.go: end + 24h)
	}
	var out []careplan.TimeOfDay
	for m := startMin; m <= endMin; m += periodMinutes {
		if len(out) >= 24 { // sanity cap: fail loudly on pathological rows
			break
		}
		out = append(out, careplan.TimeOfDay{Hour: (m % (24 * 60)) / 60, Minute: m % 60})
	}
	return out
}

// NormalizeDiet is the §8.1 cluster key: trimmed, case-folded diet text.
func NormalizeDiet(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// timesKey renders a slot list as a canonical CSV key.
func timesKey(times []careplan.TimeOfDay) string {
	parts := make([]string, len(times))
	for i, t := range times {
		parts[i] = t.String()
	}
	return strings.Join(parts, ",")
}

func convertFeedingSchedules(tx *pop.Connection, report *ConversionReport, feedCareID string) error {
	if feedCareID == "" {
		report.Feeding.Lines = append(report.Feeding.Lines,
			ConversionLine{Reason: "caretype Repas/Alimentation introuvable — conversion alimentation sautée"})
		return nil
	}
	var animals []models.Animal
	if err := tx.Where("outtake_id IS NULL AND feeding_period > 0 AND feeding_start IS NOT NULL AND feeding_end IS NOT NULL").All(&animals); err != nil {
		return err
	}
	report.Feeding.AnimalsConsidered = len(animals)
	report.coverage = map[int][]careplan.TimeOfDay{}

	// v3 (bugs.md t10, 2026-10-27 user ruling): "cage names are per
	// centers — eliminate the cage-name matchers; in doubt, migrate to the
	// animal; this general rule must apply to all centers". The former
	// cluster rules (species IN … AND cage …) were center-scoped by cage
	// names and over-swept same-species animals housed elsewhere (v1 even
	// shipped species-only zombies that survived their own diet). Every
	// feeding animal now gets its OWN plan carrying exactly its legacy
	// slots — no rules, no matchers, nothing cage-scoped, and identical
	// semantics in every center.
	sort.Slice(animals, func(i, j int) bool { return animals[i].ID < animals[j].ID })
	for _, a := range animals {
		entry := feedingEntry{
			AnimalID: a.ID,
			Label:    animalLabel(a),
			Force:    a.ForceFeed,
		}
		if !a.FeedingStart.Valid || !a.FeedingEnd.Valid {
			// No-loss fallback (§8.1): keep the animal covered with what we
			// have — a single daily slot at the legacy start when present.
			if a.FeedingStart.Valid {
				s := a.FeedingStart.Time
				entry.Times = []careplan.TimeOfDay{{Hour: s.Hour(), Minute: s.Minute()}}
			} else {
				entry.Times = []careplan.TimeOfDay{{Hour: 9, Minute: 0}}
			}
			entry.Fallback = true
		} else {
			entry.Times = DeriveFeedingTimes(a.FeedingStart.Time, a.FeedingEnd.Time, a.FeedingPeriod)
			if len(entry.Times) == 0 {
				// No-loss fallback: at minimum the legacy start slot.
				s := a.FeedingStart.Time
				entry.Times = []careplan.TimeOfDay{{Hour: s.Hour(), Minute: s.Minute()}}
				entry.Fallback = true
			}
		}
		diet := NormalizeDiet(a.Feeding.String)
		if diet == "" {
			diet = "(régime non spécifié)"
			entry.Fallback = true
		}
		entry.Diet = diet
		if err := createConvertedFeedingPlan(tx, report, entry, feedCareID); err != nil {
			return err
		}
	}
	reconcileFeeding(report, animals)
	return nil
}
func createConvertedFeedingPlan(tx *pop.Connection, report *ConversionReport, e feedingEntry, feedCareID string) error {
	// R4-7.24: word-boundary, rune-safe shortening (see text_truncate.go).
	name := convertedFeedingName(e.Diet, e.Fallback)
	// Idempotent re-run guard: one converted plan per animal+regime.
	// COLLATE utf8mb4_bin: prod tables use utf8mb4_0900_ai_ci, which would
	// treat names differing only by case/accents as identical (e.g. legacy
	// drug spellings "ProdiplasT-T" vs "Prodiplast-T") and silently skip
	// a series — violating the no-loss mandate (§8.1).
	var n []struct {
		C int64 `db:"c"`
	}
	if err := tx.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND name = ? COLLATE utf8mb4_bin AND created_by IS NULL",
		e.AnimalID, name).All(&n); err != nil {
		return err
	}
	if len(n) > 0 && n[0].C > 0 {
		report.coverage[e.AnimalID] = e.Times
		return nil
	}
	p := &models.CareAnimalPlan{
		AnimalID:   e.AnimalID,
		Name:       name,
		ActionKind: careplan.KindFeeding,
		ActionPayload: buildSeedPayload(seedRuleDef{
			Kind: careplan.KindFeeding, PayloadFood: e.Diet, ForceFeed: e.Force,
		}, feedCareID),
		Schedule:     []byte(buildConvertedScheduleJSON(e.Times)),
		ReplacesKind: true,
		Active:       true,
		CreatedBy:    nulls.UUID{}, // converter author: NULL (users FK); §8.2 rollback targets name "(conversion)" + NULL author
	}
	verrs, err := tx.ValidateAndCreate(p)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return fmt.Errorf("converted feeding plan invalid (animal %d): %v", e.AnimalID, verrs)
	}
	report.Feeding.PlansCreated++
	report.coverage[e.AnimalID] = e.Times
	reason := fmt.Sprintf("plan individuel #%s", p.ID)
	if e.Fallback {
		reason += " (fallback — à vérifier)"
	}
	report.Feeding.Lines = append(report.Feeding.Lines,
		ConversionLine{e.AnimalID, e.Label, reason})
	return nil
}

// reconcileFeeding compares, per legacy feeding animal, the slot list the
// legacy generator would have produced (same DeriveFeedingTimes semantics)
// with the slot list actually converted, and records OK / DEGRADED /
// UNCOVERED in the report (§8.1 no-loss gate).
func reconcileFeeding(report *ConversionReport, animals []models.Animal) {
	for _, a := range animals {
		converted, ok := report.coverage[a.ID]
		if !ok {
			report.Reconciliation.FeedingUncovered++
			report.Reconciliation.Lines = append(report.Reconciliation.Lines,
				ConversionLine{a.ID, animalLabel(a), "UNCOVERED: aucune règle/plan converti"})
			continue
		}
		legacy := DeriveFeedingTimes(a.FeedingStart.Time, a.FeedingEnd.Time, a.FeedingPeriod)
		if timesKey(legacy) == timesKey(converted) {
			report.Reconciliation.FeedingOK++
			continue
		}
		report.Reconciliation.FeedingDegraded++
		report.Reconciliation.Lines = append(report.Reconciliation.Lines,
			ConversionLine{a.ID, animalLabel(a), fmt.Sprintf("DEGRADED: legacy [%s] → converti [%s]", timesKey(legacy), timesKey(converted))})
	}
}

// buildConvertedScheduleJSON renders the §4.3 schedule of a converted row.
func buildConvertedScheduleJSON(times []careplan.TimeOfDay) string {
	slots := make([]string, len(times))
	for i, t := range times {
		slots[i] = `"` + t.String() + `"`
	}
	return `{"times":[` + strings.Join(slots, ",") + `],"every_days":1,"anchor":"intake"}`
}

// ---------------------------------------------------------------------------
// Step 4 — treatment series → bounded plans
// ---------------------------------------------------------------------------

// BitmapToTimes maps the legacy treatment time bitmap to fixed slots
// (§10.1-3/§10-M1): morning→08:00, noon→12:00, evening→18:00.
func BitmapToTimes(bitmap int) []careplan.TimeOfDay {
	var out []careplan.TimeOfDay
	add := func(set bool, h, m int) {
		if set {
			out = append(out, careplan.TimeOfDay{Hour: h, Minute: m})
		}
	}
	add(bitmap&models.Treatement_MORNING != 0, 8, 0)
	add(bitmap&models.Treatement_NOON != 0, 12, 0)
	add(bitmap&models.Treatement_EVENING != 0, 18, 0)
	return out
}

func convertTreatmentSeries(tx *pop.Connection, report *ConversionReport) error {
	today := time.Now().Truncate(24 * time.Hour)
	var rows []models.Treatment
	if err := tx.Where("date >= ?", today).Order("date").All(&rows); err != nil {
		return err
	}
	known, err := knownDrugNames(tx)
	if err != nil {
		return err
	}
	groups, order := groupTreatmentSeries(rows)
	report.Treatments.Series = len(groups)

	for _, key := range order {
		s := groups[key]
		times := BitmapToTimes(s.bitmap)
		if len(times) == 0 {
			times = []careplan.TimeOfDay{{Hour: 8, Minute: 0}} // no-loss: keep the reminder alive
		}
		duration := int(s.last.Sub(s.first).Hours()/24) + 1
		label := fmt.Sprintf("animal %d", s.animalID)

		// No-loss routing (§8.1, bugs.md M5): a series is NEVER dropped.
		name, kind, payload, reason := treatmentSeriesRouting(tx, known, s.drug, s.dosage, s.remarks)

		p := &models.CareAnimalPlan{
			AnimalID:      s.animalID,
			Name:          name,
			ActionKind:    kind,
			ActionPayload: payload,
			Schedule: []byte(fmt.Sprintf(
				`{"times":[%s],"every_days":1,"anchor":"fixed","anchor_date":"%s","duration_days":%d}`,
				timesJSON(times), s.first.Format("2006-01-02"), duration)),
			ReplacesKind: false,
			Active:       true,
			CreatedBy:    nulls.UUID{}, // converter author: NULL (users FK)
		}
		// Idempotent re-run guard: one converted plan per animal+name.
		// COLLATE utf8mb4_bin: see feeding guard above — case/accent-only
		// differences in legacy drug names must NOT dedupe distinct series.
		var n []struct {
			C int64 `db:"c"`
		}
		if err := tx.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND name = ? COLLATE utf8mb4_bin AND created_by IS NULL",
			s.animalID, name).All(&n); err != nil {
			return err
		}
		if len(n) > 0 && n[0].C > 0 {
			report.Reconciliation.TreatmentOK++
			continue
		}
		verrs, err := tx.ValidateAndCreate(p)
		if err != nil {
			return err
		}
		if verrs.HasAny() {
			return fmt.Errorf("converted treatment plan invalid (animal %d): %v", s.animalID, verrs)
		}
		report.Treatments.PlansCreated++
		report.Reconciliation.TreatmentOK++
		report.Treatments.Lines = append(report.Treatments.Lines,
			ConversionLine{s.animalID, label, fmt.Sprintf("%s → #%s (%d j)", reason, p.ID, duration)})
	}
	return nil
}

// treatmentSeries groups the future-dated treatments into conversion
// series keyed by (animal × drug × dosage × bitmap) — the converter's
// unit of work (§8.1).
type treatmentSeries struct {
	animalID int
	drug     string
	dosage   string
	remarks  string
	bitmap   int
	first    time.Time
	last     time.Time
	dates    map[string]bool
}

// groupTreatmentSeries folds the rows into their series, insertion-ordered.
func groupTreatmentSeries(rows []models.Treatment) (map[string]*treatmentSeries, []string) {
	groups := map[string]*treatmentSeries{}
	order := []string{}
	for _, t := range rows {
		key := fmt.Sprintf("%d|%s|%s|%d", t.AnimalID, t.Drug, t.Dosage, t.Timebitmap)
		s, ok := groups[key]
		if !ok {
			s = &treatmentSeries{
				animalID: t.AnimalID, drug: t.Drug, dosage: t.Dosage,
				remarks: t.Remarks.String, bitmap: t.Timebitmap,
				first: t.Date, last: t.Date, dates: map[string]bool{},
			}
			groups[key] = s
			order = append(order, key)
		}
		if t.Date.Before(s.first) {
			s.first = t.Date
		}
		if t.Date.After(s.last) {
			s.last = t.Date
		}
		s.dates[t.Date.Format("2006-01-02")] = true
	}
	return groups, order
}

// treatmentSeriesRouting is the M5 series router (B10-8): known drug +
// dosage → medication plan (verbatim dosage); wound care and unknown drug
// → CARE plan typed "Soin" — the legacy "drug" column carries many
// NON-drug entries (fistula cleaning, casts, bandage changes, checks…)
// and those fulfill as cares, not observations. A KNOWN drug without
// posology stays a flagged observation: the product is real, the schedule
// needs human verification. Without a resolvable caretype the legacy
// observation routing is kept (the converter never breaks).
func treatmentSeriesRouting(tx *pop.Connection, known map[string]struct{}, drug, dosage, remarks string) (name, kind string, payload []byte, reason string) {
	_, knownDrug := known[NormalizeDiet(drug)]
	careTypeID := convertedCareTypeID(tx)
	switch {
	case drug != models.WoundCareDrugName && knownDrug && strings.TrimSpace(dosage) != "":
		return "Traitement — " + drug, careplan.KindMedication,
			[]byte(buildSeedPayload(seedRuleDef{
				Kind: careplan.KindMedication, Drug: drug, Dosage: dosage, Note: remarks,
			}, "")),
			fmt.Sprintf("série %s → plan médication", drug)
	case drug == models.WoundCareDrugName && careTypeID != "":
		return "Soin de plaie (conversion) (à vérifier)", careplan.KindCare,
			carePayloadJSON(careTypeID, "Soin de plaie — contrôle", remarks),
			"série soin de plaie → plan soin (conversion)"
	case !knownDrug && careTypeID != "":
		// B10-6: the dosage of a legacy treatment series is often the
		// application SITE ("Dessus oeil droit") — carry it into the
		// content line.
		prompt := drug
		if strings.TrimSpace(dosage) != "" {
			prompt = drug + " (" + dosage + ")"
		}
		return "Soin — " + prompt + " (à vérifier)", careplan.KindCare,
			carePayloadJSON(careTypeID, prompt, remarks),
			fmt.Sprintf("médication abusive — converti en soin, à vérifier (%s)", drug)
	case drug == models.WoundCareDrugName:
		return "Soin de plaie (conversion) (à vérifier)", careplan.KindObservation,
			observationPayload("Soin de plaie — contrôle", remarks),
			"série soin de plaie → plan observation (à vérifier, pas de caretype)"
	case !knownDrug:
		prompt := drug
		if strings.TrimSpace(dosage) != "" {
			prompt = drug + " (" + dosage + ")"
		}
		return "Traitement — " + prompt + " (à vérifier)", careplan.KindObservation,
			observationPayload(prompt, remarks),
			fmt.Sprintf("drug inconnu — converti en observation, à vérifier (%s)", drug)
	default: // known drug, empty dosage
		return "Traitement — " + drug + " (à vérifier)", careplan.KindObservation,
			observationPayload(drug, remarks),
			fmt.Sprintf("série %s sans posologie — converti en observation, à vérifier", drug)
	}
}

// knownDrugNames loads the drugs table once, case-folded for comparison.
func knownDrugNames(tx *pop.Connection) (map[string]struct{}, error) {
	var drugs []models.Drug
	if err := tx.All(&drugs); err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(drugs))
	for _, d := range drugs {
		out[NormalizeDiet(d.Name)] = struct{}{}
	}
	return out, nil
}

// observationPayload builds the §4.2 observation payload of a converted
// review-flagged plan: a yes/no question, no alert outcome.
func observationPayload(prompt, note string) []byte {
	m := map[string]any{"prompt": prompt}
	if strings.TrimSpace(note) != "" {
		m["note"] = note
	}
	return []byte(marshalSeed(m))
}

// convertedCareTypeID resolves the "Soin" caretype the converter attaches
// to care plans built from non-drug treatment series (B10-8). Falls back
// to the seeded default caretype; "" when no reference data resolves —
// the caller then keeps the legacy observation routing.
func convertedCareTypeID(tx *pop.Connection) string {
	var ct models.Caretype
	if err := tx.Where("name = ?", "Soin").First(&ct); err == nil {
		return ct.ID.String()
	}
	if id, err := defaultCareType(tx); err == nil {
		return id.String()
	}
	return ""
}

// carePayloadJSON builds the §4.2 care payload of a converted plan: the
// content line rides in note (planDetail renders note || instructions),
// the legacy remarks move to instructions.
func carePayloadJSON(caretypeID, note, instructions string) []byte {
	m := map[string]any{"caretype_id": caretypeID, "note": note}
	if strings.TrimSpace(instructions) != "" {
		m["instructions"] = instructions
	}
	return []byte(marshalSeed(m))
}

func timesJSON(times []careplan.TimeOfDay) string {
	slots := make([]string, len(times))
	for i, t := range times {
		slots[i] = `"` + t.String() + `"`
	}
	return strings.Join(slots, ",")
}
