package actions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
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
	rule.Description = f.Description
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
func (v CareRulesResource) List(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	rules := &models.CareRules{}
	if err := tx.All(rules); err != nil {
		return err
	}
	matcherNames, err := matcherNamesByID(tx)
	if err != nil {
		return err
	}
	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("rules", []models.CareRule(*rules))
		c.Set("matcherNames", matcherNames)
		return c.Render(http.StatusOK, r.HTML("care_rules/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, renderJSON(rules))
	}).Respond(c)
}

// Show gets one rule. GET /care_rules/{care_rule_id}
func (v CareRulesResource) Show(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	rule := &models.CareRule{}
	if err := tx.Find(rule, c.Param("care_rule_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	return c.Render(http.StatusOK, renderJSON(rule))
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
	return responder.Wants("html", func(c buffalo.Context) error {
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
	return responder.Wants("html", func(c buffalo.Context) error {
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
	items, matches, err := previewMatcherExpression(tx, matcher.Expression, 50)
	if err != nil {
		return planError(c, http.StatusUnprocessableEntity, err)
	}
	return c.Render(http.StatusOK, renderJSON(map[string]interface{}{
		"match_count": matches,
		"animals":     items,
	}))
}
