package actions

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/require"
)

// BUG-22 (2026-10-28): every care-plan action toggle flips IN PLACE
// (○ → green ✓) and untoggles with a confirmation — no button
// disappearance, no page reload, and the "done"/view link NEVER replaces
// the toggle. These tests pin the template/JS contract:
//
//  1. The applied single-occurrence branch of `_plan_item_line` renders a
//     REAL undo toggle (visible `.plan-unapply-btn`) + its d-none apply
//     sibling (the pair contract for BOTH directions) + the view-record
//     link as a SIBLING — the former green "done" badge-link is gone.
//  2. Every partial rendering `.plan-apply-btn` / `.plan-feeding-one`
//     renders a `.plan-unapply-btn` sibling in the SAME parent scope.
//  3. The shared toggle JS carries the #planUnapplyModal confirm flow and
//     NO scheduleAnimalConvergeReload full-page reload remains.
//  4. The cage/feeding batch button flips to a green ✓ group-undo
//     (plan-cage-undo + planUnapplyBatch) instead of disappearing.

// bug22ItemLineForks: the four byte-identical forks of the shared line.
var bug22ItemLineForks = []string{
	"../templates/care_plan/_plan_item_line.plush.html",
	"../templates/care_plan/_plan_item_line.plush.fr.html",
	"../templates/care_plan/_plan_item_line.plush.de.html",
	"../templates/care_plan/_plan_item_line.plush.nl.html",
}

// bug22RenderItemLine renders one fork with the minimal context the
// partial needs (helpers stubbed — the assertions target the toggle
// markup, not the labels).
func bug22RenderItemLine(t *testing.T, fork string, card CardView) string {
	t.Helper()
	raw, err := os.ReadFile(fork)
	require.NoError(t, err, fork)
	ctx := plush.NewContextWith(map[string]interface{}{
		"card":         card,
		"showAnimal":   false,
		"feedingEntry": false,
		"t":            func(s string) string { return s },
		"tbase":        func(a, b, c string) string { return c },
		"tspecies":     func(s string) string { return s },
		"dueLabel":     func(hm, dayKey, shortDate string) string { return hm },
		"partialFeeder": carePlanPartialFeeder(t),
	})
	out, err := plush.Render(string(raw), ctx)
	require.NoError(t, err, fork)
	return out
}

func bug22AppliedCard() CardView {
	return CardView{
		SourceType:  "rule",
		SourceID:    "src-1",
		SourceName:  "Muesli protocol",
		Detail:      "granulés",
		ActionKind:  "feeding",
		AnimalID:    7,
		AnimalLabel: "472/26",
		DueAt:       time.Date(2026, 10, 28, 9, 0, 0, 0, time.Local),
		DueHM:       "09:00",
		Status:      "applied",
		Tier:        3,
		TierClass:   "btn-success",
		Applicable:  false,
		FulfillmentLink: "/feedings/f-1?back=%2Fanimals%2F7%23nav-plan",
	}
}

// 1. The applied single-occurrence branch renders the undo TOGGLE (and
// its d-none apply sibling), never a bare "done" badge-link.
func TestBug22ItemLineAppliedBranchRendersUndoToggle(t *testing.T) {
	for _, f := range bug22ItemLineForks {
		out := bug22RenderItemLine(t, f, bug22AppliedCard())

		require.Contains(t, out, "plan-unapply-btn", f,
			"the applied state IS an undo toggle")
		require.Contains(t, out, "✓ 09:00", f,
			"the green ✓ carries the occurrence time")
		require.Contains(t, out, "plan-apply-btn d-none", f,
			"the apply half renders d-none — the pair contract holds for BOTH directions")

		// The undo toggle must NOT be hidden.
		idx := strings.Index(out, "plan-unapply-btn")
		require.Greater(t, idx, 0, f)
		open := strings.LastIndex(out[:idx], "<button")
		require.Greater(t, open, 0, f)
		tag := out[open:idx]
		require.NotContains(t, tag, "d-none", f,
			"the undo toggle is the VISIBLE half of the applied pair")

		// The view-record link survives as a SIBLING anchor, never instead
		// of the toggle.
		require.Contains(t, out, `href="/feedings/f-1?back=%2Fanimals%2F7%23nav-plan"`, f,
			"the view-record link keeps targeting the logged record")
		require.Contains(t, out, "plan-item-view", f)

		// The forbidden shape is gone: no "done"-label badge replacing the
		// toggle (the link carries only the external-link icon now).
		require.NotContains(t, out, `>care_plan.status.done <`, f,
			"the done badge text no longer replaces the toggle")
	}
}

// The applied branch without a fulfillment still renders the toggle pair —
// only the view link is gated on FulfillmentLink.
func TestBug22ItemLineAppliedBranchWithoutFulfillment(t *testing.T) {
	card := bug22AppliedCard()
	card.FulfillmentLink = ""
	for _, f := range bug22ItemLineForks {
		out := bug22RenderItemLine(t, f, card)
		require.Contains(t, out, "plan-unapply-btn", f)
		require.NotContains(t, out, "plan-item-view", f,
			"no dead view link without a fulfillment record")
		require.NotContains(t, out, `badge-success"><`, f,
			"no plain done badge either — the toggle alone carries the state")
	}
}

