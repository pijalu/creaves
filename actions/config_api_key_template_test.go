package actions

import (
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"creaves/models"
)

// TestConfigAPIKeyVisibilityByRole renders the api-key fragments of the
// config _form and show templates through plush to prove the webhook API
// key is role-gated: maintainers see the full stored key (prefilled text
// input, full key on the show page) while plain admins keep the masked,
// write-only behavior (the Edit handler blanks the key in the template
// context for non-maintainers).
func TestConfigAPIKeyVisibilityByRole(t *testing.T) {
	const secret = "creaves_supersecret0123456789"

	// --- show page fragment (as in templates/config/show.plush*.html) ---
	showTpl := `<%= if (settings.WebhookAPIKey != "") { %>
          <%= if (current_user.Maintainer) { %>
            <code><%= settings.WebhookAPIKey %></code>
          <% } else { %>
            <code><%= settings.MaskedWebhookAPIKey() %></code>
          <% } %>
        <% } else { %>
          <em>not set</em>
        <% } %>`

	settings := models.ConfigSettings{WebhookAPIKey: secret}

	// Maintainer: the full key is rendered.
	maintainerCtx := plush.NewContext()
	maintainerCtx.Set("settings", settings)
	maintainerCtx.Set("current_user", &models.User{Maintainer: true})
	out, err := plush.Render(showTpl, maintainerCtx)
	require.NoError(t, err)
	assert.Contains(t, out, secret, "maintainer must see the full API key")

	// Plain admin: masked last-4 only, secret never echoed.
	adminCtx := plush.NewContext()
	adminCtx.Set("settings", settings)
	adminCtx.Set("current_user", &models.User{})
	outAdmin, err := plush.Render(showTpl, adminCtx)
	require.NoError(t, err)
	assert.NotContains(t, outAdmin, secret, "plain admin must not see the full API key")
	assert.Contains(t, outAdmin, "••••6789", "plain admin must see masked last-4 only")

	// Empty key renders the not-set branch.
	emptyCtx := plush.NewContext()
	emptyCtx.Set("settings", models.ConfigSettings{})
	emptyCtx.Set("current_user", &models.User{Maintainer: true})
	outEmpty, err := plush.Render(showTpl, emptyCtx)
	require.NoError(t, err)
	assert.Contains(t, outEmpty, "not set")

	// --- form fragment (as in templates/config/_form.plush*.html) ---
	formTpl := `<input type="<%= if (current_user.Maintainer) { %>text<% } else { %>password<% } %>" class="form-control font-monospace" id="webhook_api_key" name="Settings.WebhookAPIKey" value="<%= settings.WebhookAPIKey %>" placeholder="Enter API key" autocomplete="new-password">`

	// Maintainer: prefilled text input with the stored key.
	outMaint, err := plush.Render(formTpl, maintainerCtx)
	require.NoError(t, err)
	assert.Contains(t, outMaint, `type="text"`, "maintainer form input must be type=text")
	assert.Contains(t, outMaint, `value="`+secret+`"`, "maintainer form input must prefill the stored key")

	// Plain admin (settings blanked server-side by Configs Edit): password
	// input, secret never echoed.
	blankCtx := plush.NewContext()
	blankCtx.Set("settings", models.ConfigSettings{})
	blankCtx.Set("current_user", &models.User{})
	outPlain, err := plush.Render(formTpl, blankCtx)
	require.NoError(t, err)
	assert.Contains(t, outPlain, `type="password"`, "plain admin form input must be type=password")
	assert.NotContains(t, outPlain, secret, "plain admin form input must not echo the API key")
}
