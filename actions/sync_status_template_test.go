//go:build sqlite
// +build sqlite

package actions

import (
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWebhookResyncIndexSyncPanelRenders renders the sync-visibility panel
// fragment (as embedded in templates/webhook_resync/index.plush*.html) through
// plush to catch template syntax regressions.
func TestWebhookResyncIndexSyncPanelRenders(t *testing.T) {
	tpl := `<%= if (syncStatus) { %>
<div id="sync-visibility"><%= syncStatus.ExpectedTotal %>|<%= syncStatus.NeverSynced %>|<%= syncStatus.ExpectedChecksum %>
<%= for (y) in syncStatus.Years { %><tr><td><%= y.Year %></td><td><%= y.Total %></td><td><%= y.Confirmed %></td><td><%= y.Unconfirmed %></td></tr><% } %>
</div>
<% } %>
<span id="resync-status"></span>`

	ctx := plush.NewContext()
	ctx.Set("syncStatus", &SyncStatus{
		ExpectedTotal:    4,
		StateConfirmed:   1,
		StateUnconfirmed: 3,
		NeverSynced:      2,
		ExpectedChecksum: "sha256:abc",
		Years: []SyncStatusYear{
			{Year: 2025, Total: 2, Confirmed: 1, Unconfirmed: 1},
			{Year: 2026, Total: 2, Unconfirmed: 2},
		},
	})
	out, err := plush.Render(tpl, ctx)
	require.NoError(t, err)
	assert.Contains(t, out, "4|2|sha256:abc")
	assert.Equal(t, 2, strings.Count(out, "</tr>"), "one row per year")

	// Nil status must hide the panel entirely (no panic).
	ctxNil := plush.NewContext()
	outNil, err := plush.Render(tpl, ctxNil)
	require.NoError(t, err)
	assert.NotContains(t, outNil, "4|2|")
}
