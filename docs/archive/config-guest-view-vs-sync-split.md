# Config — guest-view instance details vs sync config split (resolved)

**Date**: 2026-09-20
**Bug**: bugs.md item #1 — the 2026-09-20 split (commit 5df5c5e) kept sync
configuration under `/config/{id}/sync` and mixed the instance ID (sync
identity) into the identity edit form.

## Expected behavior (from report)

- `/config` edit = guest-view instance details only: Name, Description,
  Active + CREAVES identity (CenterName, AsblName, BceNumber, Address,
  AccountNumber, Website, GuestText1/2). No sync fields, no instance ID.
- Sync configuration (InstanceID, EnableEventStream, Webhook* settings)
  editable from a page in the Administration > Synchronization menu.
- Config show page keeps a link to the sync page; sync page links back to
  config.
- Applied consistently in all supported languages (en-US, fr, de, nl).

## Fix

- **Route**: new `GET /sync_configuration` (`ConfigsResource.SyncEdit`),
  linked from the Administration > Synchronization menu ("Sync
  configuration", all 4 locales). The legacy `GET /config/{config_id}/sync`
  URL now redirects (301) to `/sync_configuration` for the active config
  and answers 404 for any other config — synchronization settings only ever
  live on the single active config.
- **Handler** (`actions/configs.go`):
  - `SyncEdit` loads the active config (`LoadConfig`) instead of an
    arbitrary config row.
  - `Update` binds the instance ID only for the `sync` scope (moved to
    `bindColumnsForScope`); the `identity` scope preserves it, and the
    legacy no-scope full update still binds everything. Settings scoping
    (`bindSettingsForScope`) is unchanged, so each form preserves the other
    side's stored values.
- **Templates** (all 4 locales):
  - `config/_identity_form.*`: InstanceID field removed — the `/config`
    edit form now shows only guest-view instance details.
  - `config/_sync_form.*`: new "Instance Identity" section with the
    InstanceID input above the event-stream and webhook sections.
  - `config/edit.*`: sync button now points at `/sync_configuration`.
  - `config/show.*`: "Sync configuration" button (`config.show.sync-config`
    locale key added in en-US/fr/de/nl).
  - `config/sync_edit.*`: subtitle clarifying the page edits the active
    configuration + back-link to the instance-details edit page.
  - `templates/application.*`: "Sync configuration" entry added to the
    Synchronization sub-menu.

## Tests

- `actions/configs_scope_test.go` updated:
  - Identity scope: a forged `InstanceID` value is ignored, stored value
    preserved.
  - Sync scope: instance ID updates via the sync form; identity columns
    untouched; blank API key preserves the stored key.
  - New `TestConfigSyncLegacyURLBehavior`: 301 redirect for the active
    config, 404 for any other config.
- `GO_ENV=test go test ./actions ./models -count=1` — green.
- `GO_ENV=test go test -count=1 -race -cover ./actions ./models` — green.

## E2E evidence (agent-browser, dev server on :3000, admin/admin)

1. `/config/{id}/edit` (200): form contains Name, Description, Active and
   `Settings.{CenterName,AsblName,BceNumber,Address,AccountNumber,Website,
   GuestText1,GuestText2}`; zero `InstanceID` matches; no webhook fields;
   link to `/sync_configuration` present.
2. Navbar Administration > Synchronization contains a dropdown-item link to
   `/sync_configuration` ("Sync configuration").
3. `/sync_configuration` (200): `InstanceID` input, `Settings.EnableEventStream`,
   `Settings.Webhook{Enabled,URL,APIKey,BatchSize,MaxPerMin}`, back-link to
   `/config/{id}/edit`.
4. Instance ID changed `test` → `test-e2e`, saved (303 redirect), value
   persisted on reload; restored to `test` afterwards.
5. `/config/{id}` show page has a "Sync configuration" button linking to
   `/sync_configuration`.

## Quality

- `go vet ./...` clean
- `staticcheck ./...` clean
- `gocognit -over 15 .` / `gocyclo -over 12 .`: no findings in changed
  files (remaining hits are pre-existing hotspots elsewhere)
