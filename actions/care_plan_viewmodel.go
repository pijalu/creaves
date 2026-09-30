package actions

import (
	"fmt"
	"net/url"
	"sort"
	"time"

	"creaves/models"
	"creaves/models/careplan"
)

// Work-screen view model (Phase 2, bugs.md U2): urgency tiers, positioned
// badges, zone tabs, kind chips, hour chips, auto-refresh indicator. Built
// from the assembled DayPlan plus the request params — pure projection, no
// DB access, so it is unit-testable without a fixture.

// Tier keys (template + i18n anchors). TierDoneIdx is the array index of
// the done tier (terminal statuses) used by the card builders.
const (
	TierLate  = "late"
	TierNow   = "now"
	TierLater = "later"
	TierDone  = "done"

	TierDoneIdx = 3
)

// Work views.
const (
	ViewCompact  = "compact"
	ViewDetailed = "detailed"
)

// CardView is one rendered work-screen card.
type CardView struct {
	SourceType   string
	SourceID     string
	SourceName   string
	Detail       string // per-kind content line (bugs.md U5); primary label when set
	ActionKind   string
	AnimalID     int
	AnimalLabel  string
	Zone         string
	Cage         string
	DueAt        time.Time
	HourKey      string // "15" — anchor #h-15 inside the later tier
	Status       string
	Applicable   bool
	Remaining    int    // compact: other open occurrences of the group ("+n")
	RemainingCap string // BadgeCap(Remaining)
	OverriddenBy string // detailed view only
	// bugs.md R5-2c (D-a): confirm only when fields are needed — weighing
	// (weight) and observation (answer) open the input modal; feeding,
	// medication, care and cleanup toggle instantly.
	NeedsInput bool
	// bugs.md U3/U6 (Phase 4): fast-action links, all with back=<self>.
	AnimalLink      string // /animals/{id}#nav-plan — empty without an animal row
	SourceLink      string // /care_rules/{id} (rule) or /animals/{id}#nav-plan (animal plan)
	FulfillmentLink string // done tier: /cares|/treatments/{fid} — empty otherwise
}

