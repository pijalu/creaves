//go:build sqlite
// +build sqlite

package actions

import (
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeliveryFilterClause pins the `delivery` query-param mapping used by
// EventStreamsResource.List. Unknown/empty values must yield no clause so a
// bad link cannot blank the listing.
func TestDeliveryFilterClause(t *testing.T) {
	cases := map[string]string{
		"":              "",
		"all":           "",
		"bogus":         "",
		"delivered":     "delivered_at IS NOT NULL",
		"not-delivered": "delivered_at IS NULL",
	}
	for in, want := range cases {
		assert.Equal(t, want, deliveryFilterClause(in), "delivery=%q", in)
	}
}

// stubHelpers registers minimal t/linkTo stand-ins so plush can render
// template fragments outside the buffalo render engine.
func stubHelpers(ctx *plush.Context) {
	ctx.Set("t", func(key string) string { return key })
	ctx.Set("linkTo", func(href string, opts map[string]interface{}) string {
		cls, _ := opts["class"].(string)
		return "LINK[" + href + "|" + cls + "]"
	})
}

// TestEventStreamsIndexDeliveryFilterRenders renders the filter button group
// and delivered/processed badge columns (as embedded in
// templates/event_streams/index.plush*.html) through plush to catch template
// syntax regressions and active-state mistakes.
func TestEventStreamsIndexDeliveryFilterRenders(t *testing.T) {
	tpl := `<div class="btn-group mb-3" role="group" aria-label="Delivery filter">
  <%= if (deliveryFilter == "delivered" || deliveryFilter == "not-delivered") { %>
    <%= linkTo("/event_streams", {class: "btn btn-outline-secondary active", body: t("event-streams.index.filter-all")}) %>
  <% } else { %>
    <%= linkTo("/event_streams", {class: "btn btn-secondary active", body: t("event-streams.index.filter-all")}) %>
  <% } %>
  <%= if (deliveryFilter == "not-delivered") { %>
    <%= linkTo("/event_streams?delivery=not-delivered", {class: "btn btn-warning active", body: t("event-streams.index.filter-not-delivered")}) %>
  <% } else { %>
    <%= linkTo("/event_streams?delivery=not-delivered", {class: "btn btn-outline-warning", body: t("event-streams.index.filter-not-delivered")}) %>
  <% } %>
  <%= if (deliveryFilter == "delivered") { %>
    <%= linkTo("/event_streams?delivery=delivered", {class: "btn btn-success active", body: t("event-streams.index.filter-delivered")}) %>
  <% } else { %>
    <%= linkTo("/event_streams?delivery=delivered", {class: "btn btn-outline-success", body: t("event-streams.index.filter-delivered")}) %>
  <% } %>
</div>
<%= if (event.DeliveredAt) { %><span class="badge badge-success">D-YES</span><% } else { %><span class="badge badge-warning">D-NO</span><% } %>`

	render := func(t *testing.T, filter string, delivered bool) string {
		ctx := plush.NewContext()
		stubHelpers(ctx)
		ctx.Set("deliveryFilter", filter)
		ev := &models.EventStream{}
		if delivered {
			now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
			ev.DeliveredAt = &now
		}
		ctx.Set("event", ev)
		out, err := plush.Render(tpl, ctx)
		require.NoError(t, err)
		return out
	}

	// Default (no filter): All is the active solid button.
	out := render(t, "", false)
	assert.Contains(t, out, "btn-secondary active")
	assert.NotContains(t, out, "btn-warning active")
	assert.NotContains(t, out, "btn-success active")
	assert.Contains(t, out, "D-NO")

	// not-delivered filter: warning button active.
	out = render(t, "not-delivered", false)
	assert.Contains(t, out, "btn-warning active")
	assert.NotContains(t, out, "btn-secondary active")
	assert.NotContains(t, out, "btn-success active")

	// delivered filter: success button active.
	out = render(t, "delivered", true)
	assert.Contains(t, out, "btn-success active")
	assert.NotContains(t, out, "btn-secondary active")
	assert.NotContains(t, out, "btn-warning active")
	assert.Contains(t, out, "D-YES")

	// All three links always present, exactly once each.
	for _, frag := range []string{
		"LINK[/event_streams|",
		"LINK[/event_streams?delivery=not-delivered|",
		"LINK[/event_streams?delivery=delivered|",
	} {
		assert.Equal(t, 1, strings.Count(out, frag), frag)
	}
}

// TestEventStreamsShowLocallyProcessedHintRenders renders the Locally
// Processed row incl. clarification hint (as embedded in
// templates/event_streams/show.plush*.html).
func TestEventStreamsShowLocallyProcessedHintRenders(t *testing.T) {
	tpl := `<td><strong>Locally Processed</strong></td>
<td>
  <%= if (eventStream.ProcessedAt) { %>
    <span class="badge badge-success">YES</span> - <%= eventStream.ProcessedAt.Format("2006-01-02 15:04:05") %>
  <% } else { %>
    <span class="badge badge-warning">NO</span>
  <% } %>
  <br/><small class="text-muted"><%= t("event-streams.show.locally-processed-hint") %></small>
</td>`

	ctx := plush.NewContext()
	stubHelpers(ctx)
	out, err := plush.Render(tpl, ctx)
	require.NoError(t, err)
	assert.Contains(t, out, `badge badge-warning`)
	assert.Contains(t, out, `locally-processed-hint`)
}
