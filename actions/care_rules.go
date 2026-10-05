package actions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
	"github.com/gofrs/uuid"
)

// careRuleForm is the HTML editor payload: every field arrives as a string
// so malformed JSON payloads/times surface as model validation errors
// (422 with the editor re-rendered), never as bind failures.
type careRuleForm struct {
	Name            string `form:"Name"`
	Description     string `form:"Description"`
	ActionKind      string `form:"ActionKind"`
	ActionPayload   string `form:"ActionPayload"`
	Schedule        string `form:"Schedule"`
	MatcherID       string `form:"MatcherID"`
	Active          string `form:"Active"`
	Priority        string `form:"Priority"`
	ValidFrom       string `form:"ValidFrom"`
	ValidTo         string `form:"ValidTo"`
	StopOnOuttake   bool   `form:"StopOnOuttake"`
	LatchMembership bool   `form:"LatchMembership"`
}

// apply copies the form onto the model, parsing the JSON payloads and the
// optional validity window.
func (f careRuleForm) apply(rule *models.CareRule) error {
	rule.Name = f.Name
	rule.Description = nulls.NewString(f.Description)
	rule.ActionKind = f.ActionKind
	rule.StopOnOuttake = f.StopOnOuttake
	rule.LatchMembership = f.LatchMembership
	if p := strings.TrimSpace(f.ActionPayload); p != "" {
		rule.ActionPayload = json.RawMessage(p)
	}
	if s := strings.TrimSpace(f.Schedule); s != "" {
		rule.Schedule = json.RawMessage(s)
	}
	if id, err := uuid.FromString(f.MatcherID); err == nil {
		rule.MatcherID = uuid.NullUUID{UUID: id, Valid: true}
	} else {
		rule.MatcherID = uuid.NullUUID{}
	}
	rule.Active = f.Active == "true" || f.Active == "on" || f.Active == "1"
	if p := strings.TrimSpace(f.Priority); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return fmt.Errorf("priority: %w", err)
		}
		rule.Priority = n
	}
	parseOpt := func(v string) (*time.Time, error) {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, nil
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
			if t, err := time.Parse(layout, v); err == nil {
				return &t, nil
			}
		}
		return nil, fmt.Errorf("invalid datetime %q (want 2006-01-02T15:04)", v)
	}
	var err error
	if rule.ValidFrom, err = parseOpt(f.ValidFrom); err != nil {
		return err
	}
	if rule.ValidTo, err = parseOpt(f.ValidTo); err != nil {
		return err
	}
	return nil
}

// bindCareRule binds JSON API bodies straight onto the model; HTML editor
// submissions go through careRuleForm (see above).
func bindCareRule(c buffalo.Context, rule *models.CareRule) error {
	if strings.Contains(c.Request().Header.Get("Content-Type"), "application/json") {
		return c.Bind(rule)
	}
	f := &careRuleForm{}
	if err := c.Bind(f); err != nil {
		return err
	}
	return f.apply(rule)
}

// CareRulesResource is the admin-authored generic rule level (§4.1, §7.2).
// CRUD is admin-only; the live preview (§7.1-3) reuses the matcher preview
// machinery against the rule's matcher.
type CareRulesResource struct {
	buffalo.Resource
}

