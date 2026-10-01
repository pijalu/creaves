package actions

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// Review finding #3/#4/#5: the §7.1-6/§10.3-CP6b/§10.4-M1/M4 warn-but-allow
// save guardrails.
func TestPlanRuleWarnings(t *testing.T) {
	sched := func(times []string, durationDays int) json.RawMessage {
		m := map[string]interface{}{"times": times, "anchor": "fixed", "anchor_date": "2026-06-01"}
		if durationDays > 0 {
			m["duration_days"] = durationDays
		}
		raw, err := json.Marshal(m)
		require.NoError(t, err)
		return raw
	}
	has := func(warns []string, key string) bool {
		for _, w := range warns {
			if w == key {
				return true
			}
		}
		return false
	}

	// CP6b: 07:00–23:00 inclusive window
	require.Nil(t, planRuleWarnings("feeding", sched([]string{"07:00", "23:00"}, 0)), "window bounds inclusive")
	require.True(t, has(planRuleWarnings("feeding", sched([]string{"06:59"}, 0)), "care_plan.warn.off_hours"))
	require.True(t, has(planRuleWarnings("feeding", sched([]string{"23:01"}, 0)), "care_plan.warn.off_hours"))
	require.False(t, has(planRuleWarnings("feeding", sched([]string{"08:00", "19:00"}, 5)), "care_plan.warn.off_hours"))

	// M4: open-ended medication warns, bounded does not
	require.True(t, has(planRuleWarnings("medication", sched([]string{"08:00"}, 0)), "care_plan.warn.open_ended_medication"))
	require.False(t, has(planRuleWarnings("medication", sched([]string{"08:00"}, 5)), "care_plan.warn.open_ended_medication"))
	require.False(t, has(planRuleWarnings("feeding", sched([]string{"08:00"}, 0)), "care_plan.warn.open_ended_medication"), "only medication")

	// M1: >1 medication slot in the same treatment bucket
	require.True(t, has(planRuleWarnings("medication", sched([]string{"08:00", "09:00"}, 5)), "care_plan.warn.slot_bucket"), "two morning slots")
	require.False(t, has(planRuleWarnings("medication", sched([]string{"08:00", "14:00", "19:00"}, 5)), "care_plan.warn.slot_bucket"), "one per bucket")
	require.False(t, has(planRuleWarnings("feeding", sched([]string{"08:00", "09:00"}, 5)), "care_plan.warn.slot_bucket"), "only medication")

	// broken schedule → no warnings (model validation surfaces it)
	require.Nil(t, planRuleWarnings("medication", json.RawMessage(`{broken`)))

	// unknown action kind → no medication-specific warnings
	require.True(t, has(planRuleWarnings("weighing", sched([]string{"05:00"}, 0)), "care_plan.warn.off_hours"), "off-hours applies to every kind")
}

// HTTP: saving a rule with an off-hours slot still succeeds (warn-BUT-ALLOW)
// and the flash renders on the redirected rules list, in the session locale.
func TestCareRuleSaveWarnsButAllows(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	postForm := func(form url.Values) int {
		req, err := http.NewRequest("POST", baseURL+"/care_rules", strings.NewReader(form.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "text/html")
		req.Header.Set("X-CSRF-Token", token)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}
	getHTML := func(path string) (int, string) {
		req, err := http.NewRequest("GET", baseURL+path, nil)
		require.NoError(t, err)
		req.Header.Set("Accept", "text/html")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(raw)
	}

	sched, err := json.Marshal(map[string]interface{}{
		"times": []string{"06:00"}, // outside 07:00–23:00
		"anchor": "fixed", "anchor_date": time.Now().Format("2006-01-02"),
	})
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]interface{}{
		"caretype_id": f.feedCare.String(),
		"food":        "grenouilles",
	})
	require.NoError(t, err)

	code := postForm(url.Values{
		"Name":          {"WR-" + f.marker},
		"ActionKind":    {"feeding"},
		"ActionPayload": {string(payload)},
		"Schedule":      {string(sched)},
		"Active":        {"true"},
	})
	require.Equal(t, http.StatusSeeOther, code, "warn-but-allow: the save succeeds")

	// the flash is consumed by the next page render
	code, body := getHTML("/care_rules")
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, body, "care window", "off-hours warning flashed (CP6b)")

	// rule really persisted
	n := &models.CareRules{}
	require.NoError(t, models.DB.Where("name = ?", "WR-"+f.marker).All(n))
	require.Len(t, *n, 1)
}
