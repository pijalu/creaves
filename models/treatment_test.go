package models

import (
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
)

func TestTreatmentBoolToBitmap(t *testing.T) {
	cases := []struct {
		morning, noon, evening bool
		want                   int
	}{
		{false, false, false, 0},
		{true, false, false, Treatement_MORNING},
		{false, true, false, Treatement_NOON},
		{false, false, true, Treatement_EVENING},
		{true, true, true, Treatement_MORNING | Treatement_NOON | Treatement_EVENING},
		{true, false, true, Treatement_MORNING | Treatement_EVENING},
	}
	for _, c := range cases {
		got := TreatmentBoolToBitmap(c.morning, c.noon, c.evening)
		if got != c.want {
			t.Errorf("TreatmentBoolToBitmap(%v,%v,%v) = %d, want %d", c.morning, c.noon, c.evening, got, c.want)
		}
	}
}

func TestTreatmentTemplateString(t *testing.T) {
	tt := TreatmentTemplate{Drug: "Aspirin", Dosage: "5mg"}
	if s := tt.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestTreatmentString(t *testing.T) {
	tr := Treatment{Drug: "Aspirin", Dosage: "5mg"}
	if s := tr.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestTreatmentsString(t *testing.T) {
	ts := Treatments{{Drug: "A"}, {Drug: "B"}}
	if s := ts.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestTreatmentDateFormated(t *testing.T) {
	tr := &Treatment{Date: time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)}
	got := tr.DateFormated()
	want := "2023/06/15"
	if got != want {
		t.Errorf("DateFormated() = %q, want %q", got, want)
	}
}

func TestTreatmentIsPast(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1)
	tr := &Treatment{Date: yesterday}
	if !tr.IsPast() {
		t.Error("Expected yesterday treatment to be in the past")
	}
	if tr.IsToday() {
		t.Error("Yesterday treatment should not be today")
	}
	if tr.IsFuture() {
		t.Error("Yesterday treatment should not be future")
	}
}

func TestTreatmentIsToday(t *testing.T) {
	now := time.Now()
	tr := &Treatment{Date: now}
	if !tr.IsToday() {
		t.Error("Expected today treatment to be today")
	}
	if tr.IsPast() {
		t.Error("Today treatment should not be past")
	}
	if tr.IsFuture() {
		t.Error("Today treatment should not be future")
	}
}

func TestTreatmentIsFuture(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 1)
	tr := &Treatment{Date: tomorrow}
	if !tr.IsFuture() {
		t.Error("Expected tomorrow treatment to be in the future")
	}
	if tr.IsPast() {
		t.Error("Tomorrow treatment should not be past")
	}
	if tr.IsToday() {
		t.Error("Tomorrow treatment should not be today")
	}
}

func TestTreatmentScheduleRequired(t *testing.T) {
	tr := &Treatment{Timebitmap: Treatement_MORNING | Treatement_EVENING}
	if !tr.ScheduleRequired(Treatement_MORNING) {
		t.Error("Expected morning to be required")
	}
	if tr.ScheduleRequired(Treatement_NOON) {
		t.Error("Expected noon to NOT be required")
	}
	if !tr.ScheduleRequired(Treatement_EVENING) {
		t.Error("Expected evening to be required")
	}
}

func TestTreatmentScheduleRequiredConvenience(t *testing.T) {
	tr := &Treatment{Timebitmap: Treatement_NOON}
	if tr.ScheduleRequiredMorning() {
		t.Error("Expected morning NOT required")
	}
	if !tr.ScheduleRequiredNoon() {
		t.Error("Expected noon required")
	}
	if tr.ScheduleRequiredEvening() {
		t.Error("Expected evening NOT required")
	}
}

func TestTreatmentScheduleStatus(t *testing.T) {
	// Required and done.
	tr := &Treatment{
		Timebitmap:     Treatement_MORNING,
		Timedonebitmap: Treatement_MORNING,
	}
	s := tr.ScheduleStatus(Treatement_MORNING)
	if !s.Valid || !s.Bool {
		t.Errorf("Expected valid+true status for done morning, got %+v", s)
	}
}

