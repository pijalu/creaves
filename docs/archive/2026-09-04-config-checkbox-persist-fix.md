# Fix archive — 2026-09-04 — creaves: config edit resets event forwarding and webhook flags to off

## Original report (bugs.md "creaves - config error")

> Configuration does not allow to enable event forwarding/webhook - if checked, edit will set them off.

Related: "creaves - hook config" (max-per-minute bigger than form check — that value was fine; the form `max="10000"` allows it) and "creaves - force resync" (error `webhook forwarding is disabled` follows from flags never persisting).

## Root cause

`templates/config/_form.plush*.html` (all 4 locales) render each flag as a hidden `false` input followed by the checkbox `true` input:

```html
<input type="hidden" name="Settings.EnableEventStream" value="false" />
<input type="checkbox" ... name="Settings.EnableEventStream" value="true" ...>
```

A checked box therefore submits two values `["false","true"]` (verified live via page HTML). Buffalo builds params with `url.Values` and `c.Param(key)` = `Params().Get(key)` = **first value only** (`net/url.Values.Get`). So `c.Param(...) == "true"` in `actions/configs.go` `Create`/`Update` always saw `"false"` and saved both flags off on every edit. Same flaw affected the `Active` checkbox (button-style `CheckboxTag` without hidden input was unaffected, but now uses the same helper).

## Fix

- `actions/configs.go`: new `paramIsTrue(c, key)` helper scans all submitted values in `c.Request().Form[key]` and returns true if any is `"true"`; falls back to `c.Param` when the request form is unavailable. `Create`/`Update` use it for `Active`, `Settings.EnableEventStream`, `Settings.WebhookEnabled`.
- Regression tests `actions/config_checkbox_test.go`: `TestParamIsTrueCheckedBoxes` (real Buffalo app, POST `[false true]` for each flag → true) and `TestParamIsTrueUncheckedBox` (lone `false` → false).
- Incidental: restored a dropped `if err != nil` after `tx.ValidateAndCreate` in `Create` (accidentally removed in the first edit pass; `go vet` caught unused `err`).

No template changes needed — the hidden+checkbox pattern is the standard unchecked-value convention; the server now reads it correctly.

## Validation

- `go vet ./actions/` — clean.
- `go test -tags sqlite -count=1 ./actions/ -run 'TestParamIsTrue|TestParseWebhookLimits|TestConfig'` — PASS (incl. pre-existing `TestParseWebhookLimitsClampsValues`, template tests).
- `go test -tags sqlite -count=1 ./models/ -run TestConfig` — PASS.
- Live (agent-browser, `buffalo dev` :3000, admin/admin): edit page showed both boxes unchecked; checked both → Save → show page "Config updated successfully", Event Stream ✓ / Webhook Enabled ✓; reopened edit → both boxes `checked=true`. Before fix, re-save would clear them.

## Commits

- creaves: fix + tests (uncommitted at archive time; commit with message below)
- Suggested: `fix(config): persist event-stream/webhook checkboxes on edit (first-value-wins c.Param fix)`
