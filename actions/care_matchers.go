package actions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
	"github.com/gofrs/uuid"
)

// Care rules + matcher library HTTP layer (§7.2): admin-only CRUD plus the
// §7.1-3 live preview (match count + animals + per-animal why-trace) that
// every editor surface embeds. Restricted roles fall through RoleGuard and
// are rejected here — rule/matcher authoring stays an admin capability.

// requireAdminForPlan denies non-admin accounts on the plan admin surfaces.
// It renders the 403 body itself (planError keeps the detail in every env,
// bugs.md H1) and reports whether the request may proceed.
func requireAdminForPlan(c buffalo.Context) bool {
	if u := GetCurrentUser(c); u == nil || !u.Admin {
		_ = planError(c, http.StatusForbidden, fmt.Errorf("admin only"))
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Matcher preview (§7.1-3)
// ---------------------------------------------------------------------------

// matcherTraceStep is one predicate of the why-trace in structured form so
// the preview UI can color matched clauses green, failed ones red and show
// the actual value on hover (bugs.md U19) instead of a flat string.
type matcherTraceStep struct {
	Field    string `json:"field"`
	Op       string `json:"op"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Pass     bool   `json:"pass"`
	Reason   string `json:"reason,omitempty"`
}

// matcherPreviewItem is one preview row: label + pass/fail + why-trace.
type matcherPreviewItem struct {
	AnimalID int                `json:"animal_id"`
	Label    string             `json:"label"`
	Cage     string             `json:"cage"`
	Zone     string             `json:"zone"`
	Match    bool               `json:"match"`
	Trace    []matcherTraceStep `json:"trace,omitempty"`
}

// previewMatcherExpression evaluates a DSL expression against every
// in-care animal (§7.1-3) returning the full annotated set (count +
// per-animal why). Broken expressions fail with the parser's token-level
// error, never a 500. speciesT maps a canonical species base to its
// localized display name (speciesDisplayOf — identity in tests).
func previewMatcherExpression(tx *pop.Connection, expression string, limit int, speciesT func(string) string) ([]matcherPreviewItem, int, error) {
	if speciesT == nil {
		speciesT = func(b string) string { return b }
	}
	node, err := careplan.ParseValidatedWith(expression, careplan.DefaultRegistry())
	if err != nil {
		return nil, 0, err
	}
	pa, err := loadAnimalContexts(tx, time.Now())
	if err != nil {
		return nil, 0, err
	}
	reg := careplan.DefaultRegistry()

	ids := make([]int, 0, len(pa.ctxs))
	for id := range pa.ctxs {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	items := make([]matcherPreviewItem, 0, len(ids))
	matches := 0
	for _, id := range ids {
		ctx := pa.ctxs[id]
		ok, trace := careplan.EvalTrace(reg, node, ctx)
		if ok {
			matches++
		}
		item := matcherPreviewItem{AnimalID: id, Match: ok}
		if a, present := pa.rows[id]; present {
			a.Species = speciesT(a.Species)
			item.Label = animalLabel(a)
			item.Cage = a.Cage.String
			item.Zone = a.Zone.String
		}
		for _, step := range trace {
			item.Trace = append(item.Trace, matcherTraceStep{
				Field: step.Field, Op: step.Op, Expected: step.Expected,
				Actual: step.Actual, Pass: step.Pass, Reason: step.Reason,
			})
		}
		items = append(items, item)
	}
	// Matched animals first (bugs.md U19): the caretaker wants to see what
	// the matcher selects before the misses.
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Match != items[j].Match {
			return items[i].Match
		}
		return items[i].AnimalID < items[j].AnimalID
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, matches, nil
}

// CareMatcherPreview handles POST /care_matchers/preview (§7.1-3): body
// {expression} → {match_count, animals:[…]}.
func CareMatcherPreview(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	in := struct {
		Expression string `json:"expression"`
	}{}
	if err := c.Bind(&in); err != nil {
		return err
	}
	items, matches, err := previewMatcherExpression(tx, in.Expression, 50, speciesDisplayOf(c))
	if err != nil {
		return planError(c, http.StatusUnprocessableEntity, err)
	}
	return c.Render(http.StatusOK, renderJSON(map[string]interface{}{
		"match_count": matches,
		"animals":     items,
	}))
}

// matcherSuggestCap bounds the suggestion list (bugs.md U27, D-f).
const matcherSuggestCap = 50

// matcherSuggestValue is one suggested distinct field value with the number
// of prefix-matching animals carrying it.
type matcherSuggestValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// CareMatcherSuggest handles POST /care_matchers/suggest (bugs.md U27,
// D-f "context-aware"): body {expression_prefix, field} →
// {values:[{value,count}], matched_total}. The prefix is the serialization
// of the clauses ABOVE the edited clause; it is evaluated over the same
// in-care sampling the preview uses, and the distinct values of `field`
// among the still-matching animals become the suggestions (capped). An
// empty prefix means the first clause — every animal counts. Unknown
// fields answer an empty list (the dropdown simply offers nothing);
// a broken prefix fails with the parser's column-exact error.
func CareMatcherSuggest(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	in := struct {
		ExpressionPrefix string `json:"expression_prefix"`
		Field            string `json:"field"`
	}{}
	if err := c.Bind(&in); err != nil {
		return err
	}

	reg := careplan.DefaultRegistry()
	empty := map[string]interface{}{"values": []matcherSuggestValue{}, "matched_total": 0}
	prov, ok := reg.Get(in.Field)
	if !ok || prov.Type != careplan.TypeString { // unknown / non-string field → nothing to suggest
		return c.Render(http.StatusOK, renderJSON(empty))
	}

	pa, err := loadAnimalContexts(tx, time.Now())
	if err != nil {
		return err
	}
	matchedTotal, counts, perr := suggestCounts(reg, prov, pa, in.ExpressionPrefix)
	if perr != nil {
		return planError(c, http.StatusUnprocessableEntity, perr)
	}

	values := make([]matcherSuggestValue, 0, len(counts))
	for v, n := range counts {
		values = append(values, matcherSuggestValue{Value: v, Count: n})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Count != values[j].Count {
			return values[i].Count > values[j].Count
		}
		return values[i].Value < values[j].Value
	})
	if len(values) > matcherSuggestCap {
		values = values[:matcherSuggestCap]
	}
	return c.Render(http.StatusOK, renderJSON(map[string]interface{}{
		"values":        values,
		"matched_total": matchedTotal,
	}))
}

// suggestCounts evaluates the prefix over the in-care sampling and tallies
// the distinct values of the field among the matching animals. An empty
// prefix counts every animal (first-clause case, D-f). A broken prefix
// returns the parser error.
func suggestCounts(reg *careplan.Registry, prov careplan.FieldProvider, pa *planAnimals, prefix string) (int, map[string]int, error) {
	matched := 0
	counts := make(map[string]int)
	countValue := func(ctx *careplan.AnimalContext) {
		rv := prov.Resolve(ctx)
		if rv.Missing {
			return
		}
		if s, isStr := rv.Value.(string); isStr && s != "" {
			counts[s]++
		}
	}
	if strings.TrimSpace(prefix) == "" {
		for _, ctx := range pa.ctxs {
			matched++
			countValue(ctx)
		}
		return matched, counts, nil
	}
	node, err := careplan.ParseValidatedWith(prefix, reg)
	if err != nil {
		return 0, nil, err
	}
	for _, ctx := range pa.ctxs {
		if !careplan.Eval(reg, node, ctx) {
			continue
		}
		matched++
		countValue(ctx)
	}
	return matched, counts, nil
}

// renderJSON wraps the JSON renderer used by the plan endpoints.
func renderJSON(v interface{}) render.Renderer {
	return r.JSON(v)
}

// matcherFieldOption is one registry entry surfaced in the editors' visual
// builder dropdowns (§5.1): key, localized label key, type and legal ops.
type matcherFieldOption struct {
	Key   string   `json:"key"`
	Label string   `json:"label"`
	Type  string   `json:"type"`
	Ops   []string `json:"ops"`
}

// matcherFieldOptions exposes the §5.1 field registry for the editors.
func matcherFieldOptions() []matcherFieldOption {
	providers := careplan.DefaultRegistry().Fields()
	out := make([]matcherFieldOption, 0, len(providers))
	for _, p := range providers {
		ops := make([]string, len(p.Ops))
		copy(ops, p.Ops)
		out = append(out, matcherFieldOption{Key: p.Key, Label: p.LabelKey, Type: p.Type, Ops: ops})
	}
	return out
}

// planActionKinds lists the §4.2 action kinds for the editors' selects.
func planActionKinds() []string {
	return []string{
		careplan.KindFeeding,
		careplan.KindMedication,
		careplan.KindCare,
		careplan.KindCleanup,
		careplan.KindWeighing,
		careplan.KindObservation,
	}
}

// matcherNamesByID loads id → name for every matcher (rules list + editor
// selects). Empty set is not an error: rules may exist before matchers.
func matcherNamesByID(tx *pop.Connection) (map[uuid.UUID]string, error) {
	matchers := &models.CareMatchers{}
	if err := tx.All(matchers); err != nil {
		return nil, err
	}
	names := make(map[uuid.UUID]string, len(*matchers))
	for _, m := range *matchers {
		names[m.ID] = m.Name
	}
	return names, nil
}

// ---------------------------------------------------------------------------
// CareMatchersResource — admin-only CRUD (§7.2)
// ---------------------------------------------------------------------------

// CareMatchersResource is the named-matcher library resource.
type CareMatchersResource struct {
	buffalo.Resource
}

// List gets all matchers. GET /care_matchers — JSON API plus the HTML
// matcher library page (§7.2), content-negotiated like the rules list.
func (v CareMatchersResource) List(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	matchers := &models.CareMatchers{}
	if err := tx.All(matchers); err != nil {
		return err
	}
	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("matchers", []models.CareMatcher(*matchers))
		return c.Render(http.StatusOK, r.HTML("care_matchers/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, renderJSON(matchers))
	}).Respond(c)
}

// Show gets one matcher. GET /care_matchers/{care_matcher_id}
func (v CareMatchersResource) Show(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	matcher := &models.CareMatcher{}
	if err := tx.Find(matcher, c.Param("care_matcher_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	return c.Render(http.StatusOK, renderJSON(matcher))
}

// setMatcherContext loads everything the matcher editor page needs (§5.1
// builder fields as JSON for the visual builder, expression textarea).
func setMatcherContext(c buffalo.Context, matcher *models.CareMatcher) {
	c.Set("matcher", matcher)
	c.Set("fields", matcherFieldOptions())
	if b, err := json.Marshal(matcherFieldOptions()); err == nil {
		c.Set("fieldsJSON", string(b))
	} else {
		c.Set("fieldsJSON", "[]")
	}
	c.Set("expressionStr", matcher.Expression)
}

// New renders the matcher editor (visual builder + live preview) for a new
// matcher. GET /care_matchers/new — HTML only.
func (v CareMatchersResource) New(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	matcher := &models.CareMatcher{}
	setMatcherContext(c, matcher)
	return c.Render(http.StatusOK, r.HTML("care_matchers/new.plush.html"))
}

// Edit renders the matcher editor for an existing matcher.
// GET /care_matchers/{care_matcher_id}/edit — HTML only.
func (v CareMatchersResource) Edit(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	matcher := &models.CareMatcher{}
	if err := tx.Find(matcher, c.Param("care_matcher_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	setMatcherContext(c, matcher)
	return c.Render(http.StatusOK, r.HTML("care_matchers/edit.plush.html"))
}

// Create adds a matcher. POST /care_matchers
func (v CareMatchersResource) Create(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	matcher := &models.CareMatcher{}
	if err := c.Bind(matcher); err != nil {
		return err
	}
	verrs, err := tx.ValidateAndCreate(matcher)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			setMatcherContext(c, matcher)
			c.Set("verrs", verrs)
			return c.Render(http.StatusUnprocessableEntity, r.HTML("care_matchers/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, renderJSON(verrs))
		}).Respond(c)
	}
	return responder.Wants("html", func(c buffalo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/care_matchers")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, renderJSON(matcher))
	}).Respond(c)
}

// Update changes a matcher. PUT /care_matchers/{care_matcher_id}
func (v CareMatchersResource) Update(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	matcher := &models.CareMatcher{}
	if err := tx.Find(matcher, c.Param("care_matcher_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	if err := c.Bind(matcher); err != nil {
		return err
	}
	// ID drift guard: the bound body may carry a foreign id.
	if id, err := uuid.FromString(c.Param("care_matcher_id")); err == nil {
		matcher.ID = id
	}
	verrs, err := tx.ValidateAndUpdate(matcher)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			setMatcherContext(c, matcher)
			c.Set("verrs", verrs)
			return c.Render(http.StatusUnprocessableEntity, r.HTML("care_matchers/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, renderJSON(verrs))
		}).Respond(c)
	}
	return responder.Wants("html", func(c buffalo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/care_matchers")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, renderJSON(matcher))
	}).Respond(c)
}

// Destroy deletes a matcher. DELETE /care_matchers/{care_matcher_id}
func (v CareMatchersResource) Destroy(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	matcher := &models.CareMatcher{}
	if err := tx.Find(matcher, c.Param("care_matcher_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	// usage guard (§7.1-5): a matcher referenced by rules cannot be deleted
	used, err := tx.Where("matcher_id = ?", matcher.ID).Count(&models.CareRule{})
	if err != nil {
		return err
	}
	if used > 0 {
		return planError(c, http.StatusConflict, fmt.Errorf("matcher used by %d rule(s)", used))
	}
	if err := tx.Destroy(matcher); err != nil {
		return err
	}
	return responder.Wants("html", func(c buffalo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/care_matchers")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, renderJSON(map[string]string{"status": "deleted"}))
	}).Respond(c)
}

// planTx extracts the per-request transaction.
func planTx(c buffalo.Context) *pop.Connection {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return models.DB
	}
	return tx
}
