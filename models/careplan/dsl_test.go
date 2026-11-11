package careplan

import (
	"strings"
	"testing"
)

// parseValid asserts expr parses+validates and returns the AST.
func parseValid(t *testing.T, expr string) Node {
	t.Helper()
	ast, err := Parse(expr)
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", expr, err)
	}
	return ast
}

func TestDSLGrammarBasics(t *testing.T) {
	cases := []string{
		`animal_type = "Hérissons / Insectivore"`,
		`animal_type = "Hérissons / Insectivore" AND animal_age = "bébé" AND weight_g < 300`,
		`has_parasites = true AND parasites ~ "(?i)tiques"`,
		`parasites !~ "(?i)puces"`,
		`animal_type = "Colombidés" OR species IN ("Pigeon biset", "Tourterelle turque", "Ramier")`,
		`weight_g BETWEEN 300 AND 800 AND NOT force_feed = true`,
		`days_in_care >= 7 AND (has_wounds = true OR wounds ~ "(?i)plaie")`,
		// keywords are case-insensitive
		`animal_age = "bébé" and weight_g < 300 or not force_feed = true`,
		// parenthesised grouping
		`(species = "A" OR species = "B") AND weight_g <= 500`,
		// IN with single element
		`cage IN ("A12")`,
	}
	for _, c := range cases {
		parseValid(t, c)
	}
}

func TestDSLPrecedence(t *testing.T) {
	// NOT > AND > OR: a OR b AND c  =>  a OR (b AND c)
	ast := parseValid(t, `weight_g > 100 OR animal_age = "bébé" AND force_feed = true`)
	or, ok := ast.(*OrNode)
	if !ok {
		t.Fatalf("expected OrNode at root, got %T", ast)
	}
	if _, ok := or.Left.(*PredicateNode); !ok {
		t.Fatalf("OR left must be a predicate (NOT>AND>OR), got %T", or.Left)
	}
	if _, ok := or.Right.(*AndNode); !ok {
		t.Fatalf("OR right must be an AndNode (AND binds tighter), got %T", or.Right)
	}

	// NOT binds tighter than AND: NOT a AND b => (NOT a) AND b
	ast = parseValid(t, `NOT force_feed = true AND animal_age = "bébé"`)
	and, ok := ast.(*AndNode)
	if !ok {
		t.Fatalf("expected AndNode, got %T", ast)
	}
	if _, ok := and.Left.(*NotNode); !ok {
		t.Fatalf("AND left must be NotNode, got %T", and.Left)
	}
}

func TestDSLBetweenConsumesOwnAnd(t *testing.T) {
	// BETWEEN x AND y AND z: the first AND belongs to BETWEEN.
	ast := parseValid(t, `weight_g BETWEEN 300 AND 800 AND animal_age = "bébé"`)
	and, ok := ast.(*AndNode)
	if !ok {
		t.Fatalf("expected AndNode, got %T", ast)
	}
	bw, ok := and.Left.(*BetweenNode)
	if !ok {
		t.Fatalf("expected BetweenNode on the left, got %T", and.Left)
	}
	if bw.Min != 300 || bw.Max != 800 {
		t.Fatalf("between bounds = %v..%v, want 300..800", bw.Min, bw.Max)
	}
	if _, ok := and.Right.(*PredicateNode); !ok {
		t.Fatalf("the second AND must continue the conjunction, got %T", and.Right)
	}
}

func TestDSLSyntaxErrors(t *testing.T) {
	cases := []struct {
		expr   string
		errSub string
	}{
		{`animal_type =`, "unexpected end"},
		{`AND animal_type = "x"`, "unexpected token"},
		{`animal_type = "x" AND`, "unexpected end"},
		{`(animal_age = "bébé"`, "expected )"},
		{`animal_type = "x" foo`, "unexpected token"},
		{`weight_g BETWEEN 300`, "expected AND"},
		{`species IN ("a" "b")`, "expected ,"},
		{`weight_g ~ 300`, "regex patterns must be double-quoted strings"},
	}
	for _, c := range cases {
		_, err := Parse(c.expr)
		if err == nil {
			t.Errorf("Parse(%q): expected error containing %q, got none", c.expr, c.errSub)
			continue
		}
		if !strings.Contains(err.Error(), c.errSub) {
			t.Errorf("Parse(%q): error %q does not mention %q", c.expr, err, c.errSub)
		}
	}
}

func TestDSLSemanticErrors(t *testing.T) {
	reg := NewRegistry()
	reg.Register(FieldProvider{Key: "weight_g", LabelKey: "weight_g", Type: TypeNumber, Ops: []string{OpLt, OpGt}})
	reg.Register(FieldProvider{Key: "species", LabelKey: "species", Type: TypeString, Ops: []string{OpEq, OpIn}})
	reg.Register(FieldProvider{Key: "force_feed", LabelKey: "force_feed", Type: TypeBool, Ops: []string{OpEq}})

	cases := []struct {
		expr   string
		errSub string
		col    int
	}{
		// unknown field
		{`nope = "x"`, `unknown field "nope"`, 1},
		// op not declared for field
		{`weight_g = 300`, `does not allow "="`, 10},
		// type mismatch: number literal for string field
		{`species = 42`, `expects a string literal`, 11},
		// type mismatch: string literal for number field (spec example: col of literal)
		// `"abc"` starts at col 12 (`weight_g`=1-8, space=9, `<`=10, space=11, `"`=12).
		{`weight_g < "abc"`, `expects a number literal`, 12},
		// bool field with string literal
		{`force_feed = "yes"`, `expects a boolean literal`, 14},
		// IN on number field
		{`weight_g IN ("1")`, `does not allow "IN"`, 10},
	}
	for _, c := range cases {
		_, err := ParseValidatedWith(c.expr, reg)
		if err == nil {
			t.Errorf("ParseValidated(%q): expected error containing %q", c.expr, c.errSub)
			continue
		}
		if !strings.Contains(err.Error(), c.errSub) {
			t.Errorf("ParseValidated(%q): error %q missing %q", c.expr, err, c.errSub)
		}
		pe, ok := err.(*ParseError)
		if !ok {
			t.Errorf("ParseValidated(%q): error is not *ParseError: %T", c.expr, err)
			continue
		}
		if pe.Column != c.col {
			t.Errorf("ParseValidated(%q): error column = %d, want %d (%v)", c.expr, pe.Column, c.col, err)
		}
	}
}

