package models

import (
	"testing"
	"time"
)

func TestTreatmentIsToday(t *testing.T) {
	now := time.Now()
	treatement := Treatment{
		Date: now,
	}
	if treatement.IsPast() {
		t.Fatal("treatment in the past")
	}
	if treatement.IsFuture() {
		t.Fatalf("treatment is future")
	}
	if !treatement.IsToday() {
		t.Fatalf("treatment is not today")
	}
}
