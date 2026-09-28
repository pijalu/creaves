package actions

import (
	"net/http"
	"sort"
	"time"

	"creaves/models/careplan"

	"github.com/gobuffalo/buffalo"
)

// Care schedule report (docs/care-plan-ux-fix-plan.md Phase 5, bugs.md
// U9/U10): a READ-ONLY rendering of the same BuildDayPlan read model the
// /care_plan work screen uses — "understand the schedule" without the
// apply/skip/defer controls. Group switch ?group=zone|cage|animal:
//   zone   — one section per zone, occurrences sorted by time
//   cage   — one block per cage (round order: zone, cage); animals sharing
//            the cage-dominant group (same kind+detail at the same time)
//            collapse into one grouped row; diverging animals get their own
//   animal — full per-animal chronological trace
// Legacy visual language (L1/L3/L6/L7): urgency row classes
// (danger=late/missing, warning=due, info=scheduled today,
// success/light=applied), today highlight, AM/noon/PM clock-dot SVGs for
// medication slots.

// Report grouping modes.
const (
	ReportGroupZone   = "zone"
	ReportGroupCage   = "cage"
	ReportGroupAnimal = "animal"
)

// ScheduleRowView is one rendered occurrence row of the report.
type ScheduleRowView struct {
	Time         string // "15:04"
	Date         string // "2006-01-02" (animal trace: multi-day)
	AnimalID     int
	AnimalLabel  string
	AnimalLink   string
	AnimalCount  int    // grouped cage rows: animals collapsed into the row
	Kind         string // action kind (i18n anchor: t("care_plan.kind."+Kind))
	Detail       string
	SourceName   string
	SourceLink   string
	Status       string // i18n anchor: t("care_plan.status."+Status)
	RowClass     string // table-danger|table-warning|table-info|table-success|""
	Fulfillment  string // link of an applied item (empty otherwise)
	ClockSlot    string // medication: "morning"|"noon"|"evening" — "" otherwise
}

// ScheduleZoneSection is one zone of the zone grouping.
type ScheduleZoneSection struct {
	Zone  string
	Count int
	Rows  []ScheduleRowView
}

// ScheduleCageBlock is one cage of the cage grouping.
type ScheduleCageBlock struct {
	Zone  string
	Cage  string
	Count int
	Rows  []ScheduleRowView
}

// ScheduleAnimalSection is one animal of the animal grouping.
type ScheduleAnimalSection struct {
	AnimalID    int
	AnimalLabel string
	AnimalLink  string
	Zone        string
	Cage        string
	Count       int
	Rows        []ScheduleRowView
}

// CareScheduleView is the report template context.
type CareScheduleView struct {
	Group    string
	From     string // ISO date (form value)
	To       string
	Today    string // ISO date of `now` (today highlight)
	Zones    []ScheduleZoneSection
	Cages    []ScheduleCageBlock
	Animals  []ScheduleAnimalSection
	Empty    bool
	Total    int
}

// ReportsCareScheduleIndex handles GET /reports/care_schedule (Phase 5).
// Read-only: it renders the BuildDayPlan window (same from/to params and
// 14-day cap as /care_plan) grouped per ?group= (default zone).
func ReportsCareScheduleIndex(c buffalo.Context) error {
	tx := planTx(c)
	now := time.Now()
	from, to := planWindowParams(c, now)

	plan, err := BuildDayPlan(tx, now, from, to)
	if err != nil {
		return err
	}

	group := c.Param("group")
	switch group {
	case ReportGroupCage, ReportGroupAnimal:
	default:
		group = ReportGroupZone
	}

	c.Set("view", BuildCareScheduleView(plan, group, now))
	return c.Render(http.StatusOK, r.HTML("/reports/care_schedule.plush.html"))
}