// 2. Pair contract: every partial rendering an apply control renders the
// undo sibling in the same scope (text-level pin over all forks; the
// render test above proves the pairing on one branch).
func TestBug22PairSiblingPresenceAllPartials(t *testing.T) {
	pairPartials := []string{
		"../templates/care_plan/_plan_item_line.plush.html",
		"../templates/care_plan/_plan_care_slot.plush.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.html",
	}
	for _, base := range pairPartials {
		for _, loc := range []string{"", ".fr", ".de", ".nl"} {
			f := strings.Replace(base, ".plush.html", ".plush"+loc+".html", 1)
			raw, err := os.ReadFile(f)
			if err != nil {
				continue // _plan_care_slot has no locale forks (locale-independent)
			}
			s := string(raw)
			hasApply := strings.Contains(s, "plan-apply-btn") || strings.Contains(s, "plan-feeding-one")
			if !hasApply {
				continue
			}
			require.Contains(t, s, "plan-unapply-btn", f,
				"a partial rendering an apply control must render the undo sibling (BUG-22)")
		}
	}
}

// 3a. The shared toggle JS asks the shared confirm modal on pair undo and
// keeps NO full-page convergence reload.
func TestBug22ToggleJSConfirmModalNoReload(t *testing.T) {
	for _, loc := range []string{"", ".fr", ".de", ".nl"} {
		f := "../templates/care_plan/_apply_toggle.plush" + loc + ".html"
		raw := readTemplate(t, f)
		require.Contains(t, raw, `id="planUnapplyModal"`, f,
			"the shared undo-confirm modal exists")
		require.Contains(t, raw, "openUnapply(btn)", f,
			"the pair-family undo routes through the confirm modal")
		require.Contains(t, raw, "planUnapplySubmit", f,
			"the confirm submit is wired")
		require.NotContains(t, raw, "scheduleAnimalConvergeReload", f,
			"the forced animal-page reload is REMOVED (BUG-22: no refresh)")
	}
}

// 3b. The modal's i18n keys exist in every locale.
func TestBug22UnapplyModalLabelsAllLocales(t *testing.T) {
	for _, lang := range []string{"en-us", "fr", "de", "nl"} {
		raw := readTemplate(t, "../locales/care_plan."+lang+".yaml")
		for _, key := range []string{
			"care_plan.unapply.confirm_title",
			"care_plan.unapply.confirm_expect",
			"care_plan.unapply.confirm_submit",
			"care_plan.status.undo_locked",
		} {
			i := strings.Index(raw, `- id: "`+key+`"`)
			require.GreaterOrEqual(t, i, 0, lang+" missing "+key)
			rest := raw[i:]
			j := strings.Index(rest, "\n- id:")
			require.Greater(t, j, 0, lang+" "+key)
			require.Contains(t, rest[:j], "translation:", lang+" "+key+" has no translation")
		}
	}
}

// 4. The cage/feeding batch button flips to a green group-undo instead of
// disappearing.
func TestBug22BatchButtonFlipsToGroupUndo(t *testing.T) {
	for _, loc := range []string{"", ".fr", ".de", ".nl"} {
		f := "../templates/care_plan/index.plush" + loc + ".html"
		raw := readTemplate(t, f)
		require.Contains(t, raw, "plan-cage-undo", f,
			"the batch button flips to a visible green ✓ group-undo")
		require.Contains(t, raw, "planUnapplyBatch", f,
			"the group undo loops over the row's applied chips")
		// Live-E2E 2026-10-28: the batch undo must iterate the button's
		// data-items refs, NOT row chips — single-animal rows (R4-7.5) and
		// the Done tier render no per-animal pair, so a chip lookup matched
		// zero elements and the undo never reached the server.
		i := strings.Index(raw, "window.planUnapplyBatch = function (btn) {")
		require.Greater(t, i, 0, f)
		j := strings.Index(raw[i:], "\n  };")
		require.Greater(t, j, 0, f)
		batchBody := raw[i : i+j]
		require.Contains(t, batchBody, `btn.getAttribute('data-items')`, f,
			"planUnapplyBatch iterates data-items refs (R4-7.5 single-animal rows render no chips)")
		require.NotContains(t, batchBody, "querySelectorAll('.plan-unapply-btn:not(.d-none)')", f,
			"row-chip lookup silently undid nothing on single-animal/done-tier rows")
		// flipBatchRow must no longer hide the group button.
		i = strings.Index(raw, "function flipBatchRow")
		require.Greater(t, i, 0, f)
		j = strings.Index(raw[i:], "\n  }")
		require.Greater(t, j, 0, f)
		body := raw[i : i+j]
		require.NotContains(t, body, `btn.classList.add('d-none')`, f,
			"flipBatchRow retires the disappearing-button behaviour")
	}
}

// 5. The disabled non-undoable med slot explains WHY in its tooltip
// (CanUndo=false defined behaviour, BUG-22 root cause #3).
func TestBug22LockedMedSlotCarriesExplanation(t *testing.T) {
	for _, loc := range []string{"", ".fr", ".de", ".nl"} {
		f := "../templates/care_plan/_plan_slot_toggle.plush" + loc + ".html"
		raw := readTemplate(t, f)
		require.Contains(t, raw, `t("care_plan.status.undo_locked")`, f,
			"the disabled ✓ explains why undo is impossible")
	}
}
