package actions

import (
	"fmt"
	"net/http"
	"sort"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

// "Protocols to confirm" (BUG-52e user ruling, 2026-10-14): the converter
// is definitive — it never emits "(à vérifier)" name suffixes and never
// sets needs_confirmation itself. A USER flags a protocol from the plan
// detail editor ("Need confirmation" set/unset); flagged protocols are
// listed on the admin report /reports/confirmations where an admin
// resolves them (unsets the flag, typically after checking or adjusting
// the protocol on the animal page). The flag has NO scheduling behaviour —
// occurrences are never suppressed by it.

// ConfirmationSection is one animal's flagged protocols on the report
// screen — the report groups by animal so a caretaker can walk the shelter.
type ConfirmationSection struct {
	AnimalID    int
	AnimalLabel string
	AnimalLink  string
	Rows        []ConfirmationRow
}

// ConfirmationRow is one flagged care_animal_plans row.
type ConfirmationRow struct {
	PlanID   string
	Name     string // DisplayName — conversion markers stripped
	Kind     string // i18n anchor: t("care_plan.kind."+Kind)
	Content  string // humanised payload (food / drug — dosage / prompt…)
	Schedule string // raw §4.3 schedule JSON — humanSchedule() reads it
	Created  string
}

// ConfirmationsView is the report template context.
type ConfirmationsView struct {
	Sections []ConfirmationSection
}

// Empty reports "nothing to confirm" (the healthy state — the converter
// never creates flags, so any row here was raised by a human).
func (v *ConfirmationsView) Empty() bool { return len(v.Sections) == 0 }

// CareAnimalPlanConfirmationToggle handles POST
// /care_animal_plans/{care_animal_plan_id}/confirmation: set/unset the
// MANUAL needs_confirmation flag. The needs_confirmation param
// ("1"/"true"/"0"/"false") is authoritative; without it the flag toggles.
// Any signed-in caretaker may flag (RoleGuard strips write access from
// restricted roles); resolving on the report is the same endpoint with
// needs_confirmation=0. HTML clients are redirected to `back` (Referer
// fallback), fetch clients get the updated plan JSON.
func CareAnimalPlanConfirmationToggle(c buffalo.Context) error {
	tx := planTx(c)
	plan := &models.CareAnimalPlan{}
	if err := tx.Find(plan, c.Param("care_animal_plan_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	want := !plan.NeedsConfirmation
	switch c.Param("needs_confirmation") {
	case "1", "true":
		want = true
	case "0", "false":
		want = false
	}
	plan.NeedsConfirmation = want
	// Column-scoped update: a concurrent edit of name/schedule must not be
	// clobbered by the flag write.
	if err := tx.UpdateColumns(plan, "needs_confirmation", "updated_at"); err != nil {
		return err
	}
	if wantsJSON(c) {
		return c.Render(http.StatusOK, renderJSON(plan))
	}
	back := safeRedirectTarget(c.Param("back"))
	if back == "/" {
		// BUG-R5: no safe explicit target — try the Referer, then the report.
		back = safeRedirectTarget(c.Request().Header.Get("Referer"))
	}
	if back == "/" {
		back = "/reports/confirmations"
	}
	return c.Redirect(http.StatusSeeOther, back)
}

// ReportsConfirmationsIndex handles GET /reports/confirmations — the
// "Protocols to confirm" report (admin only, like the other plan
// configuration areas): every care_animal_plan flagged needs_confirmation,
// grouped by animal, with a Resolve action (unsets the flag) and a link to
// the protocol on the animal page for the actual check/update.
func ReportsConfirmationsIndex(c buffalo.Context) error {
	if !requireAdminForPlan(c) {
		return nil
	}
	tx := planTx(c)
	plans := &models.CareAnimalPlans{}
	if err := tx.Where("needs_confirmation = ?", true).Order("created_at").All(plans); err != nil {
		return err
	}
	view, err := buildConfirmationsView(tx, plans)
	if err != nil {
		return err
	}
	c.Set("view", view)
	return c.Render(http.StatusOK, r.HTML("/reports/confirmations.plush.html"))
}

// buildConfirmationsView groups the flagged plans per animal (pure
// projection given the loaded plans — unit-testable).
func buildConfirmationsView(tx *pop.Connection, plans *models.CareAnimalPlans) (*ConfirmationsView, error) {
	byAnimal := map[int]*ConfirmationSection{}
	order := []int{}
	for _, p := range *plans {
		sec, ok := byAnimal[p.AnimalID]
		if !ok {
			sec = &ConfirmationSection{
				AnimalID:   p.AnimalID,
				AnimalLink: fmt.Sprintf("/animals/%d#nav-plan", p.AnimalID),
			}
			byAnimal[p.AnimalID] = sec
			order = append(order, p.AnimalID)
		}
		var payload planPayload
		_ = jsonUnmarshalStrictish(p.ActionPayload, &payload)
		sec.Rows = append(sec.Rows, ConfirmationRow{
			PlanID:   p.ID.String(),
			Name:     DisplayName(p.Name),
			Kind:     p.ActionKind,
			Content:  planDetailOf(p.ActionKind, payload),
			Schedule: string(p.Schedule),
			Created:  p.CreatedAt.Format("02/01/2006"),
		})
	}
	// One animals query for all labels.
	ids := append([]int{}, order...)
	animals := &models.Animals{}
	if len(ids) > 0 {
		if err := tx.Where("id IN (?)", ids).All(animals); err != nil {
			return nil, err
		}
	}
	labels := map[int]string{}
	for _, a := range *animals {
		labels[a.ID] = animalLabel(a)
	}
	out := &ConfirmationsView{}
	for _, id := range order {
		sec := byAnimal[id]
		sec.AnimalLabel = labels[id]
		if sec.AnimalLabel == "" {
			sec.AnimalLabel = "—"
		}
		sort.Slice(sec.Rows, func(i, j int) bool { return sec.Rows[i].Name < sec.Rows[j].Name })
		out.Sections = append(out.Sections, *sec)
	}
	sort.Slice(out.Sections, func(i, j int) bool {
		return out.Sections[i].AnimalLabel < out.Sections[j].AnimalLabel
	})
	return out, nil
}
