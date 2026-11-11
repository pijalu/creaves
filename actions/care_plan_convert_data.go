package actions

import (
	"fmt"
	"sort"
	"strconv"
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
	// Repeat, non-nil for windowed legacy triples (BUG-36): the schedule is
	// stored in the repeat authoring form — block + expanded times.
	// start==end rows (zero-length window) carry NO repeat (BUG-50).
	Repeat *careplan.Repeat
	Force  bool
	Diet   string
	// GraceMinutes: legacy lateness cadence (period/2, BUG-50); 0 keeps
	// the §4.3 default.
	GraceMinutes int
	// Fallback marks rows converted without a full legacy window (single
	// daily slot at the legacy start, else 09:00 — user ruling 2026-10-14:
	// definitive, never flagged). Kept for the conversion report reason.
	Fallback bool
}

// DeriveFeedingTimes expands a legacy feeding_start/feeding_end/period row
// into fixed daily slots (§8.1 step 2, §11.7): start + n×period ≤ end,
// n ≥ 0 — the start slot itself is a feeding time (legacy calculateFeeding
// semantics). start == end yields the single daily slot [start]; end < start
// is an overnight window (legacy UI adds 24 h). Degenerate inputs (period
// ≤ 0) yield nothing — the caller falls back to [start].
//
// BUG-36: the derivation IS the careplan repeat expansion — same semantics,
// one implementation. Note the result is sorted strictly ascending (the
// §4.3 times invariant): an overnight window comes back as
// [02:00, 06:00, 22:00], not in generation order — the pre-BUG-36 local
// loop produced an invalid (unsorted) schedule for overnight rows.
func DeriveFeedingTimes(start, end time.Time, periodMinutes int) []careplan.TimeOfDay {
	return careplan.ExpandRepeat(careplan.Repeat{
		From:         careplan.TimeOfDay{Hour: start.Hour(), Minute: start.Minute()},
		To:           careplan.TimeOfDay{Hour: end.Hour(), Minute: end.Minute()},
		EveryMinutes: periodMinutes,
	})
}

// legacyFeedingSchedule derives the converted schedule of one legacy
// feeding row, applying the §8.1 no-loss fallbacks. An exact legacy triple
// (non-NULL feeding_start/feeding_end + feeding_period > 0) converts to the
// repeat form (BUG-36: repeat block + expanded times — the translation is
// exact). Anything else converts DEFINITIVELY to a single daily slot at
// the legacy start when present, else 09:00 (user ruling 2026-10-14: a
// feeding without cadence is one 09:00 slot — no "à vérifier" flag; the
// third return only distinguishes exact vs fallback for the report). The
// repeat return is nil for fallback rows (no repeat block stored).
// Shared by the converter and the reconciliation gate so both agree on the
// legacy semantics (bugs.md BUG-10: NULL start/end rows included).
func legacyFeedingSchedule(a models.Animal) ([]careplan.TimeOfDay, *careplan.Repeat, bool) {
	if a.FeedingStart.Valid && a.FeedingEnd.Valid {
		r := careplan.Repeat{
			From:         careplan.TimeOfDay{Hour: a.FeedingStart.Time.Hour(), Minute: a.FeedingStart.Time.Minute()},
			To:           careplan.TimeOfDay{Hour: a.FeedingEnd.Time.Hour(), Minute: a.FeedingEnd.Time.Minute()},
			EveryMinutes: a.FeedingPeriod,
		}
		// BUG-50 (2026-10-27 user report, animal 10231 "from 10:00 to 10:00
		// every 10:00"): a ZERO-LENGTH window (start == end) with a period
		// is the legacy "à disposition" encoding (≈37 in-care animals). The
		// legacy reminder engine realizes it as ONE daily occurrence at
		// start: after a feeding at start, calculateFeeding's heuristic end
		// (end − period/2) sits before the feeding, so the next occurrence
		// lands on start + 24 h — the period never generates a second slot,
		// it only widens the late window (see legacyFeedingGraceMinutes).
		// Storing repeat{from X, to X, every P} claimed a windowing
		// precision the data does not carry and rendered verbatim in the
		// protocol editor ("from 10:00 to 10:00 every 10:00"). Convert to
		// the plain single-slot schedule — identical runtime behavior,
		// honest display, exact translation (no fallback flag).
		if r.From == r.To && r.EveryMinutes > 0 {
			return []careplan.TimeOfDay{r.From}, nil, false
		}
		if times := careplan.ExpandRepeat(r); len(times) > 0 {
			if r.FitsCap() {
				// Exact translation: repeat block + expanded times.
				return times, &r, false
			}
			// Pathological row: the window/period expansion runs past the
			// slot cap, so the repeat block would fail schedule validation
			// on save. Keep the capped slots as the conversion (slots
			// beyond the cap are dropped — definitive, no flag).
			return times, nil, true
		}
		// Degenerate (period ≤ 0): at minimum the legacy start slot.
		s := a.FeedingStart.Time
		return []careplan.TimeOfDay{{Hour: s.Hour(), Minute: s.Minute()}}, nil, true
	}
	// No-loss fallback (§8.1): keep the animal covered with what we have —
	// a single daily slot at the legacy start when present.
	if a.FeedingStart.Valid {
		s := a.FeedingStart.Time
		return []careplan.TimeOfDay{{Hour: s.Hour(), Minute: s.Minute()}}, nil, true
	}
	return []careplan.TimeOfDay{{Hour: 9, Minute: 0}}, nil, true
}

