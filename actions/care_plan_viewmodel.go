package actions

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"creaves/models"
	"creaves/models/careplan"
)

// Work-screen view model (Phase 2, bugs.md U2): urgency tiers, positioned
// badges, zone tabs, kind chips, hour chips, auto-refresh indicator. Built
// from the assembled DayPlan plus the request params — pure projection, no
// DB access, so it is unit-testable without a fixture.

// TierDoneIdx is the "terminal status" index used by the card builders
// (nav counters stop below it).
const (
	TierDoneIdx = 3
)

// Work views.
const (
	ViewCompact  = "compact"
	ViewDetailed = "detailed"
)

// TierView is ONE urgency section (§U2, the original work screen):
// a collapsible list of row cards — late · now · later.
type TierView struct {
	Key      string // "late" | "now" | "later" (care_plan.tier.* i18n suffix)
	Count    int
	CountCap string
	Cards    []CardView
	// BadgeClass is the ONE colour policy of the tier header's count pill
	// (late=danger · now=warning · later=success) — every tier panel on the
	// page takes its badge class from here, so the header can never disagree
	// with the urgency tint of the panel it heads.
	BadgeClass string
}

// Tier key constants (care_plan.tier.* i18n suffixes).
const (
	TierLate  = "late"
	TierNow   = "now"
	TierLater = "later"
)

// CardView is one rendered work-screen row.
type CardView struct {
	SourceType  string
	SourceID    string
	SourceName  string
	Detail      string // per-kind content line (bugs.md U5); primary label when set
	ActionKind  string
	AnimalID    int
	AnimalLabel string
	Zone        string
	Cage        string
	DueAt       time.Time
	// §6.2-5 date-aware due label parts (compose with t(): bare time only
	// for today, day word yesterday/tomorrow, short date beyond).
	DueHM        string
	DueDayKey    string // "" | "care_plan.time.yesterday" | "care_plan.time.tomorrow"
	DueShortDate string // "02/01" when beyond one day
	Status       string
	Tier         int // 0 late · 1 now · 2 later · 3 done (§U2 urgency)
	// TierClass is the Bootstrap button class carrying the tier colour —
	// slotTierClass(Tier), the ONE colour policy (R4-1.1) shared with the
	// medication slots: late red · due-now yellow · future white · applied
	// green. The apply/unapply toggles paint with it; the glyph and title
	// stay the accessible label (colour is additive).
	TierClass    string
	Applicable   bool
	Remaining    int    // compact: other open occurrences of the group ("+n")
	RemainingCap string // BadgeCap(Remaining)
	OverriddenBy string // detailed view only
	// Phase 4 / D4 (§3.1): repeating occurrences of the same (source ×
	// animal) merge into ONE line — Slots lists every open current
	// occurrence in due-time order, each with its own tier-coloured
	// toggle (the per-occurrence input modal is kept for weighing /
	// observation). Empty for a single-occurrence row: the line renders
	// its plain apply/undo pair then.
	Slots []ItemSlotView
	// Phase 4 / D4 (§5 level 1): the animal cell shrinks to the
	// year/number (YearNumberFormatted) — the full label stays in the
	// cell's title and the ℹ detail modal. Mirrors the medication row.
	AnimalYear string
	// History rows (§7.1): superseded open occurrences stay recordable —
	// one click = "record anyway (late)" (§6.2-2, A1).
	Superseded       bool
	SupersededReason string
	SupersededBy     string // "15:04" of the replacing occurrence (replaced)
	LateAllowed      bool
	Undoable         bool // applied + an application row exists (A8 undo)
	RecordedLate     bool // applied clearly after its due time (A1 marker)
	// bugs.md R5-2c (D-a): confirm only when fields are needed — weighing
	// (weight) and observation (answer) open the input modal; feeding,
	// medication, care and cleanup toggle instantly.
	NeedsInput bool
	// bugs.md U3/U6 (Phase 4): fast-action links, all with back=<self>.
	AnimalLink      string // /animals/{id}#nav-plan — empty without an animal row
	SourceLink      string // /care_rules/{id} (rule) or /animals/{id}#nav-plan (animal plan)
	FulfillmentLink string // done rows: /cares|/treatments/{fid} — empty otherwise
	// Rev: feeding source names repeat the diet text (the conversion names
	// them "Alimentation — <diet>"), so next to the Detail the name is the
	// same sentence twice, truncated mid-word. When true the row renders
	// the Detail alone (kind + nourriture/régime).
	SourceNameRedundant bool
}

// ItemSlotView is one open occurrence of a merged row-kind line (Phase 4 /
// D4): the per-occurrence toggle of a (source × animal) group, carrying the
// tier colour (§2.1), the due-time label and every data attribute the shared
// toggle / input modal needs. It is the row-kind analogue of MedSlotView —
// colour from the same slotTierClass policy (§2.4), glyph + localized title
// stay the accessible label (§2.3), and data-tier-class rides on the button
// so client-side undo restores the occurrence's CURRENT tier colour (§2.2).
type ItemSlotView struct {
	DueAt        time.Time
	DueAtRFC     string // "2006-01-02T15:04:05Z07:00" (data-due-at)
	DueHM        string // "15:04" — the toggle's visible time label
	DueDayKey    string // "" | yesterday | tomorrow i18n key
	DueShortDate string // "02/01" beyond one day
	// AnimalID keys the apply/unapply ref (source_type × source_id ×
	// animal_id × due_at). The merged-line partials read it from the
	// CARD (one animal per merged group); the cleanup cage row folds
	// SEVERAL animals' occurrences into one time sub-group, so the slot
	// must carry its own (phase 3 / D3).
	AnimalID     int
	Status       string
	Tier         int    // 0 late · 1 now · 2 later (slotTierOf/tierOrder)
	TierClass    string // slotTierClass(Tier) — the toggle's colour (§2.1)
	Applicable   bool
	LateAllowed  bool // past due + out of window → record-anyway (§6.2-2, A1)
	NeedsInput   bool   // weighing / observation → the apply input modal
}

// TierLink is one pill of the summary strip (R4-7.21): a jump link to a
// tier section that EXISTS, repeating that section's OWN count in the unit
// the section itself uses. The strip used to render view.Stats — a
// workload-wide count of open occurrences across days, computed before the
// sections were scoped to today — which made it disagree with the header it
// pointed at.
type TierLink struct {
	ID    string // section element id: "tier-late" (row kinds) / "feed-tier-0"
	Key   string // tier key for care_plan.tier.<Key> ("late"/"now"/"later")
	Num   int
	Cap   string
	Class string
}

// ZoneTab is one zone filter tab with its open count.
type ZoneTab struct {
	Name     string
	Count    int
	CountCap string
}

// KindChip is one action-kind filter chip with its open count.
type KindChip struct {
	Kind     string
	Count    int
	CountCap string
}

// FeedingTimeGroup is the R4-7.14b fold of the per-chip due time. A cage fed
// at 16:00 used to render `10100 16:00 | 10101 16:00 | 10102 16:00` — the
// same time repeated on every animal, which is noise that makes the list
// harder to scan than the single time it encodes. The time becomes the
// sub-group label, stated ONCE, with the animals that share it underneath.
type FeedingTimeGroup struct {
	Label      string // "16:00"
	DayKey     string // care_plan.time.yesterday / .tomorrow ("" when today)
	ShortDate  string // "02/01" beyond one day
	DueAt      time.Time
	Chips      []FeedingChip
	Count      int
	CountCap   string
	Applicable int // chips in this sub-group still actionable
}

// FeedingGroupView is one rendered feeding row (cage × diet, bugs.md U1).
type FeedingGroupView struct {
	Zone            string
	Cage            string
	Food            string
	ForceFeed       bool
	Chips           []FeedingChip
	ApplicableCount int    // chips still applicable (apply-group button)
	ChipRefsJSON    string // JSON item refs of the applicable chips (data-items)
	// R4-7.14b: the chips folded into due-time sub-groups, so the time is
	// stated once per group instead of once per animal. Parallel to Chips —
	// same order, same members — so R4-7.5's `len(fcard.Chips) > 1` checks and
	// the group count keep working unchanged.
	TimeGroups []FeedingTimeGroup
	// R4-7.14c: with more than one animal the list collapses by default. A
	// single-animal row is one line with nothing to expand, so it renders
	// open and these stay empty. The header still has to say what is due, so
	// the group carries the count and the earliest time to show there.
	Collapsible    bool
	AnimalCount    int
	AnimalCountCap string
	FirstTimeLabel string // the earliest sub-group's time, for the header
	// R8-1: the earliest sub-group's day parts, so the COLLAPSED group
	// header can say "from yesterday 18:00" / "from 02/10 18:00" instead of
	// a bare "from 18:00" that hides how late the group is.
	FirstTimeDayKey    string // care_plan.time.yesterday / .tomorrow ("" when today)
	FirstTimeShortDate string // "02/01" beyond one day
	// Rev: every sub-group's time joined ("08:00 · 17:00") for the read-only
	// detail modal — the row shows the earliest, the modal shows them all.
	DetailTimes string
	// R4-4.3: the group's urgency tier — the most urgent NON-superseded
	// chip's tier, the same late → now → later language the medication
	// sections use. -1 when the group has no open work.
	Tier      int
	TierKey   string // care_plan.tier.<key> i18n key ("" when untiered)
	TierCount int    // groups in this tier (the tier section badge)
	TierCap   string // capped TierCount
	// Phase-0b: ONE colour policy (slotTierClass) — the group toggle
	// button's tier colour, set in fillFeedTiers from Tier.
	TierClass string
	// R4-7.14: the EARLIEST due occurrence in the group that is real work
	// (non-superseded + applicable). The sort key inside a tier, so the row
	// the caregiver must act on first is the row they see first. The zero
	// value means the group has no open occurrence.
	FirstDueAt       time.Time
	GroupStatus      string // R4-4.4: the GROUP status (most urgent chip)
	GroupStatusClass string // plan-dot-<status> — ONE dot per cage × diet
	// group=animal rows (guideline §4.2/§4.3): ONE line per (animal × diet)
	// with the same per-occurrence toggle — the tinted animal cell replaces
	// the cage identity, the cage is demoted to a muted caption, and the
	// batch apply-group button disappears (batch is inherently cage-scoped).
	AnimalID    int
	AnimalLabel string
	AnimalYear  string
	AnimalLink  string
}

// MedSlotView is one medication occurrence inside a per-animal medication
// card: the slot (morning/noon/evening) toggle state plus everything the
// toggle/undo/detail UI needs.
type MedSlotView struct {
	Slot       string // "morning" | "noon" | "evening" (i18n key suffix)
	Detail     string // drug — dosage
	SourceName string
	SourceType string
	SourceID   string
	DueAt      time.Time
	DueAtRFC   string
	DueAtHM    string // "15:04"
	// R3-5: day qualifier when the slot is NOT due today — the template
	// composes the label from DueDayKey (yesterday/tomorrow i18n key) or
	// DueShortDate ("02/10"). A multi-day series renders several slots
	// with the same clock time; the qualifier keeps each occurrence
	// identifiable (yesterday's late 08:00 ≠ tomorrow's).
	DueDayKey       string
	DueShortDate    string
	Status          string
	Done            bool // applied/skipped/deferred (has an application row)
	Applied         bool
	Applicable      bool
	CanUndo         bool // applied + fulfillment treatment row still exists
	Overridden      bool
	SourceLink      string
	FulfillmentLink string
	ViewLink        string // unconditional record/treatment view (R5-2b)
	// Dash-7 (round-2 §8.2): dashboard eye deep-link — the animal page
	// resolves the occurrence server-side and opens the shared detail
	// modal on load (bookmarkable, no extra round-trip).
	DeepLink string
	// §6.2-2 (round-2, A1): past-due, unapplied, out-of-apply-window —
	// the dimmed series button stays clickable and records the missed
	// occurrence with the explicit late acknowledgment.
	LateAllowed bool
	// R4-1.1: urgency tier of this occurrence (tierOrder): 0 late/missing,
	// 1 due-now, 2 scheduled/future, 3 done. Terminal states keep their
	// own glyph treatment and are never re-coloured by tier.
	Tier int
	// TierClass is the Bootstrap button class carrying the tier colour
	// (R4-1.1): late red · due-now yellow · future white (bordered) ·
	// applied green. Colour is additive — the glyph and title stay.
	TierClass string
	// R4-1.2: Urgent marks the series' most urgent open slot (the one
	// that placed the series in its tier) — red ring, dominant.
	Urgent bool
	// R4-1.2: Future marks an open later-tier slot inside a series whose
	// urgent slot is late/due-now — recedes (reduced opacity).
	Future bool
}

