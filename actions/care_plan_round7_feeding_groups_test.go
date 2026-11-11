package actions

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// R4-7.14b: "when multiple animal - multiple time - create sub groups of
// buttons per time to avoid time repeats".
//
// Measured before, on the first multi-animal row: 6 chips, every one of them
// carrying its own `08:00` — one time, repeated six times. Across the page:
// 351 chips for 167 distinct (day, time) sub-groups.
//
// R4-7.14c: "when more than 1 animal in a list - create a collapsible to list
// them - collapse by default" — measured 29 of 162 rows holding several
// animals, all expanded, the largest 321 px tall.

// feedChipAt builds an applicable chip due at an exact wall-clock time, with
// the day-awareness fields the fold keys on.
func feedChipAt(label string, y, mo, d, hh, mm int) FeedingChip {
	due := time.Date(y, time.Month(mo), d, hh, mm, 0, 0, time.UTC)
	return FeedingChip{
		AnimalID:   1,
		Label:      label,
		Status:     "late",
		DueAt:      due,
		DueHM:      due.Format("15:04"),
		Applicable: true,
	}
}

// TestChipsFoldIntoTimeSubGroups: the time is the sub-group label, stated
// once, instead of being repeated on every animal.
func TestChipsFoldIntoTimeSubGroups(t *testing.T) {
	chips := []FeedingChip{
		feedChipAt("10100", 2026, 10, 2, 16, 0),
		feedChipAt("10101", 2026, 10, 2, 16, 0),
		feedChipAt("10102", 2026, 10, 2, 16, 0),
		feedChipAt("10103", 2026, 10, 2, 18, 30),
	}
	groups := foldChipsByTime(chips)

	require.Len(t, groups, 2, "six chips at 16:00 are ONE sub-group, not six")
	require.Equal(t, "16:00", groups[0].Label)
	require.Equal(t, 3, groups[0].Count)
	require.Equal(t, "3", groups[0].CountCap)
	require.Equal(t, 3, groups[0].Applicable)
	require.Equal(t, "18:30", groups[1].Label)
	require.Equal(t, 1, groups[1].Count)

	// First-seen order, so group 0 is always the earliest and the row header
	// can never disagree with the first sub-group.
	require.Equal(t, "16:00", groups[0].Label)

	// Every chip lands in exactly one group — the fold must not lose or
	// duplicate an animal.
	var total int
	for _, g := range groups {
		total += g.Count
	}
	require.Equal(t, len(chips), total, "the fold must conserve every chip")
}

// TestTimeGroupsKeyOnTheDayToo: 16:00 today and 16:00 tomorrow are different
// work, so the fold must not merge them into one label.
func TestTimeGroupsKeyOnTheDayToo(t *testing.T) {
	today := feedChipAt("10100", 2026, 10, 2, 16, 0)
	tomorrow := feedChipAt("10101", 2026, 10, 3, 16, 0)
	tomorrow.DueDayKey = "care_plan.time.tomorrow"

	groups := foldChipsByTime([]FeedingChip{today, tomorrow})
	require.Len(t, groups, 2,
		"the same clock time on two different days is two sub-groups")
	require.Equal(t, "", groups[0].DayKey)
	require.Equal(t, "care_plan.time.tomorrow", groups[1].DayKey)
	require.Equal(t, "16:00", groups[1].Label)
}

// TestTimeGroupsKeepOrderWhenTheChipsAreNotPreSorted: the fold must never
// reorder what it was given — auto-refresh can hand us chips in another order,
// and a header that disagrees with the first row is worse than no header.
func TestTimeGroupsKeepOrderWhenTheChipsAreNotPreSorted(t *testing.T) {
	groups := foldChipsByTime([]FeedingChip{
		feedChipAt("10100", 2026, 10, 2, 18, 30),
		feedChipAt("10101", 2026, 10, 2, 16, 0),
	})
	require.Len(t, groups, 2)
	require.Equal(t, "18:30", groups[0].Label,
		"the fold is a grouping, not a re-sort; it preserves first-seen order")
	require.Equal(t, "16:00", groups[1].Label)
}

// TestFeedingRowCollapsesOnlyWithSeveralAnimals: a one-animal row is a single
// line with nothing to expand, and collapsing it would hide the very row the
// caregiver has to act on.
func TestFeedingRowCollapsesOnlyWithSeveralAnimals(t *testing.T) {
	one := FeedingGroupView{Chips: []FeedingChip{feedChipAt("1/26", 2026, 10, 2, 9, 0)}}
	finalizeFeedingChips(&one)
	require.False(t, one.Collapsible, "one animal must not collapse")
	require.Equal(t, 1, one.AnimalCount)
	// FirstTimeLabel is computed for every row but only rendered inside the
	// collapsible header; a one-animal row shows no header at all.
	require.NotEmpty(t, one.FirstTimeLabel)

	two := FeedingGroupView{Chips: []FeedingChip{
		feedChipAt("1/26", 2026, 10, 2, 9, 0),
		feedChipAt("2/26", 2026, 10, 2, 9, 0),
	}}
	finalizeFeedingChips(&two)
	require.True(t, two.Collapsible, "two animals are worth a collapsible")
	require.Equal(t, 2, two.AnimalCount)
	require.Equal(t, "09:00", two.FirstTimeLabel,
		"the collapsed header must still state what is due")
}

