package actions

import (
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
)

// R8-5: system preferences for the day-plan VIEWS — per action kind, how
// much late and how much future work the work screen shows. NULL caps = the
// pre-preferences behavior (show everything), so seeding the table changes
// nothing until an admin sets a value. The day plan is the WORK screen, not
// the archive: hidden late work stays on the animal Plan tab (history), and
// filtering before the view model derives tiers keeps badges + counts honest.

// preferenceKinds lists the day-plan action kinds, in display order.
var preferenceKinds = actionKinds

// Default view caps — 2026-10-07 user ruling, back to 8 h (item 6 of the
// day-plan review): late work stays visible for 8 h, future work up to an
// 8 h horizon, "now" window 60 min. (The 2026-10-27 widening to 24 h is
// superseded; stale late occurrences are no longer lost to the user — the
// relative-distance heuristic folds them into a "N late" badge on the
// record, see supersedeStaleLate* in care_plan_viewmodel.go.) An admin can
// change or clear any of them per kind; empty = no cap.
var preferenceDefaults = struct{ lateHours, futureHours, nowMinutes int }{8, 8, 60}

// PreferencesEnsureSeeded creates any missing per-kind row with the R9
// defaults (idempotent — safe on every request path that needs the settings).
func PreferencesEnsureSeeded(tx *pop.Connection) error {
	for _, kind := range preferenceKinds {
		exists, err := tx.Where("kind = ?", kind).Exists(&models.Preference{})
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		p := &models.Preference{
			Kind:             kind,
			LateShowHours:    nulls.Int{Int: preferenceDefaults.lateHours, Valid: true},
			FutureShowHours:  nulls.Int{Int: preferenceDefaults.futureHours, Valid: true},
			NowWindowMinutes: nulls.Int{Int: preferenceDefaults.nowMinutes, Valid: true},
		}
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
// (R8-5): late/missing occurrences at or older than late_show_hours leave,
// and scheduled occurrences at or beyond future_show_hours leave. Everything else
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
			if now.Sub(it.Occurrence.DueAt) >= time.Duration(p.LateShowHours.Int)*time.Hour {
				continue
			}
		case status == careplan.StatusScheduled && p.FutureShowHours.Valid:
			if it.Occurrence.DueAt.Sub(now) >= time.Duration(p.FutureShowHours.Int)*time.Hour {
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

// PreferencesSaveAll handles the POST of ALL kinds' caps at once (bugs.md
// 2026-10-06 #3): the list page has exactly ONE save button and it persists
// every modified value on the page. Fields arrive per kind as
// late[<kind>] / future[<kind>] / now[<kind>]; an empty or absent value
// means "no cap" (NULL — the seeded behavior), and the now window stays
// edited in HOURS, stored in MINUTES (×60, same as the old per-row save).
func (v PreferencesResource) SaveAll(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	if err := c.Request().ParseForm(); err != nil {
		return err
	}
	var prefs []models.Preference
	if err := tx.Order("created_at").All(&prefs); err != nil {
		return err
	}
	invalid := false
	for i := range prefs {
		p := &prefs[i]
		// Nulls first: an absent/empty field means "no cap" — a kind the
		// user never touched keeps its stored value via the repopulated
		// inputs, a cleared field deliberately clears the cap.
		p.LateShowHours = nulls.Int{}
		p.FutureShowHours = nulls.Int{}
		p.NowWindowMinutes = nulls.Int{}
		kind := p.Kind
		if s := strings.TrimSpace(c.Request().PostForm.Get("late[" + kind + "]")); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 {
				p.LateShowHours = nulls.Int{Int: n, Valid: true}
			} else {
				invalid = true
			}
		}
		if s := strings.TrimSpace(c.Request().PostForm.Get("future[" + kind + "]")); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 {
				p.FutureShowHours = nulls.Int{Int: n, Valid: true}
			} else {
				invalid = true
			}
		}
		if s := strings.TrimSpace(c.Request().PostForm.Get("now[" + kind + "]")); s != "" {
			if h, err := strconv.Atoi(s); err == nil && h > 0 {
				p.NowWindowMinutes = nulls.Int{Int: h * 60, Valid: true}
			} else {
				invalid = true
			}
		}
		verrs, err := tx.ValidateAndUpdate(p)
		if err != nil {
			return err
		}
		if verrs.HasAny() {
			invalid = true
		}
	}
	if invalid {
		c.Flash().Add("danger", T.Translate(c, "preferences.err"))
		return c.Redirect(http.StatusSeeOther, "/preferences")
	}
	c.Flash().Add("success", T.Translate(c, "preferences.ok"))
	return c.Redirect(http.StatusSeeOther, "/preferences")
}
