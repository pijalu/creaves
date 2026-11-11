# 2026-09-20 — Dashboard todos, corpse marking, stay duration, config split, sync observability

Resolved 6 bug/feature reports on branch `feature/open-issues-2026-10` (creaves repo). One commit per fix, per guideline #7. Fix plan: `docs/plans/dashboard-corpses-duration-fixes.md`.

## Reports & resolutions

### 1. Dashboard — close a TODO must confirm and stay on the dashboard
> Dashboard todo: click "done" ⇒ show confirmation popup; when confirmed: close todo + dashboard must be updated. Stay on dashboard page with todos section visible (anchor to allow refresh).

**Fix** (`97daf0d`): todos forms POST to `/todos/{id}/close`; server redirects to `/#todos` (safe-redirect helper — only local paths); dashboard templates (×4 locales) add `id="todos"` anchor + JS confirm on submit (i18n key `dashboard.todos.confirm-done`). Tests: close → `/#todos`, open-redirect rejected, `#todos` anchor present.

### 2. Corpse register — mark with destination selection; admin unmark
> Mark button must allow selecting a destination for one or more selected animals, then update. Admin can unmark. With no selection, mark applies to the animal matching the row.

**Fix** (`8cdb922`): corpse rows get checkboxes (admin only) + "Mark selected" button opens modal asking destination (`corpse_destination_id`) + removal date (prefilled now); per-row "Mark" auto-selects that animal; no selection → alert, no update. Admin-only per-row Unmark clears `corpse_destination_id`. Route `POST /reports/corpses/unmark` gated by `AdminRequired`. Templates ×4 locales; i18n keys `reports.corpses.*` ×4 locales. Tests: single+multi mark, unmark, no-destination regression (existing), template key parity.

### 3. Outtake — human-friendly stay duration
> Optimize "hours" count: over 48h ⇒ show "days - hours".

**Fix** (`7695215`): Plush helper `stayDuration(hours)` — ≤48h `"N hours"`; >48h `"D days - H hours"` (remainder omitted when 0); 0 → `"< 1 hour"`; nil → empty. All strings i18n'd (`outtakes.duration.*` ×4 locales). Applied to animal show + outtakes show (×4 locales each) replacing raw `stay_duration`. Tests: 10 unit cases + integration (animal show, outtakes show).

**Note**: `stay_duration` is only computed at outtake create/update — all current prod rows are NULL, so no live >48h example exists; rendering verified empty for NULL, format verified by unit tests.

### 4. Config — split instance identity from sync configuration
> Split creaves-id from sync configuration — different views/edits (table can stay single).

**Fix** (`5df5c5e`): `/config/{id}/edit` = identity (instance_name/url, register_allowed); new `/config/{id}/sync` = webhook (URL, API key, retry) + event stream (enabled, batch size, retry, max age). Scoped partials (`_identity_form`, `_sync_form`, `sync_edit`) ×4 locales; `form_scope` hidden field makes Update bind/validate only its scope (other side preserved in DB — API key not clobbered); sidebar Sync link; identity edit shows link to sync page. i18n keys ×4 locales. Tests: identity update preserves webhook key; sync update preserves identity; scoped render.

### 5. Sync — failure logs uninformative
> "Webhook delivery failed: webhook accepted 0/1 events" is not informative; logs must carry actionable information.

**Fix** (`09adcfe`): `deliverBatch` error messages rewritten — include URL, event count, HTTP status, receiver error entries, and response-body excerpt (≤500 chars) for non-2xx. E.g. `webhook POST http://localhost:3001/webhook/events (1 events) accepted 0/1 events (receiver errors: event de01b0df…: instance block mismatch)`. Tests updated + added (network error, 500 status+body, receiver errors, excerpt truncation).

**Live evidence** (post-fix dev log): `Webhook delivery failed: webhook POST http://localhost:3001/webhook/events (1 events) accepted 0/1 events (receiver errors: event de01b0df-…: instance block mismatch)` — pinpointed the two poison events.

### 6. Sync — permanently failing event blocks the whole queue (poison event)
> Two events with stale instance_id (`BigMac.local`, config now `test`) are rejected by the console on every batch; batch order-by-ID keeps them at the head → 10 178 undelivered events accumulated.

**Fix** (`52b5fd6`): poison-queue protection on `event_streams` — additive migration `delivery_attempts INT NOT NULL DEFAULT 0`, `last_delivery_error TEXT NULL`; undelivered query excludes `delivery_attempts >= 25`; every failed batch records attempts + exact error per event; delivered events keep the record. `/event_streams` admin UI shows Attempts column + "Blocked" badge (≥25) + show page rows (×4 locales, i18n ×4). Tests: attempts increment, ≥25 excluded, delivered unaffected, never-failing batch unaffected, error text persisted.

**Live evidence**: poison event `de01b0df` at 14 attempts, error stored, visibly climbing; will drop out of the selection at 25.

## Validation
- Go tests: official suite `GO_ENV=test go test ./actions/` green; pusher suite `go test -tags sqlite -run 'TestRunOnce_|TestDeliverBatch_' ./actions/` green; models green.
- E2E (agent-browser, admin on :3000): bug 1 confirm + `/#todos` landing; bug 2 modal/destination save + no-selection alert + unmark; bug 4 scoped forms + preservation; bugs 5/6 live log/DB/UI evidence. Bug 3 unit-tested only (no >48h prod data).
- Quality: `go vet` clean, `staticcheck` clean, `gocognit -over 15` / `gocyclo -over 12` no new violations (Update 17→13 via `bindSettingsForScope` extraction), `go test -count=1 -race -cover ./...` actions/models green — 2 grifts failures pre-existing at baseline (KNOWN debt list, verified via stash).

## Open items for the user
1. Stale events disposition: `dce9f929-26aa-5559-bbb7-b65efd87e422`, `de01b0df-0323-491b-9b8a-69701d44f88a` (animal 10233, old instance `BigMac.local`) — discard or keep until auto-skip at 25 attempts.
2. Config aspect (NOT fixed per instruction): instance_id changed BigMac.local→test; console still references BigMac.local as a known instance.
3. Commits local on `feature/open-issues-2026-10`; push not requested.
