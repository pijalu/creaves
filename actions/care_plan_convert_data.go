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
}

// DeriveFeedingTimes expands a legacy feeding_start/feeding_end/period row
// into fixed daily slots (§8.1 step 2): start + n×period ≤ end, n ≥ 1.
func DeriveFeedingTimes(start, end time.Time, periodMinutes int) []careplan.TimeOfDay {
	if periodMinutes <= 0 {
		return nil
	}
	startMin := start.Hour()*60 + start.Minute()
	endMin := end.Hour()*60 + end.Minute()
	if endMin < startMin {
		return nil
	}
	var out []careplan.TimeOfDay
	for m := startMin + periodMinutes; m <= endMin; m += periodMinutes {
		out = append(out, careplan.TimeOfDay{Hour: m / 60, Minute: m % 60})
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

// modalTimes picks the most frequent derived slot set of a cluster
// (ties → lexicographically smallest, deterministic).
func modalTimes(entries []feedingEntry) []careplan.TimeOfDay {
	counts := map[string]int{}
	seen := map[string][]careplan.TimeOfDay{}
	for _, e := range entries {
		k := timesKey(e.Times)
		counts[k]++
		seen[k] = e.Times
	}
	best := ""
	for k, n := range counts {
		if best == "" || n > counts[best] || (n == counts[best] && k < best) {
			best = k
		}
	}
	return seen[best]
}

func majorityForce(entries []feedingEntry) bool {
	n := 0
	for _, e := range entries {
		if e.Force {
			n++
		}
	}
	return n*2 > len(entries)
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

	clusters := map[string][]feedingEntry{}
	for _, a := range animals {
		entry := feedingEntry{
			AnimalID: a.ID,
			Label:    animalLabel(a),
			Force:    a.ForceFeed,
			Species:  strings.TrimSpace(a.Species),
		}
		if !a.FeedingStart.Valid || !a.FeedingEnd.Valid {
			report.addSkip("feeding", ConversionLine{entry.AnimalID, entry.Label, "horaires d'alimentation incomplets"})
			continue
		}
		entry.Times = DeriveFeedingTimes(a.FeedingStart.Time, a.FeedingEnd.Time, a.FeedingPeriod)
		if len(entry.Times) == 0 {
			report.addSkip("feeding", ConversionLine{entry.AnimalID, entry.Label, "période non dérivable (start/end/période)"})
			continue
		}
		diet := NormalizeDiet(a.Feeding.String)
		if diet == "" {
			report.addSkip("feeding", ConversionLine{entry.AnimalID, entry.Label, "texte de régime vide"})
			continue
		}
		clusters[diet] = append(clusters[diet], entry)
	}

	// Deterministic cluster order (by diet text).
	diets := make([]string, 0, len(clusters))
	for d := range clusters {
		diets = append(diets, d)
	}
	sort.Strings(diets)

	for _, diet := range diets {
		entries := clusters[diet]
		report.Feeding.Clusters++
		if len(entries) >= FeedingClusterThreshold {
			converted, err := convertFeedingCluster(tx, report, diet, entries, feedCareID)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if !converted[e.AnimalID] {
					if err := createConvertedFeedingPlan(tx, report, e, diet, feedCareID); err != nil {
						return err
					}
				}
			}
			continue
		}
		for _, e := range entries {
			if err := createConvertedFeedingPlan(tx, report, e, diet, feedCareID); err != nil {
				return err
			}
		}
	}
	return nil
}

// convertFeedingCluster turns one ≥threshold diet cluster into a rule whose
// matcher is `species IN (...)` over the cluster's non-empty species names
// (§8.1 step 2). Returns the animal ids covered by the rule; members whose
// species name is empty (or otherwise not in the matcher) must be converted
// as per-animal plans by the caller.
func convertFeedingCluster(tx *pop.Connection, report *ConversionReport, diet string, entries []feedingEntry, feedCareID string) (map[int]bool, error) {
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
	food := diet
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
		ActionPayload: buildSeedPayload(seedRuleDef{Kind: careplan.KindFeeding, PayloadFood: food, ForceFeed: majorityForce(entries)}, feedCareID),
		Schedule:      []byte(buildConvertedScheduleJSON(modalTimes(entries))),
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
			report.Feeding.Lines = append(report.Feeding.Lines,
				ConversionLine{e.AnimalID, e.Label, fmt.Sprintf("règle cluster #%s", rule.ID)})
		}
	}
	return covered, nil
}

