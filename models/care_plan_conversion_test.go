package models

import (
	"testing"
	"time"
)

func TestCarePlanConversionValidateValid(t *testing.T) {
	c := CarePlanConversion{
		Key:        "startup_v1",
		FinishedAt: time.Date(2026, 9, 26, 6, 0, 0, 0, time.Local),
	}
	if verrs, err := c.Validate(nil); verrs.HasAny() || err != nil {
		t.Fatalf("expected valid conversion marker, got %v / %v", verrs, err)
	}
}

func TestCarePlanConversionValidateRequired(t *testing.T) {
	c := CarePlanConversion{}
	verrs, _ := c.Validate(nil)
	if verrs.Get("key") == nil {
		t.Error("expected error for blank Key")
	}
	if verrs.Get("finished_at") == nil {
		t.Error("expected error for zero FinishedAt")
	}
}

func TestCarePlanConversionString(t *testing.T) {
	c := CarePlanConversion{Key: "startup_v1"}
	if s := c.String(); s == "" {
		t.Error("expected non-empty string representation")
	}
	if n := len(CarePlanConversions{c}); n != 1 {
		t.Errorf("expected collection of 1, got %d", n)
	}
}
