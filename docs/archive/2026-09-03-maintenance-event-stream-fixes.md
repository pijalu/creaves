# Fixed: Creaves maintenance / event stream issues (2026-09-03)

Fixed in creaves commit `4b7c7bc` (branch feature/i18n). Validated with
agent-browser in en/fr/de/nl.

## Creaves: No full resync to creaves-console in FR — FIXED
Maintenace is not up to date in French - review all recent changes to validate they are correctly implemented in *all* languages - the admin menu is also missing event stream elements

**Fix**: rewrote `templates/maintenance/index.plush.fr.html` from a 4-line
stub to the full page (event stream card, snapshot/cleanup forms,
task-status poller, renumber section). Added missing admin-dropdown links
in `templates/application.plush.fr.html`: Flux d'événements, Traductions,
Resynchronisation webhook. DE/NL templates updated in the same pass.

## Creaves: Events — FIXED
There should a cleanup option to remove all sync events from the database - processed or not

**Fix**: new admin action `MaintenanceDeleteAllEvents` (POST
`/maintenance/delete-all-events`) + red "Delete All Events" button on the
maintenance page in all 4 locales. Verified end-to-end: 20093 events
removed, redirect + flash.

## Creaves: Unprocessed events - not clear why — FIXED
```
Event Stream Status
Enabled: true
Total Events: 20093
Unprocessed Events: 20093
Processed Events: 0
```
Setup seems correct

**Fix**: the page counted `processed_at`, which the producer never sets
(console-side field). Stats and cleanup now use `delivered_at` semantics
(undelivered = `delivered_at IS NULL`, delivered = `IS NOT NULL`). The
webhook worker now also starts unconditionally at boot (even when
forwarding is disabled) because it carries the hourly event purge;
delivery stays gated per-tick by `IsWebhookEnabled()`.

## Event retention (user request) — DONE
Automatic purge in the webhook worker: delivered events older than 1 day
and undelivered events older than 7 days are deleted hourly, so
`event_streams` cannot grow unbounded. Constants in
`actions/webhook_pusher.go`; covered by `TestPurgeOldEvents` /
`TestPurgeOldEvents_EmptyTable`.