// MedSeriesRow is one visual row of a series (≤3 slots).
type MedSeriesRow struct {
	// DividerBefore marks a slot-bucket change relative to the previous
	// row (the template draws the thin divider) — computed in Go because
	// plush cannot index Rows[ri-1] (round-3 build fix).
	DividerBefore bool
	Slots         []MedSlotView
}

// MedSeriesView is one drug line of a medication card (round-2 §8.3,
// Dash-5/Dash-8, CP1, T2): every occurrence of one animal sharing the
// same (drug, dosage) label, bucket-ordered (morning → noon → evening)
// and chunked 3 per row — a bucket change always starts a new row. The
// shared `_med_series` partial renders it identically on the care plan,
// the dashboard and the animal Treatment tab.
type MedSeriesView struct {
	Key        string // merge key: drug — dosage
	Label      string // display: drug — dosage
	Rows       []MedSeriesRow
	Tier       int       // most urgent open slot's tier (R3-5 tier placement)
	FirstDueAt time.Time // earliest open slot's due time (tier sort)
}

// MedTierLine is one rendered medication line of the work screen (R3-5):
// `<animal> — <series.Label> | hour toggles`, placed in its urgency tier.
// It is a MedGroupView narrowed to exactly one series so the shared
// `_med_series` partial renders the line unchanged.
type MedTierLine = MedGroupView

// MedGroupView is one rendered per-animal medication card: all medication
// occurrences of the day for one animal, in slot order — the single view
// of what the animal must receive.
type MedGroupView struct {
	AnimalID    int
	AnimalLabel string
	AnimalLink  string
	// Dash-2 (round-2 §8.1): year-number-only button text — sibling-table
	// parity; the full label stays in the detail modal.
	AnimalYear string
	Species    string // raw species (template translates via tspecies)
	Gender     string // raw gender "M"/"F"/"" (template renders ♂/♀/×)
	Zone       string
	Cage       string
	// Round-2 §4b-A2 (Dash-9): animal outtaken today — depupdate parity
	// (outtaken row class + dove badge); set only by the today-outtaken
	// assembly scope.
	OuttakenToday bool
	Slots         []MedSlotView
	OpenCount     int // applicable, not yet recorded
	// Round-2 §8.3: slots merged into (drug, dosage) series — the shared
	// `_med_series` partial renders these (dashboard + animal Treatment tab).
	Series []MedSeriesView
}

// CareTimeGroup is the cleanup analogue of FeedingTimeGroup (guideline
// §3.2): one time sub-group of a cleanup row — the due time stated ONCE
// as the sub-group label, with one per-occurrence toggle (ItemSlotView)
// for every occurrence that shares it.
type CareTimeGroup struct {
	Label      string // "16:00"
	DayKey     string // care_plan.time.yesterday / .tomorrow ("" when today)
	ShortDate  string // "02/01" beyond one day
	DueAt      time.Time
	Slots      []ItemSlotView
	Count      int
	CountCap   string
	Applicable int // occurrences in this sub-group still actionable
}

// CareView is one rendered cleanup row (guideline §3/§4): ONE line per
// (source × cage) in the default cage grouping, or one line per
// (source × animal) under group=animal. The line carries its open
// occurrences as time-grouped toggle slots (per-occurrence toggles,
// tier-coloured like every other kind), plus the group-apply batch
// refs (cage grouping keeps the apply-cage affordance).
type CareView struct {
	Zone       string
	Cage       string
	SourceName string
	SourceLink string
	// SourceType/SourceID ride on every per-occurrence toggle (the apply
	// ref) — the slots alone do not carry them (same shape as CardView's
	// merged Slots, which read them from the card).
	SourceType      string
	SourceID        string
	Count           int    // open, non-scheduled occurrences on the row
	ChipRefsJSON    string // JSON item refs of the applicable items (data-items)
	ApplicableCount int
	LateCount       int // applicable occurrences already past due (late tier)
	// group=animal rows: the tinted animal cell (§4.3) — year/number
	// display (§5 level 1), full label on the title, plan-tab link.
	AnimalID    int
	AnimalLabel string
	AnimalYear  string
	AnimalLink  string
	// §3.2: the occurrences folded into due-time sub-groups — the time is
	// the sub-group label, stated once; each slot is one toggle.
	TimeGroups []CareTimeGroup
	// Tier/TierClass/GroupStatus: the row's urgency tier — its most urgent
	// open occurrence (the same most-urgent rule as feedingGroupTier) —
	// the ONE slotTierClass colour policy and the group dot. Stamped by
	// fillCareTiers.
	Tier             int
	TierClass        string
	GroupStatus      string
	GroupStatusClass string
	FirstDueAt       time.Time // earliest applicable open occurrence (tier sort)
}

// DayPlanView is the full work-screen context value. Layout (bugs.md
// round-3 fixes 2-6 — the ORIGINAL work screen, restored): urgency tier
// lists (late/now/later) as plain tables — one row per open occurrence
// group INCLUDING medication (day-aware due labels; no per-hour button
// series) — the grouped feeding/cleanup sections as lists, history last.
type DayPlanView struct {
	// Tiers[i] = urgency section: 0 late · 1 now · 2 later (§U2).
	Tiers [3]TierView
	// R4-7.21: TierHasWork[i] reports whether urgency tier i has anything to
	// render. The three row-kind tier sections used to render UNCONDITIONALLY,
	// so an empty one appeared under a "0" badge (measured: a "Now 0" header
	// with no rows, on both the medication and the observation page) — a
	// header that promises nothing, and a jump target for the summary strip's
	// "Now" pill. Feeding already guarded itself on len(FeedTiers[i]).
	TierHasWork [3]bool
	// TierLinks is the rendered summary strip, in order (R4-7.21).
	TierLinks []TierLink
	// MedTiers[i] = the medication-kind urgency sections (R3-5): one line
	// per (animal × drug series) — `<animal> — <medication> | hour toggles`
	// — instead of the ordinary table rows the other kinds render.
	MedTiers [3][]MedTierLine
	// R9-2: MedDone holds the fully-applied medication series — every
	// repeated occurrence recorded (green ✓ toggles). A series is Done ONLY
	// when 100% of its occurrences are applied; a partially-applied one
	// stays in late/now/later by its next OPEN slot. MedDoneCount/
	// MedDoneCountCap feed the Done collapsible's badge (same unit as the
	// lines it lists: one per fully-done animal × drug series).
	MedDone         []MedTierLine
	MedDoneCount    int
	MedDoneCountCap string
	// R4-7.15: the medication page's animal COLUMN. It had no width and no
	// background at all, so the labels stopped reading as a column — measured
	// 13 distinct widths across 36 cells (168..181px, one 239px outlier) and
	// `bg=rgba(0,0,0,0)` on every one.
	//
	// One shared width in `ch`, taken from the LONGEST label on the page. A
	// hard-coded px would either cut the longest label or leave the short ones
	// ragged, and R4-7.24 ("never cut a description on any page") applies to
	// the animal label too. `ch` keeps it proportional to the rendered font,
	// and one value for the whole page is what makes the cells equal — a
	// per-row min-width would still leave the longest row wider.
	MedAnimalColCh int
	Feedings       []FeedingGroupView // cage × diet groups (feeding kind only)
	Cares          []CareView         // cage cleanup groups (cleanup kind only)
	// CareTiers[i] = the cleanup rows of urgency tier i (guideline §1 —
	// the shared generic distribution, phase-0b §6). CareTierOpen[i]
	// counts the tier's OPEN APPLICABLE OCCURRENCES — the summary-strip
	// unit (§1.4, R4-1.3/R4-7.21 precedent).
	CareTiers        [3][]CareView
	CareTierOpen     [3]int
	CareTierOpenCap  [3]string
	History          []CardView // terminal + superseded rows, subdued
	Zones            []ZoneTab  // without the "all" entry (rendered by the template)
	Kinds            []KindChip
	UpdatedAt        string // HH:MM of render (auto-refresh indicator, §10-CP6c)
	View             string
	// Group is the feeding/cleanup grouping level (guideline §4.2):
	// "cage" (default — one line per cage × diet/cleanup) or "animal"
	// (one line per animal, same per-occurrence toggles).
	Group string
	// Detailed switches row density only: compact folds each
	// (source × animal) group to its next open action ("+N" badge),
	// detailed lists every open current occurrence.
	Detailed   bool
	Stats      FilterStats // summary strip, counted in occurrences
	Zone       string
	Kind       string
	SelfPath   string // /care_plan?view=…&zone=…&kind=… — back target of every card link
	ZoneAll    int    // open count across all zones (for the active kind)
	ZoneAllCap string
	// ZoneCap is the badge capacity of the zone TOGGLE BUTTON (S2: every
	// badge equals the visible count): the selected zone's own cap, or the
	// all-zones cap when no zone is selected. The per-item dropdown entries
	// always show their own cap.
	ZoneCap string
	// R4-1.3: MedTierOpen[i] counts the OPEN OCCURRENCES rendered in
	// medication tier i (a series may contribute several), capped in
	// MedTierOpenCap — the same unit as the summary strip's Stats, so the
	// section badge and the strip can never disagree.
	MedTierOpen    [3]int
	MedTierOpenCap [3]string
	// R4-4.3: FeedTiers[i] = the feeding cards of urgency tier i (0 late
	// · 1 now · 2 later), the same visual language as the medication
	// tiers — feeding carries most of the workload, so urgency is the
	// FIRST level, not a per-chip afterthought. v.Feedings stays the flat
	// filtered set (nav + filters).
	FeedTiers     [3][]FeedingGroupView
	FeedTierCount [3]int
	FeedTierCap   [3]string
	// R4-7.21: FeedTierOpen[i] counts the OPEN APPLICABLE OCCURRENCES of the
	// feeding groups in tier i — the unit every other tier badge on this page
	// already speaks. FeedTierCount counts GROUPS; a group holds one chip per
	// animal, so the two disagreed by a factor of the cage size (measured:
	// 162 groups / 218 occurrences) and switching the kind tab silently
	// changed what the number meant. R4-1.3 settled the unit for medication
	// ("one unit, one number, everywhere"); feeding now follows it, and the
	// summary strip — which counts occurrences via statsOf — agrees with the
	// section header instead of contradicting it.
	FeedTierOpen    [3]int
	FeedTierOpenCap [3]string
}

// FilterStats is the CP4/D6 summary of one rendered view, counted in
// OCCURRENCES after the zone×kind filter: compact and detailed report
// the same numbers (S3) and every counted occurrence is visible either
// as its own button/chip/row or folded into a "+N" remaining badge.
type FilterStats struct {
	Late     int
	Now      int
	Later    int
	LateCap  string
	NowCap   string
	LaterCap string
}

// tierOrder maps a status to its tier index (§U2): missing|late → late,
// due → now, scheduled → later, applied|skipped|deferred → done.
// Overridden is excluded from the work screen (animal Plan tab only).
func tierOrder(s careplan.PlanStatus) int {
	switch s {
	case careplan.StatusMissing, careplan.StatusLate:
		return 0
	case careplan.StatusDue:
		return 1
	case careplan.StatusScheduled:
		return 2
	case careplan.StatusApplied, careplan.StatusSkipped, careplan.StatusDeferred:
		return 3
	}
	return -1
}

// openStatusAction mirrors the engine's open set for counter badges.
func openStatusAction(s careplan.PlanStatus) bool {
	switch s {
	case careplan.StatusScheduled, careplan.StatusDue, careplan.StatusLate, careplan.StatusMissing:
		return true
	}
	return false
}

// actionKinds lists the known kinds in display order (chip bar).
var actionKinds = []string{
	careplan.KindFeeding,
	careplan.KindMedication,
	careplan.KindCare,
	careplan.KindCleanup,
	careplan.KindWeighing,
	careplan.KindObservation,
}

