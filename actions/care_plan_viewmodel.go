package actions

import (
	"fmt"
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
	// bugs.md U3/U6 (Phase 4): fast-action links, all with back=/care_plan.
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
	Hour  string // "15:00"
	Anchor string // "h-15"
	Count int
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
	Cares     []CareView
	Zones     []ZoneTab // without the "all" entry (rendered by the template)
	Kinds     []KindChip
	Hours     []HourChip
	UpdatedAt string // HH:MM of render (auto-refresh indicator, §10-CP6c)
	View      string
	Zone      string
	Kind      string
	ZoneAll   int    // open count across all zones
	ZoneAllCap string
	KindAll   int // open count across all kinds
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
func BuildDayPlanView(plan *DayPlan, view, zone, kind string, now time.Time) *DayPlanView {
	v := &DayPlanView{
		View:      view,
		Zone:      zone,
		Kind:      kind,
		UpdatedAt: now.Format("15:04"),
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
				var refs []map[string]interface{}
				for i := range fc.Items {
					if !openStatusAction(fc.Items[i].Status) || fc.Items[i].Status == careplan.StatusScheduled {
						continue
					}
					chip := fc.Chips[i]
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
					SourceLink: cardSourceLink(string(cc.Source.SourceType()), cc.Source.SourceID(), 0),
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
	}
	if a, ok := plan.AnimalRow(it.Occurrence.AnimalID); ok {
		cv.AnimalLabel = animalLabel(a)
		cv.Zone = a.Zone.String
		cv.Cage = a.Cage.String
	}
	cv.AnimalLink = cardAnimalLink(cv.AnimalID)
	cv.SourceLink = cardSourceLink(cv.SourceType, cv.SourceID, cv.AnimalID)
	if app := it.Application; app != nil {
		cv.FulfillmentLink = cardFulfillmentLink(app.FulfillmentType, app.FulfillmentID, app.FulfillmentDeleted)
	}
	return cv
}

// cardAnimalLink points at the animal's Plan tab (spec §7.2a, bugs.md U3).
func cardAnimalLink(animalID int) string {
	if animalID == 0 {
		return ""
	}
	return fmt.Sprintf("/animals/%d#nav-plan?back=/care_plan", animalID)
}

// cardSourceLink: rule → rule show; animal plan → the animal's Plan tab
// (§7.2a). The back param returns to the work screen.
func cardSourceLink(sourceType, sourceID string, animalID int) string {
	if sourceType == string(careplan.SourceRule) {
		return fmt.Sprintf("/care_rules/%s?back=/care_plan", sourceID)
	}
	return cardAnimalLink(animalID)
}

// cardFulfillmentLink targets the care/treatment record of an applied
// item (done tier, bugs.md U3). Empty when the application carries no
// fulfillment or the record was deleted (§10-CP1).
func cardFulfillmentLink(fulfillmentType, fulfillmentID string, deleted bool) string {
	if deleted || fulfillmentID == "" || fulfillmentID == planFulfillmentNone {
		return ""
	}
	switch fulfillmentType {
	case models.ApplicationFulfillmentCare:
		return fmt.Sprintf("/cares/%s?back=/care_plan", fulfillmentID)
	case models.ApplicationFulfillmentTreatment:
		return fmt.Sprintf("/treatments/%s?back=/care_plan", fulfillmentID)
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
