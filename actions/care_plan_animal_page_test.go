package actions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/require"
)

// R5-4b (bugs.md U21 / D-c): animal page tabs. The Protocol tab lists the
// complete protocol (payload content, active window, expired status); the
// Treatment tab keeps its original look but is fed by the R5-3 per-time
// entries, protocol-backed rows link back to their source protocol, manual
// treatments render identically from their seeded entries, and legacy rows
// without entries keep the 3-bucket bitmap fallback.

// mkAnimalPlan creates one CareAnimalPlan with a fixed-anchor schedule.
func mkAnimalPlan(t *testing.T, animalID int, name, kind, payload, schedule string, active bool) *models.CareAnimalPlan {
	t.Helper()
	p := &models.CareAnimalPlan{
		AnimalID:      animalID,
		Name:          name,
		ActionKind:    kind,
		ActionPayload: json.RawMessage(payload),
		Schedule:      json.RawMessage(schedule),
		Active:        active,
	}
	require.NoError(t, models.DB.Create(p))
	return p
}

// fixedSchedule builds a §4.3 schedule JSON anchored at a fixed date.
func fixedSchedule(anchor time.Time, durationDays int) string {
	doc := map[string]interface{}{
		"times":       []string{"08:00"},
		"anchor":      "fixed",
		"anchor_date": anchor.Format("2006-01-02"),
	}
	if durationDays > 0 {
		doc["duration_days"] = durationDays
	}
	b, err := json.Marshal(doc)
	if err != nil {
		// Unreachable for a map of plain values.
		return `{"times":["08:00"],"anchor":"fixed","anchor_date":"` + anchor.Format("2006-01-02") + `"}`
	}
	return string(b)
}

