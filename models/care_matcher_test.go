package models

import (
	"strings"
	"testing"

	"github.com/gobuffalo/nulls"
)

const validMatcherExpr = `animal_type = "Hérissons / Insectivore" AND animal_age = "bébé" AND weight_g < 300`

func TestCareMatcherValidateValid(t *testing.T) {
	m := CareMatcher{
		Name:        "Hérisson bébé < 300 g",
		Expression:  validMatcherExpr,
		Description: nulls.NewString("shared by R1/R1b"),
	}
	if verrs, err := m.Validate(nil); verrs.HasAny() || err != nil {
		t.Fatalf("expected valid matcher, got %v / %v", verrs, err)
	}
}

func TestCareMatcherValidateBlankFields(t *testing.T) {
	m := CareMatcher{Name: "", Expression: validMatcherExpr}
	if verrs, _ := m.Validate(nil); verrs.Get("name") == nil {
		t.Error("expected error for blank Name")
	}
	m = CareMatcher{Name: "M1", Expression: ""}
	if verrs, _ := m.Validate(nil); verrs.Get("expression") == nil {
		t.Error("expected error for blank Expression")
	}
}

func TestCareMatcherValidateNameLength(t *testing.T) {
	m := CareMatcher{Name: strings.Repeat("x", 201), Expression: validMatcherExpr}
	if verrs, _ := m.Validate(nil); verrs.Get("name") == nil {
		t.Error("expected error for name longer than 200 chars")
	}
	m = CareMatcher{Name: strings.Repeat("x", 200), Expression: validMatcherExpr}
	if verrs, _ := m.Validate(nil); verrs.HasAny() {
		t.Errorf("expected 200-char name to be valid, got %v", verrs)
	}
}

func TestCareMatcherValidateExpressionSyntax(t *testing.T) {
	m := CareMatcher{Name: "broken", Expression: `animal_type =`}
	if verrs, _ := m.Validate(nil); verrs.Get("expression") == nil {
		t.Error("expected syntax error to fail validation")
	}
}

func TestCareMatcherValidateExpressionSemantics(t *testing.T) {
	// parses grammatically but references an unknown field — the spec
	// requires semantic validation at save (§4.4)
	m := CareMatcher{Name: "bad field", Expression: `not_a_field = "x"`}
	if verrs, _ := m.Validate(nil); verrs.Get("expression") == nil {
		t.Error("expected unknown field to fail semantic validation")
	}
}

func TestCareMatcherString(t *testing.T) {
	m := CareMatcher{Name: "M1"}
	if s := m.String(); s == "" {
		t.Error("expected non-empty string representation")
	}
	if n := len(CareMatchers{m}); n != 1 {
		t.Errorf("expected collection of 1, got %d", n)
	}
}
