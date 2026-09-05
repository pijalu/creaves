# Fix plan — creaves: config delete option shown for the active configuration

Date: 2026-09-05
Bug: `bugs.md` → `## creaves: config`

## Bug report

Deleting the currently active configuration fails with:

```
cannot delete the currently active configuration
```

Expected: if delete is not possible, the option should not be presented.

Note from reporter: validate there can be multiple configurations loaded at the
same time — to push to different console environments.

## Root cause

- `ConfigsResource.Destroy` (`actions/configs.go`) correctly refuses to delete
  the config whose ID equals the cached `CurrentConfig.ID`, returning HTTP 400
  "cannot delete the currently active configuration".
- But `templates/config/index.plush.{html,de,fr,nl}.html` renders the red
  delete button for **every** row unconditionally (line 29 in each file).
  The show page has no delete button; only the index rows do.
- So the UI offers an action that the server always rejects — confusing UX.

### Multi-config behavior (note validation)

- `Config` has an `Active` flag; nothing enforces single-active exclusivity.
  `LoadConfig` loads the *oldest* active row into the package global
  `CurrentConfig`; event production (`PublishEvent`) and the webhook delivery
  worker (`deliverBatch`) push only through `CurrentConfig`.
- Therefore: multiple configuration **records** can coexist in the DB (several
  may even be flagged active — the oldest active wins). Creating additional
  configs never fails and never deactivates the current one.
- Pushing to *different console environments simultaneously* (fan-out to N
  webhook targets) is **not** the current design: one active config → one
  webhook target. Switching targets is done by editing the active config's
  webhook URL. The bug note asks only to *validate* that multiple
  configurations can be loaded at the same time; co-existence in the DB and
  listing works — this is covered by tests below. No fan-out feature is added
  (out of scope for a bug fix).

## Change list (smallest fix)

1. `actions/configs.go` `List`: set `currentConfigID` (string) in the context
   for the HTML branch so the template can compare row IDs.
2. `templates/config/index.plush.{html,de,fr,nl}.html`: wrap the delete
   `linkTo` in `<%= if (currentConfigID != config.ID.String()) { %> … <% } %>`
   so the button is not rendered for the currently active configuration.
   (Plush compares strings; `config.ID` is a `uuid.UUID`, so compare against
   `config.ID.String()`.)

No model/migration change needed (no schema impact — safe for production data).

## Test approach

- `actions/configs_bug_test.go` (package actions, MySQL test DB via
  `GO_ENV=test`; full `App()` served by `httptest.NewServer`, admin session
  through the real `/auth` login flow with cookie jar + CSRF token):
  - `TestConfigsListHidesDeleteForActiveConfig`: seed two configs (one =
    `CurrentConfig`), GET `/config/`, assert the active row contains no
    `data-method="DELETE"` link and the inactive row keeps it.
  - `TestConfigsDestroyRejectsActiveConfig`: DELETE the active config → 400;
    row still present.
  - `TestConfigsDestroyAllowsInactiveConfig`: DELETE the inactive config →
    303; row gone.
  - `TestLoadConfigMultipleConfigsCoexist`: three configs created while one is
    active; all exist in DB, `LoadConfig` still returns the oldest active one.
- Run: `GO_ENV=test go test -count=1 -race -cover ./...`

## Validation steps (agent-browser)

1. Start dev server on :3000 (`buffalo dev`), log in as admin.
2. Open `/config`: the active configuration row must show **no** trash/delete
   button; any inactive row keeps its delete button.
3. Create a second configuration via `/config/new` → succeeds; both rows list.
4. Delete the inactive configuration via the UI → success flash, row removed.
5. Confirm the active row still has no delete button.

## Quality gates (each run separately)

- `go vet ./...`
- `staticcheck ./...`
- `gocognit -over 15 .`
- `gocyclo -over 12 .`
- `CGO_ENABLED=1 go test -tags sqlite -count=1 -race -cover ./...`

Pre-existing complexity warnings in unrelated files will be noted, not fixed.

## Issues found during testing (fixed)

1. **Minimal-app render 500s** — a stripped-down buffalo test app (only the
   config resource) could not render `config/index.plush.html`: the layout
   needs `authenticity_token` (CSRF middleware) and route helpers from the
   full route table (`rootPath`, nav links). Fixed by driving the **full**
   `App()` via `httptest.NewServer` with a real admin login (cookie jar +
   CSRF token parsed from `/auth/new`).
2. **`TestLocalizeAnnualSections` panicked once `App()` was initialized** —
   `localizeAnnualSections`'s `lit` helper guarded only on the package-global
   `T != nil`, but `T.Translate(c, key)` (mw-i18n v2.0.2) type-asserts
   `c.Value("T").(i18n.TranslateFunc)`, which panics in tests whose context
   never passed through the i18n middleware. Fixed in
   `actions/reports_annual.go`: use the context-stored translate func when
   present (`if tf, ok := c.Value("T").(i18n.TranslateFunc); ok`), else fall
   back to the raw key. Production behavior unchanged (middleware always sets
   "T"); this makes the function safe for direct unit tests. This is an issue
   found during testing → fixed here as required by the guideline.
3. **CSRF token location** — the config index has no form, so
   `authenticity_token` does not appear in the page body; tests read the
   layout's `<meta name="csrf-token">` tag instead.

## Validation results

2026-09-05, agent-browser against dev server on http://127.0.0.1:3000
(login admin/admin):

1. `/config` with a single **active** config (LaGrange / La Grange Sauvage,
   Active ✓): row shows only **View / Edit — no delete button**. ✔
2. Created second config via `/config/new` (Instance ID
   `browser-test-second`, Name "Browser Test Second", Active unchecked) →
   302 to its show page. `/config` now lists **two rows** (multi-config
   coexistence validated): the active LaGrange row still has no delete
   button; the new inactive row shows **View / Edit / Destroy**. ✔
3. Clicked Destroy on the inactive row → flash "Config deleted successfully",
   row gone; active LaGrange row remains without a delete button. ✔

Automated tests (`GO_ENV=test go test -count=1 -race -cover ./...`):
- `TestConfigsListHidesDeleteForActiveConfig` ✔
- `TestConfigsDestroyRejectsActiveConfig` ✔ (400 + row kept)
- `TestConfigsDestroyAllowsInactiveConfig` ✔ (303 + row removed)
- `TestLoadConfigMultipleConfigsCoexist` ✔ (3 configs coexist; LoadConfig
  picks the oldest active)
- Full suite green: actions 22.3%, excel 29.5%, export 13.0%, grifts 5.0%,
  models 75.9%, feeding 73.8%, utils 90.5%.

Quality gates (each run separately):
- `go vet ./...` — clean.
- `staticcheck ./...` — findings identical to pre-change baseline (verified by
  diffing against a stashed baseline; all pre-existing: U1000 unused helpers,
  ST1005 capitalized error strings, ST1001 dot imports in grifts, ST1019
  double log import in models).
- `gocognit -over 15 .` / `gocyclo -over 12 .` — no config-related entries;
  all flagged functions pre-existing and unrelated (buildEventPayloadInto,
  parseValueTuples, AnimalsResource.Update, EnrichAnimalsOptimized,
  ConsolidatedAnimal.UpdateFromPayload, etc.).
- `GO_ENV=test go test -count=1 -race -cover ./...` — all packages ok.
