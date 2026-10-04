package actions

import (
	"fmt"
	"net/http"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
)

// R8-5: system preferences for the day-plan VIEWS — per action kind, how
// much late and how much future work the work screen shows. NULL caps = the
// pre-preferences behavior (show everything), so seeding the table changes
// nothing until an admin sets a value. The day plan is the WORK screen, not
// the archive: hidden late work stays on the animal Plan tab (history), and
// filtering before the view model derives tiers keeps badges + counts honest.

// preferenceKinds lists the day-plan action kinds, in display order.
var preferenceKinds = actionKinds

// PreferencesEnsureSeeded creates any missing per-kind row with NULL caps
// (idempotent — safe on every request path that needs the settings).
func PreferencesEnsureSeeded(tx *pop.Connection) error {
	for _, kind := range preferenceKinds {
		exists, err := tx.Where("kind = ?", kind).Exists(&models.Preference{})
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		p := &models.Preference{Kind: kind}
		if err := tx.Create(p); err != nil {
			return err
		}
	}
	return nil
}

// preferencesByKind loads the per-kind view settings (R8-5). Missing rows
// behave as "no cap" — the map simply lacks the kind.
func preferencesByKind(tx *pop.Connection) (map[string]models.Preference, error) {
	out := map[string]models.Preference{}
	var prefs []models.Preference
	if err := tx.All(&prefs); err != nil {
		return nil, err
	}
	for i := range prefs {
		out[prefs[i].Kind] = prefs[i]
	}
	return out, nil
}

// applyPreferenceCaps narrows ONE kind's plan items to the view caps
// (R8-5): late/missing occurrences older than late_show_hours leave, and
// later-tier occurrences beyond future_show_hours leave. Everything else
// passes untouched. Called per kind in CarePlanIndex BEFORE the view model
// builds tiers/counts, so badges and lists cannot disagree.
func applyPreferenceCaps(items []careplan.PlanItem, p models.Preference, now time.Time) []careplan.PlanItem {
	if !p.LateShowHours.Valid && !p.FutureShowHours.Valid {
		return items
	}
	out := items[:0:0]
	for i := range items {
		it := items[i]
		status := it.Status
		switch {
		case (status == careplan.StatusLate || status == careplan.StatusMissing) && p.LateShowHours.Valid:
			if now.Sub(it.Occurrence.DueAt) > time.Duration(p.LateShowHours.Int)*time.Hour {
				continue
			}
		case status == careplan.StatusScheduled && p.FutureShowHours.Valid:
			if it.Occurrence.DueAt.Sub(now) > time.Duration(p.FutureShowHours.Int)*time.Hour {
				continue
			}
		}
		out = append(out, it)
	}
	return out
}

// PreferencesResource serves the admin view of the per-kind day-plan caps:
// a read/edit list — no create/destroy, the six kind rows are the system's.
// (Not a full buffalo.Resource: the app has no MethodOverride middleware,
// so the save is an explicit POST route.)
type PreferencesResource struct{}

// PreferencesList renders the editable per-kind list.
func (v PreferencesResource) List(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	if err := PreferencesEnsureSeeded(tx); err != nil {
		return err
	}
	var prefs []models.Preference
	if err := tx.Order("created_at").All(&prefs); err != nil {
		return err
	}
	c.Set("preferences", prefs)
	return c.Render(http.StatusOK, r.HTML("/preferences/index.plush.html"))
}

// PreferencesSave handles the POST of one kind's caps (list page form).
func (v PreferencesResource) Save(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	p := &models.Preference{}
	if err := tx.Find(p, c.Param("preference_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	// Nulls first: an empty field means "no cap" (the seeded behavior).
	p.LateShowHours = nulls.Int{}
	p.FutureShowHours = nulls.Int{}
	p.NowWindowMinutes = nulls.Int{}
	if err := c.Bind(p); err != nil {
		return err
	}
	verrs, err := tx.ValidateAndUpdate(p)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		c.Flash().Add("danger", T.Translate(c, "preferences.update.error"))
	}
	return c.Redirect(http.StatusSeeOther, "/preferences")
	c.Flash().Add("success", T.Translate(c, "preferences.update.ok"))
	return c.Redirect(http.StatusSeeOther, "/preferences")
}
