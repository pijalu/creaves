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

// Tier keys (template + i18n anchors).
const (
	TierLate  = "late"
	TierNow   = "now"
	TierLater = "later"
	TierDone  = "done"
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
	Tiers      [4]TierView
	Feedings   []FeedingGroupView
	Meds       []MedGroupView
	Cares      []CareView
	Zones      []ZoneTab // without the "all" entry (rendered by the template)
	Kinds      []KindChip
	Hours      []HourChip
	UpdatedAt  string // HH:MM of render (auto-refresh indicator, §10-CP6c)
	View       string
	Zone       string
	Kind       string
	SelfPath   string // /care_plan?view=…&zone=…&kind=… — back target of every card link
	ZoneAll    int    // open count across all zones
	ZoneAllCap string
	KindAll    int // open count across all kinds
	KindAllCap string
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

// BuildDayPlanView projects the day plan into the work-screen view model.
// view/zone/kind are the (already validated) request params; now drives
// UpdatedAt. Counters (zones, kinds, tiers) always reflect the UNFILTERED
// work window — only the tier card lists are narrowed by zone/kind.
// back (R5-2d, D-b, optional) is the page's own incoming back target
// (sanitized); it is embedded in the self URL so every card link chains
// the ORIGINAL origin (e.g. the dashboard's back=/) through the round trip.
func BuildDayPlanView(plan *DayPlan, view, zone, kind string, now time.Time, back ...string) *DayPlanView {
	backIn := ""
	if len(back) > 0 {
		backIn = back[0]
	}
	v := &DayPlanView{
		View:      view,
		Zone:      zone,
		Kind:      kind,
		UpdatedAt: now.Format("15:04"),
		SelfPath:  planSelfPath(view, zone, kind, backIn),
	}

	zoneCount := map[string]int{}
	kindCount := map[string]int{}
	hourCount := map[string]int{}

	addCard := func(tier int, cv CardView) {
		v.Tiers[tier].Cards = append(v.Tiers[tier].Cards, cv)
	}

	if view == ViewDetailed {
		// Detailed: one card per occurrence (compact grouping off);
		// overridden rows surface here only (debug).
		for i := range plan.Items {
			it := &plan.Items[i]
			src := it.Occurrence.Source
			if src == nil {
				continue
			}
			cv := v.cardFor(plan, it)
			if openStatusAction(it.Status) {
				v.openCounters(src.ActionKind(), cv, zoneCount, kindCount, hourCount, it.Status)
			}
			if it.Status == careplan.StatusOverridden {
				if zoneMatch(cv, zone) && kindMatch(cv, kind) {
					cv.OverriddenBy = it.OverriddenBy
					addCard(3, cv)
				}
				continue
			}
			if !zoneMatch(cv, zone) || !kindMatch(cv, kind) {
				continue
			}
			addCard(tierOrder(it.Status), cv)
		}
	} else {
		// Compact (default): the next open action per (source × animal),
		// remaining open work as a "+n" badge. Groups fully done today
		// reappear as a synthetic done card (done counter). Feeding items
		// leave the per-animal tiers: one card per cage × diet (bugs.md U1).
		next := careplan.NextOpenPerGroup(plan.Items)
		type gkey struct {
			typ, id string
			animal  int
		}
		seen := map[gkey]bool{}
		for _, n := range next {
			src := n.Item.Occurrence.Source
			cv := v.cardFor(plan, &n.Item)
			if g := careplan.GroupingFor(src.ActionKind()); g == careplan.GroupingCageDiet || g == careplan.GroupingCage {
				// open counters still reflect grouped work (no hour anchor:
				// feeding/care cards render outside the later tier).
				if openStatusAction(n.Item.Status) {
					zoneCount[cv.Zone]++
					kindCount[src.ActionKind()]++
				}
				continue
			}
			if src.ActionKind() == careplan.KindMedication {
				// Medication leaves the per-occurrence tiers: one card per
				// animal with slot toggles (see meds build below). Open
				// counters still count it; no hour anchor (outside tiers).
				if openStatusAction(n.Item.Status) {
					zoneCount[cv.Zone]++
					kindCount[src.ActionKind()]++
				}
				continue
			}
			cv.Remaining = n.Remaining
			cv.RemainingCap = BadgeCap(n.Remaining)
			seen[gkey{string(src.SourceType()), src.SourceID(), n.Item.Occurrence.AnimalID}] = true
			v.openCounters(src.ActionKind(), cv, zoneCount, kindCount, hourCount, n.Item.Status)
			if !zoneMatch(cv, zone) || !kindMatch(cv, kind) {
				continue
			}
			addCard(tierOrder(n.Item.Status), cv)
		}
		for i := range plan.Items {
			it := &plan.Items[i]
			src := it.Occurrence.Source
			if src == nil || openStatusAction(it.Status) || it.Status == careplan.StatusOverridden {
				continue
			}
			if g := careplan.GroupingFor(src.ActionKind()); g == careplan.GroupingCageDiet || g == careplan.GroupingCage {
				continue // grouped done state shows on the feeding/care card, not as a tier row
			}
			k := gkey{string(src.SourceType()), src.SourceID(), it.Occurrence.AnimalID}
			if seen[k] {
				continue
			}
			seen[k] = true
			cv := v.cardFor(plan, it)
			if zoneMatch(cv, zone) && kindMatch(cv, kind) {
				addCard(3, cv)
			}
		}

		// Medication cards: one per animal, every medication occurrence of
		// the window as a slot row — applied slots stay visible (state
		// visible, undoable) instead of vanishing into the done tier.
		if kind == "" || kind == careplan.KindMedication {
			v.Meds = v.buildMedGroups(plan, zone, false)
		}

		// Feeding cards (bugs.md U1): one per cage × diet, OPEN work only
		// (a scheduled future slot is not actionable — the work screen shows
		// what remains). Overridden occurrences stay off (chip filter).
		if kind == "" || kind == careplan.KindFeeding {
			_, feedings := GroupCards(plan.Items, plan)
			for _, fc := range feedings {
				if zone != "" && fc.Zone != zone {
					continue
				}
				fv := FeedingGroupView{Zone: fc.Zone, Cage: fc.Cage, Food: fc.Food, ForceFeed: fc.ForceFeed}
				// Round-2 §6.2-3: the chips arrive already deduped — one per
				// (animal × source), the earliest CURRENT occurrence. Superseded
				// chips are work-set history: they stay off the work cards (the
				// dead-end red "En retard" of CP6); the WP2/WP4 pipeline routes
				// them to the history section.
				var refs []map[string]interface{}
				for i := range fc.Chips {
					chip := fc.Chips[i]
					if chip.Superseded || !openStatusAction(careplan.PlanStatus(chip.Status)) ||
						chip.Status == string(careplan.StatusScheduled) {
						continue
					}
					chip.AnimalLink = cardAnimalLink(chip.AnimalID, v.SelfPath)
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
					v.Feedings = append(v.Feedings, fv)
				}
			}
		}

		// Cage (cleanup) cards (bugs.md U6): one batch apply per cage.
		if kind == "" || kind == careplan.KindCleanup {
			cares, _ := GroupCards(plan.Items, plan)
			for _, cc := range cares {
				if zone != "" && cc.Zone != zone {
					continue
				}
				cv := CareView{
					Zone:       cc.Zone,
					Cage:       cc.Cage,
					SourceName: DisplayName(cc.Source.Name()),
					SourceLink: cardSourceLink(string(cc.Source.SourceType()), cc.Source.SourceID(), 0, v.SelfPath),
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
					v.Cares = append(v.Cares, cv)
				}
			}
		}
	}

	// Sort later cards by DueAt so hour anchors read chronologically.
	sort.SliceStable(v.Tiers[2].Cards, func(i, j int) bool {
		return v.Tiers[2].Cards[i].DueAt.Before(v.Tiers[2].Cards[j].DueAt)
	})

	keys := []string{TierLate, TierNow, TierLater, TierDone}
	for i := range v.Tiers {
		v.Tiers[i].Key = keys[i]
		v.Tiers[i].Count = len(v.Tiers[i].Cards)
		v.Tiers[i].CountCap = BadgeCap(v.Tiers[i].Count)
	}

	// Zone tabs (sorted by name) + "all" counter.
	zoneNames := make([]string, 0, len(zoneCount))
	for z := range zoneCount {
		zoneNames = append(zoneNames, z)
	}
	sort.Strings(zoneNames)
	for _, z := range zoneNames {
		v.Zones = append(v.Zones, ZoneTab{Name: z, Count: zoneCount[z], CountCap: BadgeCap(zoneCount[z])})
		v.ZoneAll += zoneCount[z]
	}
	v.ZoneAllCap = BadgeCap(v.ZoneAll)

	// Kind chips in fixed display order.
	for _, k := range actionKinds {
		v.Kinds = append(v.Kinds, KindChip{Kind: k, Count: kindCount[k], CountCap: BadgeCap(kindCount[k])})
		v.KindAll += kindCount[k]
	}
	v.KindAllCap = BadgeCap(v.KindAll)

	// Hour chips: distinct hours of later items, chronological.
	hours := make([]string, 0, len(hourCount))
	for h := range hourCount {
		hours = append(hours, h)
	}
	sort.Strings(hours)
	for _, h := range hours {
		v.Hours = append(v.Hours, HourChip{Hour: h + ":00", Anchor: "h-" + h, Count: hourCount[h]})
	}

	return v
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
				AnimalID:      it.Occurrence.AnimalID,
				AnimalLabel:   animalLabel(a),
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
		out = append(out, *g)
	}
	return out
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

// openCounters feeds the badge maps of one OPEN card.
func (v *DayPlanView) openCounters(kind string, cv CardView, zones, kinds, hours map[string]int, status careplan.PlanStatus) {
	zones[cv.Zone]++
	kinds[kind]++
	if status == careplan.StatusScheduled {
		hours[cv.HourKey]++
	}
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

func kindMatch(cv CardView, kind string) bool {
	return kind == "" || cv.ActionKind == kind
}
