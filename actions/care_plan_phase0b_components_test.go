package actions

// Phase 0b — component extraction pins.
//
// Two policy assertions that the DOM-equivalence baseline cannot express:
//
//  1. Every care_plan component extracted in Phase 0b is forked per locale
//     and the four copies are BYTE-IDENTICAL (all user-facing strings go
//     through t(...) so the forks cannot drift).
//  2. No toggle control (plan-apply-btn / plan-unapply-btn /
//     plan-feeding-one / plan-feeding-apply / plan-cage-apply /
//     plan-med-slot) may render the hardcoded btn-outline-secondary — the
//     single colour policy is slotTierClass (R4-1.1), wired via the
//     viewmodel's TierClass fields.
//
// Non-toggle controls (detail ℹ, dropdowns, modal cancels, history undo,
// editor add-slot) legitimately keep btn-outline-secondary and are not
// matched here.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// phase0bComponents are the partial families extracted/rewired in Phase 0b,
// each expected in four byte-identical locale forks.
var phase0bComponents = []string{
	"_plan_tier",
	"_plan_med_row",
	"_plan_item_line",
	"_plan_slot_toggle",
	"_plan_tier_feed_table",
	"_apply_toggle",
	"_plan_med_toggle",
}

// TestPhase0bComponentForksByteIdentical pins the x4-locale policy: the
// fr/de/nl fork of every Phase 0b component is a byte copy of the en base.
func TestPhase0bComponentForksByteIdentical(t *testing.T) {
	for _, c := range phase0bComponents {
		base := "../templates/care_plan/" + c + ".plush.html"
		raw, err := os.ReadFile(base)
		require.NoError(t, err, base)
		for _, loc := range []string{"fr", "de", "nl"} {
			fork := "../templates/care_plan/" + c + ".plush." + loc + ".html"
			got, err := os.ReadFile(fork)
			require.NoError(t, err, fork)
			require.Equal(t, raw, got, fork+" drifted from "+base)
		}
	}
}

// phase0bToggleControls are the class hooks of the toggle buttons whose
// colour comes from slotTierClass via the TierClass viewmodel fields.
var phase0bToggleControls = []string{
	"plan-apply-btn",
	"plan-unapply-btn",
	"plan-feeding-one",
	"plan-feeding-apply",
	"plan-cage-apply",
	"plan-med-slot",
}

// phase0bToggleForks: every template fork that can render a toggle control.
var phase0bToggleForks = []string{
	"../templates/care_plan/_plan_item_line.plush.html",
	"../templates/care_plan/_plan_item_line.plush.fr.html",
	"../templates/care_plan/_plan_item_line.plush.de.html",
	"../templates/care_plan/_plan_item_line.plush.nl.html",
	"../templates/care_plan/_plan_tier_feed_table.plush.html",
	"../templates/care_plan/_plan_tier_feed_table.plush.fr.html",
	"../templates/care_plan/_plan_tier_feed_table.plush.de.html",
	"../templates/care_plan/_plan_tier_feed_table.plush.nl.html",
	"../templates/care_plan/_plan_slot_toggle.plush.html",
	"../templates/care_plan/_plan_slot_toggle.plush.fr.html",
	"../templates/care_plan/_plan_slot_toggle.plush.de.html",
	"../templates/care_plan/_plan_slot_toggle.plush.nl.html",
	"../templates/care_plan/index.plush.html",
	"../templates/care_plan/index.plush.fr.html",
	"../templates/care_plan/index.plush.de.html",
	"../templates/care_plan/index.plush.nl.html",
	"../templates/animals/show.plush.html",
	"../templates/animals/show.plush.fr.html",
	"../templates/animals/show.plush.de.html",
	"../templates/animals/show.plush.nl.html",
}

// TestPhase0bNoOutlineOnToggles pins R4-1.1: a line that carries a toggle
// control class must not hardcode btn-outline-secondary.
func TestPhase0bNoOutlineOnToggles(t *testing.T) {
	for _, f := range phase0bToggleForks {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		for _, line := range strings.Split(string(raw), "\n") {
			if !strings.Contains(line, "btn-outline-secondary") {
				continue
			}
			for _, c := range phase0bToggleControls {
				require.NotContains(t, line, c,
					f+": toggle "+c+" still hardcodes btn-outline-secondary")
			}
		}
	}
}