// BuildDayPlanView projects the day plan into the work-screen view model
// (the ORIGINAL tier layout, restored — fixes 2-6):
//
//  1. engine output (all items, no pre-narrowing)
//  2. display split: tier rows (every non-grouped kind INCLUDING
//     medication), grouped feeding/cleanup lists, history
//  3. ONE zone×kind filter pass (the handler whitelist-validates zone)
//  4. tiers filled from the filtered rows (late → now → later, each
//     sorted by due time) + FilterStats in occurrences
//  5. the zone nav matrix from the unfiltered open sets
//
// view selects the row density: compact folds each (source × animal)
// group to its next open action ("+N" badge), detailed lists every open
// current occurrence. now drives UpdatedAt. back (R5-2d, optional) is the
// page's own incoming back target (sanitized); it is embedded in the self
// URL so every card link chains the ORIGINAL origin through the round trip.
func BuildDayPlanView(plan *DayPlan, view, zone, kind, group string, now time.Time, back ...string) *DayPlanView {
	backIn := ""
	if len(back) > 0 {
		backIn = back[0]
	}
	detailed := view == ViewDetailed
	v := &DayPlanView{
		View:      view,
		Detailed:  detailed,
		Zone:      zone,
		Kind:      kind,
		Group:     group,
		UpdatedAt: now.Format("15:04"),
		SelfPath:  planSelfPath(view, zone, kind, group, backIn),
	}

	// Stage 1–2: build every section UNFILTERED (engine → display layer).
	rows := v.tierRows(plan, detailed)
	history := v.historyRows(plan, detailed)
	feeds := feedingViewsOf(plan, v.SelfPath, group)
	cares := careViewsOf(plan, v.SelfPath, group)

	// Stage 5 for the nav: the zone matrix comes from the unfiltered open
	// card sets — the zone item of zone Z counts the rows the ACTIVE kind
	// renders there (and symmetrically). Kind tabs have NO "all" entry
	// (fix 5); the handler picks the active kind.
	v.Zones, v.ZoneAll = navZoneTabs(rows, feeds, cares, kind)
	v.Kinds = navKindTabs(rows, feeds, cares, zone)
	v.ZoneAllCap = BadgeCap(v.ZoneAll)
	v.ZoneCap = v.ZoneAllCap
	if zone != "" {
		for _, zt := range v.Zones {
			if zt.Name == zone {
				v.ZoneCap = zt.CountCap
				break
			}
		}
	}

	// Stage 3: one filter pass over every collection.
	openRows := filterCards(rows, zone, kind)
	v.History = filterCards(history, zone, kind)
	v.Feedings = filterFeedings(feeds, zone, kind)
	v.Cares = filterCares(cares, zone, kind)
	// R4-4.3: feeding cards are grouped into the same late → now → later
	// tiers as every other kind; R4-4.4: each card carries ONE group dot.
	fillFeedTiers(v)
	// Phase-0b §6: the cleanup rows join the same generic distribution.
	fillCareTiers(v)

	// Stage 4: tiers (late → now → later, due-time sorted) + the summary
	// strip (occurrences, density-independent).
	fillTiers(v, openRows)
	// R3-5: the medication kind renders hour-toggle lines instead of the
	// ordinary table rows — one `<animal> — <series> | [HH:MM]…` line per
	// (animal × drug series), placed in its most urgent open slot's tier.
	if kind == careplan.KindMedication {
		v.fillMedTiers(v.buildMedGroups(plan, zone, false), plan.Now)
	}
	v.Stats = statsOf(plan, zone, kind, plan.Now)
	fillTierHasWork(v, kind)

	return v
}

// fillTierHasWork records, per urgency tier, whether anything renders in it.
// Feeding fills FeedTiers, cleanup fills CareTiers, medication fills
// MedTiers, every other row kind fills Tiers[i].Cards. The summary strip is
// gated on this too (R4-7.21): a pill whose section does not exist is a dead
// jump target that also advertised work the page never shows.
func fillTierHasWork(v *DayPlanView, kind string) {
	for i := 0; i < 3; i++ {
		switch {
		case kind == careplan.KindFeeding:
			v.TierHasWork[i] = len(v.FeedTiers[i]) > 0
		case kind == careplan.KindCleanup:
			v.TierHasWork[i] = len(v.CareTiers[i]) > 0
		case kind == careplan.KindMedication:
			v.TierHasWork[i] = len(v.MedTiers[i]) > 0
		default:
			v.TierHasWork[i] = len(v.Tiers[i].Cards) > 0
		}
	}
	v.TierLinks = tierLinks(v, kind)
}

// tierLinks builds the summary strip: one pill per tier that renders, in
// late → now → later order, each pointing at the real section id and showing
// the same number as that section's header.
func tierLinks(v *DayPlanView, kind string) []TierLink {
	keys := [3]string{TierLate, TierNow, TierLater}
	classes := [3]string{"badge-danger mr-1", "badge-warning mr-1", "badge-success mr-1"}
	feed := kind == careplan.KindFeeding
	cleanup := kind == careplan.KindCleanup
	out := make([]TierLink, 0, 3)
	for i := 0; i < 3; i++ {
		if !v.TierHasWork[i] {
			continue
		}
		tl := TierLink{Key: keys[i], Class: classes[i]}
		switch {
		case feed:
			tl.ID = "feed-tier-" + fmt.Sprint(i)
			tl.Num, tl.Cap = v.FeedTierOpen[i], v.FeedTierOpenCap[i]
		case cleanup:
			tl.ID = "care-tier-" + fmt.Sprint(i)
			tl.Num, tl.Cap = v.CareTierOpen[i], v.CareTierOpenCap[i]
		case kind == careplan.KindMedication:
			tl.ID = "tier-" + keys[i]
			tl.Num, tl.Cap = v.MedTierOpen[i], v.MedTierOpenCap[i]
		default:
			tl.ID = "tier-" + keys[i]
			tl.Num, tl.Cap = v.Tiers[i].Count, v.Tiers[i].CountCap
		}
		out = append(out, tl)
	}
	return out
}

// fillMedTiers distributes per-animal drug series over the three urgency
// tiers (R3-5): a series lands in its most urgent OPEN slot's tier and
// sorts by that slot's due time inside the tier (oldest first).
func (v *DayPlanView) fillMedTiers(groups []MedGroupView, now time.Time) {
	for _, g := range groups {
		for _, series := range g.Series {
			if seriesTierOrMinus(series) < 0 {
				v.collectDoneSeries(g, series, now)
				continue
			}
			// R4-7.7: the work screen shows TODAY. A series with no open
			// occurrence left for today disappears (in the evening the table
			// is empty), and at most ONE later occurrence survives — the
			// nearest — and only when it is nearer than the pending late
			// entry, so a far-off tomorrow slot never competes with work
			// that is due now. Everything else waits for its own day.
			series = scopeSeriesToToday(series, now)
			if len(series.Rows) == 0 {
				continue
			}
			if seriesTierOrMinus(series) < 0 {
				// R9-2: today's occurrences of this series are ALL applied
				// (its only open slots are future days, which scoping
				// dropped) — the series is Done FOR TODAY and joins the Done
				// tier showing today's applied record.
				g.Series = []MedSeriesView{series}
				v.MedDone = append(v.MedDone, g)
				continue
			}
			tier, first := seriesTier(series)
			series.Tier = tier
			series.FirstDueAt = first
			// R4-1.2: the slot that placed the series in its tier is the
			// urgent one (red ring); its later-tier open siblings recede.
			markSeriesUrgency(&series, tier)
			// R4-1.3: the badge counts the tier's occurrences across ALL
			// medication series — a series that sits in the Late section may
			// still own future slots, and those occurrences are visible on
			// this very page. One unit, one number, everywhere.
			addTierOpenCounts(v, series)
			g.Series = []MedSeriesView{series}
			v.MedTiers[tier] = append(v.MedTiers[tier], g)
		}
	}
	for i := 0; i < 3; i++ {
		v.MedTierOpenCap[i] = BadgeCap(v.MedTierOpen[i])
	}
	for i := 0; i < 3; i++ {
		sort.SliceStable(v.MedTiers[i], func(a, b int) bool {
			return v.MedTiers[i][a].Series[0].FirstDueAt.Before(v.MedTiers[i][b].Series[0].FirstDueAt)
		})
	}
	// R9-2: the Done section lists each fully-done series once, in a
	// stable order (by its earliest applied occurrence) so the refresh
	// re-sorts in place. The badge counts these series.
	sort.SliceStable(v.MedDone, func(a, b int) bool {
		return medDoneFirstDue(v.MedDone[a]).Before(medDoneFirstDue(v.MedDone[b]))
	})
	v.MedDoneCount = len(v.MedDone)
	v.MedDoneCountCap = BadgeCap(v.MedDoneCount)
	v.MedAnimalColCh = medAnimalColCh(v.MedTiers)
}

// collectDoneSeries routes a fully-terminal series to the Done tier (R9-2):
// every occurrence applied. When the series still owns a visible occurrence
// for today it is a DONE series and joins MedDone — it no longer disappears
// in the evening. With nothing due today it leaves the screen entirely.
func (v *DayPlanView) collectDoneSeries(g MedGroupView, series MedSeriesView, now time.Time) {
	series = scopeSeriesToToday(series, now)
	if len(series.Rows) == 0 {
		return // nothing of this series belongs to today
	}
	g.Series = []MedSeriesView{series}
	v.MedDone = append(v.MedDone, g)
}

// medDoneFirstDue is the sort key of a Done line: the earliest occurrence
// of its series (all applied) — the order the work was recorded.
func medDoneFirstDue(line MedTierLine) time.Time {
	var first time.Time
	for _, s := range flatSlots(line.Series[0]) {
		if first.IsZero() || s.DueAt.Before(first) {
			first = s.DueAt
		}
	}
	return first
}

// medAnimalColCh is the shared animal-column width for the whole medication
// page: the longest label across ALL THREE tiers, in `ch` plus one spare
// column. A label longer than this is never cut — the cell carries no
// overflow rule, it simply grows.
func medAnimalColCh(tiers [3][]MedTierLine) int {
	// One spare column: `ch` is the width of "0", so a label made of narrow
	// glyphs (digits, spaces, the `·` separators) renders WIDER than its rune
	// count in ch. The spare column absorbs that; without it a label right at
	// the limit would be clipped, and R4-7.24 forbids clipping.
	const room = 1
	widest := 0
	for i := range tiers {
		for _, g := range tiers[i] {
			if n := utf8.RuneCountInString(g.AnimalLabel); n > widest {
				widest = n
			}
		}
	}
	if widest == 0 {
		return 0
	}
	return widest + room
}

// seriesTier reports the tier index of a series' most urgent OPEN slot
// (0 late · 1 now · 2 later) and that slot's due time; -1 when the
// series has no open slot left (all applied/skipped/deferred/overridden).
func seriesTier(series MedSeriesView) (int, time.Time) {
	best := 3
	var first time.Time
	for _, row := range series.Rows {
		for _, s := range row.Slots {
			best, first = betterTierSlot(best, first, s)
		}
	}
	if best > 2 {
		return -1, time.Time{}
	}
	return best, first
}

// betterTierSlot folds one slot into the running (best tier, first due)
// pair: closed slots and non-tier statuses are skipped; a slot wins when
// its tier is strictly more urgent, or same-tier and earlier due.
func betterTierSlot(best int, first time.Time, s MedSlotView) (int, time.Time) {
	if s.Done || s.Overridden {
		return best, first
	}
	t := tierOrder(careplan.PlanStatus(s.Status))
	if t < 0 || t > 2 {
		return best, first
	}
	if t < best || (t == best && (first.IsZero() || s.DueAt.Before(first))) {
		return t, s.DueAt
	}
	return best, first
}

// markSeriesUrgency flags, in place, the slot that placed the series in
// its tier (`Urgent`) and the open later-tier siblings (`Future`) —
// R4-1.2: within one line the urgent occurrence is visually dominant,
// the future ones recede. The urgent slot is the earliest same-tier open
// slot, matching betterTierSlot's choice.
func markSeriesUrgency(series *MedSeriesView, tier int) {
	// Single pass: flag every open later-tier sibling as Future and keep
	// the EARLIEST same-tier open slot — the exact one betterTierSlot used
	// to place the series (ties: first encountered).
	var urgent *MedSlotView
	for ri := range series.Rows {
		for si := range series.Rows[ri].Slots {
			s := &series.Rows[ri].Slots[si]
			if s.Done || s.Overridden {
				continue
			}
			if s.Tier > tier {
				s.Future = true
			} else if s.Tier == tier && (urgent == nil || s.DueAt.Before(urgent.DueAt)) {
				urgent = s
			}
		}
	}
	if urgent != nil {
		urgent.Urgent = true
	}
}

// addTierOpenCounts folds one series' open applicable occurrences into the
// per-tier badge counters — R4-1.3: the medication section badges count
// OCCURRENCES (the same population and unit as the summary strip's
// statsOf: open, current, applicable), never series, so "Late 22" in a
// header and "Late 20" in the strip can never disagree.
func addTierOpenCounts(v *DayPlanView, series MedSeriesView) {
	for _, row := range series.Rows {
		for _, slot := range row.Slots {
			if slot.Done || slot.Overridden || !slot.Applicable {
				continue
			}
			if slot.Tier >= 0 && slot.Tier <= 2 {
				v.MedTierOpen[slot.Tier]++
			}
		}
	}
}

