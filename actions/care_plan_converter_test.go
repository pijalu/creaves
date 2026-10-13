package actions

import (
	"strings"
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

// Pure-layer tests of the §8.1 startup converter (docs/care-expert.md):
// seed library integrity (§7.4) and the legacy-row derivation helpers.
// The MySQL round-trip (idempotency, marker, report) lives in
// care_plan_converter_mysql_test.go.

// TestSeedMatcherCount pins the canonical seed set: the §7.4 13 canonical
// matchers (SM1–SM13) + R8-3 SM14, plus the 4 clearly-marked composite
// matchers needed by SR1/SR4/SR6/SR12 (care_rules reference one matcher).
func TestSeedMatcherCount(t *testing.T) {
	canonical, derived := 0, 0
	for _, def := range SeedMatchers() {
		if def.Derived {
			derived++
			continue
		}
		canonical++
		if !strings.HasPrefix(def.Key, "SM") {
			t.Errorf("canonical matcher %q must be keyed SM…", def.Key)
		}
	}
	if canonical != 14 {
		t.Errorf("want 13 canonical seed matchers (SM1–SM13), got %d", canonical)
	}
	if derived != 4 {
		t.Errorf("want 4 derived composite matchers, got %d", derived)
	}
}

// TestSeedMatcherExpressionsParse proves every seed expression is valid
// matcher DSL over the default field registry (§5.1/§5.2) — a seed that
// fails to parse must never reach the database.
func TestSeedMatcherExpressionsParse(t *testing.T) {
	for _, def := range SeedMatchers() {
		if _, err := careplan.ParseValidatedWith(def.Expression, careplan.DefaultRegistry()); err != nil {
			t.Errorf("seed matcher %s (%s): %v", def.Key, def.Name, err)
		}
		// bugs.md t10 (2026-10-27 user ruling): cage names are per-center —
		// no DEFAULT matcher may ever predicate on them. The `cage` field
		// stays DSL-resolvable for hand-made rules; seeds must not use it.
		if strings.Contains(strings.ToLower(def.Expression), "cage ") {
			t.Errorf("seed matcher %s (%s) predicates on cage names: %s", def.Key, def.Name, def.Expression)
		}
	}
}

// TestSeedBuildersStampDefaultDescription pins R9-5: every seeded §7.4 rule
// and matcher must carry the user-facing DefaultRuleDescription ("Règles par
// défaut") and never leak the internal converter provenance string
// ("Bibliothèque §7.4 KEY [source: care_plan_converter]") into the UI.
func TestSeedBuildersStampDefaultDescription(t *testing.T) {
	for _, def := range SeedMatchers() {
		m := buildSeedMatcher(def)
		if !m.Description.Valid || m.Description.String != DefaultRuleDescription {
			t.Errorf("seed matcher %s: description = %q, want %q", def.Key, m.Description.String, DefaultRuleDescription)
		}
		if strings.Contains(m.Description.String, ConverterTag) {
			t.Errorf("seed matcher %s: description leaks converter tag: %q", def.Key, m.Description.String)
		}
	}
	// Feeding/care payloads embed a caretype_id; a dummy UUID satisfies the
	// payload validator (no FK check at build time).
	ct := uuid.Must(uuid.NewV4()).String()
	for _, def := range SeedRules() {
		r, err := buildSeedRule(def, uuid.NullUUID{}, ct)
		if err != nil {
			t.Fatalf("buildSeedRule %s: %v", def.Key, err)
		}
		if !r.Description.Valid || r.Description.String != DefaultRuleDescription {
			t.Errorf("seed rule %s: description = %q, want %q", def.Key, r.Description.String, DefaultRuleDescription)
		}
		if strings.Contains(r.Description.String, ConverterTag) {
			t.Errorf("seed rule %s: description leaks converter tag: %q", def.Key, r.Description.String)
		}
	}
}

// TestSeedRuleCount pins SR1–SR13 (R8-3 adds the cleanup rule).
func TestSeedRuleCount(t *testing.T) {
	if n := len(SeedRules()); n != 13 {
		t.Fatalf("want 13 seed rules (SR1–SR13), got %d", n)
	}
	keys := map[string]bool{}
	for _, r := range SeedRules() {
		keys[r.Key] = true
	}
	for i := 1; i <= 13; i++ {
		k := "SR" + string(rune('0'+i))
		if i == 10 {
			k = "SR10"
		}
		if i >= 10 {
			k = "SR1" + string(rune('0'+i-10))
		}
		if !keys[k] {
			t.Errorf("missing seed rule %s", k)
		}
	}
}

// TestSeedRulesValidate builds every seed rule with a placeholder caretype
// id and runs the model validation (payload schema per kind §4.2, schedule
// schema §4.3). Only rules that legitimately need a caretype get one.
func TestSeedRulesValidate(t *testing.T) {
	const ct = "3f6a1b2c-0000-4000-8000-000000000002"
	for _, def := range SeedRules() {
		var caretypeID string
		if def.CaretypeName != "" {
			caretypeID = ct
		}
		rule, err := buildSeedRule(def, matcherIDForTest(), caretypeID)
		if err != nil {
			t.Errorf("seed rule %s: %v", def.Key, err)
			continue
		}
		checkSeedRule(t, def, rule)
	}
}

// checkSeedRule asserts the seed-rule invariants beyond schema validity:
// feeding seeds ship active, non-feeding seeds ship as inactive drafts
// (SR13 excepted, R8-3), and latch_membership follows DurationDays.
func checkSeedRule(t *testing.T, def seedRuleDef, rule *models.CareRule) {
	t.Helper()
	if def.Kind == careplan.KindFeeding && !rule.Active {
		t.Errorf("seed rule %s: feeding-kind seeds must ship active (§8.1 step 1)", def.Key)
	}
	// R8-3: SR13 ships ACTIVE — it exists to enforce cleanup on flagged
	// zones, an inactive copy would enforce nothing.
	if def.Kind != careplan.KindFeeding && rule.Active && def.Key != "SR13" {
		t.Errorf("seed rule %s: non-feeding seeds must be inactive drafts (§8.1 step 1)", def.Key)
	}
	if def.Key == "SR13" && !rule.Active {
		t.Errorf("seed rule SR13 must ship active (R8-3 enforcement)")
	}
	if rule.LatchMembership != (def.DurationDays > 0) {
		t.Errorf("seed rule %s: latch_membership = %v, want %v", def.Key, rule.LatchMembership, def.DurationDays > 0)
	}
}

// TestSeedRuleSchedulesParse re-parses every stored schedule document.
func TestSeedRuleSchedulesParse(t *testing.T) {
	for _, def := range SeedRules() {
		if _, err := careplan.ParseScheduleJSON([]byte(def.ScheduleJSON)); err != nil {
			t.Errorf("seed rule %s schedule: %v", def.Key, err)
		}
	}
}

// atTime builds a wall-clock time for DeriveFeedingTimes (date part unused).
func atTime(hm [2]int) time.Time {
	return time.Date(2026, 9, 20, hm[0], hm[1], 0, 0, time.Local)
}

// matcherIDForTest returns a valid-but-unused matcher reference.
func matcherIDForTest() uuid.NullUUID {
	return uuid.NullUUID{UUID: uuid.Must(uuid.NewV4()), Valid: true}
}

// TestDeriveFeedingTimes covers §8.1 step 2 slot derivation (§11.7): the
// start slot is included (n ≥ 0, legacy calculateFeeding semantics),
// start == end yields the single daily slot, overnight windows wrap, and
// degenerate inputs yield nothing. Since BUG-36 the shim delegates to
// careplan.ExpandRepeat, so the result is sorted strictly ascending (the
// §4.3 times invariant) — an overnight window comes back sorted, not in
// generation order.
func TestDeriveFeedingTimes(t *testing.T) {
	at := func(h, m int) [2]int { return [2]int{h, m} }
	cases := []struct {
		name       string
		start, end [2]int
		period     int
		want       [][2]int
	}{
		{"normal incl. start", [2]int{8, 0}, [2]int{18, 0}, 120, [][2]int{at(8, 0), at(10, 0), at(12, 0), at(14, 0), at(16, 0), at(18, 0)}},
		{"spec §11.7 example", [2]int{7, 0}, [2]int{22, 0}, 600, [][2]int{at(7, 0), at(17, 0)}},
		{"start==end single slot", [2]int{10, 0}, [2]int{10, 0}, 600, [][2]int{at(10, 0)}},
		{"single at end", [2]int{8, 0}, [2]int{18, 0}, 600, [][2]int{at(8, 0), at(18, 0)}},
		{"overnight (sorted, §4.3 invariant)", [2]int{22, 0}, [2]int{6, 0}, 240, [][2]int{at(2, 0), at(6, 0), at(22, 0)}},
		{"no slot fits beyond start", [2]int{8, 0}, [2]int{9, 0}, 120, [][2]int{at(8, 0)}},
		{"zero period", [2]int{8, 0}, [2]int{18, 0}, 0, nil},
		{"negative period", [2]int{8, 0}, [2]int{18, 0}, -5, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := atTime(tc.start)
			got := DeriveFeedingTimes(start, atTime(tc.end), tc.period)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i, s := range got {
				if s.Hour != tc.want[i][0] || s.Minute != tc.want[i][1] {
					t.Fatalf("slot %d = %v, want %02d:%02d", i, s, tc.want[i][0], tc.want[i][1])
				}
			}
		})
	}
}