// legacyFeedingGraceMinutes carries the legacy lateness cadence into the
// §4.3 schedule (BUG-50): calculateFeeding flagged a missed feeding late
// once it was older than half the period (criticalLimit = now − period/2).
// 0 keeps the §4.3 default (60 min) for rows without a usable period.
func legacyFeedingGraceMinutes(period int) int {
	if period > 0 {
		return period / 2
	}
	return 0
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
	// bugs.md BUG-10 (2026-10-14, user ruling: "all replaced action *must*
	// be converted"): the legacy query required feeding_start/end NOT NULL,
	// which silently skipped animals with a period but no times — or only a
	// diet text — leaving them with NO feeding plan at all (e.g. animal
	// 8842: period 480, NULL times, diet "15 poussins et 5 souris // EAU
	// X8"). The no-loss mandate (§8.1) covers EVERY in-care animal carrying
	// any legacy feeding trace; rows without a derivable schedule take the
	// fallback slot path below (a single daily 09:00-or-legacy-start slot).
	var animals []models.Animal
	// User ruling: an animal with NO legacy diet text gets NO feeding plan.
	// The "(régime non spécifié)" placeholder named nothing (e.g. animal
	// 10415); an empty feeding means the animal is not on a feeding regimen,
	// so there is nothing to convert. The diet text is therefore REQUIRED —
	// but a diet-only row (no period/times) still converts (BUG-10: it is a
	// real legacy feeding trace, taking the fallback slot). Excluding empty
	// diets at selection keeps the §8.1 no-loss gate out of their scope.
	if err := tx.Where("outtake_id IS NULL AND feeding IS NOT NULL AND TRIM(feeding) <> '' AND (feeding_period > 0 OR feeding_start IS NOT NULL OR feeding_end IS NOT NULL OR TRIM(feeding) <> '')").All(&animals); err != nil {
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
		times, repeat, fallback := legacyFeedingSchedule(a)
		entry := feedingEntry{
			AnimalID:     a.ID,
			Label:        animalLabel(a),
			Times:        times,
			Repeat:       repeat,
			Force:        a.ForceFeed,
			Diet:         NormalizeDiet(a.Feeding.String),
			GraceMinutes: legacyFeedingGraceMinutes(a.FeedingPeriod),
			Fallback:     fallback,
		}
		if err := createConvertedFeedingPlan(tx, report, entry, feedCareID); err != nil {
			return err
		}
	}
	reconcileFeeding(report, animals)
	return nil
}
func createConvertedFeedingPlan(tx *pop.Connection, report *ConversionReport, e feedingEntry, feedCareID string) error {
	// R4-7.24: word-boundary, rune-safe shortening (see text_truncate.go).
	name := convertedFeedingName(e.Diet)
	// Idempotent re-run guard: one converted plan per animal+regime.
	// COLLATE utf8mb4_bin: prod tables use utf8mb4_0900_ai_ci, which would
	// treat names differing only by case/accents as identical (e.g. legacy
	// drug spellings "ProdiplasT-T" vs "Prodiplast-T") and silently skip
	// a series — violating the no-loss mandate (§8.1).
	// The COLLATE must sit on the COLUMN: on a `?` placeholder MariaDB
	// rejects it (Error 1253 — placeholder charset is binary), aborting
	// the Docker boot.
	var n []struct {
		C int64 `db:"c"`
	}
	if err := tx.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND name COLLATE utf8mb4_bin = ? AND created_by IS NULL",
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
		Schedule:     []byte(buildConvertedScheduleJSON(e.Times, e.Repeat, e.GraceMinutes)),
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
		reason += " (fallback slot — régime sans horaire hérité)"
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
		// Same fallback semantics as the converter (BUG-10): rows with NULL
		// start/end (or a degenerate period) reconcile against the fallback
		// slot, not against an empty derivation.
		legacy, _, _ := legacyFeedingSchedule(a)
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
// Exact legacy triples (BUG-36) carry the repeat authoring form — block +
// expanded times; fallback rows are plain slot lists. A non-default
// grace_minutes (legacy period/2, BUG-50) rides in the document.
func buildConvertedScheduleJSON(times []careplan.TimeOfDay, repeat *careplan.Repeat, graceMinutes int) string {
	slots := make([]string, len(times))
	for i, t := range times {
		slots[i] = `"` + t.String() + `"`
	}
	doc := `{"times":[` + strings.Join(slots, ",") + `],"every_days":1,"anchor":"intake"`
	if graceMinutes > 0 && graceMinutes != 60 {
		doc += fmt.Sprintf(`,"grace_minutes":%d`, graceMinutes)
	}
	if repeat != nil {
		doc += fmt.Sprintf(",\"repeat\":{\"from\":%q,\"to\":%q,\"every_minutes\":%d}",
			repeat.From.String(), repeat.To.String(), repeat.EveryMinutes)
	}
	return doc + `}`
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
		if err := convertOneTreatmentSeries(tx, report, known, groups[key]); err != nil {
			return err
		}
	}
	return nil
}