// feedTierOpenCount totals the OPEN APPLICABLE OCCURRENCES of a feeding
// tier — the chips its apply buttons would record. R4-7.21: the tier header
// badge counts these, not the groups, so the number never changes meaning
// when the caregiver switches the kind tab.
func feedTierOpenCount(groups []FeedingGroupView) int {
	n := 0
	for _, g := range groups {
		n += g.ApplicableCount
	}
	return n
}

// fillFeedTiers distributes the filtered feeding cards over the three
// urgency tiers (R4-4.3) and stamps each card with its GROUP status —
// the most urgent non-superseded chip (R4-4.4). Cards with no open work
// keep tier -1 and stay out of the tier sections.
func fillFeedTiers(v *DayPlanView) {
	keys := []string{TierLate, TierNow, TierLater}
	tiered := make([]FeedingGroupView, 0, len(v.Feedings))
	for _, f := range v.Feedings {
		tier, status := feedingGroupTier(f)
		f.Tier = tier
		f.TierClass = slotTierClass(tier)
		f.GroupStatus = status
		f.GroupStatusClass = "plan-dot-" + status
		if tier < 0 || tier > 2 {
			continue
		}
		f.TierKey = keys[tier]
		f.FirstDueAt = firstChipDue(f)
		tiered = append(tiered, f)
	}
	v.FeedTiers = fillTierBuckets(tiered, func(fa, fb FeedingGroupView) bool {
		if fa.FirstDueAt.IsZero() != fb.FirstDueAt.IsZero() {
			return fb.FirstDueAt.IsZero()
		}
		if fa.FirstDueAt.Equal(fb.FirstDueAt) {
			if fa.Cage != fb.Cage {
				return fa.Cage < fb.Cage
			}
			return fa.Food < fb.Food
		}
		return fa.FirstDueAt.Before(fb.FirstDueAt)
	})
	for i := range v.FeedTiers {
		v.FeedTierCount[i] = len(v.FeedTiers[i])
		v.FeedTierCap[i] = BadgeCap(v.FeedTierCount[i])
		v.FeedTierOpen[i] = feedTierOpenCount(v.FeedTiers[i])
		v.FeedTierOpenCap[i] = BadgeCap(v.FeedTierOpen[i])
		// R4-7.14: within a tier, the row the caregiver must act on FIRST
		// comes first — the generic fill's sort above: earliest-due first,
		// zero-due (no open occurrence) last, cage/food tiebreak, stable
		// between two refreshes.
	}
}

// firstChipDue reports the earliest occurrence in the group that is real WORK:
// a non-superseded, applicable chip. Superseded chips are history and a
// terminal chip is nothing to do, so neither can lead the order. The zero time
// means "no open occurrence" and sorts last.
func firstChipDue(f FeedingGroupView) time.Time {
	var first time.Time
	for _, c := range f.Chips {
		if c.Superseded || !c.Applicable {
			continue
		}
		if first.IsZero() || c.DueAt.Before(first) {
			first = c.DueAt
		}
	}
	return first
}

// feedingGroupTier reports the group's urgency tier and the status of its
// most urgent non-superseded chip (R4-4.4: ONE dot per cage × diet —
// identical cage × diet always reads identical). Superseded chips are
// info, not work, so they never set the group state. -1 / "" when every
// chip is superseded or terminal.
func feedingGroupTier(f FeedingGroupView) (int, string) {
	best := 3
	status := ""
	for _, c := range f.Chips {
		if c.Superseded {
			continue
		}
		t := tierOrder(careplan.PlanStatus(c.Status))
		if t < 0 || t > 2 {
			continue
		}
		if t < best || (t == best && status == "") {
			best = t
			status = c.Status
		}
	}
	if best > 2 {
		return -1, ""
	}
	return best, status
}

// Openable is anything the generic tier-fill can distribute over the
// three urgency tiers (phase-0b §6): a tier index (-1 or >2 = no open
// work, skipped), the earliest open due time (the zero time sorts last)
// and the open-occurrence count it contributes to the tier badge.
// Implemented by CardView (row kinds), FeedingGroupView and CareView.
type Openable interface {
	TierOf() int
	FirstDue() time.Time
	OpenCount() int
}

// TierOf reports the row's urgency tier — recomputed from the status,
// the exact semantics fillTiers always had (never the cached field).
func (cv CardView) TierOf() int { return tierOrder(careplan.PlanStatus(cv.Status)) }

// FirstDue is the row's due time (tier rows always carry one).
func (cv CardView) FirstDue() time.Time { return cv.DueAt }

// OpenCount is 1: one rendered row is one open action (the row-kind
// tier badges count rows, R4-1.3 — unlike med/feed occurrence counts).
func (cv CardView) OpenCount() int { return 1 }

// TierOf reports the feeding group's tier, stamped by fillFeedTiers.
func (f FeedingGroupView) TierOf() int { return f.Tier }

// FirstDue is the group's earliest open occurrence (zero sorts last).
func (f FeedingGroupView) FirstDue() time.Time { return f.FirstDueAt }

// OpenCount is the group's applicable chips (occurrences, R4-7.21).
func (f FeedingGroupView) OpenCount() int { return f.ApplicableCount }

// TierOf reports the care row's urgency tier, stamped by fillCareTiers
// (the most urgent open occurrence of the row).
func (cv CareView) TierOf() int { return cv.Tier }

// FirstDue is the row's earliest open occurrence (zero sorts last).
func (cv CareView) FirstDue() time.Time { return cv.FirstDueAt }

// OpenCount is the row's applicable occurrences.
func (cv CareView) OpenCount() int { return cv.ApplicableCount }

// fillTierBuckets is the ONE tier distribution (phase-0b §6): items are
// appended to their TierOf() bucket (-1/>2 skipped) and each bucket is
// stably sorted with less. Used by fillTiers (row kinds), fillFeedTiers
// and fillCareTiers — the per-kind extras (TierKey stamping, badge
// counters) stay at the call sites.
func fillTierBuckets[T Openable](items []T, less func(a, b T) bool) [3][]T {
	var out [3][]T
	for _, it := range items {
		t := it.TierOf()
		if t < 0 || t > 2 {
			continue
		}
		out[t] = append(out[t], it)
	}
	for i := range out {
		b := out[i]
		sort.SliceStable(b, func(x, y int) bool { return less(b[x], b[y]) })
	}
	return out
}

// fillTiers distributes the filtered open rows over the three urgency
// tiers and sorts each tier by due time (§U2 screen order: late → now →
// later, oldest first within a tier).
func fillTiers(v *DayPlanView, rows []CardView) {
	buckets := fillTierBuckets(rows, func(a, b CardView) bool {
		return a.DueAt.Before(b.DueAt)
	})
	keys := []string{TierLate, TierNow, TierLater}
	badges := []string{"badge-danger", "badge-warning", "badge-success"}
	for i := range v.Tiers {
		v.Tiers[i].Cards = buckets[i]
		v.Tiers[i].Key = keys[i]
		v.Tiers[i].BadgeClass = badges[i]
		v.Tiers[i].Count = len(v.Tiers[i].Cards)
		v.Tiers[i].CountCap = BadgeCap(v.Tiers[i].Count)
	}
}

// fillCareTiers stamps every cleanup row with its urgency tier (its MOST
// URGENT open occurrence — the same most-urgent rule as feedingGroupTier),
// its group dot and its earliest open due time, then distributes the rows
// over the three urgency tiers via the shared generic distribution
// (phase-0b §6). The per-tier open counters speak OCCURRENCES — the same
// unit as the summary strip (§1.4).
func fillCareTiers(v *DayPlanView) {
	tiered := make([]CareView, 0, len(v.Cares))
	for _, cv := range v.Cares {
		tier, status := careRowTier(cv)
		cv.Tier = tier
		cv.TierClass = slotTierClass(tier)
		cv.GroupStatus = status
		cv.GroupStatusClass = "plan-dot-" + status
		cv.FirstDueAt = firstCareDue(cv)
		if tier < 0 || tier > 2 {
			continue
		}
		tiered = append(tiered, cv)
	}
	v.CareTiers = fillTierBuckets(tiered, func(a, b CareView) bool {
		if a.FirstDueAt.IsZero() != b.FirstDueAt.IsZero() {
			return b.FirstDueAt.IsZero()
		}
		if !a.FirstDueAt.Equal(b.FirstDueAt) {
			return a.FirstDueAt.Before(b.FirstDueAt)
		}
		if a.Cage != b.Cage {
			return a.Cage < b.Cage
		}
		return a.SourceName < b.SourceName
	})
	for i := range v.CareTiers {
		v.CareTierOpen[i] = careTierOpenCount(v.CareTiers[i])
		v.CareTierOpenCap[i] = BadgeCap(v.CareTierOpen[i])
	}
}

// careRowTier reports the cleanup row's urgency tier and the status of its
// most urgent open occurrence — the GroupStatus dot's one status (R4-4.4
// parity). -1 / "" when the row has no open work (care rows are built only
// with open applicable work, so this is the defensive path).
func careRowTier(cv CareView) (int, string) {
	best := 3
	status := ""
	for _, tg := range cv.TimeGroups {
		for _, s := range tg.Slots {
			t := tierOrder(careplan.PlanStatus(s.Status))
			if t < 0 || t > 2 {
				continue
			}
			if t < best || (t == best && status == "") {
				best = t
				status = s.Status
			}
		}
	}
	if best > 2 {
		return -1, ""
	}
	return best, status
}

// firstCareDue reports the row's earliest APPLICABLE open occurrence — the
// tier sort key (firstChipDue parity). The zero time sorts last.
func firstCareDue(cv CareView) time.Time {
	var first time.Time
	for _, tg := range cv.TimeGroups {
		for _, s := range tg.Slots {
			if !s.Applicable {
				continue
			}
			if first.IsZero() || s.DueAt.Before(first) {
				first = s.DueAt
			}
		}
	}
	return first
}

// careTierOpenCount totals the OPEN APPLICABLE OCCURRENCES of a cleanup
// tier — the unit the summary strip speaks (feedTierOpenCount parity).
func careTierOpenCount(rows []CareView) int {
	n := 0
	for _, cv := range rows {
		n += cv.ApplicableCount
	}
	return n
}

// groupedKind reports whether a kind renders as a grouped LIST section
// (feeding: cage × diet; cleanup: cage) instead of tier rows.
func groupedKind(kind string) bool {
	g := careplan.GroupingFor(kind)
	return g == careplan.GroupingCageDiet || g == careplan.GroupingCage
}

// isTierRow reports whether an occurrence is tier-row material right now:
// a non-grouped kind (medication, care, weighing, observation — feeding
// and cleanup render as grouped lists), open, and still CURRENT (§6.2-1) —
// superseded occurrences are history, never work.
func isTierRow(it *careplan.PlanItem, now time.Time) bool {
	src := it.Occurrence.Source
	return src != nil && !groupedKind(src.ActionKind()) &&
		openStatusAction(it.Status) && IsCurrent(it, now)
}