// TestBitmapToTimes pins the §10.1-3/§10-M1 bitmap mapping.
func TestBitmapToTimes(t *testing.T) {
	cases := []struct {
		bitmap int
		want   []string
	}{
		{0, nil},
		{models.Treatement_MORNING, []string{"08:00"}},
		{models.Treatement_NOON, []string{"12:00"}},
		{models.Treatement_EVENING, []string{"18:00"}},
		{3, []string{"08:00", "12:00"}},
		{7, []string{"08:00", "12:00", "18:00"}},
	}
	for _, tc := range cases {
		got := BitmapToTimes(tc.bitmap)
		if len(got) != len(tc.want) {
			t.Fatalf("bitmap %d: got %v, want %v", tc.bitmap, got, tc.want)
		}
		for i, s := range got {
			if s.String() != tc.want[i] {
				t.Errorf("bitmap %d slot %d = %s, want %s", tc.bitmap, i, s, tc.want[i])
			}
		}
	}
}

// TestNormalizeDiet: cluster key is trimmed + case-folded.
func TestNormalizeDiet(t *testing.T) {
	if NormalizeDiet("  Grains Pigeons EAU ") != "grains pigeons eau" {
		t.Errorf("got %q", NormalizeDiet("  Grains Pigeons EAU "))
	}
	if NormalizeDiet("   ") != "" {
		t.Error("blank diet must normalize to empty")
	}
}

