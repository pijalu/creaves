package models

import (
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
)

func TestAnimalsByTypeMapOrderedKeys(t *testing.T) {
	m := AnimalsByTypeMap{
		{ID: "1", Name: "Zebra"}:   Animals{},
		{ID: "2", Name: "Apple"}:   Animals{},
		{ID: "3", Name: "Mango"}:   Animals{},
	}

	keys := m.OrderedKeys()
	if len(keys) != 3 {
		t.Fatalf("Expected 3 keys, got %d", len(keys))
	}

	// Keys must be sorted alphabetically by Name.
	if keys[0].Name != "Apple" {
		t.Errorf("Expected first key 'Apple', got %q", keys[0].Name)
	}
	if keys[1].Name != "Mango" {
		t.Errorf("Expected second key 'Mango', got %q", keys[1].Name)
	}
	if keys[2].Name != "Zebra" {
		t.Errorf("Expected third key 'Zebra', got %q", keys[2].Name)
	}
}

func TestAnimalsByTypeMapOrderedKeysEmpty(t *testing.T) {
	m := AnimalsByTypeMap{}
	keys := m.OrderedKeys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys for empty map, got %d", len(keys))
	}
}

func TestAnimalByZoneMapOrderedKeys(t *testing.T) {
	m := AnimalByZoneMap{
		{ID: "1", Name: "Zone C"}: Animals{},
		{ID: "2", Name: "Zone A"}: Animals{},
		{ID: "3", Name: "Zone B"}: Animals{},
	}

	keys := m.OrderedKeys()
	if len(keys) != 3 {
		t.Fatalf("Expected 3 keys, got %d", len(keys))
	}

	if keys[0].Name != "Zone A" {
		t.Errorf("Expected first key 'Zone A', got %q", keys[0].Name)
	}
	if keys[1].Name != "Zone B" {
		t.Errorf("Expected second key 'Zone B', got %q", keys[1].Name)
	}
	if keys[2].Name != "Zone C" {
		t.Errorf("Expected third key 'Zone C', got %q", keys[2].Name)
	}
}

func TestAnimalYearNumberFormatted(t *testing.T) {
	a := Animal{Year: 2023, YearNumber: 42}
	got := a.YearNumberFormatted()
	want := "42/23"
	if got != want {
		t.Errorf("YearNumberFormatted() = %q, want %q", got, want)
	}
}

func TestAnimalYearNumberFormattedCentury(t *testing.T) {
	// %d does not zero-pad: year 2000 yields "0" (not "00") as the modulo.
	a := Animal{Year: 2000, YearNumber: 7}
	got := a.YearNumberFormatted()
	want := "7/0"
	if got != want {
		t.Errorf("YearNumberFormatted() = %q, want %q", got, want)
	}
}

func TestAnimalZoneAsString(t *testing.T) {
	a := Animal{Zone: nulls.NewString("Aviary")}
	if got := a.ZoneAsString(); got != "Aviary" {
		t.Errorf("ZoneAsString() = %q, want %q", got, "Aviary")
	}
}

func TestAnimalZoneAsStringInvalid(t *testing.T) {
	a := Animal{Zone: nulls.String{}}
	if got := a.ZoneAsString(); got != "" {
		t.Errorf("ZoneAsString() with invalid zone = %q, want empty", got)
	}
}