// TestFeedingRowHeaderStatesTheEarliestTime: a collapsed row must still say
// what is due, or collapsing hides the work instead of tidying it.
func TestFeedingRowHeaderStatesTheEarliestTime(t *testing.T) {
	groups := foldChipsByTime([]FeedingChip{
		feedChipAt("10100", 2026, 10, 2, 16, 0),
		feedChipAt("10101", 2026, 10, 2, 18, 30),
	})
	require.Equal(t, "16:00", groups[0].Label,
		"the header time is the first sub-group, so the two can never disagree")
}

// TestFeedingRowMarkupCollapsesAndFoldsTheTime: the template, in all four
// locale forks. Phase 0b moved the feeding cage×diet table out of
// index.plush.html into the _plan_tier_feed_table partial (still x4 forks),
// so that partial is what this scans.
func TestFeedingRowMarkupCollapsesAndFoldsTheTime(t *testing.T) {
	forks := []string{
		"../templates/care_plan/_plan_tier_feed_table.plush.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.fr.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.de.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)

		// R4-7.14c: a collapse that is CLOSED by default.
		require.Contains(t, raw, `aria-expanded="false"`,
			f+": the animal list must start collapsed")
		require.Contains(t, raw, `class="collapse" id="feed-animals-`,
			f+": a multi-animal row needs its own collapse target")
		require.Contains(t, raw, "fcard.Collapsible",
			f+": the collapse must be gated on having several animals")
		require.Equal(t, 1, strings.Count(raw, `class="collapse" id="feed-animals-`),
			f+": one collapse target per row, not one per time group")

		// R4-7.14b: the chips come from the time sub-groups. bugs.md second
		// batch #3 (2026-10-27) dropped the sub-group time LABEL: no time on
		// top of the animal — each toggle carries its own time (B10-3).
		require.Contains(t, raw, "for (tg) in fcard.TimeGroups",
			f+": the chips must be folded into time sub-groups")
		require.Contains(t, raw, "for (chip) in tg.Chips",
			f+": the chip list must come from a sub-group")
		require.NotContains(t, raw, "plan-time-label",
			f+": bugs.md #3 — no time label above the animal; the button carries the time")
		// B10-3: the per-chip time is BACK — ON the toggle (○ HH:MM ⇄
		// ✓ HH:MM, the §2.3 med-parity nomenclature), not as a repeated
		// text annotation.
		require.Contains(t, raw, `○ <%= chip.DueHM %>`,
			f+": the per-chip toggle must carry the time (B10-3)")
		require.Contains(t, raw, `✓ <%= chip.DueHM %>`,
			f+": the per-chip undo must carry the time (B10-3)")
		require.NotContains(t, raw, "plan-chip-due",
			f+": no chip may carry its own time annotation any more")
	}
}

// TestFeedingCollapseNeedsItsLocaleKeys: an untranslated key renders raw, which
// is R4-7.10 all over again.
func TestFeedingCollapseNeedsItsLocaleKeys(t *testing.T) {
	keys := []string{
		"care_plan.feeding.show_animals",
		"care_plan.feeding.earliest",
		"care_plan.feeding.at_time",
	}
	for _, f := range []string{
		"../locales/care_plan.en-us.yaml",
		"../locales/care_plan.fr.yaml",
		"../locales/care_plan.de.yaml",
		"../locales/care_plan.nl.yaml",
	} {
		raw := readTemplate(t, f)
		for _, k := range keys {
			require.Contains(t, raw, k, f+" is missing "+k)
			require.Contains(t, raw, `id: "`+k+`"`, f+" has no entry for "+k)
		}
		// A translation must not be left empty: Plush renders "" and the row
		// silently loses its "what is due" statement.
		for _, k := range keys {
			i := strings.Index(raw, `id: "`+k+`"`)
			require.GreaterOrEqual(t, i, 0, f+" missing "+k)
			rest := raw[i:]
			j := strings.Index(rest, "translation:")
			require.GreaterOrEqual(t, j, 0, f+" has no translation for "+k)
			nl := strings.Index(rest[j:], "\n")
			if nl < 0 {
				nl = len(rest) - j
			}
			line := rest[j : j+nl]
			require.NotContains(t, line, `translation: ""`,
				f+": "+k+" must have a real translation, not an empty string")
		}
	}
}

// TestFeedingCollapseHasStylesForItsNewParts: a collapse with no chevron rule
// leaves the affordance pointing the wrong way when open.
func TestFeedingCollapseHasStylesForItsNewParts(t *testing.T) {
	css := readTemplate(t, "../assets/css/care-plan.scss")
	for _, sel := range []string{
		".plan-feed-header",
		".plan-time-label",
		".plan-time-group",
	} {
		require.Contains(t, css, sel, "the new feeding markup needs a rule for "+sel)
	}
	require.Contains(t, css, `.plan-feed-header[aria-expanded="false"]`,
		"the collapsed chevron must be rotated, or the affordance lies")
}