// treatmentSeriesTimes maps the series bitmap to fixed slots, keeping the
// reminder alive with a single 08:00 slot when the legacy row carries no
// time bitmap. Shared by converter and validator.
func treatmentSeriesTimes(s *treatmentSeries) []careplan.TimeOfDay {
	times := BitmapToTimes(s.bitmap)
	if len(times) == 0 {
		times = []careplan.TimeOfDay{{Hour: 8, Minute: 0}} // no-loss: keep the reminder alive
	}
	return times
}

// convertOneTreatmentSeries converts one grouped series into its care plan
// row(s) (idempotent — skips names that already exist) and updates the
// conversion report. The per-series conversion contract lives in
// treatmentSeriesPlanSpecs (single source of truth shared with the
// validator).
func convertOneTreatmentSeries(tx *pop.Connection, report *ConversionReport, known map[string]struct{}, s *treatmentSeries) error {
	times := treatmentSeriesTimes(s)
	label := fmt.Sprintf("animal %d", s.animalID)

	// No-loss routing (§8.1, bugs.md M5): a series is NEVER dropped, and
	// every conversion is definitive (no "à vérifier" flags).
	name, kind, payload, reason := treatmentSeriesRouting(tx, known, s.drug, s.dosage, s.remarks)

	created := 0
	for _, spec := range treatmentSeriesPlanSpecs(s, times, name, kind, payload) {
		p := &models.CareAnimalPlan{
			AnimalID:      s.animalID,
			Name:          spec.name,
			ActionKind:    spec.kind,
			ActionPayload: spec.payload,
			Schedule:      []byte(spec.schedule),
			ReplacesKind:  false,
			Active:        true,
			CreatedBy:     nulls.UUID{}, // converter author: NULL (users FK)
		}
		// Idempotent re-run guard: one converted plan per animal+name.
		// COLLATE utf8mb4_bin: see feeding guard above — case/accent-only
		// differences in legacy drug names must NOT dedupe distinct series.
		// COLLATE on the column, not the placeholder (MariaDB Error 1253).
		var n []struct {
			C int64 `db:"c"`
		}
		if err := tx.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND name COLLATE utf8mb4_bin = ? AND created_by IS NULL",
			s.animalID, spec.name).All(&n); err != nil {
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
		created++
		report.Treatments.PlansCreated++
	}
	report.Reconciliation.TreatmentOK++
	report.Treatments.Lines = append(report.Treatments.Lines,
		ConversionLine{s.animalID, label, fmt.Sprintf("%s → %d plan(s)", reason, created)})
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

// treatmentSeriesCadence infers the day-step of a series from its sorted
// dates: 1 for a single date or a dense daily run, the constant gap G for a
// regularly spaced series (BUG-52), 0 when the gaps are mixed (no single
// step reproduces the legacy dates).
func treatmentSeriesCadence(s *treatmentSeries) int {
	if len(s.dates) <= 1 {
		return 1
	}
	days := make([]int, 0, len(s.dates))
	for d := range s.dates {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			return 0 // defensive: grouping only writes Format()ed keys
		}
		days = append(days, int(t.Truncate(24*time.Hour).Unix()/86400))
	}
	sort.Ints(days)
	gap := days[1] - days[0]
	for i := 2; i < len(days); i++ {
		if days[i]-days[i-1] != gap {
			return 0 // irregular: mixed gaps
		}
	}
	return gap
}