// List gets all rules. GET /care_rules — JSON API plus the HTML rules
// list page (§7.2), content-negotiated like the other admin surfaces.
//
// Defect D6: the HTML branch supports whitelist sorting (sort=name|kind|
// priority|active, dir=asc|desc), filters (kind, active, matcher_id), a
// case-insensitive q= substring search over name+description and pagination
// via PaginateFromParams. The JSON branch deliberately keeps the historical
// contract: the full set, unsorted, no filters — API consumers (the care-plan
// engine) read it as-is.
func (v CareRulesResource) List(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	return responder.Wants("html", func(c buffalo.Context) error {
		rules := &models.CareRules{}
		// Paginate HTML results. Params "page" and "per_page" control
		// pagination; defaults page=1 per_page=20.
		q := tx.PaginateFromParams(c.Params())
		q = applyCareRuleListFilters(q, c)
		q = applyCareRuleSort(q, c)
		if err := q.All(rules); err != nil {
			return err
		}
		matcherNames, err := matcherNamesByID(tx)
		if err != nil {
			return err
		}
		// Matcher dropdown for the matcher_id filter (name-ordered).
		matchers := &models.CareMatchers{}
		if err := tx.Order("name asc").All(matchers); err != nil {
			return err
		}
		c.Set("pagination", q.Paginator)
		c.Set("rules", []models.CareRule(*rules))
		c.Set("matcherNames", matcherNames)
		c.Set("matchers", matchers)
		c.Set("actionKinds", planActionKinds())
		// Sticky filter values for the filter bar.
		c.Set("filterKind", strings.TrimSpace(c.Param("kind")))
		c.Set("filterActive", strings.ToLower(strings.TrimSpace(c.Param("active"))))
		c.Set("filterMatcherID", strings.TrimSpace(c.Param("matcher_id")))
		c.Set("filterQ", strings.TrimSpace(c.Param("q")))
		// Chip label for the matcher filter: resolve the name when the
		// matcher_id param parses (invalid values render no chip).
		filterMatcherName := ""
		if id, err := uuid.FromString(c.Param("matcher_id")); err == nil {
			filterMatcherName = matcherNames[id]
		}
		c.Set("filterMatcherName", filterMatcherName)
		return c.Render(http.StatusOK, r.HTML("care_rules/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		rules := &models.CareRules{}
		if err := tx.All(rules); err != nil {
			return err
		}
		return c.Render(http.StatusOK, renderJSON(rules))
	}).Respond(c)
}

// Show gets one rule. GET /care_rules/{care_rule_id} — JSON API plus the
// read-only HTML detail page (defect D2), content-negotiated like List.
// The back link honours ?back= (sanitized to a local path) so the
// care_plan source links round-trip to the work screen.
func (v CareRulesResource) Show(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	rule := &models.CareRule{}
	if err := tx.Find(rule, c.Param("care_rule_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	return responder.Wants("html", func(c buffalo.Context) error {
		matcherNames, err := matcherNamesByID(tx)
		if err != nil {
			return err
		}
		c.Set("rule", rule)
		c.Set("matcherName", matcherNames[rule.MatcherID.UUID])
		back := localBackParam(c.Param("back"))
		if back == "" {
			back = "/care_rules"
		}
		c.Set("backTarget", unwrapBackChain(back))
		c.Set("payloadStr", string(rule.ActionPayload))
		c.Set("scheduleStr", string(rule.Schedule))
		return c.Render(http.StatusOK, r.HTML("care_rules/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, renderJSON(rule))
	}).Respond(c)
}

// setRuleContext loads everything the rule editor page needs (matcher
// select, §5.1 builder fields, payload/schedule textarea contents).
func setRuleContext(c buffalo.Context, tx *pop.Connection, rule *models.CareRule) error {
	matchers := &models.CareMatchers{}
	if err := tx.Order("name asc").All(matchers); err != nil {
		return err
	}
	c.Set("rule", rule)
	c.Set("matchers", matchers)
	c.Set("fields", matcherFieldOptions())
	c.Set("actionKinds", planActionKinds())
	c.Set("payloadStr", string(rule.ActionPayload))
	c.Set("scheduleStr", string(rule.Schedule))
	// Structured editors (bugs.md U17): caretype/drug dropdown data.
	if err := setPlanEditorData(c, tx); err != nil {
		return err
	}
	if rule.ValidFrom != nil {
		c.Set("validFromStr", rule.ValidFrom.Format("2006-01-02T15:04"))
	} else {
		c.Set("validFromStr", "")
	}
	if rule.ValidTo != nil {
		c.Set("validToStr", rule.ValidTo.Format("2006-01-02T15:04"))
	} else {
		c.Set("validToStr", "")
	}
	return nil
}

// New renders the rule editor (visual builder + live preview) for a new
// rule. GET /care_rules/new — HTML only.
func (v CareRulesResource) New(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	rule := &models.CareRule{Active: true}
	if err := setRuleContext(c, tx, rule); err != nil {
		return err
	}
	// R9-6: per-language name/description inputs.
	if err := setTranslationValues(c, tx, "care_rules", "", []string{"name", "description"}); err != nil {
		return err
	}
	return c.Render(http.StatusOK, r.HTML("care_rules/new.plush.html"))
}

// Edit renders the rule editor for an existing rule. The preview button
// calls GET /care_rules/{id}/preview (§7.1-3) from the page JS.
// GET /care_rules/{care_rule_id}/edit — HTML only.
func (v CareRulesResource) Edit(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	rule := &models.CareRule{}
	if err := tx.Find(rule, c.Param("care_rule_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	if err := setRuleContext(c, tx, rule); err != nil {
		return err
	}
	// R9-6: per-language name/description inputs.
	if err := setTranslationValues(c, tx, "care_rules", rule.ID.String(), []string{"name", "description"}); err != nil {
		return err
	}
	return c.Render(http.StatusOK, r.HTML("care_rules/edit.plush.html"))
}

// Create adds a rule. POST /care_rules
func (v CareRulesResource) Create(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	rule := &models.CareRule{}
	if err := bindCareRule(c, rule); err != nil {
		return err
	}
	verrs, err := tx.ValidateAndCreate(rule)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			tx2 := planTx(c)
			if err := setRuleContext(c, tx2, rule); err != nil {
				return err
			}
			c.Set("verrs", verrs)
			return c.Render(http.StatusUnprocessableEntity, r.HTML("care_rules/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, renderJSON(verrs))
		}).Respond(c)
	}
	// R9-6: persist per-language name/description.
	if err := saveTranslations(c, tx, "care_rules", rule.ID.String(), []string{"name", "description"}); err != nil {
		return err
	}
	return responder.Wants("html", func(c buffalo.Context) error {
		flashPlanRuleWarnings(c, rule)
		return c.Redirect(http.StatusSeeOther, "/care_rules")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, renderJSON(rule))
	}).Respond(c)
}

// Update changes a rule. PUT /care_rules/{care_rule_id}
func (v CareRulesResource) Update(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	rule := &models.CareRule{}
	if err := tx.Find(rule, c.Param("care_rule_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	if err := bindCareRule(c, rule); err != nil {
		return err
	}
	// ID drift guard: the bound body may carry a foreign id.
	if id, err := uuid.FromString(c.Param("care_rule_id")); err == nil {
		rule.ID = id
	}
	verrs, err := tx.ValidateAndUpdate(rule)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			tx2 := planTx(c)
			if err := setRuleContext(c, tx2, rule); err != nil {
				return err
			}
			c.Set("verrs", verrs)
			return c.Render(http.StatusUnprocessableEntity, r.HTML("care_rules/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, renderJSON(verrs))
		}).Respond(c)
	}
	// R9-6: persist per-language name/description.
	if err := saveTranslations(c, tx, "care_rules", rule.ID.String(), []string{"name", "description"}); err != nil {
		return err
	}
	return responder.Wants("html", func(c buffalo.Context) error {
		flashPlanRuleWarnings(c, rule)
		return c.Redirect(http.StatusSeeOther, "/care_rules")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, renderJSON(rule))
	}).Respond(c)
}

// Destroy deletes a rule. DELETE /care_rules/{care_rule_id}
func (v CareRulesResource) Destroy(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	rule := &models.CareRule{}
	if err := tx.Find(rule, c.Param("care_rule_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	if err := tx.Destroy(rule); err != nil {
		return err
	}
	return responder.Wants("html", func(c buffalo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/care_rules")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, renderJSON(map[string]string{"status": "deleted"}))
	}).Respond(c)
}

// planRuleWarnings implements the §7.1-6/§10.3-CP6b/§10.4-M1/M4
// warn-but-allow save guardrails: slots outside the 07:00–23:00 work
// window, an open-ended medication course and >1 medication slot in the
// same treatment bucket (§10-M1 dedupe keeps only one entry per bucket)
// never block the save — each is reported as an i18n key for the editor
// flash. Broken schedules report nothing (they are model validation
// errors, surfaced separately).
func planRuleWarnings(actionKind string, schedule json.RawMessage) []string {
	sched, err := careplan.ParseScheduleJSON([]byte(schedule))
	if err != nil {
		return nil
	}
	var warns []string
	for _, t := range sched.Times {
		// §10-CP6b window 07:00–23:00 inclusive, minute-exact (23:01 is out).
		mod := t.Hour*60 + t.Minute
		if mod < 7*60 || mod > 23*60 {
			warns = append(warns, "care_plan.warn.off_hours")
			break
		}
	}
	if actionKind == careplan.KindMedication {
		if sched.OpenEnded() {
			warns = append(warns, "care_plan.warn.open_ended_medication")
		}
		buckets := map[int]int{}
		for _, t := range sched.Times {
			buckets[bucketForHour(t.Hour)]++
		}
		for _, n := range buckets {
			if n > 1 {
				warns = append(warns, "care_plan.warn.slot_bucket")
				break
			}
		}
	}
	return warns
}

// flashPlanRuleWarnings reports the warn-but-allow guardrails on the next
// rendered page (the rules list the save redirects to).
func flashPlanRuleWarnings(c buffalo.Context, rule *models.CareRule) {
	for _, key := range planRuleWarnings(rule.ActionKind, rule.Schedule) {
		c.Flash().Add("warning", T.Translate(c, key))
	}
}

// CareRulePreview handles GET /care_rules/{care_rule_id}/preview (§7.1-3):
// live evaluation of the rule's matcher over the in-care animals.
func CareRulePreview(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	rule := &models.CareRule{}
	if err := tx.Find(rule, c.Param("care_rule_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	if !rule.MatcherID.Valid {
		// §7.1-6 guardrail: no matcher matches ALL animals — say so.
		return c.Render(http.StatusOK, renderJSON(map[string]interface{}{
			"match_count": "all",
			"all_animals": true,
		}))
	}
	matcher := &models.CareMatcher{}
	if err := tx.Find(matcher, rule.MatcherID.UUID); err != nil {
		return planError(c, http.StatusNotFound, fmt.Errorf("rule matcher missing: %w", err))
	}
	items, matches, err := previewMatcherExpression(tx, matcher.Expression, 50, speciesDisplayOf(c))
	if err != nil {
		return planError(c, http.StatusUnprocessableEntity, err)
	}
	return c.Render(http.StatusOK, renderJSON(map[string]interface{}{
		"match_count": matches,
		"animals":     items,
	}))
}
