//go:build sqlite
// +build sqlite

package actions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The resync page (templates/webhook_resync/index.plush*.html) polls
// /webhook_resync/status.json and must surface the delivery accounting of a
// run: delivered and failed counters plus the failure diagnostic alert
// (bugs.md #1). All four locales share the same contract.

var resyncTemplateLocales = []string{
	"index.plush.html",
	"index.plush.de.html",
	"index.plush.fr.html",
	"index.plush.nl.html",
}

// extractResyncScript pulls the polling <script> body out of a resync index
// template (the last <script> block contains the poller).
func extractResyncScript(t *testing.T, tpl string) string {
	t.Helper()
	i := strings.LastIndex(tpl, "<script>")
	j := strings.LastIndex(tpl, "</script>")
	require.GreaterOrEqual(t, j, i, "script block not found")
	require.Greater(t, j, i+8, "empty script block")
	return tpl[i+8 : j]
}

func TestResyncIndexTemplatesRenderDeliveryAccounting(t *testing.T) {
	for _, name := range resyncTemplateLocales {
		name := name
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "templates", "webhook_resync", name))
			require.NoError(t, err)
			tpl := string(raw)

			// The poller must read the new accounting fields and render the
			// failure alert (escaped) for failed runs.
			script := extractResyncScript(t, tpl)
			assert.Contains(t, script, "run.events_delivered", "delivered counter missing in %s", name)
			assert.Contains(t, script, "run.events_failed", "failed counter missing in %s", name)
			assert.Contains(t, script, "resync-failure", "failure alert missing in %s", name)
			assert.Contains(t, script, "esc(", "run-provided strings must be HTML-escaped in %s", name)
			// The cancel form (plush authenticity_token) must survive.
			assert.Contains(t, tpl, "authenticity_token")
		})
	}
}

// TestResyncStatusPayloadExposesAccounting pins the JSON contract between the
// status endpoint and the page poller: the model's JSON tags must expose
// events_delivered/events_failed so the UI can reflect them.
func TestResyncStatusPayloadExposesAccounting(t *testing.T) {
	now := time.Now()
	run := &models.ResyncRun{
		ID:              uuid.Must(uuid.NewV4()),
		InstanceID:      "test-instance",
		Status:          "failed",
		StartedAt:       now,
		FinishedAt:      &now,
		EventsCreated:   10,
		EventsDelivered: 7,
		EventsFailed:    3,
	}
	data, err := json.Marshal(run)
	require.NoError(t, err)
	out := string(data)
	assert.Contains(t, out, `"events_delivered":7`)
	assert.Contains(t, out, `"events_failed":3`)
}