func createConvertedFeedingPlan(tx *pop.Connection, report *ConversionReport, e feedingEntry, diet string, feedCareID string) error {
	title := diet
	if len(title) > 60 {
		title = title[:60]
	}
	name := fmt.Sprintf("Alimentation — %s (conversion)", title)
	// Idempotent re-run guard: one converted plan per animal+regime.
	var n []struct {
		C int64 `db:"c"`
	}
	if err := tx.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND name = ? AND created_by IS NULL",
		e.AnimalID, name).All(&n); err != nil {
		return err
	}
	if len(n) > 0 && n[0].C > 0 {
		return nil
	}
	p := &models.CareAnimalPlan{
		AnimalID:   e.AnimalID,
		Name:       name,
		ActionKind: careplan.KindFeeding,
		ActionPayload: buildSeedPayload(seedRuleDef{
			Kind: careplan.KindFeeding, PayloadFood: diet, ForceFeed: e.Force,
		}, feedCareID),
		Schedule:     []byte(buildConvertedScheduleJSON(e.Times)),
		ReplacesKind: true,
		Active:       true,
		CreatedBy:    nulls.UUID{}, // converter author: NULL (users FK)
	}
	verrs, err := tx.ValidateAndCreate(p)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return fmt.Errorf("converted feeding plan invalid (animal %d): %v", e.AnimalID, verrs)
	}
	report.Feeding.PlansCreated++
	report.Feeding.Lines = append(report.Feeding.Lines,
		ConversionLine{e.AnimalID, e.Label, fmt.Sprintf("plan individuel #%s", p.ID)})
	return nil
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
		if s.drug == models.WoundCareDrugName {
			report.addSkip("treatment", ConversionLine{s.animalID, fmt.Sprintf("animal %d", s.animalID), "série soin de plaie (sans posologie)"})
			continue
		}
		times := BitmapToTimes(s.bitmap)
		if len(times) == 0 {
			report.addSkip("treatment", ConversionLine{s.animalID, fmt.Sprintf("animal %d", s.animalID), "série " + s.drug + " sans créneau"})
			continue
		}
		if strings.TrimSpace(s.dosage) == "" {
			report.addSkip("treatment", ConversionLine{s.animalID, fmt.Sprintf("animal %d", s.animalID), "série " + s.drug + " sans posologie"})
			continue
		}
		duration := int(s.last.Sub(s.first).Hours()/24) + 1
		name := "Traitement — " + s.drug
		p := &models.CareAnimalPlan{
			AnimalID:   s.animalID,
			Name:       name,
			ActionKind: careplan.KindMedication,
			ActionPayload: []byte(buildSeedPayload(seedRuleDef{
				Kind: careplan.KindMedication, Drug: s.drug, Dosage: s.dosage, Note: s.remarks,
			}, "")),
			Schedule: []byte(fmt.Sprintf(
				`{"times":[%s],"every_days":1,"anchor":"fixed","anchor_date":"%s","duration_days":%d}`,
				timesJSON(times), s.first.Format("2006-01-02"), duration)),
			ReplacesKind: false,
			Active:       true,
			CreatedBy:    nulls.UUID{}, // converter author: NULL (users FK)
		}
		// Idempotent re-run guard: one converted plan per animal+drug.
		var n []struct {
			C int64 `db:"c"`
		}
		if err := tx.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND name = ? AND created_by IS NULL",
			s.animalID, name).All(&n); err != nil {
			return err
		}
		if len(n) > 0 && n[0].C > 0 {
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
		report.Treatments.Lines = append(report.Treatments.Lines,
			ConversionLine{s.animalID, fmt.Sprintf("animal %d", s.animalID),
				fmt.Sprintf("série %s → plan #%s (%d j)", s.drug, p.ID, duration)})
	}
	return nil
}

func timesJSON(times []careplan.TimeOfDay) string {
	slots := make([]string, len(times))
	for i, t := range times {
		slots[i] = `"` + t.String() + `"`
	}
	return strings.Join(slots, ",")
}
