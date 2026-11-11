package models

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
)

var (
	appDue     = time.Date(2026, 10, 1, 7, 0, 0, 0, time.Local)
	appApplied = time.Date(2026, 10, 1, 7, 3, 0, 0, time.Local)
)

func validApplication() CarePlanApplication {
	return CarePlanApplication{
		SourceType:      ApplicationSourceRule,
		SourceID:        uuid.Must(uuid.NewV4()),
		SourceSnapshot:  []byte(`{"name":"Hérisson bébé — gavage","action_kind":"feeding"}`),
		AnimalID:        472,
		DueAt:           appDue,
		AppliedAt:       appApplied,
		UserID:          uuid.Must(uuid.NewV4()),
		FulfillmentType: ApplicationFulfillmentCare,
		FulfillmentID:   uuid.Must(uuid.NewV4()).String(),
		Status:          ApplicationStatusApplied,
	}
}

func TestCarePlanApplicationValidateValid(t *testing.T) {
	a := validApplication()
	if verrs, err := a.Validate(nil); verrs.HasAny() || err != nil {
		t.Fatalf("expected valid application, got %v / %v", verrs, err)
	}
	// note is optional for applied (§10-CP4 only mandates it for skip/defer)
}

func TestCarePlanApplicationValidateInclusions(t *testing.T) {
	a := validApplication()
	a.SourceType = "template"
	if verrs, _ := a.Validate(nil); verrs.Get("source_type") == nil {
		t.Error("expected error for invalid source_type")
	}
	a = validApplication()
	a.FulfillmentType = "email"
	if verrs, _ := a.Validate(nil); verrs.Get("fulfillment_type") == nil {
		t.Error("expected error for invalid fulfillment_type")
	}
	a = validApplication()
	a.Status = "postponed"
	if verrs, _ := a.Validate(nil); verrs.Get("status") == nil {
		t.Error("expected error for invalid status")
	}
}

func TestCarePlanApplicationValidateRequired(t *testing.T) {
	a := validApplication()
	a.SourceID = uuid.Nil
	if verrs, _ := a.Validate(nil); verrs.Get("source_id") == nil {
		t.Error("expected error for blank SourceID")
	}
	a = validApplication()
	a.UserID = uuid.Nil
	if verrs, _ := a.Validate(nil); verrs.Get("user_id") == nil {
		t.Error("expected error for blank UserID")
	}
	a = validApplication()
	a.AnimalID = 0
	if verrs, _ := a.Validate(nil); verrs.Get("animal_id") == nil {
		t.Error("expected error for zero AnimalID")
	}
	a = validApplication()
	a.FulfillmentID = ""
	if verrs, _ := a.Validate(nil); verrs.Get("fulfillment_id") == nil {
		t.Error("expected error for blank FulfillmentID")
	}
	a = validApplication()
	a.SourceSnapshot = nil
	if verrs, _ := a.Validate(nil); verrs.Get("source_snapshot") == nil {
		t.Error("expected error for empty SourceSnapshot")
	}
	a = validApplication()
	a.DueAt = time.Time{}
	if verrs, _ := a.Validate(nil); verrs.Get("due_at") == nil {
		t.Error("expected error for zero DueAt")
	}
	a = validApplication()
	a.AppliedAt = time.Time{}
	if verrs, _ := a.Validate(nil); verrs.Get("applied_at") == nil {
		t.Error("expected error for zero AppliedAt")
	}
}

func TestCarePlanApplicationValidateSkipped(t *testing.T) {
	a := validApplication()
	a.Status = ApplicationStatusSkipped
	if verrs, _ := a.Validate(nil); verrs.Get("note") == nil {
		t.Error("expected mandatory note for skipped application (§10-CP4)")
	}
	a.Note = "food not touched"
	if verrs, _ := a.Validate(nil); verrs.HasAny() {
		t.Errorf("expected valid skip with reason, got %v", verrs)
	}
}

func TestCarePlanApplicationValidateDeferred(t *testing.T) {
	a := validApplication()
	a.Status = ApplicationStatusDeferred
	a.Note = "caretaker busy"
	until := appDue.Add(2 * time.Hour)
	a.DeferredUntil = &until
	if verrs, _ := a.Validate(nil); verrs.HasAny() {
		t.Fatalf("expected valid defer, got %v", verrs)
	}

	// defer without deferred_until fails (§10-A2)
	a.DeferredUntil = nil
	if verrs, _ := a.Validate(nil); verrs.Get("deferred_until") == nil {
		t.Error("expected error for defer without DeferredUntil")
	}

	// defer without note fails
	until2 := appDue.Add(2 * time.Hour)
	a.DeferredUntil = &until2
	a.Note = ""
	if verrs, _ := a.Validate(nil); verrs.Get("note") == nil {
		t.Error("expected error for defer without note")
	}
}

func TestCarePlanApplicationValidateDeferredOnlyOnDeferred(t *testing.T) {
	a := validApplication()
	until := appDue.Add(2 * time.Hour)
	a.DeferredUntil = &until
	if verrs, _ := a.Validate(nil); verrs.Get("deferred_until") == nil {
		t.Error("expected error for applied application carrying DeferredUntil (§10-A2)")
	}
	// skipped must also reject it
	a.Status = ApplicationStatusSkipped
	a.Note = "reason"
	if verrs, _ := a.Validate(nil); verrs.Get("deferred_until") == nil {
		t.Error("expected error for skipped application carrying DeferredUntil")
	}
}

func TestCarePlanApplicationString(t *testing.T) {
	a := validApplication()
	if s := a.String(); s == "" {
		t.Error("expected non-empty string representation")
	}
	if n := len(CarePlanApplications{a}); n != 1 {
		t.Errorf("expected collection of 1, got %d", n)
	}
}