// tierRows builds the tier rows (§U2): open, CURRENT occurrences of the
// non-grouped kinds — MEDICATION INCLUDED (fix 6). Phase 4 / D4 (§3.1):
// repeating occurrences of the same (source × animal) MERGE into ONE line —
// the representative card carries a `Slots` list of every open current
// occurrence (due-time ordered), each with its own tier-coloured toggle.
// The line lands in its most urgent slot's tier; Remaining keeps the "+N"
// fold count. Superseded and terminal occurrences go to historyRows.
func (v *DayPlanView) tierRows(plan *DayPlan, detailed bool) []CardView {
	_ = detailed // Phase 4: the merged (source × animal) line replaces both
	// densities for the row kinds — every open current occurrence is a
	// toggle on the ONE line, so compact/detailed no longer differ here.
	now := plan.Now
	current := make([]careplan.PlanItem, 0, len(plan.Items))
	for i := range plan.Items {
		if isTierRow(&plan.Items[i], now) {
			current = append(current, plan.Items[i])
		}
	}

	// Group the current occurrences by (source × animal), first-seen order,
	// then fold each group into ONE representative card with a Slots list.
	type key struct {
		typ, id string
		animal  int
	}
	type acc struct {
		rep   int // index into rows of the representative card
		first time.Time
	}
	groups := map[key]*acc{}
	order := []key{}
	rows := make([]CardView, 0, len(current))
	for i := range current {
		it := &current[i]
		src := it.Occurrence.Source
		k := key{string(src.SourceType()), src.SourceID(), it.Occurrence.AnimalID}
		slot := itemSlotFor(now, it)
		g, ok := groups[k]
		if !ok {
			cv := v.cardFor(plan, it)
			cv.Slots = []ItemSlotView{slot}
			groups[k] = &acc{rep: len(rows), first: it.Occurrence.DueAt}
			order = append(order, k)
			rows = append(rows, cv)
			continue
		}
		rep := &rows[g.rep]
		rep.Slots = append(rep.Slots, slot)
		// The line's own tier/colour/due label is its MOST URGENT slot's
		// (the work the caregiver must see first) — late beats now beats
		// later; on a tie the earliest due time wins.
		if slot.Tier < rep.Tier || (slot.Tier == rep.Tier && it.Occurrence.DueAt.Before(rep.DueAt)) {
			rep.Tier, rep.TierClass = slot.Tier, slot.TierClass
			rep.DueAt, rep.DueHM = slot.DueAt, slot.DueHM
			rep.DueDayKey, rep.DueShortDate = slot.DueDayKey, slot.DueShortDate
			rep.Status = slot.Status
		}
		if it.Occurrence.DueAt.Before(g.first) {
			g.first = it.Occurrence.DueAt
		}
	}

	// Due-time order each line's slots, count the fold, then sort the lines
	// by (tier, earliest due) so the urgent work reads first.
	for _, k := range order {
		rep := &rows[groups[k].rep]
		sort.SliceStable(rep.Slots, func(i, j int) bool {
			return rep.Slots[i].DueAt.Before(rep.Slots[j].DueAt)
		})
		rep.Remaining = len(rep.Slots) - 1
		rep.RemainingCap = BadgeCap(rep.Remaining)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Tier != rows[j].Tier {
			return rows[i].Tier < rows[j].Tier
		}
		return rows[i].DueAt.Before(rows[j].DueAt)
	})
	return rows
}

// historyKey dedupes compact history rows per (source × animal).
type historyKey struct {
	typ, id string
	animal  int
}

// historyRows builds the subdued history section: terminal
// occurrences (applied/skipped/deferred) and superseded open ones of the
// non-grouped kinds — MEDICATION INCLUDED (its done rows carry undo,
// fulfillment links and the late-record affordance; feeding and cleanup
// history stays on their grouped sections). Superseded rows stay
// per-occurrence in BOTH densities: each carries its own late-record
// action (§6.2-2/A1). Terminal rows collapse to one per (source × animal)
// group in compact; overridden surfaces in detailed only. Sorted most
// recent first.
func (v *DayPlanView) historyRows(plan *DayPlan, detailed bool) []CardView {
	now := plan.Now
	seen := map[historyKey]bool{}
	rows := make([]CardView, 0, len(plan.Items))
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || groupedKind(src.ActionKind()) {
			continue
		}
		if openStatusAction(it.Status) {
			if IsCurrent(it, now) {
				continue // current open work → tierRows
			}
			rows = append(rows, v.supersededRow(plan, it, now))
			continue
		}
		if cv, ok := v.terminalRow(plan, it, detailed, seen); ok {
			rows = append(rows, cv)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].DueAt.After(rows[j].DueAt)
	})
	return rows
}

// supersededRow renders one superseded OPEN occurrence (§6.2-1): history,
// but late-recordable when its due time already passed (A1).
func (v *DayPlanView) supersededRow(plan *DayPlan, it *careplan.PlanItem, now time.Time) CardView {
	cv := v.cardFor(plan, it)
	cv.Superseded = true
	cv.SupersededReason = SupersededReason(it, now)
	if cv.SupersededReason == "replaced" {
		cv.SupersededBy = it.NextDue.Format("15:04")
	}
	cv.Tier = tierOrder(it.Status)
	cv.TierClass = slotTierClass(cv.Tier)
	cv.LateAllowed = it.Occurrence.DueAt.Before(now)
	return cv
}

// terminalRow renders one terminal (applied/skipped/deferred/overridden)
// occurrence: group-collapsed in compact, overridden detailed-only, with
// the A8 undo affordance when an application row exists. Returns false
// when the row is suppressed (dedupe / overridden in compact).
func (v *DayPlanView) terminalRow(plan *DayPlan, it *careplan.PlanItem, detailed bool, seen map[historyKey]bool) (CardView, bool) {
	src := it.Occurrence.Source
	if it.Status == careplan.StatusOverridden && !detailed {
		return CardView{}, false
	}
	if !detailed {
		k := historyKey{string(src.SourceType()), src.SourceID(), it.Occurrence.AnimalID}
		if seen[k] {
			return CardView{}, false
		}
		seen[k] = true
	}
	cv := v.cardFor(plan, it)
	cv.Tier = tierOrder(it.Status)
	cv.TierClass = slotTierClass(cv.Tier)
	if it.Status == careplan.StatusOverridden {
		cv.OverriddenBy = it.OverriddenBy
	}
	cv.Undoable = it.Application != nil
	if it.Application != nil {
		cv.RecordedLate = it.Application.AppliedAt.After(it.Occurrence.DueAt)
	}
	return cv, true
}

// statsOf counts the workload in OCCURRENCES after the zone×kind filter
// (CP4/D6): the summary strip is density-independent (S3 — compact and
// detailed report the same numbers) and every counted occurrence shows
// on the screen, either as its own button/chip/row or folded into a
// "+N" remaining badge on its group. Superseded and terminal
// occurrences are not work — they belong to the history section.
func statsOf(plan *DayPlan, zone, kind string, now time.Time) FilterStats {
	var buckets [3]int
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || (kind != "" && src.ActionKind() != kind) ||
			it.Status == careplan.StatusOverridden {
			continue
		}
		if a, ok := plan.AnimalRow(it.Occurrence.AnimalID); !ok || (zone != "" && a.Zone.String != zone) {
			continue
		}
		if !openStatusAction(it.Status) || !IsCurrent(it, now) {
			continue
		}
		buckets[tierOrder(it.Status)]++
	}
	st := FilterStats{Late: buckets[0], Now: buckets[1], Later: buckets[2]}
	st.LateCap = BadgeCap(st.Late)
	st.NowCap = BadgeCap(st.Now)
	st.LaterCap = BadgeCap(st.Later)
	return st
}

// openCurrentCounts counts the OPEN CURRENT feeding occurrences per
// (source × animal) — the chip badge's "+N" remaining workload (CP4).
func openCurrentCounts(plan *DayPlan) map[string]int {
	openCurrent := map[string]int{}
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || src.ActionKind() != careplan.KindFeeding ||
			!openStatusAction(it.Status) || !IsCurrent(it, plan.Now) {
			continue
		}
		openCurrent[fmt.Sprintf("%s|%s|%d", src.SourceType(), src.SourceID(), it.Occurrence.AnimalID)]++
	}
	return openCurrent
}

// feedingViewOf enriches one feeding card's chips (links, "+N" remaining,
// batch refs). Returns false when no chip survived (fully-scheduled card).
func feedingViewOf(fc *FeedingCard, openCurrent map[string]int, selfPath string) (FeedingGroupView, bool) {
	fv := FeedingGroupView{Zone: fc.Zone, Cage: fc.Cage, Food: fc.Food, ForceFeed: fc.ForceFeed}
	var refs []map[string]interface{}
	for i := range fc.Chips {
		chip := fc.Chips[i]
		// Superseded chips stay as dimmed info chips (§6.2-3, CP5): the
		// animal's next occurrence is visible without being work. Only
		// OPEN CURRENT occurrences are actionable.
		if !chip.Superseded && (!openStatusAction(careplan.PlanStatus(chip.Status)) ||
			chip.Status == string(careplan.StatusScheduled)) {
			continue
		}
		chip.AnimalLink = cardAnimalLink(chip.AnimalID, selfPath)
		if n := openCurrent[fmt.Sprintf("%s|%s|%d", chip.SourceType, chip.SourceID, chip.AnimalID)]; n > 0 {
			chip.Remaining = n - 1
		}
		fv.Chips = append(fv.Chips, chip)
		if chip.Applicable {
			fv.ApplicableCount++
			refs = append(refs, map[string]interface{}{
				"source_type": chip.SourceType,
				"source_id":   chip.SourceID,
				"animal_id":   chip.AnimalID,
				"due_at":      chip.DueAt.Format(time.RFC3339),
			})
		}
	}
	if len(fv.Chips) == 0 {
		return fv, false
	}
	finalizeFeedingChips(&fv)
	if raw, err := jsonMarshal(refs); err == nil {
		fv.ChipRefsJSON = string(raw)
	}
	return fv, true
}

// finalizeFeedingChips derives everything the row needs from its chip list:
// the due-time sub-groups, the counts, and whether the list collapses.
//
// R4-7.14c: only a multi-animal row is worth collapsing — a one-line list has
// nothing to expand, and a closed one would hide the very row the caregiver
// has to act on.
func finalizeFeedingChips(fv *FeedingGroupView) {
	fv.TimeGroups = foldChipsByTime(fv.Chips)
	fv.AnimalCount = len(fv.Chips)
	fv.AnimalCountCap = BadgeCap(fv.AnimalCount)
	fv.Collapsible = fv.AnimalCount > 1
	if len(fv.TimeGroups) > 0 {
		// The header time IS the first sub-group's, so the two cannot drift.
		fv.FirstTimeLabel = fv.TimeGroups[0].Label
		fv.FirstTimeDayKey = fv.TimeGroups[0].DayKey
		fv.FirstTimeShortDate = fv.TimeGroups[0].ShortDate
		// Rev: the detail modal lists every sub-group's time, not just the
		// earliest — the row collapses them, the modal must not. Sorted by
		// due time: the fold keeps first-seen order, which can interleave
		// days ("17:00 · 09:00").
		groups := make([]FeedingTimeGroup, len(fv.TimeGroups))
		copy(groups, fv.TimeGroups)
		sort.SliceStable(groups, func(i, j int) bool { return groups[i].DueAt.Before(groups[j].DueAt) })
		labels := make([]string, 0, len(groups))
		for i := range groups {
			labels = append(labels, groups[i].Label)
		}
		fv.DetailTimes = strings.Join(labels, " · ")
	}
}

// foldChipsByTime groups a card's chips by the occurrence time they share, so
// the time is stated once per group instead of once per animal (R4-7.14b).
//
// The groups keep the chips' existing order: a chip already arrives sorted by
// due time, so a stable first-seen walk yields ascending time groups for free
// and "the earliest" is always group 0 — no re-sort, no chance of the header
// disagreeing with the first sub-group.
func foldChipsByTime(chips []FeedingChip) []FeedingTimeGroup {
	var groups []FeedingTimeGroup
	index := map[string]int{}
	for _, c := range chips {
		key := c.DueDayKey + "|" + c.DueShortDate + "|" + c.DueHM
		at, ok := index[key]
		if !ok {
			groups = append(groups, FeedingTimeGroup{
				Label:     c.DueHM,
				DayKey:    c.DueDayKey,
				ShortDate: c.DueShortDate,
				DueAt:     c.DueAt,
			})
			at = len(groups) - 1
			index[key] = at
		}
		groups[at].Chips = append(groups[at].Chips, c)
		groups[at].Count++
		if c.Applicable {
			groups[at].Applicable++
		}
	}
	for i := range groups {
		groups[i].CountCap = BadgeCap(groups[i].Count)
	}
	return groups
}

// feedingViewsOf builds every feeding row (guideline §4): ONE row per
// (cage × diet, bugs.md U1) with its deduped chips (§6.2-3) in the default
// cage grouping — or ONE row per (animal × diet) under group=animal, with
// the same per-occurrence toggle (§4.2). Unfiltered; §7.2 stage 3 filters
// in one pass. OPEN work only: superseded and scheduled chips stay off the
// work cards (superseded ones resurface in the history section, WP4).
func feedingViewsOf(plan *DayPlan, selfPath, group string) []FeedingGroupView {
	_, feedings := GroupCards(plan.Items, plan)
	// CP4 honesty: open CURRENT feeding occurrences per (source × animal)
	// — the chip badge's "+N" remaining (the workload the deduped chip
	// stands for).
	openCurrent := openCurrentCounts(plan)
	if group == "animal" {
		return feedingAnimalViewsOf(feedings, plan, openCurrent, selfPath)
	}
	out := make([]FeedingGroupView, 0, len(feedings))
	for _, fc := range feedings {
		if fv, ok := feedingViewOf(fc, openCurrent, selfPath); ok {
			out = append(out, fv)
		}
	}
	return out
}

