# Fix archive — 2026-09-12 — Webhook API key visible to maintainers (role-gated)

## Original report (bugs.md)
> ## Webhook API key never visible to maintainers
> **Observed**: The stored Console webhook API key cannot be viewed by anyone: config form is write-only (`type=password`, no prefill), show page renders only `MaskedWebhookAPIKey()`, and `Config.MarshalJSON` redacts it.
> **Expected**: Maintainers can see the full stored key on the config show and edit pages; plain admins keep the current masked/write-only behavior; JSON responses stay redacted for everyone.

Note: an earlier fix (see `2026-09-03-api-key-visibility.md`) made the key fully
visible; a later security hardening re-introduced the never-echo rule for everyone
(`config_api_key_template_test.go`). This change restores maintainer visibility in a
role-gated way.

## Fix

- `actions/configs.go` (`ConfigsResource.Edit`, GET): `settings.WebhookAPIKey` is zeroed
  in the template context unless the current user is a maintainer — non-maintainers
  never receive the secret in HTML, and their blank submit still preserves the stored
  key in `Update`.
- `templates/config/_form.plush{,.de,.fr,.nl}.html`: API-key input `type` is conditional
  (`text` for maintainers, `password` otherwise) and prefilled with
  `settings.WebhookAPIKey` (empty for non-maintainers). Help text updated in all 4
  locales: maintainers can view the stored key; other admins see a write-only field.
- `templates/config/show.plush{,.de,.fr,.nl}.html`: maintainer branch renders the full
  key; plain admins keep the masked last-4 + prefix hint. Unchanged for empty key
  ("not set").
- JSON redaction (`models/config.go MarshalJSON`) intentionally untouched — API JSON
  responses never expose the key for any role.

## Tests
- `actions/config_api_key_template_test.go` rewritten as
  `TestConfigAPIKeyVisibilityByRole`: maintainer context renders the full key (show) and
  a `type="text"` input prefilled with the stored key (form); plain-admin context renders
  the masked last-4 only, a `type="password"` input, and never echoes the secret; empty
  key still renders the not-set branch. PASS.
- `models/config_mask_test.go` / `config_redact_test.go` untouched and passing.
- E2E (agent-browser, authenticated admin/maintainer): `/config/<id>/edit/` API-key
  input `type=text` prefilled with `creaves_c861…061e`;
  `/config/<id>` show page displays the full key `creaves_c861aef3-…` in a code element.

## Verification
```
go test ./actions -run 'TestConfigAPIKeyVisibilityByRole' -count=1   # ok
# browser: edit input type=text + prefilled; show page full key (maintainer)
```
Residual: plain-admin browser path (masked show / write-only form) is covered by the
plush template tests, not by an authenticated e2e session (no plain-admin account in
dev DB).
