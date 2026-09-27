package models

import (
	"strings"
	"testing"

	"github.com/gofrs/uuid"
)

func TestCareRuleExclusionValidateValid(t *testing.T) {
	e := CareRuleExclusion{
		RuleID:    uuid.Must(uuid.NewV4()),
		AnimalID:  42,
		Reason:    "recovering, handled separately",
		CreatedBy: uuid.Must(uuid.NewV4()),
	}
	if verrs, err := e.Validate(nil); verrs.HasAny() || err != nil {
		t.Fatalf("expected valid exclusion, got %v / %v", verrs, err)
	}
	// reason and created_by are optional
	e.Reason = ""
	e.CreatedBy = uuid.Nil
	if verrs, err := e.Validate(nil); verrs.HasAny() || err != nil {
		t.Fatalf("expected optional reason/created_by, got %v / %v", verrs, err)
	}
}

func TestCareRuleExclusionValidateRequired(t *testing.T) {
	e := CareRuleExclusion{}
	verrs, _ := e.Validate(nil)
	if verrs.Get("rule_id") == nil {
		t.Error("expected error for blank RuleID")
	}
	if verrs.Get("animal_id") == nil {
		t.Error("expected error for zero AnimalID")
	}
}

func TestCareRuleExclusionValidateReasonLength(t *testing.T) {
	e := CareRuleExclusion{
		RuleID:   uuid.Must(uuid.NewV4()),
		AnimalID: 1,
		Reason:   strings.Repeat("r", 500),
	}
	if verrs, _ := e.Validate(nil); verrs.Get("reason") != nil {
		t.Error("expected 500-char reason to be valid")
	}
	e.Reason = strings.Repeat("r", 501)
	if verrs, _ := e.Validate(nil); verrs.Get("reason") == nil {
		t.Error("expected error for reason longer than 500 chars")
	}
}

func TestCareRuleExclusionString(t *testing.T) {
	e := CareRuleExclusion{AnimalID: 7}
	if s := e.String(); s == "" {
		t.Error("expected non-empty string representation")
	}
	if n := len(CareRuleExclusions{e}); n != 1 {
		t.Errorf("expected collection of 1, got %d", n)
	}
}