func TestDSLValidatedOK(t *testing.T) {
	reg := DefaultRegistry()
	if _, err := ParseValidatedWith(`has_parasites = true AND parasites ~ "(?i)tiques"`, reg); err != nil {
		t.Fatalf("valid expression rejected: %v", err)
	}
	if _, err := ParseValidatedWith(`animal_type = "Hérissons / Insectivore" AND weight_g BETWEEN 200 AND 300`, reg); err != nil {
		t.Fatalf("valid BETWEEN rejected: %v", err)
	}
}

func TestDSLStringEscapes(t *testing.T) {
	ast := parseValid(t, `intake_remarks = "a \"quoted\" value"`)
	p := ast.(*PredicateNode)
	if p.Literal.Str != `a "quoted" value` {
		t.Fatalf("escape handling broken: %q", p.Literal.Str)
	}
}

// TestDSLCaseInsensitiveOperators (bugs.md U26/U27 R5-1a): the explicit CI
// operators =*, !=* and INCI(...) parse, keywords case-insensitively, and
// carry the exact DSL spelling on the node.
func TestDSLCaseInsensitiveOperators(t *testing.T) {
	ast := parseValid(t, `cage =* "Enclos renards"`)
	if p := ast.(*PredicateNode); p.Op != OpEqCI {
		t.Fatalf("=* parsed as op %q", p.Op)
	}
	ast = parseValid(t, `cage !=* "VE5"`)
	if p := ast.(*PredicateNode); p.Op != OpNeqCI {
		t.Fatalf("!=* parsed as op %q", p.Op)
	}
	// INCI keyword is case-insensitive like IN/CONTAINS.
	for _, expr := range []string{`zone INCI ("A", "b")`, `zone inci ("A", "b")`, `zone Inci ("A")`} {
		ast = parseValid(t, expr)
		in := ast.(*InNode)
		if in.Op != OpInCI {
			t.Fatalf("%s: parsed as op %q", expr, in.Op)
		}
		if len(in.Literals) == 0 {
			t.Fatalf("%s: literals lost", expr)
		}
	}
	// INCI () parses (validation, not the grammar, rejects empty lists).
	parseValid(t, `zone INCI ()`)
	// CS twins unchanged.
	ast = parseValid(t, `cage = "A12"`)
	if p := ast.(*PredicateNode); p.Op != OpEq {
		t.Fatalf("CS = broken: op %q", p.Op)
	}
}

// TestDSLCaseInsensitiveValidation: CI ops validate like their CS twins —
// allowed on string fields that allow eq/neq/in, rejected elsewhere with
// the exact DSL spelling quoted at the operator column.
func TestDSLCaseInsensitiveValidation(t *testing.T) {
	reg := DefaultRegistry()
	// !=* is only legal where the plain != already is (species, not cage:
	// NOT cage =* covers negation for eq-only fields).
	for _, expr := range []string{
		`species !=* "Renard Roux"`,
		`cage =* "Enclos renards"`,
		`cage INCI ("Enclos Renards", "enclos renards")`,
		`species =* "renard roux" AND zone INCI ("salle 1")`,
	} {
		if _, err := ParseValidatedWith(expr, reg); err != nil {
			t.Errorf("ParseValidated(%q) rejected: %v", expr, err)
		}
	}
	cases := []struct {
		expr   string
		errSub string
		col    int
	}{
		// cage allows eq/in but not neq — so no CI neq either
		{`cage !=* "VE5"`, `does not allow "!=*"`, 6},
		// free-text fields allow neither eq nor CI eq
		{`parasites =* "x"`, `does not allow "=*"`, 11},
		// numbers: CI is meaningless
		{`weight_g =* 3`, `does not allow "=*"`, 10},
		// bools: eq only, CS
		{`force_feed =* true`, `does not allow "=*"`, 12},
		// INCI literal type must match the field
		{`cage INCI (3)`, `expects a string literal`, 12},
		// empty list rejected like IN ()
		{`cage INCI ()`, `needs at least one value`, 6},
	}
	for _, c := range cases {
		_, err := ParseValidatedWith(c.expr, reg)
		if err == nil {
			t.Errorf("ParseValidated(%q): expected error containing %q", c.expr, c.errSub)
			continue
		}
		if !strings.Contains(err.Error(), c.errSub) {
			t.Errorf("ParseValidated(%q): error %q missing %q", c.expr, err.Error(), c.errSub)
		}
		if pe, ok := err.(*ParseError); ok && pe.Column != c.col {
			t.Errorf("ParseValidated(%q): column = %d, want %d", c.expr, pe.Column, c.col)
		}
	}
}