func TestTreatmentScheduleStatusRequiredNotDone(t *testing.T) {
	tr := &Treatment{
		Timebitmap:     Treatement_MORNING,
		Timedonebitmap: 0,
	}
	s := tr.ScheduleStatus(Treatement_MORNING)
	if !s.Valid || s.Bool {
		t.Errorf("Expected valid+false status for not-done morning, got %+v", s)
	}
}

func TestTreatmentScheduleStatusNotRequired(t *testing.T) {
	tr := &Treatment{
		Timebitmap:     0,
		Timedonebitmap: Treatement_MORNING,
	}
	s := tr.ScheduleStatus(Treatement_MORNING)
	if s.Valid {
		t.Errorf("Expected invalid status when not required, got %+v", s)
	}
}

func TestTreatmentScheduleStatusConvenience(t *testing.T) {
	tr := &Treatment{
		Timebitmap:     Treatement_MORNING | Treatement_NOON | Treatement_EVENING,
		Timedonebitmap: Treatement_MORNING | Treatement_EVENING,
	}
	if got := tr.ScheduleStatusMorning(); !got.Valid || !got.Bool {
		t.Errorf("Morning status = %+v, want valid+true", got)
	}
	if got := tr.ScheduleStatusNoon(); !got.Valid || got.Bool {
		t.Errorf("Noon status = %+v, want valid+false", got)
	}
	if got := tr.ScheduleStatusEvening(); !got.Valid || !got.Bool {
		t.Errorf("Evening status = %+v, want valid+true", got)
	}
}

func TestTreatmentSetAllScheduleRequired(t *testing.T) {
	tr := &Treatment{Timebitmap: 0}
	tr.SetAllScheduleRequired(true, false, true)
	want := Treatement_MORNING | Treatement_EVENING
	if tr.Timebitmap != want {
		t.Errorf("Timebitmap = %d, want %d", tr.Timebitmap, want)
	}
}

func TestTreatmentSetAllScheduleRequiredNone(t *testing.T) {
	tr := &Treatment{Timebitmap: Treatement_MORNING}
	tr.SetAllScheduleRequired(false, false, false)
	if tr.Timebitmap != 0 {
		t.Errorf("Timebitmap = %d, want 0", tr.Timebitmap)
	}
}

func TestTreatmentSetAllScheduleStatus(t *testing.T) {
	tr := &Treatment{Timedonebitmap: 0}
	tr.SetAllScheduleStatus(true, true, false)
	want := Treatement_MORNING | Treatement_NOON
	if tr.Timedonebitmap != want {
		t.Errorf("Timedonebitmap = %d, want %d", tr.Timedonebitmap, want)
	}
}

func TestTreatmentSetAllScheduleStatusNone(t *testing.T) {
	tr := &Treatment{Timedonebitmap: Treatement_MORNING}
	tr.SetAllScheduleStatus(false, false, false)
	if tr.Timedonebitmap != 0 {
		t.Errorf("Timedonebitmap = %d, want 0", tr.Timedonebitmap)
	}
}

func TestTreatmentsMapOrderedKeys(t *testing.T) {
	d1 := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)
	m := TreatmentsMap{
		{Date: d1}: Treatments{},
		{Date: d2}: Treatments{},
		{Date: d3}: Treatments{},
	}
	keys := m.OrderedKeys()
	if len(keys) != 3 {
		t.Fatalf("Expected 3 keys, got %d", len(keys))
	}
	// Descending by date (most recent first).
	if !keys[0].Date.Equal(d3) {
		t.Errorf("Expected first key date %v, got %v", d3, keys[0].Date)
	}
	if !keys[1].Date.Equal(d2) {
		t.Errorf("Expected second key date %v, got %v", d2, keys[1].Date)
	}
	if !keys[2].Date.Equal(d1) {
		t.Errorf("Expected third key date %v, got %v", d1, keys[2].Date)
	}
}

