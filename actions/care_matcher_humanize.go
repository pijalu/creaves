package actions

// Matcher-expression humanizer (bugs.md U18): renders a stored matcher DSL
// expression as a short readable sentence for the request locale — field
// names shown as their localized labels ("Force feed"), never the raw
// careplan.field.* i18n keys or DSL identifiers. The raw DSL stays visible
// behind a disclosure in the UI. Unparseable input is returned verbatim
// (save-time validation rejects bad expressions, so this is only a display
// fallback for legacy rows).

import (
	"fmt"
	"strings"

	"creaves/models/careplan"

	"github.com/gobuffalo/buffalo"
)

// humanizeMatcher renders a matcher DSL expression as a localized readable
// sentence for the request locale.
func humanizeMatcher(c buffalo.Context, expr string) string {
	return humanizeMatcherWith(expr, func(id string, args map[string]interface{}) string {
		if args == nil {
			return T.Translate(c, id)
		}
		return T.Translate(c, id, args)
	})
}

// humanizeMatcherWith is the locale-independent core: tr resolves a
// translation id so tests can inject a language without a request context.
func humanizeMatcherWith(expr string, tr func(id string, args map[string]interface{}) string) string {
	node, err := careplan.Parse(expr)
	if err != nil {
		return expr
	}
	label := fieldLabelResolver(tr)
	var b strings.Builder
	writeMatcherNode(&b, node, label, tr, false)
	return b.String()
}

// fieldLabelResolver maps a DSL field key to its localized display label.
func fieldLabelResolver(tr func(id string, args map[string]interface{}) string) func(string) string {
	labels := map[string]string{}
	for _, f := range careplan.DefaultRegistry().Fields() {
		labels[f.Key] = tr(f.LabelKey, nil)
	}
	return func(key string) string {
		if v, ok := labels[key]; ok && v != "" {
			return v
		}
		return key
	}
}

// matcherOpLabel maps a DSL operator to its localized short label, reusing
// the builder op translations.
func matcherOpLabel(op string, tr func(id string, args map[string]interface{}) string) string {
	keys := map[string]string{
		"=":        "care_plan.builder.op.eq",
		"!=":       "care_plan.builder.op.neq",
		"<":        "care_plan.builder.op.lt",
		"<=":       "care_plan.builder.op.lte",
		">":        "care_plan.builder.op.gt",
		">=":       "care_plan.builder.op.gte",
		"IN":       "care_plan.builder.op.in",
		"BETWEEN":  "care_plan.builder.op.between",
		"~":        "care_plan.builder.op.regex",
		"!~":       "care_plan.builder.op.notregex",
		"CONTAINS": "care_plan.builder.op.contains",
	}
	if k, ok := keys[op]; ok {
		return tr(k, nil)
	}
	return op
}

func writeMatcherNode(b *strings.Builder, n careplan.Node, label func(string) string, tr func(id string, args map[string]interface{}) string, nested bool) {
	switch node := n.(type) {
	case *careplan.OrNode:
		if nested {
			b.WriteString("(")
		}
		writeMatcherNode(b, node.Left, label, tr, true)
		b.WriteString(" " + tr("care_plan.human_expr.or", nil) + " ")
		writeMatcherNode(b, node.Right, label, tr, true)
		if nested {
			b.WriteString(")")
		}
	case *careplan.AndNode:
		if nested {
			b.WriteString("(")
		}
		writeMatcherNode(b, node.Left, label, tr, true)
		b.WriteString(" " + tr("care_plan.human_expr.and", nil) + " ")
		writeMatcherNode(b, node.Right, label, tr, true)
		if nested {
			b.WriteString(")")
		}
	case *careplan.NotNode:
		b.WriteString(tr("care_plan.human_expr.not", nil) + " ")
		writeMatcherNode(b, node.Expr, label, tr, true)
	case *careplan.PredicateNode:
		b.WriteString(label(node.Field))
		if node.Literal.Kind == careplan.LitBool && node.Op == "=" {
			// bool fields read naturally as "Force feed: yes".
			if node.Literal.Bool {
				b.WriteString(": " + tr("care_plan.builder.bool_true", nil))
			} else {
				b.WriteString(": " + tr("care_plan.builder.bool_false", nil))
			}
			return
		}
		b.WriteString(" " + matcherOpLabel(node.Op, tr) + " ")
		b.WriteString(matcherLiteralText(node.Literal, tr))
	case *careplan.BetweenNode:
		b.WriteString(label(node.Field))
		b.WriteString(" " + matcherOpLabel("BETWEEN", tr) + " ")
		b.WriteString(fmt.Sprintf("%v – %v", node.Min, node.Max))
	case *careplan.InNode:
		b.WriteString(label(node.Field))
		b.WriteString(" " + matcherOpLabel("IN", tr) + " ")
		parts := make([]string, 0, len(node.Literals))
		for _, lit := range node.Literals {
			parts = append(parts, matcherLiteralText(lit, tr))
		}
		b.WriteString(strings.Join(parts, ", "))
	default:
		b.WriteString("?")
	}
}

// matcherLiteralText renders a DSL literal for display: strings in quotes,
// numbers bare, booleans localized.
func matcherLiteralText(l careplan.Literal, tr func(id string, args map[string]interface{}) string) string {
	switch l.Kind {
	case careplan.LitString:
		return "“" + l.Str + "”"
	case careplan.LitNumber:
		return fmt.Sprintf("%v", l.Num)
	default:
		if l.Bool {
			return tr("care_plan.builder.bool_true", nil)
		}
		return tr("care_plan.builder.bool_false", nil)
	}
}