// BuildCareScheduleView projects the day plan into the report view model
// (pure projection, no DB access — unit-testable like BuildDayPlanView).
func BuildCareScheduleView(plan *DayPlan, group string, now time.Time) *CareScheduleView {
	v := &CareScheduleView{
		Group: group,
		From:  plan.From.Format("2006-01-02"),
		To:    plan.To.Format("2006-01-02"),
		Today: now.Format("2006-01-02"),
	}

	rows := make([]ScheduleRowView, 0, len(plan.Items))
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || it.Status == careplan.StatusOverridden {
			continue // overridden occurrences are noise for a schedule report
		}
		row := ScheduleRowView{
			Date:        it.Occurrence.DueAt.Format("2006-01-02"),
			Time:        it.Occurrence.DueAt.Format("15:04"),
			Kind:        src.ActionKind(),
			Detail:      planDetail(src),
			SourceName:  DisplayName(src.Name()),
			Status:      string(it.Status),
			AnimalID:    it.Occurrence.AnimalID,
			AnimalCount: 1,
			RowClass:    scheduleRowClass(it.Status),
			ClockSlot:   scheduleClockSlot(src.ActionKind(), it.Occurrence.DueAt),
		}
		if a, ok := plan.AnimalRow(it.Occurrence.AnimalID); ok {
			row.AnimalLabel = animalLabel(a)
		}
		row.AnimalLink = cardAnimalLink(row.AnimalID)
		row.SourceLink = cardSourceLink(string(src.SourceType()), src.SourceID(), row.AnimalID)
		if app := it.Application; app != nil {
			row.Fulfillment = cardFulfillmentLink(app.FulfillmentType, app.FulfillmentID, app.FulfillmentDeleted)
		}
		rows = append(rows, row)
	}

	byTime := func(rs []ScheduleRowView) {
		sort.SliceStable(rs, func(i, j int) bool {
			if rs[i].Date != rs[j].Date {
				return rs[i].Date < rs[j].Date
			}
			if rs[i].Time != rs[j].Time {
				return rs[i].Time < rs[j].Time
			}
			return rs[i].AnimalLabel < rs[j].AnimalLabel
		})
	}

	switch group {
	case ReportGroupCage:
		v.Cages = scheduleCageBlocks(plan, rows, byTime)
	case ReportGroupAnimal:
		v.Animals = scheduleAnimalSections(plan, rows, byTime)
	default:
		v.Zones = scheduleZoneSections(plan, rows, byTime)
	}
	v.Total = len(rows)
	v.Empty = len(rows) == 0
	return v
}

// scheduleRowClass maps the §6.1 status to the legacy urgency row class
// (L1/L3/L6): late|missing → danger, due → warning, scheduled → info,
// applied|skipped|deferred → success (light).
func scheduleRowClass(s careplan.PlanStatus) string {
	switch s {
	case careplan.StatusLate, careplan.StatusMissing:
		return "table-danger"
	case careplan.StatusDue:
		return "table-warning"
	case careplan.StatusScheduled:
		return "table-info"
	case careplan.StatusApplied, careplan.StatusSkipped, careplan.StatusDeferred:
		return "table-success"
	}
	return ""
}

// scheduleClockSlot maps a medication occurrence time to the legacy
// AM/noon/PM clock-dot slot (L7): before 11:00 → morning, 11:00–14:59 →
// noon, 15:00+ → evening. Non-medication kinds render no dot.
func scheduleClockSlot(kind string, due time.Time) string {
	if kind != careplan.KindMedication {
		return ""
	}
	switch h := due.Hour(); {
	case h < 11:
		return "morning"
	case h < 15:
		return "noon"
	default:
		return "evening"
	}
}

// animalPlacement resolves the zone/cage display values of one animal row
// (§10.5-N1 dash convention for uncaged animals).
func animalPlacement(plan *DayPlan, animalID int) (zone, cage string) {
	if a, ok := plan.AnimalRow(animalID); ok {
		zone, cage = a.Zone.String, a.Cage.String
	}
	if cage == "" {
		cage = "—"
	}
	return zone, cage
}

// scheduleZoneSections groups rows per zone, sorted by zone name, rows by
// time (legacy AnimalByZoneMap convention).
func scheduleZoneSections(plan *DayPlan, rows []ScheduleRowView, byTime func([]ScheduleRowView)) []ScheduleZoneSection {
	sections := map[string]*ScheduleZoneSection{}
	var order []string
	for _, r := range rows {
		zone, _ := animalPlacement(plan, r.AnimalID)
		sec, ok := sections[zone]
		if !ok {
			sec = &ScheduleZoneSection{Zone: zone}
			sections[zone] = sec
			order = append(order, zone)
		}
		sec.Rows = append(sec.Rows, r)
	}
	sort.Strings(order)
	out := make([]ScheduleZoneSection, 0, len(order))
	for _, z := range order {
		sec := sections[z]
		byTime(sec.Rows)
		sec.Count = len(sec.Rows)
		out = append(out, *sec)
	}
	return out
}