// feedingAnimalViewsOf builds the group=animal feeding rows (guideline
// §4.2): ONE line per (animal × diet) — the tinted animal cell replaces
// the cage identity — carrying the animal's deduped chip with the SAME
// per-occurrence toggle as the cage row. NO batch apply-group button
// (§4.3: batch is inherently cage-scoped), so ChipRefsJSON stays empty.
// Rows keep the (cage × diet) card order, chips keep their in-card order.
func feedingAnimalViewsOf(feedings []*FeedingCard, plan *DayPlan, openCurrent map[string]int, selfPath string) []FeedingGroupView {
	out := make([]FeedingGroupView, 0, len(feedings))
	for _, fc := range feedings {
		fv, ok := feedingViewOf(fc, openCurrent, selfPath)
		if !ok {
			continue
		}
		for _, chip := range fv.Chips {
			row := FeedingGroupView{
				Zone:      fv.Zone,
				Cage:      fv.Cage,
				Food:      fv.Food,
				ForceFeed: fv.ForceFeed,
				AnimalID:  chip.AnimalID,
				Chips:     []FeedingChip{chip},
			}
			if a, ok := plan.AnimalRow(chip.AnimalID); ok {
				row.AnimalLabel = animalLabel(a)
				row.AnimalYear = a.YearNumberFormatted()
			}
			row.AnimalLink = cardAnimalLink(chip.AnimalID, selfPath)
			if chip.Applicable {
				row.ApplicableCount = 1
			}
			// Single-chip row: one time sub-group, never collapsible — one
			// line with nothing to expand (R4-7.14c).
			finalizeFeedingChips(&row)
			out = append(out, row)
		}
	}
	return out
}

// careItemCounts folds one cleanup item into its row (late / applicable
// counters) and returns its batch ref. Returns false when the item is not
// applicable open work (scheduled never renders).
func careItemCounts(cv *CareView, srcType, srcID string, it *careplan.PlanItem) (map[string]interface{}, bool) {
	if !openStatusAction(it.Status) || it.Status == careplan.StatusScheduled || !it.Applicable {
		return nil, false
	}
	if it.Status == careplan.StatusLate || it.Status == careplan.StatusMissing {
		cv.LateCount++
	}
	cv.ApplicableCount++
	return map[string]interface{}{
		"source_type": srcType,
		"source_id":   srcID,
		"animal_id":   it.Occurrence.AnimalID,
		"due_at":      it.Occurrence.DueAt.Format(time.RFC3339),
	}, true
}

// careViewsOf builds every cleanup row (guideline §4): ONE row per
// (source × cage) in the default cage grouping — the cage keeps its batch
// apply (bugs.md U6) — or ONE row per (source × animal) under
// group=animal. Every row carries its open occurrences as time-grouped
// toggle slots (§3). Unfiltered; applicable, non-scheduled items only.
func careViewsOf(plan *DayPlan, selfPath, group string) []CareView {
	cares, _ := GroupCards(plan.Items, plan)
	if group == "animal" {
		return careAnimalViewsOf(cares, plan, selfPath)
	}
	out := make([]CareView, 0, len(cares))
	for _, cc := range cares {
		cv := CareView{
			Zone:       cc.Zone,
			Cage:       cc.Cage,
			SourceName: DisplayName(cc.Source.Name()),
			SourceLink: cardSourceLink(string(cc.Source.SourceType()), cc.Source.SourceID(), 0, selfPath),
			SourceType: string(cc.Source.SourceType()),
			SourceID:   cc.Source.SourceID(),
		}
		var refs []map[string]interface{}
		for i := range cc.Items {
			if r, ok := careItemCounts(&cv, cv.SourceType, cv.SourceID, &cc.Items[i]); ok {
				refs = append(refs, r)
				cv.TimeGroups = foldCareTimeGroup(cv.TimeGroups, itemSlotFor(plan.Now, &cc.Items[i]))
			}
		}
		cv.Count = cv.ApplicableCount
		if cv.ApplicableCount == 0 {
			continue
		}
		if raw, err := jsonMarshal(refs); err == nil {
			cv.ChipRefsJSON = string(raw)
		}
		out = append(out, cv)
	}
	return out
}

// careAnimalViewsOf builds the group=animal rows (guideline §4.2): ONE
// line per (source × animal) — the tinted animal cell replaces the cage
// identity — with the same per-occurrence toggles as the cage rows. Rows
// sort by zone, cage, then animal year number (the round order).
func careAnimalViewsOf(cares []*CageCard, plan *DayPlan, selfPath string) []CareView {
	out := make([]CareView, 0, len(cares))
	for _, cc := range cares {
		byAnimal := map[int]*CareView{}
		var ids []int
		for i := range cc.Items {
			it := &cc.Items[i]
			cv, ok := byAnimal[it.Occurrence.AnimalID]
			if !ok {
				cv = &CareView{
					Zone:       cc.Zone,
					Cage:       cc.Cage,
					SourceName: DisplayName(cc.Source.Name()),
					SourceType: string(cc.Source.SourceType()),
					SourceID:   cc.Source.SourceID(),
					AnimalID:   it.Occurrence.AnimalID,
				}
				if a, ok := plan.AnimalRow(it.Occurrence.AnimalID); ok {
					cv.AnimalLabel = animalLabel(a)
					cv.AnimalYear = a.YearNumberFormatted()
				}
				cv.AnimalLink = cardAnimalLink(cv.AnimalID, selfPath)
				cv.SourceLink = cardSourceLink(cv.SourceType, cv.SourceID, cv.AnimalID, selfPath)
				byAnimal[it.Occurrence.AnimalID] = cv
				ids = append(ids, it.Occurrence.AnimalID)
			}
			if _, ok := careItemCounts(cv, cv.SourceType, cv.SourceID, it); ok {
				cv.TimeGroups = foldCareTimeGroup(cv.TimeGroups, itemSlotFor(plan.Now, it))
			}
		}
		sort.Ints(ids)
		for _, id := range ids {
			cv := byAnimal[id]
			cv.Count = cv.ApplicableCount
			if cv.ApplicableCount == 0 {
				continue
			}
			out = append(out, *cv)
		}
	}
	return out
}

// foldCareTimeGroup appends one occurrence slot to the row's due-time
// sub-groups (foldChipsByTime parity, guideline §3.2): items arrive
// chronologically, so a stable first-seen walk yields ascending groups.
func foldCareTimeGroup(groups []CareTimeGroup, slot ItemSlotView) []CareTimeGroup {
	key := slot.DueDayKey + "|" + slot.DueShortDate + "|" + slot.DueHM
	for i := range groups {
		g := &groups[i]
		if g.DayKey+"|"+g.ShortDate+"|"+g.Label == key {
			g.Slots = append(g.Slots, slot)
			g.Count++
			g.CountCap = BadgeCap(g.Count)
			if slot.Applicable {
				g.Applicable++
			}
			return groups
		}
	}
	g := CareTimeGroup{
		Label:     slot.DueHM,
		DayKey:    slot.DueDayKey,
		ShortDate: slot.DueShortDate,
		DueAt:     slot.DueAt,
		Slots:     []ItemSlotView{slot},
		Count:     1,
		CountCap:  BadgeCap(1),
	}
	if slot.Applicable {
		g.Applicable = 1
	}
	return append(groups, g)
}

// navZoneTabs builds the zone column of the nav matrix: the count of
// zone Z is the number of OPEN rows/lists the ACTIVE kind renders there
// (all kinds when none is active). Grouped sections count one row per
// group with open work.
func navZoneTabs(cards []CardView, feeds []FeedingGroupView, cares []CareView, kind string) ([]ZoneTab, int) {
	count := map[string]int{}
	for _, tc := range cards {
		if tc.Tier < TierDoneIdx && kindMatch(tc, kind) {
			count[tc.Zone]++
		}
	}
	countOpenSectionZones(count, feeds, cares, kind)
	names := make([]string, 0, len(count))
	for z := range count {
		names = append(names, z)
	}
	sort.Strings(names)
	tabs := make([]ZoneTab, 0, len(names))
	total := 0
	for _, z := range names {
		tabs = append(tabs, ZoneTab{Name: z, Count: count[z], CountCap: BadgeCap(count[z])})
		total += count[z]
	}
	return tabs, total
}

// countOpenSectionZones adds the grouped sections' open rows per zone to
// the nav count: a section renders under its kind (or unfiltered) and its
// builders only produce rows with open work.
func countOpenSectionZones(count map[string]int, feeds []FeedingGroupView, cares []CareView, kind string) {
	if kind == "" || kind == careplan.KindFeeding {
		for _, f := range feeds {
			count[f.Zone]++
		}
	}
	if kind == "" || kind == careplan.KindCleanup {
		for _, cv := range cares {
			count[cv.Zone]++
		}
	}
}

// navKindTabs builds the kind tabs (fix 5 — NO "all/TOUT" tab): each tab
// counts the OPEN rows/lists kind K renders in the ACTIVE zone. Fixed
// display order (actionKinds).
func navKindTabs(cards []CardView, feeds []FeedingGroupView, cares []CareView, zone string) []KindChip {
	count := map[string]int{}
	for _, tc := range cards {
		if tc.Tier < TierDoneIdx && zoneMatch(tc, zone) {
			count[tc.ActionKind]++
		}
	}
	for _, f := range feeds {
		if inZone(zone, f.Zone) {
			count[careplan.KindFeeding]++
		}
	}
	for _, cv := range cares {
		if inZone(zone, cv.Zone) {
			count[careplan.KindCleanup]++
		}
	}
	chips := make([]KindChip, 0, len(actionKinds))
	for _, k := range actionKinds {
		chips = append(chips, KindChip{Kind: k, Count: count[k], CountCap: BadgeCap(count[k])})
	}
	return chips
}

// filterCards keeps the cards the active zone×kind renders (§7.2
// stage 3 — the ONLY narrowing in the pipeline).
func filterCards(cards []CardView, zone, kind string) []CardView {
	out := make([]CardView, 0, len(cards))
	for _, cv := range cards {
		if zoneMatch(cv, zone) && kindMatch(cv, kind) {
			out = append(out, cv)
		}
	}
	return out
}

// filterFeedings keeps the feeding cards of the active zone; the section
// renders only under the feeding kind (or unfiltered).
func filterFeedings(feeds []FeedingGroupView, zone, kind string) []FeedingGroupView {
	if kind != "" && kind != careplan.KindFeeding {
		return nil
	}
	out := make([]FeedingGroupView, 0, len(feeds))
	for _, f := range feeds {
		if inZone(zone, f.Zone) {
			out = append(out, f)
		}
	}
	return out
}

// filterCares keeps the cage cleanup cards of the active zone.
func filterCares(cares []CareView, zone, kind string) []CareView {
	if kind != "" && kind != careplan.KindCleanup {
		return nil
	}
	out := make([]CareView, 0, len(cares))
	for _, cv := range cares {
		if inZone(zone, cv.Zone) {
			out = append(out, cv)
		}
	}
	return out
}