// TierView is one urgency tier of the work screen.
type TierView struct {
	Key      string
	Cards    []CardView
	Count    int    // len(Cards)
	CountCap string // BadgeCap(Count)
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

// HourChip jumps to one hour anchor of the later tier (no reload).
type HourChip struct {
	Hour   string // "15:00"
	Anchor string // "h-15"
	Count  int
}

// FeedingGroupView is one rendered feeding card (cage × diet, bugs.md U1).
type FeedingGroupView struct {
	Zone            string
	Cage            string
	Food            string
	ForceFeed       bool
	Chips           []FeedingChip
	ApplicableCount int    // chips still applicable (apply-group button)
	ChipRefsJSON    string // JSON item refs of the applicable chips (data-items)
}

// MedSlotView is one medication occurrence inside a per-animal medication
// card: the slot (morning/noon/evening) toggle state plus everything the
// toggle/undo/detail UI needs.
type MedSlotView struct {
	Slot            string // "morning" | "noon" | "evening" (i18n key suffix)
	Detail          string // drug — dosage
	SourceName      string
	SourceType      string
	SourceID        string
	DueAt           time.Time
	DueAtRFC        string
	DueAtHM         string // "15:04"
	Status          string
	Done            bool // applied/skipped/deferred (has an application row)
	Applied         bool
	Applicable      bool
	CanUndo         bool // applied + fulfillment treatment row still exists
	Overridden      bool
	SourceLink      string
	FulfillmentLink string
	ViewLink        string // unconditional record/treatment view (R5-2b)
	// §6.2-2 (round-2, A1): past-due, unapplied, out-of-apply-window —
	// the dimmed series button stays clickable and records the missed
	// occurrence with the explicit late acknowledgment.
	LateAllowed bool
}

// MedSeriesView is one drug line of a medication card (round-2 §8.3,
// Dash-5/Dash-8, CP1, T2): every occurrence of one animal sharing the
// same (drug, dosage) label, bucket-ordered (morning → noon → evening)
// and chunked 3 per row — a bucket change always starts a new row. The
// shared `_med_series` partial renders it identically on the care plan,
// the dashboard and the animal Treatment tab.
type MedSeriesView struct {
	Key   string // merge key: drug — dosage
	Label string // display: drug — dosage
	Rows  [][]MedSlotView
}

// MedGroupView is one rendered per-animal medication card: all medication
// occurrences of the day for one animal, in slot order — the single view
// of what the animal must receive.
type MedGroupView struct {
	AnimalID    int
	AnimalLabel string
	AnimalLink  string
	Species     string // raw species (template translates via tspecies)
	Zone        string
	Cage        string
	// Round-2 §4b-A2 (Dash-9): animal outtaken today — depupdate parity
	// (outtaken row class + dove badge); set only by the today-outtaken
	// assembly scope.
	OuttakenToday bool
	Slots         []MedSlotView
	OpenCount     int // applicable, not yet recorded
	// Round-2 §8.3: slots merged into (drug, dosage) series — the shared
	// `_med_series` partial renders these; Slots stays until every
	// consumer switched (deleted together with the tier structure, WP4+).
	Series []MedSeriesView
}

// CareView is one rendered cage (cleanup) card — one card per
// (source × cage), bugs.md U6 fast action: one batch apply per cage.
type CareView struct {
	Zone            string
	Cage            string
	SourceName      string
	SourceLink      string
	Count           int    // open, non-scheduled occurrences on the card
	ChipRefsJSON    string // JSON item refs of the applicable items (data-items)
	ApplicableCount int
}

// DayPlanView is the full work-screen context value.
type DayPlanView struct {
	Tiers     [4]TierView
	Feedings  []FeedingGroupView
	Meds      []MedGroupView
	Cares     []CareView
	Zones     []ZoneTab // without the "all" entry (rendered by the template)
	Kinds     []KindChip
	Hours     []HourChip
	UpdatedAt string // HH:MM of render (auto-refresh indicator, §10-CP6c)
	View      string
	// CP3/D3 (round-2 §7.1): one build path — compact and detailed render
	// the SAME sections/groups; Detailed only switches card density.
	Detailed   bool
	Stats      FilterStats // CP4/D6: summary strip, filtered like the screen
	Zone       string
	Kind       string
	SelfPath   string // /care_plan?view=…&zone=…&kind=… — back target of every card link
	ZoneAll    int    // open count across all zones (for the active kind)
	ZoneAllCap string
	KindAll    int // open count across all kinds (for the active zone)
	KindAllCap string
}

// FilterStats is the CP4/D6 single-pass summary of one rendered view:
// every count derives from the same zone×kind-filtered card set the
// screen renders — impossible states like "0 future (17)" cannot be built.
type FilterStats struct {
	Late     int
	Now      int
	Later    int
	Done     int
	LateCap  string
	NowCap   string
	LaterCap string
	DoneCap  string
}

// tierCard pairs a card with its urgency tier before filtering (§7.2:
// build unfiltered, then one zone×kind filter pass).
type tierCard struct {
	cv   CardView
	tier int
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
// (round-2 §7.2 pipeline, fixes CP2/CP3/CP4):
//
//  1. engine output (all items, no pre-narrowing — CP2 root cause removed)
//  2. display split into card collections (tiers + sections), unfiltered
//  3. ONE zone×kind filter pass (the handler whitelist-validates zone)
//  4. sections/groups from the filtered set; urgency-first tiers
//  5. FilterStats + the zone×kind nav matrix (CP4/D6: badges == rendered)
//
// view selects the card density (CP3/D3): compact and detailed render the
// same sections/groups — detailed adds per-occurrence cards, overridden
// rows and window/grace info. now drives UpdatedAt. back (R5-2d, D-b,
// optional) is the page's own incoming back target (sanitized); it is
// embedded in the self URL so every card link chains the ORIGINAL origin
// (e.g. the dashboard's back=/) through the round trip.
func BuildDayPlanView(plan *DayPlan, view, zone, kind string, now time.Time, back ...string) *DayPlanView {
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
		UpdatedAt: now.Format("15:04"),
		SelfPath:  planSelfPath(view, zone, kind, backIn),
	}

	// Stage 1–2: build every card UNFILTERED (engine → display layer).
	cards := v.tierCards(plan, detailed)
	feeds := feedingViewsOf(plan, v.SelfPath)
	cares := careViewsOf(plan, v.SelfPath)
	meds := v.buildMedGroups(plan, "", false)

	// Stage 5 for the nav (D6): the zone×kind matrix comes from the
	// unfiltered open card sets — the zone tab of zone Z counts the cards
	// the ACTIVE kind would render there (and symmetrically for kinds).
	v.Zones, v.ZoneAll = navZoneTabs(cards, feeds, cares, meds, kind)
	v.Kinds, v.KindAll = navKindChips(cards, feeds, cares, meds, zone)
	v.ZoneAllCap = BadgeCap(v.ZoneAll)
	v.KindAllCap = BadgeCap(v.KindAll)

	// Stage 3: one filter pass over every collection.
	visible := filterTierCards(cards, zone, kind)
	v.Feedings = filterFeedings(feeds, zone, kind)
	v.Cares = filterCares(cares, zone, kind)
	v.Meds = filterMeds(meds, zone, kind)

	// Stage 4: tiers + summary from the SAME visible set (CP4 invariant).
	keys := []string{TierLate, TierNow, TierLater, TierDone}
	for t := range v.Tiers {
		v.Tiers[t].Key = keys[t]
		for _, tc := range visible {
			if tc.tier == t {
				v.Tiers[t].Cards = append(v.Tiers[t].Cards, tc.cv)
			}
		}
		sort.SliceStable(v.Tiers[t].Cards, func(i, j int) bool {
			return v.Tiers[t].Cards[i].DueAt.Before(v.Tiers[t].Cards[j].DueAt)
		})
		v.Tiers[t].Count = len(v.Tiers[t].Cards)
		v.Tiers[t].CountCap = BadgeCap(v.Tiers[t].Count)
	}
	v.Stats = FilterStats{
		Late:  v.Tiers[0].Count,
		Now:   v.Tiers[1].Count,
		Later: v.Tiers[2].Count,
		Done:  v.Tiers[3].Count,
	}
	v.Stats.LateCap = BadgeCap(v.Stats.Late)
	v.Stats.NowCap = BadgeCap(v.Stats.Now)
	v.Stats.LaterCap = BadgeCap(v.Stats.Later)
	v.Stats.DoneCap = BadgeCap(v.Stats.Done)

	// Hour chips: distinct hours of the VISIBLE later cards (CP4: a
	// filtered view never shows a chip whose target was filtered away).
	// Deleted together with the tier structure in WP4 (§7.1).
	v.Hours = hourChipsOf(visible)

	return v
}

// tierCards builds the urgency-tier cards (§7.2 stage 2). detailed renders
// one card per occurrence (overridden rows surface here only); compact
// renders the next open action per (source × animal) with a "+n" badge.
// Grouped kinds (feeding, cage cleanup, medication) never yield tier cards
// — they render as sections.
func (v *DayPlanView) tierCards(plan *DayPlan, detailed bool) []tierCard {
	if detailed {
		return v.tierCardsDetailed(plan)
	}
	return v.tierCardsCompact(plan)
}

// tierCardsDetailed: one card per occurrence; overridden items land in the
// done tier carrying their suppressing plan (debug surface).
func (v *DayPlanView) tierCardsDetailed(plan *DayPlan) []tierCard {
	cards := make([]tierCard, 0, len(plan.Items))
	for i := range plan.Items {
		it := &plan.Items[i]
		if it.Occurrence.Source == nil {
			continue
		}
		cv := v.cardFor(plan, it)
		t := tierOrder(it.Status)
		if it.Status == careplan.StatusOverridden {
			cv.OverriddenBy = it.OverriddenBy
			t = TierDoneIdx
		}
		if t < 0 {
			continue
		}
		cards = append(cards, tierCard{cv: cv, tier: t})
	}
	return cards
}

// tierCardsCompact: the next open action per (source × animal), remaining
// open work as a "+n" badge; fully-done groups reappear as a synthetic
// done card (done counter).
func (v *DayPlanView) tierCardsCompact(plan *DayPlan) []tierCard {
	next := careplan.NextOpenPerGroup(plan.Items)
	type gkey struct {
		typ, id string
		animal  int
	}
	seen := map[gkey]bool{}
	cards := make([]tierCard, 0, len(next))
	for _, n := range next {
		src := n.Item.Occurrence.Source
		if tierKindExcluded(src.ActionKind()) {
			continue // grouped kinds render as sections, not tier rows
		}
		cv := v.cardFor(plan, &n.Item)
		cv.Remaining = n.Remaining
		cv.RemainingCap = BadgeCap(n.Remaining)
		seen[gkey{string(src.SourceType()), src.SourceID(), n.Item.Occurrence.AnimalID}] = true
		if t := tierOrder(n.Item.Status); t >= 0 {
			cards = append(cards, tierCard{cv: cv, tier: t})
		}
	}
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || openStatusAction(it.Status) || it.Status == careplan.StatusOverridden {
			continue
		}
		if tierKindExcluded(src.ActionKind()) {
			continue // grouped done state shows on the section card
		}
		k := gkey{string(src.SourceType()), src.SourceID(), it.Occurrence.AnimalID}
		if seen[k] {
			continue
		}
		seen[k] = true
		cards = append(cards, tierCard{cv: v.cardFor(plan, it), tier: TierDoneIdx})
	}
	return cards
}

