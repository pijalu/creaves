package models

import (
	"strconv"
	"testing"

	"github.com/gofrs/uuid"
)

// TestLogAnimalAuditPersists proves LogAnimalAudit writes a complete,
// queryable audit row (user name, entity, action, change summary).
// Rows are cleaned up after the test.
func TestLogAnimalAuditPersists(t *testing.T) {
	if DB == nil {
		t.Skip("no database connection")
	}

	animalID := 999001
	defer func() {
		if err := DB.RawQuery("DELETE FROM animal_audits WHERE animal_id = ?", animalID).Exec(); err != nil {
			t.Logf("cleanup failed: %v", err)
		}
	}()

	user := &User{Login: "audit-test-user"}
	err := LogAnimalAudit(DB, user, animalID, AuditEntityCare, "42", AuditActionUpdate, "weight: 100 -> 120")
	if err != nil {
		t.Fatalf("LogAnimalAudit failed: %v", err)
	}

	got := &AnimalAudit{}
	if err := DB.Where("animal_id = ?", animalID).First(got); err != nil {
		t.Fatalf("audit row not found: %v", err)
	}

	if got.UserName != "audit-test-user" {
		t.Errorf("UserName = %q, want %q", got.UserName, "audit-test-user")
	}
	if got.Entity != AuditEntityCare {
		t.Errorf("Entity = %q, want %q", got.Entity, AuditEntityCare)
	}
	if got.EntityID != "42" {
		t.Errorf("EntityID = %q, want %q", got.EntityID, "42")
	}
	if got.Action != AuditActionUpdate {
		t.Errorf("Action = %q, want %q", got.Action, AuditActionUpdate)
	}
	if got.Changes != "weight: 100 -> 120" {
		t.Errorf("Changes = %q, want %q", got.Changes, "weight: 100 -> 120")
	}
	if got.ID == uuid.Nil {
		t.Error("expected non-zero ID")
	}
}

// TestLogAnimalAuditSystemUser proves a nil user produces a "system" entry.
func TestLogAnimalAuditSystemUser(t *testing.T) {
	if DB == nil {
		t.Skip("no database connection")
	}

	animalID := 999002
	defer func() {
		if err := DB.RawQuery("DELETE FROM animal_audits WHERE animal_id = ?", animalID).Exec(); err != nil {
			t.Logf("cleanup failed: %v", err)
		}
	}()

	if err := LogAnimalAudit(DB, nil, animalID, AuditEntityAnimal, strconv.Itoa(animalID), AuditActionCreate, ""); err != nil {
		t.Fatalf("LogAnimalAudit failed: %v", err)
	}

	got := &AnimalAudit{}
	if err := DB.Where("animal_id = ?", animalID).First(got); err != nil {
		t.Fatalf("audit row not found: %v", err)
	}
	if got.UserName != "system" {
		t.Errorf("UserName = %q, want %q", got.UserName, "system")
	}
}