func animalShowHTML(t *testing.T, animalID int, lang string) string {
	t.Helper()
	client, baseURL := planAdminClient(t)
	req, err := http.NewRequest("GET", baseURL+fmt.Sprintf("/animals/%d", animalID), nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	if lang != "" {
		req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "animal show rendered: %s", raw[:min(len(raw), 3000)])
	return string(raw)
}

func TestAnimalsShowProtocolTabComplete(t *testing.T) {
	f := setupPlanFixture(t)
	animalID := f.animalIDs[0]
	today := time.Now()

	// Active, current window (starts today, runs 5 days).
	active := mkAnimalPlan(t, animalID, "ActiveProto-"+f.marker, "medication",
		`{"drug":"ProtoDrug-`+f.marker+`","dosage":"2 ml"}`,
		fixedSchedule(today, 5), true)
	require.NotNil(t, active)

	// Expired: started 10 days ago, lasted 5 days.
	expired := mkAnimalPlan(t, animalID, "ExpiredProto-"+f.marker, "feeding",
		`{"food":"ExpFood-`+f.marker+`"}`,
		fixedSchedule(today.AddDate(0, 0, -10), 5), true)

	// Inactive plan (never active, open-ended).
	inactive := mkAnimalPlan(t, animalID, "InactiveProto-"+f.marker, "care",
		`{"instructions":"Inact-`+f.marker+`"}`,
		fixedSchedule(today, 0), false)

	html := animalShowHTML(t, animalID, "en-US")

	// Complete list: every plan renders regardless of status/window.
	require.Contains(t, html, "ActiveProto-"+f.marker)
	require.Contains(t, html, "ExpiredProto-"+f.marker)
	require.Contains(t, html, "InactiveProto-"+f.marker)

	// Payload summary column (drug+dosage / food / instructions).
	require.Contains(t, html, "ProtoDrug-"+f.marker+" — 2 ml")
	require.Contains(t, html, "ExpFood-"+f.marker)
	require.Contains(t, html, "Inact-"+f.marker)

	// Active window column: ISO start → end dates.
	wantWindow := today.Format("2006-01-02") + " → " + today.AddDate(0, 0, 4).Format("2006-01-02")
	require.Contains(t, html, wantWindow, "active window shown as ISO date range")
	require.Contains(t, html, "∞", "open-ended plan window")

	// Status: active badge for the current plan, Expired for the ended
	// one, Inactive for the switched-off one.
	require.Contains(t, html, ">Active</span>")
	require.Contains(t, html, ">Expired</span>")
	require.Contains(t, html, ">Inactive</span>")
	require.NotContains(t, html, "Non requis", "EN locale renders no FR fallback titles")
	_ = expired
	_ = inactive
}

func TestAnimalsShowTreatmentTabFromEntries(t *testing.T) {
	f := setupPlanFixture(t)
	animalID := f.animalIDs[0]
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)

	// Protocol (animal plan) + protocol-backed treatment with one entry.
	plan := mkAnimalPlan(t, animalID, "BacklinkProto-"+f.marker, "medication",
		`{"drug":"BackDrug-`+f.marker+`","dosage":"1 ml"}`,
		fixedSchedule(today, 5), true)
	protoTr := &models.Treatment{
		Date:     today,
		AnimalID: animalID,
		Drug:     "BackDrug-" + f.marker,
		Dosage:   "1 ml",
	}
	require.NoError(t, models.DB.Create(protoTr))
	protoEntry := &models.TreatmentTimeEntry{
		TreatmentID: protoTr.ID, AnimalID: animalID,
		DueAt: today.Add(11*time.Hour + 30*time.Minute), TimeLabel: "11:30",
		Status: models.TreatmentEntryStatusDone, Source: models.TreatmentEntrySourceProtocol,
	}
	protoEntry.AppliedAt = nulls.NewTime(today.Add(11 * time.Hour))
	require.NoError(t, models.DB.Create(protoEntry))
	app := &models.CarePlanApplication{
		SourceType:      models.ApplicationSourceAnimal,
		SourceID:        plan.ID,
		SourceSnapshot:  json.RawMessage(`{"name":"BacklinkProto-` + f.marker + `"}`),
		AnimalID:        animalID,
		DueAt:           today.Add(8 * time.Hour),
		AppliedAt:       time.Now(),
		UserID:          f.userID,
		FulfillmentType: models.ApplicationFulfillmentTreatment,
		FulfillmentID:   protoTr.ID.String(),
		Status:          models.ApplicationStatusApplied,
	}
	require.NoError(t, models.DB.Create(app))

	// Manual treatment with seeded entries: done / pending / skipped.
	manualTr := &models.Treatment{
		Date:     today,
		AnimalID: animalID,
		Drug:     "ManualDrug-" + f.marker,
		Dosage:   "0.5 ml",
	}
	require.NoError(t, models.DB.Create(manualTr))
	manualEntries := models.TreatmentTimeEntries{
		{TreatmentID: manualTr.ID, AnimalID: animalID, DueAt: today.Add(8 * time.Hour), TimeLabel: "08:00", Status: models.TreatmentEntryStatusDone, Source: models.TreatmentEntrySourceManual},
		{TreatmentID: manualTr.ID, AnimalID: animalID, DueAt: today.Add(12 * time.Hour), TimeLabel: "12:00", Status: models.TreatmentEntryStatusPending, Source: models.TreatmentEntrySourceManual},
		{TreatmentID: manualTr.ID, AnimalID: animalID, DueAt: today.Add(18 * time.Hour), TimeLabel: "18:00", Status: models.TreatmentEntryStatusSkipped, Source: models.TreatmentEntrySourceManual},
	}
	for i := range manualEntries {
		require.NoError(t, models.DB.Create(&manualEntries[i]))
	}

	// Legacy treatment without entries → bitmap fallback block, no crash.
	legacyTr := &models.Treatment{
		Date:     today,
		AnimalID: animalID,
		Drug:     "LegacyDrug-" + f.marker,
		Dosage:   "1 ml",
		Timebitmap: models.Treatement_MORNING,
	}
	require.NoError(t, models.DB.Create(legacyTr))

	html := animalShowHTML(t, animalID, "en-US")

	// One dot per expected time: entry labels appear in the dot tooltips.
	require.Contains(t, html, `title="11:30 · Done`)
	require.Contains(t, html, `title="08:00 · Done`)
	require.Contains(t, html, `title="12:00 · To do`)
	require.Contains(t, html, `title="18:00 · Skipped`)

	// Protocol-backed rows link to the source protocol (same-page anchor).
	require.Contains(t, html,
		fmt.Sprintf(`href="/animals/%d#nav-plan" title="View the source protocol">`, animalID)+
			`<i class="fas fa-file-medical"></i> BacklinkProto-`+f.marker,
		"protocol-backed treatment links to its source protocol")

	// Manual + legacy rows: no protocol backlink for them — the marker-named
	// plan must be linked exactly once.
	require.Equal(t, 1, strings.Count(html, "BacklinkProto-"+f.marker+"</a>"),
		"exactly one protocol backlink (the protocol-backed row)")

	// Legacy bitmap fallback still renders for the entry-less treatment
	// (MORNING bitmap: noon/evening are "not required" minus-buttons).
	require.Contains(t, html, "Not required at noon", "legacy 3-dot fallback kept")

	// Add New treatment button (original look kept).
	require.Contains(t, html, "Add New treatment")
}