// buildMedGroups projects every medication plan item into per-animal
// cards (compact view only). Slot = morning/noon/evening from the due
// hour (same buckets as the treatments bitmap, §6.2). todayOnly is the
// dashboard mode (bugs.md R5-2a): only slots whose DueAt falls inside
// the plan window's today are kept and Overridden occurrences are
// suppressed — the badge stays an honest today count.
func (v *DayPlanView) buildMedGroups(plan *DayPlan, zone string, todayOnly bool) []MedGroupView {
	back := v.SelfPath
	if todayOnly {
		back = "/" // dashboard links return to the dashboard (R5-2b)
	}
	order := map[string]int{"morning": 0, "noon": 1, "evening": 2}
	groups := map[int]*MedGroupView{}
	var ids []int
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || src.ActionKind() != careplan.KindMedication {
			continue
		}
		if todayOnly {
			if it.Status == careplan.StatusOverridden {
				continue // overridden occurrences stay off the dashboard
			}
			if it.Occurrence.DueAt.Before(plan.From) || it.Occurrence.DueAt.After(plan.To) {
				continue // today-only window: no yesterday, no tomorrow
			}
		}
		a, ok := plan.AnimalRow(it.Occurrence.AnimalID)
		if !ok {
			continue
		}
		if zone != "" && a.Zone.String != zone {
			continue
		}
		g, ok := groups[it.Occurrence.AnimalID]
		if !ok {
			g = &MedGroupView{
				AnimalID:    it.Occurrence.AnimalID,
				AnimalLabel: animalLabel(a),
				// Dash-2 (§8.1): year-number-only button, sibling-table parity.
				AnimalYear:    a.YearNumberFormatted(),
				AnimalLink:    cardAnimalLink(it.Occurrence.AnimalID, back),
				Species:       a.Species,
				Gender:        a.Gender.String,
				Zone:          a.Zone.String,
				Cage:          a.Cage.String,
				OuttakenToday: a.Outtake != nil, // §4b-A2 (Dash-9)
			}
			groups[it.Occurrence.AnimalID] = g
			ids = append(ids, it.Occurrence.AnimalID)
		}
		slot := medSlotFor(plan.Now, it, back)
		// R3-5: day qualifier for non-today slots (multi-day series clarity).
		if dp := DueLabelPartsOf(it.Occurrence.DueAt, plan.Now); dp.DayKey != "" || dp.ShortDate != "" {
			slot.DueDayKey = dp.DayKey
			slot.DueShortDate = dp.ShortDate
		}
		if slot.Applicable && !slot.Done {
			g.OpenCount++
		}
		g.Slots = append(g.Slots, slot)
	}
	sort.Ints(ids)
	out := make([]MedGroupView, 0, len(ids))
	for _, id := range ids {
		g := groups[id]
		sort.SliceStable(g.Slots, func(i, j int) bool {
			if g.Slots[i].DueAt.Equal(g.Slots[j].DueAt) {
				return order[g.Slots[i].Slot] < order[g.Slots[j].Slot]
			}
			return g.Slots[i].DueAt.Before(g.Slots[j].DueAt)
		})
		g.Series = seriesOf(g.Slots, order)
		out = append(out, *g)
	}
	return out
}

// slotLateAllowed reports whether an occurrence is late-recordable
// (§6.2-2, A1): past-due, unapplied, out of the apply window — the
// dimmed series button stays clickable for the late record.
func slotLateAllowed(now time.Time, slot MedSlotView, it *careplan.PlanItem) bool {
	return !slot.Done && !slot.Applicable && !slot.Overridden &&
		it.Occurrence.DueAt.Before(now)
}

// medSlotFor projects one medication plan item into the togglable
// MedSlotView shared by the care plan, the dashboard and the animal
// tabs: apply/undo state, undo link only for treatment-backed
// applications, late recording when the window passed, unconditional
// view link (bugs.md R5-2b) and the dashboard deep link. `back` is the
// return path embedded in every link ("" on the animal page).
func medSlotFor(now time.Time, it *careplan.PlanItem, back string) MedSlotView {
	src := it.Occurrence.Source
	slot := MedSlotView{
		Slot:       medSlotOf(it.Occurrence.DueAt),
		Detail:     planDetail(src),
		SourceName: DisplayName(src.Name()),
		SourceType: string(src.SourceType()),
		SourceID:   src.SourceID(),
		DueAt:      it.Occurrence.DueAt,
		DueAtRFC:   it.Occurrence.DueAt.Format("2006-01-02T15:04:05Z07:00"),
		DueAtHM:    it.Occurrence.DueAt.Format("15:04"),
		Status:     string(it.Status),
		Applied:    it.Status == careplan.StatusApplied,
		Applicable: it.Applicable,
		Overridden: it.Status == careplan.StatusOverridden,
		SourceLink: cardSourceLink(string(src.SourceType()), src.SourceID(), it.Occurrence.AnimalID, back),
	}
	slot.Done = slot.Applied || it.Status == careplan.StatusSkipped || it.Status == careplan.StatusDeferred
	slot.LateAllowed = slotLateAllowed(now, slot, it)
	slot.Tier = slotTierOf(slot, it.Status)
	slot.TierClass = slotTierClass(slot.Tier)
	if app := it.Application; app != nil {
		slot.CanUndo = slot.Applied && app.FulfillmentType == models.ApplicationFulfillmentTreatment &&
			app.FulfillmentID != "" && app.FulfillmentID != planFulfillmentNone && !app.FulfillmentDeleted
		slot.FulfillmentLink = cardFulfillmentLink(app.FulfillmentType, app.FulfillmentID, app.FulfillmentDeleted, back)
	}
	// Unconditional view link (bugs.md R5-2b): an existing fulfillment
	// targets its care/treatment record, anything else the animal's
	// Treatment tab — present before AND after the toggle, sibling of
	// the toggle button, never moves.
	slot.ViewLink = slot.FulfillmentLink
	if slot.ViewLink == "" {
		slot.ViewLink = animalTreatmentLink(it.Occurrence.AnimalID, back)
	}
	slot.DeepLink = animalItemDeepLink(it.Occurrence.AnimalID, string(src.SourceType()), src.SourceID(), slot.DueAtRFC)
	return slot
}

// slotTierOf is the R4-1.1 slot tier: the urgency tier of an open slot,
// 3 for applied (green ✓) and -1 for the other terminal states — skipped,
// deferred and overridden keep their own glyph treatment and are never
// re-coloured by tier.
func slotTierOf(slot MedSlotView, status careplan.PlanStatus) int {
	slot.Tier = tierOrder(status)
	switch {
	case !slot.Done && !slot.Overridden:
		// open occurrence — urgency tier
	case slot.Applied:
		slot.Tier = 3
	default:
		slot.Tier = -1
	}
	return slot.Tier
}

// slotTierClass is the R4-1.1 tier→colour mapping: 0 late red, 1 due-now
// yellow, 2 future white (bordered so it stays visible on the white tier
// body), 3 applied green. Terminal states (-1) keep btn-light with their
// own glyph (⊘ skipped, ⏸ deferred) — they are not colour-encodable.
func slotTierClass(tier int) string {
	switch tier {
	case 0:
		return "btn-danger"
	case 1:
		return "btn-warning"
	case 2:
		return "btn-light border"
	case 3:
		return "btn-success"
	}
	return "btn-light"
}

// seriesOf merges medication slots into (drug, dosage) series (Dash-5:
// one line per drug, no duplicated label). Within a series the slots are
// bucket-ordered (morning → noon → evening, due time within a bucket)
// and chunked 3 per row — a bucket change always starts a new row
// (Dash-8, the pseudo-grouped divider). Series keep first-seen order
// (chronological by first occurrence).
func seriesOf(slots []MedSlotView, bucketOrder map[string]int) []MedSeriesView {
	groups := map[string][]MedSlotView{}
	var labels []string
	for _, s := range slots {
		if _, ok := groups[s.Detail]; !ok {
			labels = append(labels, s.Detail)
		}
		groups[s.Detail] = append(groups[s.Detail], s)
	}
	out := make([]MedSeriesView, 0, len(labels))
	for _, label := range labels {
		g := groups[label]
		sort.SliceStable(g, func(i, j int) bool {
			bi, bj := bucketOrder[g[i].Slot], bucketOrder[g[j].Slot]
			if bi != bj {
				return bi < bj
			}
			return g[i].DueAt.Before(g[j].DueAt)
		})
		out = append(out, seriesForRow(g, label))
	}
	return out
}

// seriesForRow assembles one (drug, dosage) series and marks its urgency
// (R4-1.2) so every surface rendering a series — care plan, dashboard and
// animal tabs — highlights the same urgent slot. tierOrder/seriesTier are
// deterministic, so fillMedTiers' later pass marks identically.
func seriesForRow(g []MedSlotView, label string) MedSeriesView {
	series := MedSeriesView{Key: label, Label: label, Rows: chunkSeriesRows(g)}
	if tier, first := seriesTier(series); tier >= 0 {
		series.Tier = tier
		series.FirstDueAt = first
		markSeriesUrgency(&series, tier)
	}
	return series
}

// chunkSeriesRows bucket-chunks one series' ordered slots: at most 3 per
// row, a bucket change always starts a new row (Dash-8 pseudo-grouped
// divider).
func chunkSeriesRows(g []MedSlotView) []MedSeriesRow {
	var rows []MedSeriesRow
	var cur []MedSlotView
	for _, s := range g {
		if len(cur) >= 3 || (len(cur) > 0 && cur[0].Slot != s.Slot) {
			rows = append(rows, MedSeriesRow{Slots: cur})
			cur = nil
		}
		cur = append(cur, s)
	}
	if len(cur) > 0 {
		rows = append(rows, MedSeriesRow{Slots: cur})
	}
	// DividerBefore: row i>0 divides when its first slot's bucket differs
	// from the previous row's last slot bucket.
	for i := 1; i < len(rows); i++ {
		prev := rows[i-1].Slots[len(rows[i-1].Slots)-1]
		if rows[i].Slots[0].Slot != prev.Slot {
			rows[i].DividerBefore = true
		}
	}
	return rows
}

// seriesTierOrMinus is seriesTier for a caller that only needs to know
// whether the series still has open work (R4-7.7 filtering).
func seriesTierOrMinus(series MedSeriesView) int {
	tier, _ := seriesTier(series)
	return tier
}

// scopeSeriesToToday is R4-7.7: the work screen shows the DAY's work, not
// the schedule. Kept: every slot due up to the end of today (open or
// terminal — the done ones are the day's record), plus AT MOST ONE later
// open slot, the nearest, and only when it is nearer than the pending late
// entry ("if the duration from now to the entry is shorter than the current
// one"). Everything else waits for its own day, so the table is empty in the
// evening. The surviving later slot is bucketed "tomorrow", following
// morning/noon/evening like any other bucket.
func scopeSeriesToToday(series MedSeriesView, now time.Time) MedSeriesView {
	endOfDay := endOfDayOf(now)

	// Without an open slot due today there is no open WORK to scope around —
	// but a fully-applied series still owns today's DONE record (R9-2): keep
	// the day's terminal slots so the Done tier can render them. With no
	// slot due today at all (open or terminal) the series leaves the screen.
	urgentDue, ok := mostUrgentSlotDue(series, endOfDay)
	if !ok {
		kept := todaysSlots(series, endOfDay)
		if len(kept) == 0 {
			return emptySeries(series)
		}
		out := series
		out.Rows = chunkSeriesRows(kept)
		return out
	}

	// The one later slot worth showing: the nearest open one, and only when
	// it beats the pending late entry.
	kept := todaysSlots(series, endOfDay)
	if nextDue, found := nearestLaterSlot(series, endOfDay); found &&
		nextDue.Sub(now) < now.Sub(urgentDue) {
		kept = append(kept, tomorrowSlot(series, nextDue)...)
	}

	// R9 next-in-future rule: a series must not advertise the same
	// treatment twice. A PAST-due slot is shown only while the next
	// treatment is still in the future; once an occurrence of the series
	// is DUE now (the caregiver is on it), the stale past ones leave —
	// the due slot carries the series from here. (The preference caps
	// already filtered the too-old past slots upstream.)
	if hasDueNowSlot(kept) {
		kept = dropPastDueSlots(kept)
	}

	if len(kept) == 0 {
		return emptySeries(series)
	}
	out := series
	out.Rows = chunkSeriesRows(kept)
	return out
}

// emptySeries keeps only the identity of a series whose scoped rows are gone,
// so the caller can recognise it while rendering no line for it.
func emptySeries(series MedSeriesView) MedSeriesView {
	return MedSeriesView{Key: series.Key, Label: series.Label}
}

// endOfDayOf is the last instant of `now`'s day.
func endOfDayOf(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
}

// slotIsOpenWork reports whether the slot still needs something done (an
// occurrence with no application and no terminal/suppressed state).
func slotIsOpenWork(s MedSlotView) bool {
	tier := tierOrder(careplan.PlanStatus(s.Status))
	return !s.Done && !s.Overridden && tier >= 0 && tier <= 2
}

// flatSlots is every slot of a series, in order.
func flatSlots(series MedSeriesView) []MedSlotView {
	var out []MedSlotView
	for _, row := range series.Rows {
		out = append(out, row.Slots...)
	}
	return out
}

// mostUrgentSlotDue returns the due time of the most urgent open slot due up
// to `endOfDay`, and whether one exists. With none, the day's work is done or
// not yet due and the series leaves the work screen.
func mostUrgentSlotDue(series MedSeriesView, endOfDay time.Time) (time.Time, bool) {
	var due time.Time
	found := false
	for _, s := range flatSlots(series) {
		if s.DueAt.After(endOfDay) || !slotIsOpenWork(s) {
			continue
		}
		if !found || betterSeriesSlot(s, due) {
			due, found = s.DueAt, true
		}
	}
	return due, found
}