// TestBuildConvertedScheduleJSON: the converted schedule must round-trip
// through the strict §4.3 parser — both the fallback form (plain slots,
// no repeat) and the exact-triple repeat form (BUG-36: block + expanded
// times consistent with the block, or the parse fails validation).
func TestBuildConvertedScheduleJSON(t *testing.T) {
	raw := buildConvertedScheduleJSON([]careplan.TimeOfDay{
		{Hour: 10, Minute: 0}, {Hour: 18, Minute: 30},
	}, nil, 0)
	sched, err := careplan.ParseScheduleJSON([]byte(raw))
	if err != nil {
		t.Fatalf("converted schedule does not parse: %v", err)
	}
	if len(sched.Times) != 2 || sched.Times[0].String() != "10:00" || sched.Times[1].String() != "18:30" {
		t.Errorf("times = %v", sched.Times)
	}
	if sched.Anchor != careplan.AnchorIntake || sched.EveryDays != 1 {
		t.Errorf("anchor/every_days = %s/%d", sched.Anchor, sched.EveryDays)
	}
	if sched.Repeat != nil {
		t.Errorf("fallback schedule must not carry a repeat block, got %+v", sched.Repeat)
	}
}

// TestBuildConvertedScheduleJSONRepeatForm pins the BUG-36 exact-triple
// form: repeat block (from/to = legacy start/end, every_minutes = legacy
// period) + times equal to its expansion. The stored document must parse
// AND validate — the repeat consistency check (times == ExpandRepeat)
// runs inside ParseScheduleJSON.
func TestBuildConvertedScheduleJSONRepeatForm(t *testing.T) {
	// Day window.
	checkRepeatFormSchedule(t, "day", careplan.Repeat{
		From:         careplan.TimeOfDay{Hour: 8, Minute: 0},
		To:           careplan.TimeOfDay{Hour: 18, Minute: 0},
		EveryMinutes: 120,
	})

	// Overnight legacy window: end < start stays verbatim in the block
	// (the editor shows from=22:00 to=06:00), times sorted per §4.3.
	checkRepeatFormSchedule(t, "overnight", careplan.Repeat{
		From:         careplan.TimeOfDay{Hour: 22, Minute: 0},
		To:           careplan.TimeOfDay{Hour: 6, Minute: 0},
		EveryMinutes: 240,
	})
}

