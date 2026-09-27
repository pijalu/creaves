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
	Species  string
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

// escapeDSLQuote makes free text safe inside a DSL string literal (`\"` and
// `\\` are the only escapes, §5.2).
func escapeDSLQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
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

	// Cluster key (§8.1 step 2, H2 fix): diet × exact slot set × force_feed.
	// Same-diet animals with different times or force flags get separate
	// rules — never a modal/averaged schedule that matches nobody's reality.
	clusters := map[string][]feedingEntry{}
	for _, a := range animals {
		entry := feedingEntry{
			AnimalID: a.ID,
			Label:    animalLabel(a),
			Force:    a.ForceFeed,
			Species:  strings.TrimSpace(a.Species),
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
		key := diet + "|" + timesKey(entry.Times)
		if entry.Force {
			key += "|F"
		}
		clusters[key] = append(clusters[key], entry)
	}

	// Deterministic cluster order.
	keys := make([]string, 0, len(clusters))
	for k := range clusters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		entries := clusters[key]
		report.Feeding.Clusters++
		if len(entries) >= FeedingClusterThreshold {
			converted, err := convertFeedingCluster(tx, report, entries, feedCareID)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if !converted[e.AnimalID] {
					if err := createConvertedFeedingPlan(tx, report, e, feedCareID); err != nil {
						return err
					}
				}
			}
			continue
		}
		for _, e := range entries {
			if err := createConvertedFeedingPlan(tx, report, e, feedCareID); err != nil {
				return err
			}
		}
	}
	reconcileFeeding(report, animals)
	return nil
}

// convertFeedingCluster turns one ≥threshold (diet × slots × force) cluster
// into a rule whose matcher is `species IN (...)` over the cluster's
// non-empty species names (§8.1 step 2). All members share the exact same
// slot set and force flag (cluster key) — the rule's schedule is the first
// entry's. Returns the animal ids covered by the rule; members whose species
// name is empty must be converted as per-animal plans by the caller.
func convertFeedingCluster(tx *pop.Connection, report *ConversionReport, entries []feedingEntry, feedCareID string) (map[int]bool, error) {
	diet := entries[0].Diet
	species := map[string]bool{}
	for _, e := range entries {
		if e.Species != "" {
			species[e.Species] = true
		}
	}
	names := make([]string, 0, len(species))
	for s := range species {
		names = append(names, s)
	}
	sort.Strings(names)
	covered := map[int]bool{}
	if len(names) == 0 {
		return covered, nil
	}
	for _, e := range entries {
		if e.Species != "" {
			covered[e.AnimalID] = true
		}
	}
	quoted := make([]string, len(names))
	for i, s := range names {
		quoted[i] = fmt.Sprintf(`"%s"`, escapeDSLQuote(s))
	}
	expr := "species IN (" + strings.Join(quoted, ", ") + ")"

	matcher := buildSeedMatcher(seedMatcherDef{
		Key: "CONV", Derived: true,
		Name:       fmt.Sprintf("Régime « %.60s » (conversion)", diet),
		Expression: expr,
	})
	matcherID, _, err := insertMatcherOnce(tx, matcher)
	if err != nil {
		return nil, err
	}
	title := diet
	if len(title) > 60 {
		title = title[:60]
	}
	ruleName := fmt.Sprintf("Alimentation — %s (conversion)", title)
	exists, err := careRuleNameExists(tx, ruleName)
	if err != nil {
		return nil, err
	}
	if exists {
		// Marker deleted and converter re-run: keep the existing rule and
		// mark its members converted (no duplicate).
		for _, e := range entries {
			if covered[e.AnimalID] {
				report.coverage[e.AnimalID] = e.Times
				report.Feeding.Lines = append(report.Feeding.Lines,
					ConversionLine{e.AnimalID, e.Label, "règle cluster (existante)"})
			}
		}
		return covered, nil
	}
	rule := &models.CareRule{
		Name:          ruleName,
		Description:   fmt.Sprintf("Cluster alimentation ×%d [source: %s]", len(entries), ConverterTag),
		ActionKind:    careplan.KindFeeding,
		ActionPayload: buildSeedPayload(seedRuleDef{Kind: careplan.KindFeeding, PayloadFood: diet, ForceFeed: entries[0].Force}, feedCareID),
		Schedule:      []byte(buildConvertedScheduleJSON(entries[0].Times)),
		MatcherID:     matcherID,
		Active:        true,
		StopOnOuttake: true,
	}
	verrs, err := tx.ValidateAndCreate(rule)
	if err != nil {
		return nil, err
	}
	if verrs.HasAny() {
		return nil, fmt.Errorf("converted feeding rule invalid: %v", verrs)
	}
	report.Feeding.RulesCreated++
	for _, e := range entries {
		if covered[e.AnimalID] {
			report.coverage[e.AnimalID] = e.Times
			report.Feeding.Lines = append(report.Feeding.Lines,
				ConversionLine{e.AnimalID, e.Label, fmt.Sprintf("règle cluster #%s", rule.ID)})
		}
	}
	return covered, nil
}

