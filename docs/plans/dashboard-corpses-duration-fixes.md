# Fix Plan — Dashboard TODO confirm, Corpse register mark flow, Outtake duration format

Bugs tracked in [../bugs.md](../bugs.md) items 1–3. All work in the `creaves/` project
(2 repos note: workspace root is not a git repo; commits land in `creaves/`).

## Bug 1 — Dashboard TODO: "done" needs confirmation + stay on dashboard

**Observed**: dashboard TODO "Done" form POSTs to `/todos/{id}/done` which redirects
to `/todos` — the user leaves the dashboard and loses the TODOs section context.
No confirmation either.

**Fix**:
1. `templates/dashboard/dashboard.plush.{html,fr,de,nl}.html`:
   - Add `id="todos"` anchor on the TODOs `<h3>` section header.
   - Add `onsubmit="return confirm(...)"` (localized message) on the done form.
   - Add hidden input `redirect=/#todos` inside the done form.
2. `actions/todos.go` `TodosDone`: read optional `redirect` param; accept only
   values starting with `/` but not `//` (open-redirect guard); default `/todos`.
3. `/todos` index page behavior unchanged (no redirect param → `/todos`).

**Tests**:
- Go: extend `actions/todos_test.go` — POST done with `redirect=/#todos` → 303 to
  `/#todos`; POST with `redirect=https://evil` → falls back to `/todos`; POST
  without param → `/todos`.
- e2e: dashboard → click Done → confirm dialog appears → OK → still on dashboard
  at TODOs section, todo marked done; Cancel → todo untouched.

## Bug 2 — Corpse register: mark must prompt for destination; admin unmark

**Observed**: per-row "Mark" submits the form immediately (with whatever is in the
destination field, often empty) — no chance to pick the destination at click time.

**Fix** (templates `reports/corpses.plush.{html,fr,de,nl}.html` + actions):
1. Replace the always-visible destination card + instant quick-mark with a
   Bootstrap 4 modal (`#markModal`) containing: destination input (existing
   `/suggestions/corpse_destination` autocomplete), datetime-local field
   (defaults to now), confirm/cancel buttons.
2. Row "Mark" button → opens modal scoped to that row only (unchecks others).
3. Bulk "Mark destination" button → opens modal only if ≥1 checkbox selected,
   else alert (localized). Interpretation of "if there is no selection, the mark
   button will use the animal matching the selection": the row-level Mark button
   is the no-selection path — it applies to exactly its own row's animal.
4. Modal confirm → copies values into the existing `#markForm` fields → submit
   (POST `/reports/corpses/mark` unchanged contract).
5. **Unmark (admin only)**: new `POST /reports/corpses/unmark` →
   `ReportsCorpsesUnmark` clears `corpse_destination/_at/_by_id` on selected
   dead-type outtakes; 403 for non-admin. Template shows per-row "Unmark"
   button only when `isAdmin` and the row has a destination; plus a bulk unmark
   in the modal? — No: keep minimal, per-row unmark + admin check.
   `ReportsCorpsesIndex` must `c.Set("isAdmin", ...)`.

**Tests**:
- Go: extend `actions/reports_corpses_test.go` — admin unmark clears fields and
  report no longer shows destination; non-admin unmark → 403 (need a non-admin
  client helper — check existing test helpers, else validate handler guard via
  direct call pattern used in todos tests).
- e2e: select 1 animal → Mark → modal appears → choose destination (autocomplete)
  → confirm → row shows destination/date/user. Row Mark with no selection →
  modal for that row only. Admin: Unmark → destination cleared. Bulk with no
  selection → alert, no request.

## Bug 3 — Outtake stay duration: human-friendly over 48h

**Observed**: `Stay duration` renders raw hours (`123 h`) in
`outtakes/show.plush.*` and `animals/show.plush.*` (4 locales each).