func TestTreatmentsMapGrouping(t *testing.T) {
	d := time.Date(2023, 6, 15, 10, 0, 0, 0, time.UTC)
	ts := Treatments{
		{Date: d, Drug: "A"},
		{Date: d, Drug: "B"},
		{Date: time.Date(2023, 6, 16, 10, 0, 0, 0, time.UTC), Drug: "C"},
	}
	m := ts.TreatmentsMap()
	// Two distinct dates → two keys.
	if len(m) != 2 {
		t.Fatalf("Expected 2 groups, got %d", len(m))
	}
	// Find the group with two treatments.
	var count int
	for _, v := range m {
		if len(v) == 2 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected one group with 2 treatments, got %d groups", count)
	}
}

func TestTreatmentsMapKeyFlags(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1)
	tomorrow := time.Now().AddDate(0, 0, 1)
	ts := Treatments{
		{Date: yesterday},
		{Date: tomorrow},
	}
	m := ts.TreatmentsMap()
	for k := range m {
		if k.Date.Equal(nowDt()) {
			if !k.Current {
				t.Error("Expected Current flag for today")
			}
		} else if k.Date.Before(nowDt()) {
			if !k.Past {
				t.Error("Expected Past flag for yesterday")
			}
		} else {
			if !k.Future {
				t.Error("Expected Future flag for tomorrow")
			}
		}
	}
}

func TestTreatmentsTodayStatitics(t *testing.T) {
	// Two treatments: morning required+done, noon required+not-done.
	ts := Treatments{
		{
			Timebitmap:     Treatement_MORNING,
			Timedonebitmap: Treatement_MORNING,
		},
		{
			Timebitmap:     Treatement_NOON,
			Timedonebitmap: 0,
		},
	}
	stat := ts.TodayStatitics()
	// Morning: required and done in first, not required in second → stays valid true.
	if !stat.Morning.Valid || !stat.Morning.Bool {
		t.Errorf("Morning stat = %+v, want valid+true", stat.Morning)
	}
	// Noon: not required in first, required+not-done in second → valid false.
	if !stat.Noon.Valid || stat.Noon.Bool {
		t.Errorf("Noon stat = %+v, want valid+false", stat.Noon)
	}
	// Evening: not required in either → invalid.
	if stat.Evening.Valid {
		t.Errorf("Evening stat = %+v, want invalid", stat.Evening)
	}
}

func TestTreatmentsTodayStatiticsAndLogic(t *testing.T) {
	// Both treatments require morning: one done, one not → result false.
	ts := Treatments{
		{
			Timebitmap:     Treatement_MORNING,
			Timedonebitmap: Treatement_MORNING,
		},
		{
			Timebitmap:     Treatement_MORNING,
			Timedonebitmap: 0,
		},
	}
	stat := ts.TodayStatitics()
	if !stat.Morning.Valid || stat.Morning.Bool {
		t.Errorf("Morning stat = %+v, want valid+false (true AND false)", stat.Morning)
	}
}

func TestAndBothValid(t *testing.T) {
	b := nulls.NewBool(true)
	ba := nulls.NewBool(false)
	and(&b, ba)
	if !b.Valid || b.Bool {
		t.Errorf("and(true,false) = %+v, want valid+false", b)
	}
}

func TestAndFirstInvalid(t *testing.T) {
	b := nulls.Bool{}
	ba := nulls.NewBool(true)
	and(&b, ba)
	if !b.Valid || !b.Bool {
		t.Errorf("and(invalid,true) = %+v, want valid+true", b)
	}
}

func TestAndSecondInvalid(t *testing.T) {
	b := nulls.NewBool(true)
	ba := nulls.Bool{}
	and(&b, ba)
	if !b.Valid || !b.Bool {
		t.Errorf("and(true,invalid) = %+v, want valid+true (unchanged)", b)
	}
}