// checkRepeatFormSchedule builds the converted schedule JSON for a repeat
// block, parses it back and asserts the block round-trips verbatim with
// times equal to its expansion.
func checkRepeatFormSchedule(t *testing.T, label string, r careplan.Repeat) {
	t.Helper()
	raw := buildConvertedScheduleJSON(careplan.ExpandRepeat(r), &r, 0)
	sched, err := careplan.ParseScheduleJSON([]byte(raw))
	if err != nil {
		t.Fatalf("%s repeat-form schedule does not parse/validate: %v", label, err)
	}
	if sched.Repeat == nil || *sched.Repeat != r {
		t.Fatalf("%s repeat block = %+v, want %+v", label, sched.Repeat, r)
	}
	got := make([]string, 0, len(sched.Times))
	for _, ts := range sched.Times {
		got = append(got, ts.String())
	}
	want := []string{}
	for _, ts := range careplan.ExpandRepeat(r) {
		want = append(want, ts.String())
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s times = %v, want %v", label, got, want)
	}
}

// TestLegacyFeedingScheduleRepeatForm covers the BUG-36 converter mapping
// itself: exact legacy triples (non-NULL start/end + period > 0) yield the
// repeat form with NO fallback flag (exact translation); NULL start/end,
// degenerate periods and over-cap windows keep the flagged fallback slots
// with no repeat block.
func TestLegacyFeedingScheduleRepeatForm(t *testing.T) {
	// BUG-50: start==end rows (≈37 in-care animals: the "à disposition"
	// grain cages) carry a period but a ZERO-LENGTH window. The legacy
	// reminder engine (feeding.go calculateFeeding) answers a feeding at
	// start with the heuristic end = end − period/2 < last ⇒ next =
	// start + 24 h — ONE daily occurrence at start; the period only
	// widens the late window (period/2). The pre-BUG-50 converter stored
	// repeat{from X, to X, every P}, which the protocol editor renders
	// verbatim as "from 10:00 to 10:00 every 10:00" — nonsense to
	// caregivers and a precision the legacy data does not carry. The
	// conversion must be the plain single-slot schedule: same runtime
	// behavior, no fake repeat block, NOT flagged (exact translation).
	cases := []legacyFeedingScheduleCase{
		{name: "exact triple → repeat form, no flag",
			start: "08:00", end: "18:00", period: 120,
			wantRepeat: &careplan.Repeat{From: careplan.TimeOfDay{Hour: 8}, To: careplan.TimeOfDay{Hour: 18}, EveryMinutes: 120},
			wantLen:    6, wantFirst: "08:00", wantLast: "18:00"},
		{name: "overnight triple → repeat form with end < start preserved",
			start: "22:00", end: "06:00", period: 240,
			wantRepeat: &careplan.Repeat{From: careplan.TimeOfDay{Hour: 22}, To: careplan.TimeOfDay{Hour: 6}, EveryMinutes: 240},
			wantLen:    3, wantFirst: "02:00", wantLast: "22:00"},
		{name: "NULL end → single slot at start, flagged, no repeat",
			start: "07:30", period: 120, wantFallback: true, wantLen: 1, wantFirst: "07:30"},
		{name: "NULL start+end (period-only row, BUG-10) → 09:00 fallback, flagged",
			period: 480, wantFallback: true, wantLen: 1, wantFirst: "09:00"},
		{name: "degenerate period with start/end set → single slot at start, flagged",
			start: "08:00", end: "18:00", wantFallback: true, wantLen: 1, wantFirst: "08:00"},
		{name: "BUG-50 start==end with period → single slot, NO repeat, NOT flagged (exact translation)",
			start: "10:00", end: "10:00", period: 600,
			wantLen: 1, wantFirst: "10:00"},
		{name: "start==end with a degenerate period keeps the flagged fallback slot",
			start: "19:00", end: "19:00", wantFallback: true, wantLen: 1, wantFirst: "19:00"},
		{name: "over-cap window (1 min over 23:59 h → 1440 slots): capped slots, flagged, no repeat",
			start: "00:00", end: "23:59", period: 1, wantFallback: true, wantLen: 24},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { tc.run(t) })
	}
}

