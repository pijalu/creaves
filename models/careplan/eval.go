package careplan

import (
	"regexp"
	"strconv"
	"strings"
)

// Evaluator (§5.3–§5.5): pure AST → bool over an enriched AnimalContext,
// with a per-predicate why-trace. No SQL is ever generated — values come
// from the registry's FieldProviders (DIP), predicates on fields with no
// value fail CLOSED (§5.2: missing value → predicate false + trace reason
// "no value"), and so do unknown fields (registry drift, §4.4) and type
// mismatches. Both sides of AND/OR are always evaluated (no short-circuit):
// expressions are tiny and the complete trace is what powers the
// explanation UIs (§5.5).

// TraceStep is one per-predicate entry of the why-trace (§5.5): field, op,
// expected vs actual, verdict and a fail reason for inline display.
type TraceStep struct {
	Field    string
	Op       string
	Expected string
	Actual   string
	Pass     bool
	Reason   string // "" | "no value" | "unknown field" | "type mismatch" | "invalid pattern"
	Pos      Pos    // predicate position (expression-editor anchor)
}

// Eval evaluates the matcher against one animal context.
func Eval(reg *Registry, n Node, a *AnimalContext) bool {
	ok, _ := EvalTrace(reg, n, a)
	return ok
}

// EvalTrace evaluates the matcher and returns the verdict plus the
// per-predicate trace (§5.5): rendered by the animal page ("why this rule
// applies"), the matcher test panel and the day-plan tooltips.
func EvalTrace(reg *Registry, n Node, a *AnimalContext) (bool, []TraceStep) {
	pass, _, tr := evalNode(reg, n, a)
	return pass, tr
}

func evalNode(reg *Registry, n Node, a *AnimalContext) (pass, closed bool, tr []TraceStep) {
	switch t := n.(type) {
	case *OrNode:
		l, lc, lt := evalNode(reg, t.Left, a)
		r, rc, rt := evalNode(reg, t.Right, a)
		return l || r, lc || rc, append(lt, rt...)
	case *AndNode:
		l, lc, lt := evalNode(reg, t.Left, a)
		r, rc, rt := evalNode(reg, t.Right, a)
		return l && r, lc || rc, append(lt, rt...)
	case *NotNode:
		b, closed, tr := evalNode(reg, t.Expr, a)
		// Fail-closed propagates through NOT (§5.2): a missing value must
		// not be resurrected into a match by negation — an animal with no
		// weight on record is NOT known to be `>= 300`.
		return !b && !closed, closed, tr
	case *PredicateNode:
		return evalPredicate(reg, t, a)
	case *BetweenNode:
		return evalBetween(reg, t, a)
	case *InNode:
		return evalIn(reg, t, a)
	}
	return false, true, []TraceStep{{Reason: "unknown node"}}
}

func evalPredicate(reg *Registry, n *PredicateNode, a *AnimalContext) (bool, bool, []TraceStep) {
	step := TraceStep{Field: n.Field, Op: n.Op, Pos: n.pos, Expected: expectedOf(n.Op, n.Literal)}
	p, ok := reg.Get(n.Field)
	if !ok { // runtime registry drift (§4.4): fail closed, flag "broken"
		step.Reason = "unknown field"
		return false, true, []TraceStep{step}
	}
	rv := p.Resolve(a)
	if rv.Missing {
		step.Reason, step.Actual = "no value", "no value"
		return false, true, []TraceStep{step}
	}
	step.Actual = describeValue(rv.Value)

	var pass bool
	var reason string
	switch n.Op {
	case OpEq, OpNeq:
		eq, typed := valuesEqual(rv.Value, n.Literal)
		if !typed {
			reason = "type mismatch"
		} else if n.Op == OpEq {
			pass = eq
		} else {
			pass = !eq
		}
	case OpLt, OpLte, OpGt, OpGte:
		v, okNum := rv.Value.(float64)
		if !okNum || n.Literal.Kind != LitNumber {
			reason = "type mismatch"
			break
		}
		switch n.Op {
		case OpLt:
			pass = v < n.Literal.Num
		case OpLte:
			pass = v <= n.Literal.Num
		case OpGt:
			pass = v > n.Literal.Num
		case OpGte:
			pass = v >= n.Literal.Num
		}
	case OpRegex, OpNotRegex:
		s, okStr := rv.Value.(string)
		if !okStr {
			reason = "type mismatch"
			break
		}
		re := n.re // compiled at validation time (§5.2: compiled once)
		if re == nil {
			re, _ = compileRegex(n.Literal.Str) // unvalidated-AST fallback
		}
		if re == nil {
			reason = "invalid pattern"
			break
		}
		m := re.MatchString(s)
		pass = m != (n.Op == OpNotRegex) // regex: match, !~: no-match
	case OpContains:
		s, okStr := rv.Value.(string)
		if !okStr {
			reason = "type mismatch"
			break
		}
		pass = strings.Contains(strings.ToLower(s), strings.ToLower(n.Literal.Str))
	default:
		reason = "unknown op"
	}
	step.Pass, step.Reason = pass, reason
	return pass, reason != "", []TraceStep{step} // closed: failed for a structural reason
}

