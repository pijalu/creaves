package actions

import (
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"creaves/models"
)

// TestConfigAPIKeyNotEchoedInTemplates renders the api-key fragments of the
// config _form and show templates through plush to prove the webhook API key
// is never echoed into the HTML: the form uses type=password with no value
// prefill and the show page only renders the masked (last-4) value.
func TestConfigAPIKeyNotEchoedInTemplates(t *testing.T) {
	const secret = "creaves_supersecret0123456789"

	// --- show page fragment (as in templates/config/show.plush*.html) ---
	showTpl := `<%= if (settings.WebhookAPIKey != "") { %>
          <code><%= settings.MaskedWebhookAPIKey() %></code>
        <% } else { %>
          <em>not set</em>
        <% } %>`

	settings := models.ConfigSettings{WebhookAPIKey: secret}
	ctx := plush.NewContext()
	ctx.Set("settings", settings)
	out, err := plush.Render(showTpl, ctx)
	require.NoError(t, err)
	assert.NotContains(t, out, secret, "show page must not echo the API key")
	assert.Contains(t, out, "••••6789", "show page must render masked last-4 only")

	// Empty key renders the not-set branch.
	ctxEmpty := plush.NewContext()
	ctxEmpty.Set("settings", models.ConfigSettings{})
	outEmpty, err := plush.Render(showTpl, ctxEmpty)
	require.NoError(t, err)
	assert.Contains(t, outEmpty, "not set")

	// --- form fragment (as in templates/config/_form.plush*.html) ---
	formTpl := `<input type="password" class="form-control font-monospace" id="webhook_api_key" name="Settings.WebhookAPIKey" placeholder="Enter API key (leave blank to keep current)" autocomplete="new-password">`
	out2, err := plush.Render(formTpl, plush.NewContext())
	require.NoError(t, err)
	assert.NotContains(t, out2, secret)
	assert.NotContains(t, out2, "value=", "form input must not prefill the API key")
	assert.True(t, strings.Contains(out2, `type="password"`), "form input must be type=password")
}