**Fix**:
1. New plush helper `stayDuration(hours nulls.Int, dayUnit string, hourUnit string)`
   in `actions/render.go`: ≤48h → `"123 h"` (unchanged); >48h → `"5 d - 3 h"`.
   Units passed per locale template: EN `d/h`, FR `j/h`, DE `T/h`, NL `d/u`.
   Invalid (NULL) → empty string.
2. Replace the 8 template render sites to use the helper with locale units.
3. No DB/webhook change (payload `stay_duration` stays raw hours).

**Tests**:
- Go: unit-test the helper function directly (0, 1, 47, 48, 49, 51, 123 h;
  invalid input).
- e2e: open an animal whose outtake stay >48h → shows `X d - Y h`; an animal
  ≤48h → still `N h`.

## Quality gates (per bugs.md guideline, run separately)

- `go vet ./...`
- `staticcheck ./...`
- `gocognit -over 15 .`
- `gocyclo -over 12 .`
- `go test -count=1 -race -cover ./...`
- e2e via agent-browser skill (login admin/admin on `buffalo dev`).

## Commits (in creaves repo, one per fix)

1. `fix(dashboard): confirm + stay on dashboard when marking TODO done`
2. `fix(reports): corpse register mark prompts for destination; admin unmark`
3. `feat(outtakes): human-friendly stay duration (days-hours over 48h)`

Then archive this plan + bug entries to `docs/archive/2026-09-dashboard-corpses-duration-fixes.md`
and reset `bugs.md` to guidelines-only.

## Bug 4 — Config: split creaves-id from sync configuration (added mid-session)

**Observed**: instance identity and webhook sync config share one view/form.

**Fix**: split `templates/config/_form.plush.*` / configs UI into two sections with
separate edit views (same `configs` table, same handlers — UI split only).
Inspect `actions/configs.go` + `templates/config/` first; keep `LoadConfig`
contract untouched. 4 locales.

**Tests**: Go handler tests still pass; e2e: both views render, each saves only
its own fields.

## Bug 5 — Sync errors must be actionable (added mid-session)

**Observed**: `Webhook delivery failed: webhook accepted 0/1 events` — no HTTP
status, no response body, no transport error.

**Fix**: in `actions/webhook_pusher.go` delivery path, capture and log:
- transport error (err from `client.Do`) verbatim,
- HTTP status code on non-200,
- response body excerpt (console error payload, capped ~500 chars),
- webhook URL + event count.
Keep circuit-breaker semantics unchanged.

**Tests**: Go test with `httptest.Server` returning 500 + JSON error body →
assert log/error contains status + body; unreachable server → transport error
surfaced. e2e optional (log inspection).

## Bug 6 — Webhook queue poisoned by stale-instance events (added mid-session)

**Observed**: 2 events with historical `instance_id` (`BigMac.local`, ≠ current
config `test`) are rejected by the console (`instance block mismatch`) on every
tick; ordered-first, they block all 10k+ events behind them forever.
Pusher error handling ignores per-event `errors[]` content entirely.

**Fix** (creaves only, additive schema change — production-safe):
1. Migration (additive): `event_streams.delivery_attempts INT NOT NULL DEFAULT 0`,
   `event_streams.last_delivery_error TEXT NULL`.
2. Pusher `deliverBatch`: on `responseErr`, record per-event attempts++ and the
   console's `errors[]` strings (joined, capped) into `last_delivery_error` for
   the batch's unaccepted events; also capture per-event error when the console
   message carries the event id.
3. Selection: `WHERE delivered_at IS NULL AND delivery_attempts < maxAttempts`
   (const 25). Events at/over the cap stay undelivered but are skipped — queue
   drains around them.
4. Surface: event_streams admin list shows attempts + last error for failed
   rows (and count on `/webhook_resync` if trivial).
5. Operator resolution of the 2 stale `BigMac.local` events = user decision
   (config/history, not code).

**Tests**: httptest server returning 200 with `errors` + `processed_ids: []` →
attempts incremented, error stored; event over cap excluded from next batch
query; second event with different instance id gets delivered behind a poison
one (no head-of-line blocking). e2e: reproduce the BigMac.local scenario
against local console and watch the queue drain past the poison events.