// legacyFeedingScheduleCase is one TestLegacyFeedingScheduleRepeatForm
// scenario: a legacy column triple plus the expected conversion result.
type legacyFeedingScheduleCase struct {
	name         string
	start, end   string
	period       int
	wantFallback bool
	wantRepeat   *careplan.Repeat
	wantLen      int
	wantFirst    string // optional: first slot, "" = skip
	wantLast     string // optional: last slot, "" = skip
}

func (tc legacyFeedingScheduleCase) run(t *testing.T) {
	t.Helper()
	times, repeat, fallback := legacyFeedingSchedule(legacyFeedingAnimalFixture(tc.start, tc.end, tc.period))
	if fallback != tc.wantFallback {
		t.Errorf("fallback = %v, want %v", fallback, tc.wantFallback)
	}
	switch {
	case tc.wantRepeat == nil && repeat != nil:
		t.Errorf("repeat = %+v, want none", repeat)
	case tc.wantRepeat != nil && (repeat == nil || *repeat != *tc.wantRepeat):
		t.Errorf("repeat = %+v, want %+v", repeat, *tc.wantRepeat)
	}
	if len(times) != tc.wantLen {
		t.Fatalf("len(times) = %d, want %d", len(times), tc.wantLen)
	}
	if tc.wantFirst != "" && times[0].String() != tc.wantFirst {
		t.Errorf("first slot = %s, want %s", times[0], tc.wantFirst)
	}
	if tc.wantLast != "" && times[len(times)-1].String() != tc.wantLast {
		t.Errorf("last slot = %s, want %s", times[len(times)-1], tc.wantLast)
	}
}

// legacyFeedingAnimalFixture builds an animal row from "HH:MM" legacy
// feeding columns ("" = NULL).
func legacyFeedingAnimalFixture(start, end string, period int) models.Animal {
	base := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	a := models.Animal{FeedingPeriod: period}
	if start != "" {
		st, _ := time.Parse("15:04", start)
		a.FeedingStart = nulls.NewTime(base.Add(time.Duration(st.Hour())*time.Hour + time.Duration(st.Minute())*time.Minute))
	}
	if end != "" {
		en, _ := time.Parse("15:04", end)
		a.FeedingEnd = nulls.NewTime(base.Add(time.Duration(en.Hour())*time.Hour + time.Duration(en.Minute())*time.Minute))
	}
	return a
}

// TestLegacyFeedingGraceMinutes pins the BUG-50 lateness faithfulness:
// the legacy day plan marked a missed slot late once it was older than
// half the period (calculateFeeding criticalLimit = now − period/2). The
// converted schedule carries that cadence as grace_minutes; rows without
// a usable period keep the §4.3 default (60).
func TestLegacyFeedingGraceMinutes(t *testing.T) {
	cases := []struct {
		period, want int
	}{
		{0, 0},   // no period → §4.3 default (omit from the document)
		{60, 30}, // faithful even below the default
		{240, 120},
		{480, 240},
		{600, 300},
	}
	for _, tc := range cases {
		if got := legacyFeedingGraceMinutes(tc.period); got != tc.want {
			t.Errorf("legacyFeedingGraceMinutes(%d) = %d, want %d", tc.period, got, tc.want)
		}
	}
}

