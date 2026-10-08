# Plan — config = guest-view details; sync config → admin Synchronization area

Bug: `bugs.md` #1 (2026-09-20 rework of original bug 4, commit 5df5c5e).

## Intent (user wording)
Split "sync config" from "instance details used in guest view":
- **config** (`/config` edit) = guest-view instance details only: Name, Description, Active + CREAVES identity (CenterName, AsblName, BceNumber, Address, AccountNumber, Website, GuestText1/2). **No InstanceID, no sync fields.**
- **sync configuration** (InstanceID, EnableEventStream, WebhookEnabled/URL/APIKey/BatchSize/MaxPerMin) moves to the **Administration > Synchronization** menu (next to Event Stream / Webhook resync).

## Changes

1. `templates/config/_identity_form.plush.{html,fr,de,nl}` — remove `InstanceID` input (×4 locales).
2. `templates/config/_sync_form.plush.{html,fr,de,nl}` — add `InstanceID` input at top ("Instance ID — identifies this center in webhook envelopes") (×4 locales).
3. `templates/config/edit.plush.{html,fr,de,nl}` — drop "Sync configuration" button (sync no longer lives under config).
4. `actions/configs.go`:
   - `Update`: move `config.InstanceID = c.Param("InstanceID")` from the `scope != "sync"` branch into sync-scope-only handling (`scope == "" || scope == "sync"`); identity scope keeps Name/Description/Active only.
   - New handler `SyncConfigEdit(c)` — GET `/sync_config`: loads the active config (`CurrentConfigGet()` with DB fallback: first config row), renders `config/sync_edit.plush.html` for it (admin-only).
   - Keep `SyncEdit` (`/config/{config_id}/sync`) as a redirect to `/sync_config` (back-compat for bookmarks).
5. `actions/app.go` — route `app.GET("/sync_config", ConfigsResource{}.SyncConfigEdit)` (inside Authorize group; handler enforces admin).
6. `templates/application.plush.html` — add menu item under Synchronization sub-menu: `<a class="dropdown-item" href="/sync_config"><i class="fas fa-plug"></i> Sync configuration</a>` (menu labels are hardcoded in this file — existing pattern).
7. `templates/config/sync_edit.plush.{html,fr,de,nl}` — title no longer depends on config_id path; Cancel back to `/config` list... keep `/config/<id>` cancel (config still in context).

## Test approach
- Update `configs_scope_test.go`: identity update no longer changes InstanceID even if param passed; sync update sets InstanceID.
- New test: `GET /sync_config` as admin → 200, renders sync form with InstanceID + webhook fields; as non-admin → 403/redirect.
- New test: `/config/{id}/sync` redirects to `/sync_config`.
- Template parity: all 4 locale variants contain/omit the same fields.

## Validation
- `GO_ENV=test go test ./actions/ -run Config` green; full suite green.
- e2e (agent-browser, admin): /config edit shows guest-view fields only (no Instance ID, no webhook); Administration > Synchronization menu shows "Sync configuration"; /sync_config edits instance ID + webhook, save preserves guest-view fields (DB check); guest identity intact.
- Quality: go vet, staticcheck, gocognit -over 15, gocyclo -over 12, go test -count=1 -race -cover (each separately).
- One commit; archive appended; bugs.md reset.