func evalBetween(reg *Registry, n *BetweenNode, a *AnimalContext) (bool, bool, []TraceStep) {
	step := TraceStep{
		Field: n.Field, Op: OpBetween, Pos: n.pos,
		Expected: fmtNum(n.Min) + ".." + fmtNum(n.Max),
	}
	p, ok := reg.Get(n.Field)
	if !ok {
		step.Reason = "unknown field"
		return false, true, []TraceStep{step}
	}
	rv := p.Resolve(a)
	if rv.Missing {
		step.Reason, step.Actual = "no value", "no value"
		return false, true, []TraceStep{step}
	}
	v, okNum := rv.Value.(float64)
	if !okNum {
		step.Reason, step.Actual = "type mismatch", describeValue(rv.Value)
		return false, true, []TraceStep{step}
	}
	step.Actual = fmtNum(v)
	step.Pass = v >= n.Min && v <= n.Max // SQL BETWEEN: inclusive bounds
	return step.Pass, step.Reason != "", []TraceStep{step}
}

func evalIn(reg *Registry, n *InNode, a *AnimalContext) (bool, bool, []TraceStep) {
	want := make([]string, 0, len(n.Literals))
	for _, lit := range n.Literals {
		want = append(want, lit.desc())
	}
	step := TraceStep{Field: n.Field, Op: OpIn, Pos: n.pos, Expected: strings.Join(want, ", ")}
	p, ok := reg.Get(n.Field)
	if !ok {
		step.Reason = "unknown field"
		return false, true, []TraceStep{step}
	}
	rv := p.Resolve(a)
	if rv.Missing {
		step.Reason, step.Actual = "no value", "no value"
		return false, true, []TraceStep{step}
	}
	step.Actual = describeValue(rv.Value)
	for _, lit := range n.Literals {
		if eq, typed := valuesEqual(rv.Value, lit); typed && eq {
			step.Pass = true
			return true, false, []TraceStep{step}
		}
	}
	return false, step.Reason != "", []TraceStep{step}
}

// valuesEqual compares a resolved field value with a literal; ok is false
// when the types are incompatible (fail closed, never panic).
func valuesEqual(v interface{}, lit Literal) (eq, ok bool) {
	switch lit.Kind {
	case LitString:
		s, isStr := v.(string)
		return isStr && s == lit.Str, isStr
	case LitNumber:
		f, isNum := v.(float64)
		return isNum && f == lit.Num, isNum
	case LitBool:
		b, isBool := v.(bool)
		return isBool && b == lit.Bool, isBool
	}
	return false, false
}

// expectedOf renders what the operator asks for (error/trace display).
func expectedOf(op string, lit Literal) string {
	switch op {
	case OpRegex, OpNotRegex:
		return strconv.Quote(lit.Str)
	case OpContains:
		return strconv.Quote(lit.Str)
	default:
		return lit.desc()
	}
}

// describeValue renders a resolved field value for the trace.
func describeValue(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strconv.Quote(t)
	case float64:
		return fmtNum(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	}
	return "no value"
}

func fmtNum(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

// compileRegex compiles a RE2 pattern (RE2: no backtracking → ReDoS-safe
// on user-supplied patterns, §5.1); it is the single compile point shared
// by validation and the unvalidated-AST eval fallback.
func compileRegex(pattern string) (*regexp.Regexp, error) { return regexp.Compile(pattern) }

// PreviewHit is one animal's preview outcome with its why-trace.
type PreviewHit struct {
	Animal  *AnimalContext
	Matched bool
	Trace   []TraceStep
}

// Preview is part of the matcher contract (§5.2): evaluate the matcher over
// animals in order and return the first `limit` MATCHING animals with their
// traces — the matcher-library test panel and the rule editor live preview
// (§7). limit <= 0 means no cap.
func Preview(reg *Registry, n Node, animals []*AnimalContext, limit int) []PreviewHit {
	hits := make([]PreviewHit, 0, 8)
	for _, a := range animals {
		matched, trace := EvalTrace(reg, n, a)
		if !matched {
			continue
		}
		hits = append(hits, PreviewHit{Animal: a, Matched: true, Trace: trace})
		if limit > 0 && len(hits) >= limit {
			break
		}
	}
	return hits
}