// scheduleCageBlocks groups rows per cage in round order (zone, cage),
// collapsing animals that share the cage-dominant group — same kind +
// detail at the same time — into one grouped row ("cage A12 · 3
// animaux"). Animals diverging from the dominant group keep their own row.
func scheduleCageBlocks(plan *DayPlan, rows []ScheduleRowView, byTime func([]ScheduleRowView)) []ScheduleCageBlock {
	type cageKey struct{ zone, cage string }
	blocks := map[cageKey]*ScheduleCageBlock{}
	var order []cageKey
	for _, r := range rows {
		zone, cage := animalPlacement(plan, r.AnimalID)
		k := cageKey{zone, cage}
		b, ok := blocks[k]
		if !ok {
			b = &ScheduleCageBlock{Zone: zone, Cage: cage}
			blocks[k] = b
			order = append(order, k)
		}
		b.Rows = append(b.Rows, r)
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].zone != order[j].zone {
			return order[i].zone < order[j].zone
		}
		return order[i].cage < order[j].cage
	})

	out := make([]ScheduleCageBlock, 0, len(order))
	for _, k := range order {
		b := blocks[k]
		b.Rows = collapseCageRows(b.Rows)
		byTime(b.Rows)
		b.Count = len(b.Rows)
		out = append(out, *b)
	}
	return out
}

// collapseCageRows folds rows sharing (date, time, kind, detail) into one
// grouped row whose AnimalCount is the number of collapsed animals; the
// row label is resolved in the template ("N animaux" convention). Rows
// with a distinct kind/detail/time (diverging animals) pass through.
// Worst-urgency status wins inside a group so the row class reflects the
// most pressing member.
func collapseCageRows(rows []ScheduleRowView) []ScheduleRowView {
	type gkey struct {
		date, time, kind, detail string
	}
	groups := map[gkey]int{} // key → index in out
	out := make([]ScheduleRowView, 0, len(rows))
	for _, r := range rows {
		k := gkey{r.Date, r.Time, r.Kind, r.Detail}
		if idx, ok := groups[k]; ok {
			g := &out[idx]
			g.AnimalCount++
			if urgencyRankString(r.Status) < urgencyRankString(g.Status) {
				g.Status = r.Status
				g.RowClass = r.RowClass
			}
			if g.Fulfillment == "" {
				g.Fulfillment = r.Fulfillment
			}
			continue
		}
		groups[k] = len(out)
		out = append(out, r)
	}
	return out
}

// urgencyRankString orders statuses by report urgency (lower = more
// urgent), mirroring the engine's urgencyRank for the open set.
func urgencyRankString(s string) int {
	switch s {
	case string(careplan.StatusMissing):
		return 0
	case string(careplan.StatusLate):
		return 1
	case string(careplan.StatusDue):
		return 2
	case string(careplan.StatusScheduled):
		return 3
	case string(careplan.StatusDeferred):
		return 4
	case string(careplan.StatusSkipped):
		return 5
	case string(careplan.StatusApplied):
		return 6
	}
	return 7
}

// scheduleAnimalSections groups rows per animal (sorted by label), each a
// full chronological trace of the window.
func scheduleAnimalSections(plan *DayPlan, rows []ScheduleRowView, byTime func([]ScheduleRowView)) []ScheduleAnimalSection {
	sections := map[int]*ScheduleAnimalSection{}
	var order []int
	for _, r := range rows {
		sec, ok := sections[r.AnimalID]
		if !ok {
			zone, cage := animalPlacement(plan, r.AnimalID)
			sec = &ScheduleAnimalSection{
				AnimalID:    r.AnimalID,
				AnimalLabel: r.AnimalLabel,
				AnimalLink:  r.AnimalLink,
				Zone:        zone,
				Cage:        cage,
			}
			sections[r.AnimalID] = sec
			order = append(order, r.AnimalID)
		}
		sec.Rows = append(sec.Rows, r)
	}
	sort.SliceStable(order, func(i, j int) bool {
		return sections[order[i]].AnimalLabel < sections[order[j]].AnimalLabel
	})
	out := make([]ScheduleAnimalSection, 0, len(order))
	for _, id := range order {
		sec := sections[id]
		byTime(sec.Rows)
		sec.Count = len(sec.Rows)
		out = append(out, *sec)
	}
	return out
}
