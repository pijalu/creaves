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
		wantLatch := def.DurationDays > 0
		if rule.LatchMembership != wantLatch {
			t.Errorf("seed rule %s: latch_membership = %v, want %v", def.Key, rule.LatchMembership, wantLatch)
		}
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
// degenerate inputs yield nothing.
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
		{"overnight", [2]int{22, 0}, [2]int{6, 0}, 240, [][2]int{at(22, 0), at(2, 0), at(6, 0)}},
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
// through the strict §4.3 parser.
func TestBuildConvertedScheduleJSON(t *testing.T) {
	raw := buildConvertedScheduleJSON([]careplan.TimeOfDay{
		{Hour: 10, Minute: 0}, {Hour: 18, Minute: 30},
	})
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
