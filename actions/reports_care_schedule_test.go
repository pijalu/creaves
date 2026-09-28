package actions

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// TestReportsCareScheduleSmoke renders GET /reports/care_schedule for the
// three grouping modes (Phase 5, bugs.md U9/U10): 200 + group markers,
// read-only (no apply buttons), legacy urgency row classes.
func TestReportsCareScheduleSmoke(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	_ = f.feedRule(t, models.DB, itemDueSoon(time.Now()))

	markers := map[string]string{
		"zone":   "reportGroupSwitch",
		"cage":   "reportGroupSwitch",
		"animal": "reportGroupSwitch",
	}
	for _, group := range []string{"", "zone", "cage", "animal"} {
		group := group
		name := group
		if name == "" {
			name = "default"
		}
		t.Run(name, func(t *testing.T) {
			url := baseURL + "/reports/care_schedule"
			if group != "" {
				url += "?group=" + group
			}
			req, err := http.NewRequest("GET", url, nil)
			require.NoError(t, err)
			req.Header.Set("Accept", "text/html")
			resp, err := client.Do(req)
			require.NoError(t, err)
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode,
				"group=%s rendered: %.500s", group, raw)
			body := string(raw)
			g := group
			if g == "" {
				g = "zone"
			}
			require.Contains(t, body, markers[g])
			// Read-only: none of the work-screen apply controls.
			require.NotContains(t, body, "plan-apply-btn")
			require.NotContains(t, body, "plan-skip-btn")
			require.NotContains(t, body, "/care_plan/apply_batch")
			// i18n title resolved (no missing-key artifact). The default
			// UI language is en-US (app.go i18n.New).
			require.Contains(t, body, "Care schedule")
		})
	}
}

// TestReportsCareScheduleLocales renders the report in all four UI
// languages (locale fork smoke — a broken fork fails here).
func TestReportsCareScheduleLocales(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	_ = f.feedRule(t, models.DB, itemDueSoon(time.Now()))

	want := map[string]string{
		"fr":    "Planning des soins",
		"en-US": "Care schedule",
		"de":    "Pflegeplan",
		"nl":    "Zorgplanning",
	}
	for _, lang := range []string{"fr", "en-US", "de", "nl"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			req, err := http.NewRequest("GET", baseURL+"/reports/care_schedule?group=cage", nil)
			require.NoError(t, err)
			req.Header.Set("Accept", "text/html")
			req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
			resp, err := client.Do(req)
			require.NoError(t, err)
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode, "%s: %.500s", lang, raw)
			require.Contains(t, string(raw), want[lang],
				"%s must render the localized report title", lang)
		})
	}
}

// TestScheduleRowClass pins the legacy urgency mapping (L1/L3/L6).
func TestScheduleRowClass(t *testing.T) {
	require.Equal(t, "table-danger", scheduleRowClass(careplan.StatusLate))
	require.Equal(t, "table-danger", scheduleRowClass(careplan.StatusMissing))
	require.Equal(t, "table-warning", scheduleRowClass(careplan.StatusDue))
	require.Equal(t, "table-info", scheduleRowClass(careplan.StatusScheduled))
	require.Equal(t, "table-success", scheduleRowClass(careplan.StatusApplied))
	require.Equal(t, "table-success", scheduleRowClass(careplan.StatusSkipped))
	require.Equal(t, "table-success", scheduleRowClass(careplan.StatusDeferred))
	require.Equal(t, "", scheduleRowClass(careplan.StatusOverridden))
}

// TestScheduleClockSlot pins the AM/noon/PM medication dot mapping (L7).
func TestScheduleClockSlot(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2026, 9, 28, h, 30, 0, 0, time.Local) }
	require.Equal(t, "morning", scheduleClockSlot(careplan.KindMedication, at(8)))
	require.Equal(t, "morning", scheduleClockSlot(careplan.KindMedication, at(10)))
	require.Equal(t, "noon", scheduleClockSlot(careplan.KindMedication, at(11)))
	require.Equal(t, "noon", scheduleClockSlot(careplan.KindMedication, at(14)))
	require.Equal(t, "evening", scheduleClockSlot(careplan.KindMedication, at(15)))
	require.Equal(t, "evening", scheduleClockSlot(careplan.KindMedication, at(20)))
	require.Equal(t, "", scheduleClockSlot(careplan.KindFeeding, at(8)))
	require.Equal(t, "", scheduleClockSlot(careplan.KindCare, at(18)))
}

