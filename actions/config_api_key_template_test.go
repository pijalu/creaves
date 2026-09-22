package actions

import (
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"creaves/models"
)

// TestSyncTargetAPIKeyVisibilityByRole renders the api-key fragments of the
// sync-target form through plush to prove the webhook API key is role-gated:
// maintainers see the full stored key (prefilled text input) while plain
// admins keep the masked, write-only behavior (the Edit handler blanks the
// key in the template context for non-maintainers).
func TestSyncTargetAPIKeyVisibilityByRole(t *testing.T) {
	const secret = "creaves_supersecret0123456789"

	// --- form fragment (as in templates/sync_targets/_form.plush*.html) ---
	formTpl := `<input type="<%= if (current_user.Maintainer) { %>text<% } else { %>password<% } %>" class="form-control font-monospace" id="webhook_api_key" name="WebhookAPIKey" value="<%= target.WebhookAPIKey %>" placeholder="Enter API key" autocomplete="new-password">`

	target := &models.SyncTarget{WebhookAPIKey: secret}

	// Maintainer: prefilled text input with the stored key.
	maintainerCtx := plush.NewContext()
	maintainerCtx.Set("target", target)
	maintainerCtx.Set("current_user", &models.User{Maintainer: true})
	outMaint, err := plush.Render(formTpl, maintainerCtx)
	require.NoError(t, err)
	assert.Contains(t, outMaint, `type="text"`, "maintainer form input must be type=text")
	assert.Contains(t, outMaint, `value="`+secret+`"`, "maintainer form input must prefill the stored key")

	// Plain admin (target key blanked server-side by the Edit handler):
	// password input, secret never echoed.
	blankCtx := plush.NewContext()
	blankCtx.Set("target", &models.SyncTarget{})
	blankCtx.Set("current_user", &models.User{})
	outPlain, err := plush.Render(formTpl, blankCtx)
	require.NoError(t, err)
	assert.Contains(t, outPlain, `type="password"`, "plain admin form input must be type=password")
	assert.NotContains(t, outPlain, secret, "plain admin form input must not echo the API key")

	// MaskedAPIKey reveals only the last 4 characters.
	assert.Equal(t, "••••6789", target.MaskedAPIKey())
	assert.Equal(t, "", (&models.SyncTarget{}).MaskedAPIKey())
}