// treatmentPlanSpec is one concrete care plan row a series converts to:
// routed identity (name/kind/payload) plus its exact schedule JSON.
type treatmentPlanSpec struct {
	name     string
	kind     string
	payload  []byte
	schedule string
}

// treatmentSeriesPlanSpecs is the SINGLE source of truth for what a legacy
// treatment series converts to — the converter creates exactly these rows
// and the validator expects exactly these names. Every series converts
// EXACTLY (user ruling 2026-10-14: a conversion is always correct, never
// flagged "à vérifier"):
//   - single date / dense daily run → one plan every_days:1 over the
//     calendar span
//   - weekly weekday pattern (e.g. Mon+Wed+Fri) → one plan every_days:1
//     restricted to those weekdays, duration = date count
//   - regularly spaced (constant gap G>1, non-weekly) → one plan
//     every_days:G, duration = date count
//   - anything else → one exact single-day plan per legacy date (name
//     suffixed with the ISO date) — the legacy dates ARE the truth
func treatmentSeriesPlanSpecs(s *treatmentSeries, times []careplan.TimeOfDay, name, kind string, payload []byte) []treatmentPlanSpec {
	cadence := treatmentSeriesCadence(s)
	switch {
	case cadence == 1: // single date or dense daily run
		span := int(s.last.Sub(s.first).Hours()/24) + 1
		return []treatmentPlanSpec{{name, kind, payload,
			treatmentScheduleJSON(times, 1, s.first.Format("2006-01-02"), span, nil)}}
	case treatmentSeriesWeekdays(s) != nil:
		return []treatmentPlanSpec{{name, kind, payload,
			treatmentScheduleJSON(times, 1, s.first.Format("2006-01-02"), len(s.dates), treatmentSeriesWeekdays(s))}}
	case cadence > 1:
		return []treatmentPlanSpec{{name, kind, payload,
			treatmentScheduleJSON(times, cadence, s.first.Format("2006-01-02"), len(s.dates), nil)}}
	default: // irregular: one exact single-day plan per legacy date
		dates := sortedTreatmentDates(s)
		specs := make([]treatmentPlanSpec, 0, len(dates))
		for _, d := range dates {
			specs = append(specs, treatmentPlanSpec{
				name + " — " + d, kind, payload,
				treatmentScheduleJSON(times, 1, d, 1, nil),
			})
		}
		return specs
	}
}