// TestCollapseCageRows: same kind+detail at the same time collapses into
// one grouped row (worst urgency wins); diverging rows pass through.
func TestCollapseCageRows(t *testing.T) {
	rows := []ScheduleRowView{
		{Date: "2026-09-28", Time: "09:00", Kind: "feeding", Detail: "vers", Status: "due", RowClass: "table-warning", AnimalCount: 1, AnimalLabel: "A"},
		{Date: "2026-09-28", Time: "09:00", Kind: "feeding", Detail: "vers", Status: "missing", RowClass: "table-danger", AnimalCount: 1, AnimalLabel: "B"},
		{Date: "2026-09-28", Time: "09:00", Kind: "feeding", Detail: "vers", Status: "applied", RowClass: "table-success", AnimalCount: 1, AnimalLabel: "C"},
		{Date: "2026-09-28", Time: "09:00", Kind: "feeding", Detail: "souris", Status: "due", RowClass: "table-warning", AnimalCount: 1, AnimalLabel: "D"},
		{Date: "2026-09-28", Time: "10:00", Kind: "cleanup", Detail: "", Status: "scheduled", RowClass: "table-info", AnimalCount: 1, AnimalLabel: "E"},
	}
	out := collapseCageRows(rows)
	require.Len(t, out, 3)
	// Grouped row: 3 animals, worst status (missing) wins.
	require.Equal(t, 3, out[0].AnimalCount)
	require.Equal(t, "missing", out[0].Status)
	require.Equal(t, "table-danger", out[0].RowClass)
	// Diverging detail and different kind/time stay separate.
	require.Equal(t, 1, out[1].AnimalCount)
	require.Equal(t, "souris", out[1].Detail)
	require.Equal(t, 1, out[2].AnimalCount)
	require.Equal(t, "10:00", out[2].Time)
}

// TestCareScheduleLocaleKeysPresent mirrors TestAnnualLocaleKeysPresent:
// every care_plan.report.* key + the nav key exists in all 4 locales.
func TestCareScheduleLocaleKeysPresent(t *testing.T) {
	reportKeys := []string{
		"care_plan.report.title",
		"care_plan.report.from",
		"care_plan.report.to",
		"care_plan.report.filter",
		"care_plan.report.group.zone",
		"care_plan.report.group.cage",
		"care_plan.report.group.animal",
		"care_plan.report.col.time",
		"care_plan.report.col.date",
		"care_plan.report.col.animal",
		"care_plan.report.col.animals",
		"care_plan.report.col.care",
		"care_plan.report.col.status",
		"care_plan.report.animals",
		"care_plan.report.empty",
		"care_plan.report.no_zone",
	}
	for _, lang := range []string{"en-us", "fr", "de", "nl"} {
		b, err := os.ReadFile("../locales/care_plan." + lang + ".yaml")
		if err != nil {
			t.Fatalf("read locale %s: %v", lang, err)
		}
		for _, key := range reportKeys {
			if !strings.Contains(string(b), `id: "`+key+`"`) {
				t.Errorf("locale care_plan.%s.yaml missing key %s", lang, key)
			}
		}
		nb, err := os.ReadFile("../locales/reports." + lang + ".yaml")
		if err != nil {
			t.Fatalf("read locale %s: %v", lang, err)
		}
		if !strings.Contains(string(nb), `id: "nav.reports_care_schedule"`) {
			t.Errorf("locale reports.%s.yaml missing key nav.reports_care_schedule", lang)
		}
	}
}

// TestBuildCareScheduleViewGroups projects a synthetic plan into the three
// groupings (pure view model — no DB).
func TestBuildCareScheduleViewGroups(t *testing.T) {
	plan := &DayPlan{
		From: time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local),
		To:   time.Date(2026, 9, 30, 23, 59, 59, 0, time.Local),
		Now:  time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local),
	}
	require.NotNil(t, plan)

	// No items: every grouping renders an empty, non-nil view.
	for _, g := range []string{ReportGroupZone, ReportGroupCage, ReportGroupAnimal} {
		v := BuildCareScheduleView(plan, g, plan.Now)
		require.True(t, v.Empty, "group=%s", g)
		require.Equal(t, 0, v.Total)
		require.Equal(t, "2026-09-28", v.From)
		require.Equal(t, "2026-09-30", v.To)
		require.Equal(t, "2026-09-28", v.Today)
	}
}
