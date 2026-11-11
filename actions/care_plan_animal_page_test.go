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
		Date:       today,
		AnimalID:   animalID,
		Drug:       "LegacyDrug-" + f.marker,
		Dosage:     "1 ml",
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
	// plan must be linked exactly once in the treatment entries. (R4-6: the
	// applicable-protocol trace links the same name again, in its own
	// table — hence the scoping here.)
	require.Equal(t, 1, strings.Count(html, "fa-file-medical\"></i> BacklinkProto-"+f.marker+"</a>"),
		"exactly one protocol backlink (the protocol-backed row)")

	// Legacy bitmap fallback still renders for the entry-less treatment
	// (MORNING bitmap: noon/evening are "not required" minus-buttons) —
	// now inside the legacy-treatments journal on the protocol tab
	// (bugs.md 2026-10-27 #2: the Treatment tab retired, its unique
	// journal moved there as a collapsed history card).
	require.Contains(t, html, "Not required at noon", "legacy 3-dot fallback kept")
	require.Contains(t, html, `id="planLegacyJournal"`, "the legacy journal lives in the protocol tab")
	// The legacy per-animal "Add New treatment" shortcut is gone with the
	// tab; legacy treatments remain creatable from the edit page and the
	// global New menu.
	require.NotContains(t, html, "Add New treatment")
}

// TestAnimalPlanTodayRows: the Treatment tab's plan rows (bugs.md U26 —
// fix 7) fold THIS animal's today medication + observation + care
// occurrences into accordion rows — statuses (done/skipped/pending/missed),
// sorted by due time; other animals' items and overridden occurrences
// stay out, and no row repeats the animal label (the tab already names
// the animal).
func TestAnimalPlanTodayRows(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	from, to := TodayPlanWindow(now)
	med := testSource(careplan.KindMedication, "atr-med", "Citramox", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})
	obs := testSource(careplan.KindObservation, "atr-obs", "Nettoyage", map[string]interface{}{"prompt": "Nettoyage Fistule"})
	care := testSource(careplan.KindCare, "atr-care", "Bandage", map[string]interface{}{"note": "Changer bandage"})
	plan := testPlan()
	plan.From, plan.To, plan.Now = from, to, now

	done := testItem(med, 1, careplan.StatusApplied)
	done.Occurrence.DueAt = time.Date(2026, 9, 28, 7, 0, 0, 0, time.Local)
	done.Application = &careplan.ApplicationView{Status: "applied", AppliedAt: time.Date(2026, 9, 28, 7, 5, 0, 0, time.Local)}
	missed := testItem(obs, 1, careplan.StatusDue)
	missed.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	skipped := testItem(care, 1, careplan.StatusSkipped)
	skipped.Occurrence.DueAt = time.Date(2026, 9, 28, 9, 30, 0, 0, time.Local)
	pending := testItem(med, 1, careplan.StatusDue)
	pending.Occurrence.DueAt = time.Date(2026, 9, 28, 14, 0, 0, 0, time.Local)
	other := testItem(med, 2, careplan.StatusDue) // another animal — excluded
	other.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	overridden := testItem(med, 1, careplan.StatusOverridden)
	overridden.Occurrence.DueAt = time.Date(2026, 9, 28, 7, 30, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{done, missed, skipped, pending, other, overridden}

	animal := &models.Animal{ID: 1, YearNumber: 11, Year: 2026, Species: "Hérisson", Cage: nulls.NewString("C1")}
	rows, err := animalPlanTodayRows(models.DB, plan, animal, nil)
	require.NoError(t, err)
	require.Len(t, rows, 4)

	// Sorted by due time, one row per occurrence with its status.
	require.Equal(t, "07:00", rows[0].DueHM)
	require.Equal(t, "done", rows[0].Status)
	require.Equal(t, "2026-09-28T07:05:00+02:00", rows[0].AppliedAt)
	require.Equal(t, "08:00", rows[1].DueHM)
	require.Equal(t, "missed", rows[1].Status)
	require.Equal(t, "Nettoyage Fistule", rows[1].Label)
	require.Equal(t, "09:30", rows[2].DueHM)
	require.Equal(t, "skipped", rows[2].Status)
	require.Equal(t, "Changer bandage", rows[2].Label)
	require.Equal(t, "14:00", rows[3].DueHM)
	require.Equal(t, "pending", rows[3].Status)
	require.Equal(t, "Citramox (0.5 ml)", rows[3].Label)

	// No animal label anywhere — the tab already names the animal (U26).
	for _, r := range rows {
		require.NotContains(t, r.Label, "Hérisson")
		require.NotContains(t, r.Label, "C1")
	}
}

// TestAnimalPlanTodayRowsDedupe: occurrences whose drug/prompt already
// exists as a legacy treatment of the SAME day are deduped away — the
// legacy row keeps showing that work (new must not stack on top of old,
// bugs.md U26); same-label treatments of other days do not dedupe.
func TestAnimalPlanTodayRowsDedupe(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	from, to := TodayPlanWindow(now)
	plan := testPlan()
	plan.From, plan.To, plan.Now = from, to, now
	medDup := testSource(careplan.KindMedication, "atd-med", "Dup", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})
	obsDup := testSource(careplan.KindObservation, "atd-obs", "Obs", map[string]interface{}{"prompt": "Nettoyage Fistule"})
	medKeep := testSource(careplan.KindMedication, "atd-new", "New", map[string]interface{}{"drug": "Itra"})
	dup1 := testItem(medDup, 1, careplan.StatusDue)
	dup1.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	dup2 := testItem(obsDup, 1, careplan.StatusDue)
	dup2.Occurrence.DueAt = time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)
	keep := testItem(medKeep, 1, careplan.StatusDue)
	keep.Occurrence.DueAt = time.Date(2026, 9, 28, 14, 0, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{dup1, dup2, keep}

	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	animal := &models.Animal{ID: 1, Treatments: models.Treatments{
		{Date: today, Drug: "citramox "}, // case/whitespace-insensitive match
		{Date: today, Drug: "Nettoyage Fistule"},
		{Date: today.AddDate(0, 0, -2), Drug: "Itra"}, // other day → no dedupe
	}}
	rows, err := animalPlanTodayRows(models.DB, plan, animal, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Itra", rows[0].Label, "dosage-less medication keeps the bare drug label")
	require.Equal(t, "14:00", rows[0].DueHM)
	require.Equal(t, "pending", rows[0].Status)
}

// TestAnimalPlanTodayRowsEmpty: no today occurrences (and a nil plan)
// yield no rows — the accordion renders only the legacy treatments.
func TestAnimalPlanTodayRowsEmpty(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	from, to := TodayPlanWindow(now)
	plan := testPlan()
	plan.From, plan.To, plan.Now = from, to, now
	animal := &models.Animal{ID: 3, YearNumber: 13, Year: 2026}
	rows, err := animalPlanTodayRows(models.DB, plan, animal, nil)
	require.NoError(t, err)
	require.Empty(t, rows)

	rows, err = animalPlanTodayRows(models.DB, nil, animal, nil)
	require.NoError(t, err)
	require.Empty(t, rows)
}