// treatmentScheduleJSON builds the §4.3 schedule document of a converted
// treatment plan (times/every_days/weekdays/anchor/duration).
func treatmentScheduleJSON(times []careplan.TimeOfDay, everyDays int, anchorDate string, durationDays int, weekdays []int) string {
	wd := ""
	if len(weekdays) > 0 {
		strs := make([]string, len(weekdays))
		for i, w := range weekdays {
			strs[i] = strconv.Itoa(w)
		}
		wd = fmt.Sprintf(",\"weekdays\":[%s]", strings.Join(strs, ","))
	}
	return fmt.Sprintf(
		`{"times":[%s],"every_days":%d%s,"anchor":"fixed","anchor_date":"%s","duration_days":%d}`,
		timesJSON(times), everyDays, wd, anchorDate, durationDays)
}

// treatmentSeriesWeekdays reports the series' weekly weekday pattern when
// the legacy dates are EXACTLY the weekday-matching days of their window:
// every day between first and last whose weekday appears in the series must
// itself be a series date. Under that closure a plan with
// every_days:1 + weekdays + anchor=first + duration=len(dates) generates
// precisely the legacy dates — never more, never less. Returns nil when
// the pattern does not hold.
func treatmentSeriesWeekdays(s *treatmentSeries) []int {
	if len(s.dates) < 2 {
		return nil
	}
	set := map[int]bool{}
	for d := range s.dates {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			return nil // defensive: grouping only writes Format()ed keys
		}
		set[int(t.Weekday())] = true
	}
	for day := s.first; !day.After(s.last); day = day.AddDate(0, 0, 1) {
		if s.dates[day.Format("2006-01-02")] {
			continue
		}
		if set[int(day.Weekday())] {
			return nil // generator would emit a day the legacy series lacks
		}
	}
	out := make([]int, 0, len(set))
	for w := range set {
		out = append(out, w)
	}
	sort.Ints(out)
	return out
}

// sortedTreatmentDates returns the series' dates in chronological order.
func sortedTreatmentDates(s *treatmentSeries) []string {
	dates := make([]string, 0, len(s.dates))
	for d := range s.dates {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	return dates
}

// treatmentSeriesRouting is the M5 series router (B10-8): known drug +
// dosage → medication plan (verbatim dosage); wound care and unknown drug
// → CARE plan typed "Soin" — the legacy "drug" column carries many
// NON-drug entries (fistula cleaning, casts, bandage changes, checks…)
// and those fulfill as cares, not observations. A KNOWN drug without
// posology becomes an observation: the dosage field is required for a
// medication payload (§4.2), and the legacy row has none to give. Every
// arm produces a DEFINITIVE conversion (user ruling 2026-10-14: nothing
// is left "à vérifier" — the converter decides, the caregiver sees a
// complete plan, never a flag with no resolution path).
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
		return "Soin de plaie (conversion)", careplan.KindCare,
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
		return "Soin — " + prompt, careplan.KindCare,
			carePayloadJSON(careTypeID, prompt, remarks),
			fmt.Sprintf("médication abusive — converti en soin (%s)", drug)
	case drug == models.WoundCareDrugName:
		return "Soin de plaie (conversion)", careplan.KindObservation,
			observationPayload("Soin de plaie — contrôle", remarks),
			"série soin de plaie → plan observation (pas de caretype)"
	case !knownDrug:
		prompt := drug
		if strings.TrimSpace(dosage) != "" {
			prompt = drug + " (" + dosage + ")"
		}
		return "Traitement — " + prompt, careplan.KindObservation,
			observationPayload(prompt, remarks),
			fmt.Sprintf("drug inconnu — converti en observation (%s)", drug)
	default: // known drug, empty dosage
		return "Traitement — " + drug, careplan.KindObservation,
			observationPayload(drug, remarks),
			fmt.Sprintf("série %s sans posologie — converti en observation", drug)
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