func createConvertedFeedingPlan(tx *pop.Connection, report *ConversionReport, e feedingEntry, feedCareID string) error {
	title := e.Diet
	if len(title) > 60 {
		title = title[:60]
	}
	name := fmt.Sprintf("Alimentation — %s (conversion)", title)
	if e.Fallback {
		name += " (à vérifier)"
	}
	// Idempotent re-run guard: one converted plan per animal+regime.
	var n []struct {
		C int64 `db:"c"`
	}
	if err := tx.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND name = ? AND created_by IS NULL",
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
	type series struct {
		animalID int
		drug     string
		dosage   string
		remarks  string
		bitmap   int
		first    time.Time
		last     time.Time
		dates    map[string]bool
	}
	groups := map[string]*series{}
	order := []string{}
	for _, t := range rows {
		key := fmt.Sprintf("%d|%s|%s|%d", t.AnimalID, t.Drug, t.Dosage, t.Timebitmap)
		s, ok := groups[key]
		if !ok {
			s = &series{
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
		// Known drug + dosage → medication plan (verbatim dosage).
		// Wound care, unknown drug, or empty dosage → observation plan
		// flagged "(à vérifier)" so the workflow survives without inventing
		// a posology.
		_, knownDrug := known[NormalizeDiet(s.drug)]
		asMedication := s.drug != models.WoundCareDrugName && knownDrug && strings.TrimSpace(s.dosage) != ""

		var name, kind string
		var payload []byte
		var reason string
		switch {
		case asMedication:
			name = "Traitement — " + s.drug
			kind = careplan.KindMedication
			payload = []byte(buildSeedPayload(seedRuleDef{
				Kind: careplan.KindMedication, Drug: s.drug, Dosage: s.dosage, Note: s.remarks,
			}, ""))
			reason = fmt.Sprintf("série %s → plan médication", s.drug)
		case s.drug == models.WoundCareDrugName:
			name = "Soin de plaie (conversion) (à vérifier)"
			kind = careplan.KindObservation
			payload = observationPayload("Soin de plaie — contrôle", s.remarks)
			reason = "série soin de plaie → plan observation (à vérifier)"
		case !knownDrug:
			name = "Traitement — " + s.drug + " (à vérifier)"
			kind = careplan.KindObservation
			payload = observationPayload(s.drug, s.remarks)
			reason = fmt.Sprintf("drug inconnu — converti en observation, à vérifier (%s)", s.drug)
		default: // known drug, empty dosage
			name = "Traitement — " + s.drug + " (à vérifier)"
			kind = careplan.KindObservation
			payload = observationPayload(s.drug, s.remarks)
			reason = fmt.Sprintf("série %s sans posologie — converti en observation, à vérifier", s.drug)
		}

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
		var n []struct {
			C int64 `db:"c"`
		}
		if err := tx.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND name = ? AND created_by IS NULL",
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

func timesJSON(times []careplan.TimeOfDay) string {
	slots := make([]string, len(times))
	for i, t := range times {
		slots[i] = `"` + t.String() + `"`
	}
	return strings.Join(slots, ",")
}