// TestParseItemRef: the ?item= deep-link reference (round-2 §8.2,
// Dash-7) splits into source type + id; anything malformed resolves to
// ok=false so a stale bookmark degrades to a normal page render.
func TestParseItemRef(t *testing.T) {
	typ, id, ok := parseItemRef("rule:med-1")
	require.True(t, ok)
	require.Equal(t, "rule", typ)
	require.Equal(t, "med-1", id)

	typ, id, ok = parseItemRef("animal:0198c0aa-7b34-73d3-8f3e-2a6f88b1d947")
	require.True(t, ok)
	require.Equal(t, "animal", typ)
	require.Equal(t, "0198c0aa-7b34-73d3-8f3e-2a6f88b1d947", id)

	// Malformed: no separator, empty halves.
	for _, ref := range []string{"", ":", "rule:", ":med-1", "nominalseparator"} {
		_, _, ok = parseItemRef(ref)
		require.False(t, ok, "parseItemRef(%q) must not parse", ref)
	}
}

// TestAnimalTodayBlock: the Treatment tab's TODAY block (round-2 §10,
// T1/T2) folds THIS animal's today medication + care occurrences into the
// shared series view; other animals' items and overridden occurrences
// stay out, and the group binds to the shared _med_series partial
// context (AnimalID/Label/Link of this animal).
func TestAnimalTodayBlock(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	from, to := TodayPlanWindow(now)
	med := testSource(careplan.KindMedication, "atm-med", "Citramox", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})
	care := testSource(careplan.KindCare, "atm-care", "Bandage", map[string]interface{}{"note": "Changer bandage"})
	plan := testPlan()
	plan.From, plan.To, plan.Now = from, to, now

	medToday := testItem(med, 1, careplan.StatusDue)
	medToday.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	careToday := testItem(care, 1, careplan.StatusDue)
	careToday.Occurrence.DueAt = time.Date(2026, 9, 28, 9, 30, 0, 0, time.Local)
	other := testItem(med, 2, careplan.StatusDue) // another animal — excluded
	other.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	overridden := testItem(med, 1, careplan.StatusOverridden)
	overridden.Occurrence.DueAt = time.Date(2026, 9, 28, 7, 30, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{medToday, careToday, other, overridden}

	animal := &models.Animal{ID: 1, YearNumber: 11, Year: 2026, Species: "Hérisson", Cage: nulls.NewString("C1")}
	b := animalTodayBlock(plan, animal)
	require.False(t, b.Empty)
	require.Equal(t, animalLabel(*animal), b.Label)
	require.Equal(t, 1, b.Group.AnimalID)
	require.Equal(t, "/animals/1", b.Group.AnimalLink)

	// Medication series first (buildMedGroups), care series appended.
	require.Len(t, b.Group.Series, 2)
	require.Equal(t, "Citramox — 0.5 ml", b.Group.Series[0].Label)
	require.Len(t, b.Group.Series[0].Rows[0], 1, "one open today slot (overridden suppressed)")
	require.Equal(t, "08:00", b.Group.Series[0].Rows[0][0].DueAtHM)
	// Care series: one line per source, note label, its own due button.
	require.Equal(t, "Changer bandage", b.Group.Series[1].Label)
	require.Equal(t, "09:30", b.Group.Series[1].Rows[0][0].DueAtHM)
	require.True(t, b.Group.Series[1].Rows[0][0].Applicable)
}

// TestAnimalTodayBlockEmpty: no today occurrences → Empty (block not
// rendered by the template).
func TestAnimalTodayBlockEmpty(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	from, to := TodayPlanWindow(now)
	plan := testPlan()
	plan.From, plan.To, plan.Now = from, to, now
	animal := &models.Animal{ID: 3, YearNumber: 13, Year: 2026}
	b := animalTodayBlock(plan, animal)
	require.True(t, b.Empty)
}
