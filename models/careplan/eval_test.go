package careplan

import "testing"

// parseForEval parses+validates expr against the default registry (§5.1).
func parseForEval(t *testing.T, expr string) Node {
	t.Helper()
	n, err := ParseValidatedWith(expr, DefaultRegistry())
	if err != nil {
		t.Fatalf("ParseValidatedWith(%q): %v", expr, err)
	}
	return n
}

func TestEvalOperators(t *testing.T) {
	ctx := testContext() // Hérisson bébé · 240 g · tiques · A12 …
	cases := []struct {
		expr string
		want bool
	}{
		{`animal_type = "Hérissons / Insectivore"`, true},
		{`animal_type = "Faucon"`, false},
		{`species != "Buse variable"`, true}, // species declares eq/neq/in/regex/contains
		{`species != "Hérisson d'Europe"`, false},
		{`weight_g < 300`, true},  // 240 g
		{`weight_g <= 240`, true}, // inclusive bound
		{`weight_g > 240`, false},
		{`weight_g >= 240`, true},
		{`weight_g BETWEEN 200 AND 300`, true},
		{`weight_g BETWEEN 300 AND 800`, false},
		{`weight_g BETWEEN 240 AND 240`, true}, // inclusive both ends
		{`species IN ("Hérisson d'Europe", "Buse variable")`, true},
		{`species IN ("Buse variable", "Pigeon biset")`, false},
		{`cage = "A12"`, true},
		{`cage IN ("A12", "B01")`, true},
		{`parasites ~ "(?i)tiques"`, true}, // "puces ++++ - tiques"
		{`parasites !~ "(?i)tiques"`, false},
		{`parasites ~ "^tiques$"`, false},          // anchored: no match
		{`intake_remarks CONTAINS "SHYDRA"`, true}, // "déshydraté", case-insensitive
		{`intake_remarks CONTAINS "xyz"`, false},
		{`NOT weight_g < 200`, true},
		{`has_parasites = true AND parasites ~ "(?i)tiques"`, true}, // seed SM7
		{`has_parasites = true AND parasites ~ "(?i)myiases"`, false},
		{`weight_g < 300 OR has_wounds = true`, true},
		{`days_in_care >= 7 AND (has_wounds = true OR wounds ~ "plaie")`, false}, // 5 days, no wounds
	}
	for _, c := range cases {
		if got := Eval(DefaultRegistry(), parseForEval(t, c.expr), ctx); got != c.want {
			t.Errorf("Eval(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestEvalMissingFailsClosed(t *testing.T) {
	reg := DefaultRegistry()
	ctx := testContext()
	ctx.LastWeightG = nil // no weight on record (§10-B4)
	ctx.Wounds = ""       // empty free text

	if Eval(reg, parseForEval(t, `weight_g < 300`), ctx) {
		t.Errorf("weight predicate without weight must fail closed (false)")
	}
	if Eval(reg, parseForEval(t, `NOT weight_g < 300`), ctx) {
		t.Errorf("NOT must not resurrect a fail-closed missing value")
	}
	// ...and the fail-closed propagation is visible in the trace.
	_, trace := EvalTrace(reg, parseForEval(t, `NOT weight_g < 300`), ctx)
	if len(trace) != 1 || trace[0].Reason != "no value" {
		t.Errorf("trace = %+v, want the \"no value\" step preserved under NOT", trace)
	}
	if Eval(reg, parseForEval(t, `wounds ~ "(?i)plaie"`), ctx) {
		t.Errorf("regex on empty text must fail closed")
	}
	// The trace explains WHY: reason "no value".
	_, trace = EvalTrace(reg, parseForEval(t, `weight_g < 300`), ctx)
	if len(trace) != 1 || trace[0].Reason != "no value" || trace[0].Pass {
		t.Errorf("trace = %+v, want one step failing with reason \"no value\"", trace)
	}
}

func TestEvalCageEmptyStringIsValue(t *testing.T) {
	// §3 decision 5: the cage is the one field where "" is meaningful
	// (the « sans cage » bucket) — it must NOT fail closed.
	reg := DefaultRegistry()
	ctx := testContext()
	ctx.Cage = ""
	if !Eval(reg, parseForEval(t, `cage = ""`), ctx) {
		t.Errorf(`cage = "" must match the sans-cage bucket (empty string is a value)`)
	}
}

func TestEvalTraceShape(t *testing.T) {
	reg := DefaultRegistry()
	ctx := testContext()

	// single predicate → one step, fully populated
	ok, trace := EvalTrace(reg, parseForEval(t, `weight_g < 300`), ctx)
	if !ok || len(trace) != 1 {
		t.Fatalf("trace = %+v (ok=%v)", trace, ok)
	}
	s := trace[0]
	if s.Field != "weight_g" || s.Op != "<" || !s.Pass || s.Reason != "" {
		t.Errorf("step = %+v", s)
	}
	if s.Expected != "300" || s.Actual != "240" {
		t.Errorf("expected/actual = %q/%q, want 300/240", s.Expected, s.Actual)
	}

	// conjunction → two steps
	_, trace = EvalTrace(reg, parseForEval(t, `has_parasites = true AND weight_g < 300`), ctx)
	if len(trace) != 2 {
		t.Fatalf("AND trace steps = %d, want 2", len(trace))
	}

	// NOT: overall fails; the inner step keeps its RAW result (passing) —
	// the explanation shows what the negated predicate actually found.
	ok, trace = EvalTrace(reg, parseForEval(t, `NOT weight_g < 300`), ctx)
	if ok {
		t.Errorf("240 < 300 under NOT must be false")
	}
	if len(trace) != 1 || !trace[0].Pass {
		t.Errorf("NOT trace should expose the inner predicate's raw result: %+v", trace)
	}
}

func TestEvalRegexCompiledAtValidation(t *testing.T) {
	// §5.2: regex compiled once at parse/validation time and cached on the node.
	n := parseForEval(t, `parasites ~ "(?i)tiques"`)
	p, ok := n.(*PredicateNode)
	if !ok {
		t.Fatalf("expected *PredicateNode, got %T", n)
	}
	if p.Regex() == nil {
		t.Fatalf("validated regex predicate must carry a compiled pattern")
	}
	if !p.Regex().MatchString("Puces ++++ - Tiques") {
		t.Errorf("compiled pattern lost (?i) semantics")
	}
}

func TestEvalUnknownFieldFailsClosed(t *testing.T) {
	// §4.4 runtime registry drift: provider removed/renamed → evaluation
	// fails closed (no match) and reports "unknown field" in the trace —
	// never an error on the plan view.
	n, err := Parse(`ghost_field = "x"`) // unvalidated parse
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ok, trace := EvalTrace(DefaultRegistry(), n, testContext())
	if ok {
		t.Errorf("unknown field must fail closed")
	}
	if len(trace) != 1 || trace[0].Reason != "unknown field" {
		t.Errorf("trace = %+v, want one \"unknown field\" step", trace)
	}
}

func TestPreviewReturnsMatchingWithTrace(t *testing.T) {
	reg := DefaultRegistry()
	n := parseForEval(t, `animal_age = "bébé"`)

	young := testContext()
	old := testContext()
	old.AnimalAge = "adulte"
	other := testContext()
	other.ID = 2
	other.AnimalAge = "juvénile"

	animals := []*AnimalContext{young, old, other}
	hits := Preview(reg, n, animals, 0)
	if len(hits) != 1 {
		t.Fatalf("preview hits = %d, want 1 (only the bébé)", len(hits))
	}
	if hits[0].Animal.ID != young.ID || len(hits[0].Trace) == 0 {
		t.Errorf("hit = %+v, want the matching animal with its trace", hits[0])
	}

	// limit caps the result set (matcher test panel, §7).
	if got := Preview(reg, n, animals, 1); len(got) != 1 {
		t.Errorf("limit=1 hits = %d, want 1", len(got))
	}

	// no match at all → empty, not nil-error.
	hits = Preview(reg, parseForEval(t, `species = "Faucon crecerelle"`), animals, 0)
	if len(hits) != 0 {
		t.Errorf("hits = %d, want 0", len(hits))
	}
}

// TestEvalCaseInsensitiveOperators (bugs.md U26 R5-1a): =* / !=* / INCI
// match strings case-insensitively; CS twins stay exact; non-string
// operands fall back to the CS comparison even under a CI op.
func TestEvalCaseInsensitiveOperators(t *testing.T) {
	reg := DefaultRegistry()
	a := testContext()
	a.Cage = "Enclos Renards"

	cases := []struct {
		expr string
		want bool
	}{
		{`cage =* "enclos renards"`, true},
		{`cage = "enclos renards"`, false},
		{`cage =* "enclos renards "`, false},
		{`cage !=* "enclos RENARDS"`, false},
		{`cage !=* "VE5"`, true},
		{`cage INCI ("VE5", "enclos renards")`, true},
		{`cage INCI ("ve5")`, false},
		{`zone INCI ("salle 1")`, true},
		{`zone IN ("salle 1")`, false},
	}
	for _, c := range cases {
		if got := Eval(reg, parseValid(t, c.expr), a); got != c.want {
			t.Errorf("Eval(%q) = %v, want %v", c.expr, got, c.want)
		}
	}

	// Non-string operands under a CI op behave exactly like the CS variant
	// (unvalidated AST — the registry never allows CI on numbers).
	n := &PredicateNode{Field: "weight_g", Op: OpEqCI, Literal: Literal{Kind: LitNumber, Num: 240}}
	if !Eval(reg, n, a) {
		t.Error("CI eq on numbers must fall back to exact comparison (240)")
	}
	n.Literal = Literal{Kind: LitNumber, Num: 241}
	if Eval(reg, n, a) {
		t.Error("CI eq on numbers must stay exact (241 must not match 240)")
	}

	// The why-trace quotes the explicit CI spelling.
	_, tr := EvalTrace(reg, parseValid(t, `cage =* "enclos renards"`), a)
	if len(tr) != 1 || tr[0].Op != OpEqCI {
		t.Errorf("trace op = %+v, want %q", tr, OpEqCI)
	}
}