// nearestLaterSlot returns the due time of the nearest open slot after
// `endOfDay`, and whether one exists.
func nearestLaterSlot(series MedSeriesView, endOfDay time.Time) (time.Time, bool) {
	var due time.Time
	found := false
	for _, s := range flatSlots(series) {
		if !s.DueAt.After(endOfDay) || !slotIsOpenWork(s) {
			continue
		}
		if !found || s.DueAt.Before(due) {
			due, found = s.DueAt, true
		}
	}
	return due, found
}

// todaysSlots keeps every slot due up to the end of today — the open ones and
// the terminal ones (the day's record). A slot that is neither open nor
// terminal is dropped: there is nothing to do and nothing to remember.
func todaysSlots(series MedSeriesView, endOfDay time.Time) []MedSlotView {
	var kept []MedSlotView
	for _, s := range flatSlots(series) {
		if s.DueAt.After(endOfDay) {
			continue
		}
		if !s.Done && !s.Overridden && tierOrder(careplan.PlanStatus(s.Status)) > 2 {
			continue
		}
		kept = append(kept, s)
	}
	return kept
}

// hasDueNowSlot reports whether any slot of the set is DUE now — open, not
// late yet (R9 next-in-future rule trigger).
func hasDueNowSlot(slots []MedSlotView) bool {
	for _, s := range slots {
		if !s.Done && !s.Overridden && careplan.PlanStatus(s.Status) == careplan.StatusDue {
			return true
		}
	}
	return false
}

// dropPastDueSlots removes the series' open past-due slots (late/missing):
// the due-now slot takes over the series (R9 next-in-future rule). Done and
// overridden slots stay — the day's record must survive.
func dropPastDueSlots(slots []MedSlotView) []MedSlotView {
	out := slots[:0:0]
	for _, s := range slots {
		if !s.Done && !s.Overridden && tierOrder(careplan.PlanStatus(s.Status)) == 0 {
			continue
		}
		out = append(out, s)
	}
	return out
}

// tomorrowSlot returns the slots due at exactly `nextDue`, rebucketed into
// the "tomorrow" group so they read apart from the day's own buckets.
func tomorrowSlot(series MedSeriesView, nextDue time.Time) []MedSlotView {
	var kept []MedSlotView
	for _, s := range flatSlots(series) {
		if s.DueAt.Equal(nextDue) {
			s.Slot = slotTomorrow
			kept = append(kept, s)
		}
	}
	return kept
}

// betterSeriesSlot reports whether slot `s` is more urgent than the slot
// due at `have` (same helper semantics as betterTierSlot, kept separate so
// the scoping pass does not depend on tier bookkeeping).
func betterSeriesSlot(s MedSlotView, have time.Time) bool {
	t := tierOrder(careplan.PlanStatus(s.Status))
	return t <= 1 || (t == 2 && s.DueAt.Before(have))
}

// slotTomorrow is the bucket key of the "next day" group (R4-7.7); its
// label is care_plan.slot.tomorrow.
const slotTomorrow = "tomorrow"

// animalItemDeepLink builds the dashboard eye URL (Dash-7, §8.2): the
// animal's Treatment tab (sibling-table #nav-* convention) plus the
// occurrence reference (?item=&due=) the Show handler resolves
// server-side to open the shared detail modal on load.
func animalItemDeepLink(animalID int, srcType, srcID, dueRFC string) string {
	q := url.Values{}
	q.Set("item", srcType+":"+srcID)
	if dueRFC != "" {
		q.Set("due", dueRFC)
	}
	return fmt.Sprintf("/animals/%d?%s#nav-treatment", animalID, q.Encode())
}

// BuildDashboardMedView projects the dashboard "Medication today"
// medication section (bugs.md R5-2a): the same per-animal slot cards as
// the compact work screen, but in dashboard mode — today-only slots,
// Overridden occurrences suppressed, badge = honest today count. The
// caller assembles the plan over TodayPlanWindow; /care_plan keeps its
// 4-day DefaultPlanWindow.
func BuildDashboardMedView(plan *DayPlan, zone string) []MedGroupView {
	v := &DayPlanView{
		View:     ViewCompact,
		SelfPath: planSelfPath(ViewCompact, "", "", "", "/"),
	}
	return v.buildMedGroups(plan, zone, true)
}

// medSlotOf buckets a due time into morning/noon/evening — the same
// boundaries as treatmentBucketBit (§6.2) so the card toggle matches the
// treatments bitmap written at apply time.
func medSlotOf(due time.Time) string {
	switch {
	case due.Hour() < 11:
		return "morning"
	case due.Hour() <= 15:
		return "noon"
	default:
		return "evening"
	}
}

// cardFor builds the base card of one plan item (labels via §10.5-N1,
// conversion markers stripped, per-kind detail line — bugs.md U5).
// itemSlotFor projects one open occurrence of a row-kind (source × animal)
// group into its per-occurrence toggle (Phase 4 / D4, §2/§3). Colour comes
// from the ONE slotTierClass policy (§2.4); weighing/observation keep
// NeedsInput so the toggle opens the shared apply modal (§3.4).
func itemSlotFor(now time.Time, it *careplan.PlanItem) ItemSlotView {
	src := it.Occurrence.Source
	parts := DueLabelPartsOf(it.Occurrence.DueAt, now)
	kind := src.ActionKind()
	slot := ItemSlotView{
		DueAt:        it.Occurrence.DueAt,
		DueAtRFC:     it.Occurrence.DueAt.Format("2006-01-02T15:04:05Z07:00"),
		DueHM:        parts.TimeHM,
		DueDayKey:    parts.DayKey,
		DueShortDate: parts.ShortDate,
		AnimalID:     it.Occurrence.AnimalID,
		Status:       string(it.Status),
		Applicable:   it.Applicable,
		NeedsInput:   kind == careplan.KindWeighing || kind == careplan.KindObservation,
	}
	slot.Tier = tierOrder(it.Status)
	slot.TierClass = slotTierClass(slot.Tier)
	slot.LateAllowed = it.Applicable && it.Occurrence.DueAt.Before(now)
	return slot
}

func (v *DayPlanView) cardFor(plan *DayPlan, it *careplan.PlanItem) CardView {
	src := it.Occurrence.Source
	parts := DueLabelPartsOf(it.Occurrence.DueAt, plan.Now)
	cv := CardView{
		SourceType:   string(src.SourceType()),
		SourceID:     src.SourceID(),
		SourceName:   DisplayName(src.Name()),
		Detail:       planDetail(src),
		ActionKind:   src.ActionKind(),
		AnimalID:     it.Occurrence.AnimalID,
		DueAt:        it.Occurrence.DueAt,
		DueHM:        parts.TimeHM,
		DueDayKey:    parts.DayKey,
		DueShortDate: parts.ShortDate,
		Status:       string(it.Status),
		Tier:         tierOrder(it.Status),
		TierClass:    slotTierClass(tierOrder(it.Status)),
		Applicable:   it.Applicable,
		NeedsInput:   src.ActionKind() == careplan.KindWeighing || src.ActionKind() == careplan.KindObservation,
	}
	// R8-2: observation/care/weighing (and feeding) source names repeat the
	// detail title ("Traitement — Sexage" beside "Sexage") — the day-plan
	// row keeps kind + title only.
	cv.SourceNameRedundant = sourceNameRedundantWithDetail(src, cv.SourceName, cv.Detail)

	if a, ok := plan.AnimalRow(it.Occurrence.AnimalID); ok {
		cv.AnimalLabel = animalLabel(a)
		cv.AnimalYear = a.YearNumberFormatted()
		cv.Zone = a.Zone.String
		cv.Cage = a.Cage.String
	}
	cv.AnimalLink = cardAnimalLink(cv.AnimalID, v.SelfPath)
	cv.SourceLink = cardSourceLink(cv.SourceType, cv.SourceID, cv.AnimalID, v.SelfPath)
	if app := it.Application; app != nil {
		cv.FulfillmentLink = cardFulfillmentLink(app.FulfillmentType, app.FulfillmentID, app.FulfillmentDeleted, v.SelfPath)
	}
	return cv
}

// planSelfPath is the canonical URL of the work screen with its current
// filters — the back target propagated to every card link so a round trip
// (animal page, rule page, record page) returns to the same zone/kind.
// R5-2d (D-b): an incoming back target is carried in the self URL (and
// thus through every card link); invalid targets are dropped — the
// fallback is the plain care_plan self URL.
//
// R4-7.18: `view` is no longer emitted. The screen has ONE density (the
// compact/detailed toggle rendered identically for five of the six kinds), so
// carrying the parameter in every propagated URL was noise. The parameter is
// still ACCEPTED on the way in, so old links and bookmarks keep working.
//
// Guideline §4.2: `group` IS emitted (cage ⇄ animal grouping of the
// feeding/cleanup sections) — it is a real layout choice, not a density
// alias, so a round trip must return to the same grouping.
func planSelfPath(view, zone, kind, group, back string) string {
	_ = view // kept for call-site symmetry; the work screen has one density
	q := url.Values{}
	if zone != "" {
		q.Set("zone", zone)
	}
	if kind != "" {
		q.Set("kind", kind)
	}
	if group != "" {
		q.Set("group", group)
	}
	if b := localBackParam(back); b != "" {
		q.Set("back", b)
	}
	// No dangling "?" — the self URL is copied into every card link and into
	// the caregiver's bookmark bar; "/care_plan?" reads like a broken URL.
	if len(q) == 0 {
		return "/care_plan"
	}
	return "/care_plan?" + q.Encode()
}

// cardAnimalLink points at the animal's Plan tab (spec §7.2a, bugs.md U3).
// The back param goes BEFORE the hash — after "#" it would be part of the
// fragment and never reach the server.
func cardAnimalLink(animalID int, back string) string {
	if animalID == 0 {
		return ""
	}
	return fmt.Sprintf("/animals/%d?back=%s#nav-plan", animalID, url.QueryEscape(back))
}

// animalTreatmentLink targets the animal's Treatment tab — the fallback
// of the R5-2b unconditional view link (no fulfillment record yet). Like
// cardAnimalLink the back param goes before the #nav-treatment fragment.
func animalTreatmentLink(animalID int, back string) string {
	if animalID == 0 {
		return ""
	}
	return fmt.Sprintf("/animals/%d?back=%s#nav-treatment", animalID, url.QueryEscape(back))
}

// cardSourceLink: rule → rule show; animal plan → the animal's Plan tab
// (§7.2a). The back param returns to the work screen.
// R8-2: animal-plan links carry `src=<sourceID>` so the Protocol tab can
// open its Details trace table and highlight the producing protocol — the
// tab's Details card is collapsed by default, which made the link dead.
func cardSourceLink(sourceType, sourceID string, animalID int, back string) string {
	if sourceType == string(careplan.SourceRule) {
		return fmt.Sprintf("/care_rules/%s?back=%s", sourceID, url.QueryEscape(back))
	}
	// `src` MUST precede the #nav-plan fragment — a param after the hash
	// is part of the fragment and invisible to the server/URLSearchParams.
	return fmt.Sprintf("/animals/%d?back=%s&src=%s#nav-plan",
		animalID, url.QueryEscape(back), url.QueryEscape(sourceID))
}

// cardFulfillmentLink targets the care/treatment record of an applied
// item (done tier, bugs.md U3). Empty when the application carries no
// fulfillment or the record was deleted (§10-CP1).
func cardFulfillmentLink(fulfillmentType, fulfillmentID string, deleted bool, back string) string {
	if deleted || fulfillmentID == "" || fulfillmentID == planFulfillmentNone {
		return ""
	}
	switch fulfillmentType {
	case models.ApplicationFulfillmentCare:
		return fmt.Sprintf("/cares/%s?back=%s", fulfillmentID, url.QueryEscape(back))
	case models.ApplicationFulfillmentTreatment:
		return fmt.Sprintf("/treatments/%s?back=%s", fulfillmentID, url.QueryEscape(back))
	}
	return ""
}

// BadgeCap renders a count badge capped at 99+ (bugs.md U2): a
// three-digit badge breaks pill layout and carries no actionable
// information. Actions-side twin of careplan.BadgeCap (engine).
func BadgeCap(n int) string {
	return careplan.BadgeCap(n)
}

func zoneMatch(cv CardView, zone string) bool {
	return zone == "" || cv.Zone == zone
}

// inZone is the zoneMatch twin for section cards (they carry the zone
// directly instead of inside a CardView).
func inZone(activeZone, cardZone string) bool {
	return activeZone == "" || cardZone == activeZone
}

func kindMatch(cv CardView, kind string) bool {
	return kind == "" || cv.ActionKind == kind
}