// tierKindExcluded lists the kinds that never render as tier cards:
// medication (per-animal cards) and the cage-grouped kinds (bugs.md U1).
func tierKindExcluded(kind string) bool {
	if kind == careplan.KindMedication {
		return true
	}
	g := careplan.GroupingFor(kind)
	return g == careplan.GroupingCageDiet || g == careplan.GroupingCage
}

// feedingViewsOf builds every feeding card (cage × diet, bugs.md U1) with
// its deduped chips (§6.2-3) — unfiltered; §7.2 stage 3 filters in one
// pass. OPEN work only: superseded and scheduled chips stay off the work
// cards (superseded ones resurface in the history section, WP4).
func feedingViewsOf(plan *DayPlan, selfPath string) []FeedingGroupView {
	_, feedings := GroupCards(plan.Items, plan)
	out := make([]FeedingGroupView, 0, len(feedings))
	for _, fc := range feedings {
		fv := FeedingGroupView{Zone: fc.Zone, Cage: fc.Cage, Food: fc.Food, ForceFeed: fc.ForceFeed}
		var refs []map[string]interface{}
		for i := range fc.Chips {
			chip := fc.Chips[i]
			if chip.Superseded || !openStatusAction(careplan.PlanStatus(chip.Status)) ||
				chip.Status == string(careplan.StatusScheduled) {
				continue
			}
			chip.AnimalLink = cardAnimalLink(chip.AnimalID, selfPath)
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
		if len(fv.Chips) > 0 {
			if raw, err := jsonMarshal(refs); err == nil {
				fv.ChipRefsJSON = string(raw)
			}
			out = append(out, fv)
		}
	}
	return out
}

// careViewsOf builds every cage cleanup card (bugs.md U6: one batch apply
// per cage) — unfiltered; applicable, non-scheduled items only.
func careViewsOf(plan *DayPlan, selfPath string) []CareView {
	cares, _ := GroupCards(plan.Items, plan)
	out := make([]CareView, 0, len(cares))
	for _, cc := range cares {
		cv := CareView{
			Zone:       cc.Zone,
			Cage:       cc.Cage,
			SourceName: DisplayName(cc.Source.Name()),
			SourceLink: cardSourceLink(string(cc.Source.SourceType()), cc.Source.SourceID(), 0, selfPath),
		}
		var refs []map[string]interface{}
		for i := range cc.Items {
			it := &cc.Items[i]
			if !openStatusAction(it.Status) || it.Status == careplan.StatusScheduled || !it.Applicable {
				continue
			}
			cv.ApplicableCount++
			refs = append(refs, map[string]interface{}{
				"source_type": string(cc.Source.SourceType()),
				"source_id":   cc.Source.SourceID(),
				"animal_id":   it.Occurrence.AnimalID,
				"due_at":      it.Occurrence.DueAt.Format(time.RFC3339),
			})
		}
		cv.Count = cv.ApplicableCount
		if cv.ApplicableCount > 0 {
			if raw, err := jsonMarshal(refs); err == nil {
				cv.ChipRefsJSON = string(raw)
			}
			out = append(out, cv)
		}
	}
	return out
}

// navZoneTabs builds the zone column of the CP4 zone×kind matrix: the
// count of zone Z is the number of OPEN cards the ACTIVE kind renders
// there (all kinds when none is active). Grouped sections count one card
// per group with open work.
func navZoneTabs(cards []tierCard, feeds []FeedingGroupView, cares []CareView, meds []MedGroupView, kind string) ([]ZoneTab, int) {
	count := map[string]int{}
	for _, tc := range cards {
		if tc.tier < TierDoneIdx && kindMatch(tc.cv, kind) {
			count[tc.cv.Zone]++
		}
	}
	countOpenSectionZones(count, feeds, cares, meds, kind)
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

// countOpenSectionZones adds the grouped sections' open cards per zone to
// the nav count: a section renders under its kind (or unfiltered) and its
// builders only produce cards with open work.
func countOpenSectionZones(count map[string]int, feeds []FeedingGroupView, cares []CareView, meds []MedGroupView, kind string) {
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
	if kind == "" || kind == careplan.KindMedication {
		for _, m := range meds {
			if m.OpenCount > 0 {
				count[m.Zone]++
			}
		}
	}
}

// navKindChips builds the kind column of the CP4 zone×kind matrix: the
// count of kind K is the number of OPEN cards K renders in the ACTIVE
// zone (symmetric to navZoneTabs). Fixed display order (chip bar).
func navKindChips(cards []tierCard, feeds []FeedingGroupView, cares []CareView, meds []MedGroupView, zone string) ([]KindChip, int) {
	count := map[string]int{}
	for _, tc := range cards {
		if tc.tier < TierDoneIdx && zoneMatch(tc.cv, zone) {
			count[tc.cv.ActionKind]++
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
	for _, m := range meds {
		if m.OpenCount > 0 && inZone(zone, m.Zone) {
			count[careplan.KindMedication]++
		}
	}
	chips := make([]KindChip, 0, len(actionKinds))
	total := 0
	for _, k := range actionKinds {
		chips = append(chips, KindChip{Kind: k, Count: count[k], CountCap: BadgeCap(count[k])})
		total += count[k]
	}
	return chips, total
}

// filterTierCards keeps the cards the active zone×kind renders (§7.2
// stage 3 — the ONLY narrowing in the pipeline).
func filterTierCards(cards []tierCard, zone, kind string) []tierCard {
	out := make([]tierCard, 0, len(cards))
	for _, tc := range cards {
		if zoneMatch(tc.cv, zone) && kindMatch(tc.cv, kind) {
			out = append(out, tc)
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

// filterMeds keeps the per-animal medication cards of the active zone.
// Cards with no open slot still render (done slots stay visible with
// their undo/record view links) but never count in the nav matrix.
func filterMeds(meds []MedGroupView, zone, kind string) []MedGroupView {
	if kind != "" && kind != careplan.KindMedication {
		return nil
	}
	out := make([]MedGroupView, 0, len(meds))
	for _, m := range meds {
		if inZone(zone, m.Zone) {
			out = append(out, m)
		}
	}
	return out
}

// hourChipsOf derives the hour anchors from the VISIBLE later cards.
func hourChipsOf(visible []tierCard) []HourChip {
	hourCount := map[string]int{}
	for _, tc := range visible {
		if tc.tier == 2 {
			hourCount[tc.cv.HourKey]++
		}
	}
	hours := make([]string, 0, len(hourCount))
	for h := range hourCount {
		hours = append(hours, h)
	}
	sort.Strings(hours)
	chips := make([]HourChip, 0, len(hours))
	for _, h := range hours {
		chips = append(chips, HourChip{Hour: h + ":00", Anchor: "h-" + h, Count: hourCount[h]})
	}
	return chips
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
				// back (not v.SelfPath): the dashboard mode overrides the
				// chain to "/" so EVERY card link returns to the dashboard
				// (R5-2d/D-b); on the work screen back == v.SelfPath.
				AnimalLink:    cardAnimalLink(it.Occurrence.AnimalID, back),
				Species:       a.Species,
				Zone:          a.Zone.String,
				Cage:          a.Cage.String,
				OuttakenToday: a.Outtake != nil, // §4b-A2 (Dash-9)
			}
			groups[it.Occurrence.AnimalID] = g
			ids = append(ids, it.Occurrence.AnimalID)
		}
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
		slot.LateAllowed = slotLateAllowed(plan.Now, slot, it)
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
		out = append(out, MedSeriesView{Key: label, Label: label, Rows: chunkSeriesRows(g)})
	}
	return out
}

// chunkSeriesRows bucket-chunks one series' ordered slots: at most 3 per
// row, a bucket change always starts a new row (Dash-8 pseudo-grouped
// divider).
func chunkSeriesRows(g []MedSlotView) [][]MedSlotView {
	var rows [][]MedSlotView
	var cur []MedSlotView
	for _, s := range g {
		if len(cur) >= 3 || (len(cur) > 0 && cur[0].Slot != s.Slot) {
			rows = append(rows, cur)
			cur = nil
		}
		cur = append(cur, s)
	}
	if len(cur) > 0 {
		rows = append(rows, cur)
	}
	return rows
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
		SelfPath: planSelfPath(ViewCompact, "", "", "/"),
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
func (v *DayPlanView) cardFor(plan *DayPlan, it *careplan.PlanItem) CardView {
	src := it.Occurrence.Source
	cv := CardView{
		SourceType: string(src.SourceType()),
		SourceID:   src.SourceID(),
		SourceName: DisplayName(src.Name()),
		Detail:     planDetail(src),
		ActionKind: src.ActionKind(),
		AnimalID:   it.Occurrence.AnimalID,
		DueAt:      it.Occurrence.DueAt,
		HourKey:    fmt.Sprintf("%02d", it.Occurrence.DueAt.Hour()),
		Status:     string(it.Status),
		Applicable: it.Applicable,
		NeedsInput: src.ActionKind() == careplan.KindWeighing || src.ActionKind() == careplan.KindObservation,
	}
	if a, ok := plan.AnimalRow(it.Occurrence.AnimalID); ok {
		cv.AnimalLabel = animalLabel(a)
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
// (animal page, rule page, record page) returns to the same view/zone/kind.
// R5-2d (D-b): an incoming back target is carried in the self URL (and
// thus through every card link); invalid targets are dropped — the
// fallback is the plain care_plan self URL.
func planSelfPath(view, zone, kind, back string) string {
	q := url.Values{}
	if view != "" {
		q.Set("view", view)
	}
	if zone != "" {
		q.Set("zone", zone)
	}
	if kind != "" {
		q.Set("kind", kind)
	}
	if b := localBackParam(back); b != "" {
		q.Set("back", b)
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
func cardSourceLink(sourceType, sourceID string, animalID int, back string) string {
	if sourceType == string(careplan.SourceRule) {
		return fmt.Sprintf("/care_rules/%s?back=%s", sourceID, url.QueryEscape(back))
	}
	return cardAnimalLink(animalID, back)
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
