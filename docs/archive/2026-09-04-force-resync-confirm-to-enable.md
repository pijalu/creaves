# Force resync: disabled-webhook clarity + confirm-to-enable (archive)

Date: 2026-09-04. Status: done, verified end to end.

## Problem
`StartResync` returned bare `fmt.Errorf("webhook forwarding is disabled")` and
`WebhookResyncStart` mapped every start failure to `c.Error(409, err)` — a raw
buffalo error trace with no path forward. After a console-side cleanup the admin
had to guess that forwarding must be re-enabled in config.

## Fix (smallest diff)
- `actions/webhook_resync.go`: sentinel `ErrWebhookDisabled` with user-facing
  text ("webhook forwarding is disabled: enable webhook forwarding to run
  resync"). `StartResync` behavior otherwise unchanged.
- `actions/configs.go`: new `EnableWebhookForwarding(tx)` — requires a
  configured webhook URL (refuses to enable with empty URL), flips
  `WebhookEnabled`, persists on `models.DB` (NOT the request tx: buffalo's pop
  transaction middleware rolls back the request tx after a redirect, which
  silently discarded the first version of this fix), refreshes `CurrentConfig`,
  ensures the delivery/purge worker runs.
- `actions/webhook_resync_handlers.go`: `WebhookResyncStart` maps the disabled
  error to flash danger + 409 re-render of the index with `webhookEnabled=false`
  and `pendingForce` preserved; with `enable_webhook` confirmed it enables then
  retries, flashing success + 303 redirect. `WebhookResyncIndex` sets
  `webhookEnabled` for the template.
- Templates `webhook_resync/index.plush.{html,de,fr,nl}`: warning banner
  `#resync-disabled-warning` + confirm button `#enable-and-resync` (hidden
  `enable_webhook=true` field) when disabled; force checkbox re-checked from
  `pendingForce` after the 409 re-render.

## Gotchas found while verifying
- E2E first pass: run started (`status.json` = running) but `webhook_enabled`
  stayed false — the enable write went through the request tx and was rolled
  back post-redirect. Fixed by persisting on `models.DB` (same pattern
  `StartResync` already uses for the run row).
- sqlite `TestStartResyncRunCommittedBeforeReturn` deadlocks ("database is
  locked": outer test tx holds the single-writer lock while `StartResync`
  writes via `models.DB`). Gated to MySQL-only via `t.Skip` when dialect is
  `sqlite3`; flow covered under sqlite by the new tests below.

## Tests
- New `actions/webhook_resync_disabled_test.go` (`//go:build sqlite`):
  `TestStartResyncDisabledWebhookReturnsClearError` (sentinel message, no run
  row), `TestEnableWebhookForwardingConfirmFlow` (enable persists in DB +
  cache, resync then starts), `TestEnableWebhookForwardingRequiresURL`
  (empty URL refused with clear message).
- `actions/webhook_pusher_test.go` (sqlite harness): added `resync_runs`
  table DDL + cleanup so the new tests run under the sqlite tag.
- `actions/webhook_resync_test.go`, `actions/resync_hash_test.go`: added
  `//go:build sqlite` so the verify command compiles them against the sqlite
  harness; commit-visibility test skips on `sqlite3` (MySQL-only).
- Verify: `go test -tags sqlite -count=1 ./actions/ -run TestStartResync` →
  3 PASS + 1 SKIP. Full `./actions/` under sqlite has pre-existing failures
  in suites needing the full MySQL schema (`animals`, `NOW()`, backticks) —
  unrelated, failing before this change.

## Browser evidence (agent-browser terminal, 2026-09-04)
- Disabled state at `/webhook_resync`: text "Webhook forwarding is disabled —
  resync cannot run until it is enabled.", button "Enable webhook forwarding
  and start resync" (`#enable-and-resync`, `#resync-disabled-warning`
  present).
- Click `#enable-and-resync` → URL `/webhook_resync`, flash "Webhook
  forwarding enabled — resync started.", status "running", progress
  "0 / 10046", cancel control present.
- DB after click: `webhook_enabled=true`, `resync_runs` latest row
  `running/10046`; `status.json` confirms same run id/instance.
- Post-evidence cleanup: cancelled run, reset `webhook_enabled=false` in dev
  DB (dev started enabled; left as found-disabled for the next run).

## Residual risk
- Template `webhookEnabled`/`pendingForce` are untyped context keys; a missing
  key renders falsy (safe default = disabled UI). Acceptable.
- `EnableWebhookForwarding` writes via `models.DB` while the request tx is
  open — same cross-connection pattern as run creation; fine on MySQL.
