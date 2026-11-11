package models

import (
	"testing"
	"time"
)

var validCareRule = CareRule{
	Name:          "Hérisson bébé — gavage 5x/j",
	ActionKind:    "feeding",
	ActionPayload: []byte(`{"caretype_id":"3f6a1b2c-0000-4000-8000-000000000001","food":"Croquettes + 4 VDF","force_feed":true}`),
	Schedule:      []byte(`{"times":["07:00","09:30","12:30","15:30","19:00"],"every_days":1,"anchor":"intake","grace_minutes":45}`),
	Active:        true,
	Priority:      100,
}

func TestCareRuleValidateValid(t *testing.T) {
	r := validCareRule
	if verrs, err := r.Validate(nil); verrs.HasAny() || err != nil {
		t.Fatalf("expected valid rule, got %v / %v", verrs, err)
	}
}

func TestCareRuleValidateBlankFields(t *testing.T) {
	r := validCareRule
	r.Name = ""
	if verrs, _ := r.Validate(nil); !verrs.HasAny() {
		t.Error("expected error for blank Name")
	}
	r = validCareRule
	r.ActionKind = ""
	if verrs, _ := r.Validate(nil); verrs.Get("action_kind") == nil {
		t.Error("expected error for blank ActionKind")
	}
	r = validCareRule
	r.ActionPayload = nil
	if verrs, _ := r.Validate(nil); verrs.Get("action_payload") == nil {
		t.Error("expected error for empty ActionPayload")
	}
	r = validCareRule
	r.Schedule = nil
	if verrs, _ := r.Validate(nil); verrs.Get("schedule") == nil {
		t.Error("expected error for empty Schedule")
	}
}

func TestCareRuleValidateActionKind(t *testing.T) {
	r := validCareRule
	r.ActionKind = "grooming"
	if verrs, _ := r.Validate(nil); verrs.Get("action_kind") == nil {
		t.Error("expected error for unknown action kind")
	}
}

func TestCareRuleValidateActionPayloadPerKind(t *testing.T) {
	// medication without any dosage path must fail (§4.2)
	r := validCareRule
	r.ActionKind = "medication"
	r.ActionPayload = []byte(`{"drug":"Ivomec 1% (SC)"}`)
	if verrs, _ := r.Validate(nil); verrs.Get("action_payload") == nil {
		t.Error("expected error for medication payload without dosage")
	}
	// medication with a literal dosage passes
	r.ActionPayload = []byte(`{"drug":"Ivomec 1% (SC)","dosage":"0.1 ml / 100g"}`)
	if verrs, err := r.Validate(nil); verrs.HasAny() || err != nil {
		t.Errorf("expected valid medication rule, got %v / %v", verrs, err)
	}
	// feeding payload missing caretype_id must fail
	r = validCareRule
	r.ActionPayload = []byte(`{"food":"Croquettes"}`)
	if verrs, _ := r.Validate(nil); verrs.Get("action_payload") == nil {
		t.Error("expected error for feeding payload without caretype_id")
	}
	// unknown payload keys must fail (strict decoding)
	r = validCareRule
	r.ActionPayload = []byte(`{"caretype_id":"3f6a1b2c-0000-4000-8000-000000000001","bogus":1}`)
	if verrs, _ := r.Validate(nil); verrs.Get("action_payload") == nil {
		t.Error("expected error for unknown payload key")
	}
}

func TestCareRuleValidateSchedule(t *testing.T) {
	r := validCareRule
	r.Schedule = []byte(`{"times":["7am"]}`)
	if verrs, _ := r.Validate(nil); verrs.Get("schedule") == nil {
		t.Error("expected error for malformed time slot")
	}
	r = validCareRule
	r.Schedule = []byte(`{"times":["07:00"],"wat":1}`)
	if verrs, _ := r.Validate(nil); verrs.Get("schedule") == nil {
		t.Error("expected error for unknown schedule key")
	}
	r = validCareRule
	r.Schedule = []byte(`{"times":["07:00"],"duration_days":0}`)
	if verrs, _ := r.Validate(nil); verrs.Get("schedule") == nil {
		t.Error("expected error for non-positive duration_days")
	}
}

func TestCareRuleValidateWindowOrder(t *testing.T) {
	r := validCareRule
	from := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	r.ValidFrom = &from
	r.ValidTo = &to
	if verrs, _ := r.Validate(nil); verrs.Get("valid_to") == nil {
		t.Error("expected error when ValidTo is before ValidFrom")
	}
	r.ValidTo = &from
	if verrs, _ := r.Validate(nil); verrs.HasAny() {
		t.Errorf("expected equal window bounds to be valid, got %v", verrs)
	}
	// only one bound set — fine
	r.ValidTo = nil
	if verrs, _ := r.Validate(nil); verrs.HasAny() {
		t.Errorf("expected nil ValidTo to be valid, got %v", verrs)
	}
}

func TestCareRuleString(t *testing.T) {
	r := validCareRule
	if s := r.String(); s == "" {
		t.Error("expected non-empty string representation")
	}
	if n := len(CareRules{r, r}); n != 2 {
		t.Errorf("expected collection of 2, got %d", n)
	}
}