func TestAnimalString(t *testing.T) {
	a := Animal{ID: 1, Year: 2023, YearNumber: 5}
	if s := a.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestAnimalsString(t *testing.T) {
	as := Animals{{ID: 1}, {ID: 2}}
	if s := as.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestAnimalFeedingStartFmtDefault(t *testing.T) {
	a := Animal{FeedingStart: nulls.Time{}}
	if got := a.FeedingStartFmt(); got != DEF_FEEDING_START {
		t.Errorf("FeedingStartFmt() = %q, want default %q", got, DEF_FEEDING_START)
	}
}

func TestAnimalFeedingStartFmtSet(t *testing.T) {
	a := Animal{FeedingStart: nulls.NewTime(time.Date(2023, 1, 2, 9, 30, 0, 0, time.UTC))}
	if got := a.FeedingStartFmt(); got != "09:30" {
		t.Errorf("FeedingStartFmt() = %q, want %q", got, "09:30")
	}
}

func TestAnimalFeedingEndFmtDefault(t *testing.T) {
	a := Animal{FeedingEnd: nulls.Time{}}
	if got := a.FeedingEndFmt(); got != DEF_FEEDING_END {
		t.Errorf("FeedingEndFmt() = %q, want default %q", got, DEF_FEEDING_END)
	}
}

func TestAnimalFeedingEndFmtSet(t *testing.T) {
	a := Animal{FeedingEnd: nulls.NewTime(time.Date(2023, 1, 2, 18, 45, 0, 0, time.UTC))}
	if got := a.FeedingEndFmt(); got != "18:45" {
		t.Errorf("FeedingEndFmt() = %q, want %q", got, "18:45")
	}
}

func TestAnimalFeedingPeriodHourMinute(t *testing.T) {
	cases := []struct {
		period int
		want   string
	}{
		{0, "00:00"},
		{60, "01:00"},
		{90, "01:30"},
		{125, "02:05"},
		{1440, "24:00"},
	}
	for _, c := range cases {
		a := Animal{FeedingPeriod: c.period}
		got := a.FeedingPeriodHourMinute()
		if got != c.want {
			t.Errorf("FeedingPeriodHourMinute(%d) = %q, want %q", c.period, got, c.want)
		}
	}
}

func TestAnimalLastWeightNoCares(t *testing.T) {
	a := Animal{}
	w := a.LastWeight()
	if w.Valid {
		t.Errorf("Expected invalid weight with no cares, got %+v", w)
	}
}

func TestAnimalLastWeightSingleCare(t *testing.T) {
	a := Animal{Cares: []Care{
		{Date: time.Date(2023, 1, 10, 0, 0, 0, 0, time.UTC), Weight: nulls.NewString("250")},
	}}
	w := a.LastWeight()
	if !w.Valid || w.Int != 250 {
		t.Errorf("Expected valid weight 250, got %+v", w)
	}
}

func TestAnimalLastWeightPicksLatest(t *testing.T) {
	a := Animal{Cares: []Care{
		{Date: time.Date(2023, 1, 10, 0, 0, 0, 0, time.UTC), Weight: nulls.NewString("100")},
		{Date: time.Date(2023, 2, 15, 0, 0, 0, 0, time.UTC), Weight: nulls.NewString("300")},
		{Date: time.Date(2023, 1, 5, 0, 0, 0, 0, time.UTC), Weight: nulls.NewString("200")},
	}}
	w := a.LastWeight()
	if !w.Valid || w.Int != 300 {
		t.Errorf("Expected latest weight 300, got %+v", w)
	}
}

func TestAnimalLastWeightEmptyWeightString(t *testing.T) {
	// When a later care has an empty weight and earlier care has a valid weight,
	// the earlier valid weight is retained (strings.TrimSpace length check).
	a := Animal{Cares: []Care{
		{Date: time.Date(2023, 1, 10, 0, 0, 0, 0, time.UTC), Weight: nulls.NewString("150")},
		{Date: time.Date(2023, 2, 15, 0, 0, 0, 0, time.UTC), Weight: nulls.NewString("")},
	}}
	w := a.LastWeight()
	if !w.Valid || w.Int != 150 {
		t.Errorf("Expected retained weight 150, got %+v", w)
	}
}

func TestAnimalLastWeightNonNumeric(t *testing.T) {
	a := Animal{Cares: []Care{
		{Date: time.Date(2023, 1, 10, 0, 0, 0, 0, time.UTC), Weight: nulls.NewString("abc")},
	}}
	w := a.LastWeight()
	if w.Valid {
		t.Errorf("Expected invalid weight for non-numeric string, got %+v", w)
	}
}
