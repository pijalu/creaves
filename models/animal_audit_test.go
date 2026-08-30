package models

import (
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

func TestAnimalAuditString(t *testing.T) {
	a := AnimalAudit{Entity: "care", Action: "update"}
	if s := a.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestAnimalAuditsString(t *testing.T) {
	as := AnimalAudits{{Entity: "care"}, {Entity: "animal"}}
	if s := as.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestAnimalAuditCreatedAtFormated(t *testing.T) {
	a := AnimalAudit{CreatedAt: time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)}
	got := a.CreatedAtFormated()
	want := "2023/06/15 10:30"
	if got != want {
		t.Errorf("CreatedAtFormated() = %q, want %q", got, want)
	}
}

type auditSample struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
	Note   string  `json:"note"`
	Clean  bool    `json:"clean"`
}

func TestComputeChangesNoChange(t *testing.T) {
	old := auditSample{Name: "a", Weight: 1, Note: "n", Clean: true}
	new := auditSample{Name: "a", Weight: 1, Note: "n", Clean: true}
	if got := ComputeChanges(old, new); got != "" {
		t.Errorf("ComputeChanges() = %q, want empty", got)
	}
}

func TestComputeChangesSingleField(t *testing.T) {
	old := auditSample{Name: "a", Weight: 1, Note: "n", Clean: false}
	new := auditSample{Name: "b", Weight: 1, Note: "n", Clean: false}
	got := ComputeChanges(old, new)
	want := "name: a -> b"
	if got != want {
		t.Errorf("ComputeChanges() = %q, want %q", got, want)
	}
}

func TestComputeChangesMultipleFieldsSorted(t *testing.T) {
	old := auditSample{Name: "a", Weight: 1, Note: "n", Clean: false}
	new := auditSample{Name: "b", Weight: 2, Note: "n", Clean: true}
	got := ComputeChanges(old, new)
	// Fields must be alphabetically sorted: clean, name, weight.
	want := "clean: false -> true; name: a -> b; weight: 1 -> 2"
	if got != want {
		t.Errorf("ComputeChanges() = %q, want %q", got, want)
	}
}

func TestComputeChangesEmptyStringMarkers(t *testing.T) {
	old := auditSample{Name: "a", Weight: 1, Note: "x", Clean: false}
	new := auditSample{Name: "a", Weight: 1, Note: "", Clean: false}
	got := ComputeChanges(old, new)
	want := "note: x -> <empty>"
	if got != want {
		t.Errorf("ComputeChanges() = %q, want %q", got, want)
	}
}

func TestComputeChangesCreation(t *testing.T) {
	new := auditSample{Name: "a", Weight: 1, Note: "", Clean: false}
	got := ComputeChanges(nil, new)
	// All fields appear as new values, sorted alphabetically.
	want := "clean: <none> -> false; name: <none> -> a; note: <none> -> <empty>; weight: <none> -> 1"
	if got != want {
		t.Errorf("ComputeChanges() = %q, want %q", got, want)
	}
}

func TestComputeChangesDeletion(t *testing.T) {
	old := auditSample{Name: "a", Weight: 1, Note: "", Clean: false}
	got := ComputeChanges(old, nil)
	want := "clean: false -> <deleted>; name: a -> <deleted>; note: <empty> -> <deleted>; weight: 1 -> <deleted>"
	if got != want {
		t.Errorf("ComputeChanges() = %q, want %q", got, want)
	}
}

func TestComputeChangesNilBoth(t *testing.T) {
	if got := ComputeChanges(nil, nil); got != "" {
		t.Errorf("ComputeChanges(nil, nil) = %q, want empty", got)
	}
}

func TestComputeChangesIgnoresTimestamps(t *testing.T) {
	type rec struct {
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Name      string    `json:"name"`
	}
	old := rec{CreatedAt: time.Now(), UpdatedAt: time.Now(), Name: "a"}
	new := rec{CreatedAt: time.Now().Add(time.Hour), UpdatedAt: time.Now().Add(time.Hour), Name: "a"}
	if got := ComputeChanges(old, new); got != "" {
		t.Errorf("ComputeChanges() = %q, want empty (timestamps ignored)", got)
	}
}

func TestLogAnimalAuditNilTx(t *testing.T) {
	err := LogAnimalAudit(nil, nil, 1, AuditEntityAnimal, "1", AuditActionUpdate, "")
	if err == nil {
		t.Error("Expected error for nil connection")
	}
}

func TestLogAnimalAuditZeroAnimalID(t *testing.T) {
	err := LogAnimalAudit(nil, nil, 0, AuditEntityAnimal, "1", AuditActionUpdate, "")
	if err == nil {
		t.Error("Expected error for zero animal ID")
	}
}

func TestAnimalAuditValidate(t *testing.T) {
	a := &AnimalAudit{}
	verrs, err := a.Validate(nil)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !verrs.HasAny() {
		t.Error("Expected validation errors for empty audit")
	}

	id, _ := uuid.NewV4()
	a = &AnimalAudit{
		AnimalID: 42,
		UserID:   nulls.NewUUID(id),
		UserName: "admin",
		Entity:   AuditEntityCare,
		Action:   AuditActionUpdate,
	}
	verrs, err = a.Validate(nil)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if verrs.HasAny() {
		t.Errorf("Expected no validation errors, got %v", verrs)
	}
}
