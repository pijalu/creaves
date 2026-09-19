package models

import (
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
)

func TestAnnotateCaresForDisplay(t *testing.T) {
	now := time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)
	yesterday := now.AddDate(0, 0, -1)
	twoDaysAgo := now.AddDate(0, 0, -2)

	// Ordered by date descending, as the animal page loads them.
	cares := []Care{
		{Date: now, Weight: nulls.NewString("90")}, // today, up vs 100? no: 90 < 100 → down vs next weighted (100)
		{Date: now.Add(-2 * time.Hour), Weight: nulls.NewString("")}, // today, unweighted → no trend
		{Date: yesterday, Weight: nulls.NewString("100")},            // down vs 110
		{Date: twoDaysAgo, Weight: nulls.NewString("110")},           // no earlier weighted care
	}

	AnnotateCaresForDisplay(cares, now)

	// Day groups: cares 0+1 share today (one group), then yesterday, then two days ago.
	if !cares[0].DayGroupStart || cares[0].DayGroupEnd {
		t.Errorf("care0 must start but not end the today group: %+v", cares[0])
	}
	if cares[1].DayGroupStart || !cares[1].DayGroupEnd {
		t.Errorf("care1 must end the today group without starting it: %+v", cares[1])
	}
	if !cares[2].DayGroupStart || !cares[2].DayGroupEnd {
		t.Errorf("care2 should start and end its day group: %+v", cares[2])
	}
	if !cares[3].DayGroupStart || !cares[3].DayGroupEnd {
		t.Errorf("care3 should start and end its day group: %+v", cares[3])
	}

	// Same-day neighbours: unweighted care between two weighted ones.
	cares2 := []Care{
		{Date: now, Weight: nulls.NewString("90")},
		{Date: now.Add(-1 * time.Hour), Weight: nulls.NewString("")},
		{Date: now.Add(-2 * time.Hour), Weight: nulls.NewString("95")},
		{Date: now.AddDate(0, 0, -1), Weight: nulls.NewString("95")},
	}
	AnnotateCaresForDisplay(cares2, now)
	if !cares2[0].DayGroupStart || cares2[0].DayGroupEnd {
		t.Errorf("care0 must start but not end the today group: %+v", cares2[0])
	}
	if cares2[1].DayGroupStart || cares2[1].DayGroupEnd {
		t.Errorf("care1 is inside the today group: %+v", cares2[1])
	}
	if cares2[2].DayGroupStart || !cares2[2].DayGroupEnd {
		t.Errorf("care2 must end the today group: %+v", cares2[2])
	}

	// Trends: 90 vs nearest earlier weighted 95 → down; unweighted skipped; 95 vs 95 → flat.
	if cares2[0].WeightTrend != "down" {
		t.Errorf("care0 trend = %q, want down", cares2[0].WeightTrend)
	}
	if cares2[1].WeightTrend != "" {
		t.Errorf("unweighted care must have no trend, got %q", cares2[1].WeightTrend)
	}
	if cares2[2].WeightTrend != "" {
		t.Errorf("equal weight must have no trend, got %q", cares2[2].WeightTrend)
	}

	// Today flag.
	if !cares2[0].IsToday || !cares2[2].IsToday {
		t.Errorf("cares2[0..2] are today: %+v", cares2)
	}
	if cares2[3].IsToday {
		t.Errorf("cares2[3] is yesterday, IsToday must be false: %+v", cares2[3])
	}
}

func TestAnnotateCaresWeightTrendUp(t *testing.T) {
	now := time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)
	cares := []Care{
		{Date: now, Weight: nulls.NewString("105,5")}, // comma decimal
		{Date: now.AddDate(0, 0, -1), Weight: nulls.NewString("100")},
	}
	AnnotateCaresForDisplay(cares, now)
	if cares[0].WeightTrend != "up" {
		t.Errorf("trend = %q, want up", cares[0].WeightTrend)
	}
	if cares[1].WeightTrend != "" {
		t.Errorf("oldest weighted care must have no trend, got %q", cares[1].WeightTrend)
	}
}

func TestParseCareWeight(t *testing.T) {
	tests := []struct {
		in   nulls.String
		want float64
		ok   bool
	}{
		{nulls.NewString("123.4"), 123.4, true},
		{nulls.NewString("123,4"), 123.4, true},
		{nulls.NewString(" 100 "), 100, true},
		{nulls.NewString(""), 0, false},
		{nulls.NewString("abc"), 0, false},
		{nulls.String{}, 0, false},
	}
	for _, tt := range tests {
		got, ok := ParseCareWeight(tt.in)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("ParseCareWeight(%v) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestWeightTrendClass(t *testing.T) {
	tests := []struct {
		trend string
		want  string
	}{
		{"down", "text-danger font-weight-bold"},
		{"up", "text-success font-weight-bold"},
		{"", "text-muted"},
		{"other", "text-muted"},
	}
	for _, tt := range tests {
		c := Care{WeightTrend: tt.trend}
		if got := c.WeightTrendClass(); got != tt.want {
			t.Errorf("WeightTrendClass(%q) = %q; want %q", tt.trend, got, tt.want)
		}
	}
}

func TestCareTemplateValidate(t *testing.T) {
	tpl := &CareTemplate{Name: "Standard", Content: "Poids: __ g, propreté: oui"}
	if verrs, err := tpl.Validate(nil); err != nil || verrs.HasAny() {
		t.Fatalf("valid template rejected: %v %v", verrs, err)
	}
	empty := &CareTemplate{}
	verrs, err := empty.Validate(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verrs.HasAny() {
		t.Fatal("empty template must be rejected")
	}
}
