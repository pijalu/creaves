package models

import (
	"testing"
)

var validCareAnimalPlan = CareAnimalPlan{
	AnimalID:      472,
	Name:          "Pansement patte G — 2x/j",
	ActionKind:    "care",
	ActionPayload: []byte(`{"caretype_id":"3f6a1b2c-0000-4000-8000-000000000002","note":"Changer bandage"}`),
	Schedule:      []byte(`{"times":["08:00","20:00"],"anchor":"intake","duration_days":5}`),
	Active:        true,
}

func TestCareAnimalPlanValidateValid(t *testing.T) {
	p := validCareAnimalPlan
	if verrs, err := p.Validate(nil); verrs.HasAny() || err != nil {
		t.Fatalf("expected valid plan, got %v / %v", verrs, err)
	}
}

func TestCareAnimalPlanValidateRequired(t *testing.T) {
	p := validCareAnimalPlan
	p.AnimalID = 0
	if verrs, _ := p.Validate(nil); verrs.Get("animal_id") == nil {
		t.Error("expected error for zero AnimalID")
	}
	p = validCareAnimalPlan
	p.Name = ""
	if verrs, _ := p.Validate(nil); verrs.Get("name") == nil {
		t.Error("expected error for blank Name")
	}
	p = validCareAnimalPlan
	p.ActionKind = ""
	if verrs, _ := p.Validate(nil); verrs.Get("action_kind") == nil {
		t.Error("expected error for blank ActionKind")
	}
}

func TestCareAnimalPlanValidatePayloadParity(t *testing.T) {
	// same payload schema as rules (§4.7): feeding without caretype_id fails
	p := validCareAnimalPlan
	p.ActionKind = "feeding"
	p.ActionPayload = []byte(`{"food":"graines"}`)
	if verrs, _ := p.Validate(nil); verrs.Get("action_payload") == nil {
		t.Error("expected error for feeding payload without caretype_id")
	}
	// observation payload with prompt passes (§4.2)
	p.ActionKind = "observation"
	p.ActionPayload = []byte(`{"prompt":"Mange seul ?","alert_on":"no"}`)
	if verrs, err := p.Validate(nil); verrs.HasAny() || err != nil {
		t.Errorf("expected valid observation plan, got %v / %v", verrs, err)
	}
	// invalid alert_on value fails
	p.ActionPayload = []byte(`{"prompt":"Mange seul ?","alert_on":"maybe"}`)
	if verrs, _ := p.Validate(nil); verrs.Get("action_payload") == nil {
		t.Error("expected error for invalid alert_on")
	}
}

func TestCareAnimalPlanValidateScheduleParity(t *testing.T) {
	// same schedule schema as rules (§4.7): bad slot time fails
	p := validCareAnimalPlan
	p.Schedule = []byte(`{"times":["25:00"]}`)
	if verrs, _ := p.Validate(nil); verrs.Get("schedule") == nil {
		t.Error("expected error for out-of-range time slot")
	}
	// weekdays restriction passes
	p = validCareAnimalPlan
	p.Schedule = []byte(`{"times":["07:00"],"weekdays":[1,2,3,4,5]}`)
	if verrs, err := p.Validate(nil); verrs.HasAny() || err != nil {
		t.Errorf("expected valid weekdays schedule, got %v / %v", verrs, err)
	}
}

func TestCareAnimalPlanString(t *testing.T) {
	p := validCareAnimalPlan
	if s := p.String(); s == "" {
		t.Error("expected non-empty string representation")
	}
	if n := len(CareAnimalPlans{p}); n != 1 {
		t.Errorf("expected collection of 1, got %d", n)
	}
}