// TestBuildConvertedScheduleJSONGrace: a non-default grace rides in the
// document and parses; the default (60) and the no-period 0 stay omitted
// so the §4.3 default applies.
func TestBuildConvertedScheduleJSONGrace(t *testing.T) {
	times := []careplan.TimeOfDay{{Hour: 10, Minute: 0}}

	raw := buildConvertedScheduleJSON(times, nil, 300)
	if !strings.Contains(raw, `"grace_minutes":300`) {
		t.Errorf("grace 300 missing from %s", raw)
	}
	sched, err := careplan.ParseScheduleJSON([]byte(raw))
	if err != nil {
		t.Fatalf("graced schedule does not parse: %v", err)
	}
	if sched.GraceMinutes != 300 {
		t.Errorf("GraceMinutes = %d, want 300", sched.GraceMinutes)
	}

	for _, g := range []int{0, 60} {
		raw := buildConvertedScheduleJSON(times, nil, g)
		if strings.Contains(raw, "grace_minutes") {
			t.Errorf("grace %d must stay omitted (§4.3 default), got %s", g, raw)
		}
	}
}

// TestReconcileFeeding covers the §1.6 no-loss gate: OK when converted
// slots equal legacy, DEGRADED when they differ, UNCOVERED when no
// rule/plan was recorded for the animal.
func TestReconcileFeeding(t *testing.T) {
	mk := func(id int, start, end string, period int) models.Animal {
		base := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
		at, _ := time.Parse("15:04", start)
		bt, _ := time.Parse("15:04", end)
		return models.Animal{
			ID:            id,
			FeedingStart:  nulls.NewTime(base.Add(time.Duration(at.Hour())*time.Hour + time.Duration(at.Minute())*time.Minute)),
			FeedingEnd:    nulls.NewTime(base.Add(time.Duration(bt.Hour())*time.Hour + time.Duration(bt.Minute())*time.Minute)),
			FeedingPeriod: period,
		}
	}
	exact := []careplan.TimeOfDay{{Hour: 8}, {Hour: 14}} // == derive(08:00,14:00,360)
	other := []careplan.TimeOfDay{{Hour: 9}}

	report := &ConversionReport{coverage: map[int][]careplan.TimeOfDay{
		1: exact,
		2: other,
		// 3: no coverage entry → UNCOVERED
	}}
	animals := []models.Animal{
		mk(1, "08:00", "14:00", 360),
		mk(2, "08:00", "14:00", 360),
		mk(3, "08:00", "14:00", 360),
	}
	reconcileFeeding(report, animals)
	if report.Reconciliation.FeedingOK != 1 {
		t.Errorf("FeedingOK = %d, want 1", report.Reconciliation.FeedingOK)
	}
	if report.Reconciliation.FeedingDegraded != 1 {
		t.Errorf("FeedingDegraded = %d, want 1", report.Reconciliation.FeedingDegraded)
	}
	if report.Reconciliation.FeedingUncovered != 1 {
		t.Errorf("FeedingUncovered = %d, want 1", report.Reconciliation.FeedingUncovered)
	}
	if len(report.Reconciliation.Lines) != 2 {
		t.Errorf("reconciliation lines = %d, want 2 (DEGRADED + UNCOVERED)", len(report.Reconciliation.Lines))
	}
}

// TestConvertDataEmitsNoCageMatchers pins the v3 direction (bugs.md t10,
// 2026-10-27 user ruling: "cage names are per centers — eliminate the
// cage-name matchers; in doubt, migrate to the animal"). The feeding
// converter builds per-animal plans only: it must not construct cluster
// matchers, cage clauses, or rules at all. Any of these symbols coming
// back into care_plan_convert_data.go is a regression toward cage-scoped
// matchers, which cannot survive a center's cage renaming and (see the
// 2026-10-27 clone audit) silently over-sweep same-species animals housed
// elsewhere.
func TestConvertDataEmitsNoCageMatchers(t *testing.T) {
	src := readTemplate(t, "../actions/care_plan_convert_data.go")
	for _, banned := range []string{
		"cage INCI",                  // cage-scoped cluster matcher (R5-1d, retired)
		"cage =*",                    // ditto, single-enclosure form
		"cageClause",                 // the clause builder
		"upsertClusterMatcher",       // matcher upsert (no converter matchers anymore)
		"convertedMatcherName(diet)", // matcher naming (no converter matchers anymore)
	} {
		if strings.Contains(src, banned) {
			t.Errorf("v3 regression: care_plan_convert_data.go contains %q — the converter must not emit cage-scoped cluster matchers", banned)
		}
	}
}
