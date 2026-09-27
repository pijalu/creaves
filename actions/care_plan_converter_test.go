package actions

import (
	"strings"
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gofrs/uuid"
)

// Pure-layer tests of the §8.1 startup converter (docs/care-expert.md):
// seed library integrity (§7.4) and the legacy-row derivation helpers.
// The MySQL round-trip (idempotency, marker, report) lives in
// care_plan_converter_mysql_test.go.

// TestSeedMatcherCount pins the §7.4 canonical seed set: exactly 13
// canonical matchers (SM1–SM13) plus the 4 clearly-marked composite
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
	if canonical != 13 {
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
	}
}

// TestSeedRuleCount pins SR1–SR12.
func TestSeedRuleCount(t *testing.T) {
	if n := len(SeedRules()); n != 12 {
		t.Fatalf("want 12 seed rules (SR1–SR12), got %d", n)
	}
	keys := map[string]bool{}
	for _, r := range SeedRules() {
		keys[r.Key] = true
	}
	for i := 1; i <= 12; i++ {
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
		if def.Kind != careplan.KindFeeding && rule.Active {
			t.Errorf("seed rule %s: non-feeding seeds must be inactive drafts (§8.1 step 1)", def.Key)
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

// TestDeriveFeedingTimes covers §8.1 step 2 slot derivation: start+n×period
// while ≤ end, no partial slots, degenerate inputs yield nothing.
func TestDeriveFeedingTimes(t *testing.T) {
	at := func(h, m int) [2]int { return [2]int{h, m} }
	cases := []struct {
		name       string
		start, end [2]int
		period     int
		want       [][2]int
	}{
		{"normal", [2]int{8, 0}, [2]int{18, 0}, 120, [][2]int{at(10, 0), at(12, 0), at(14, 0), at(16, 0), at(18, 0)}},
		{"single", [2]int{8, 0}, [2]int{18, 0}, 600, [][2]int{at(18, 0)}},
		{"no slot fits", [2]int{8, 0}, [2]int{9, 0}, 120, nil},
		{"end before start", [2]int{18, 0}, [2]int{8, 0}, 120, nil},
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

// TestModalTimes: most frequent slot set wins; ties resolve lexicographically
// for determinism.
func TestModalTimes(t *testing.T) {
	a := []careplan.TimeOfDay{{Hour: 8}, {Hour: 18}}
	b := []careplan.TimeOfDay{{Hour: 9}}
	entries := []feedingEntry{
		{Times: a}, {Times: a},
		{Times: b},
	}
	if got := timesKey(modalTimes(entries)); got != timesKey(a) {
		t.Errorf("modal = %s, want %s", got, timesKey(a))
	}
	tie := []feedingEntry{{Times: b}, {Times: a}}
	if got := timesKey(modalTimes(tie)); got != timesKey(a) {
		t.Errorf("tie modal = %s, want lexicographic first %s", got, timesKey(a))
	}
}
