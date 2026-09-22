# Bug 6 — Care form note templates: select/insert only; management moved to admin CRUD

Source: `bugs.md` bug 6.

> In http://localhost:3000/cares/new/ — Note Template should **only** allow a
> select/insert. All note template management should be moved on a dedicated
> admin CRUD to list/edit/delete at admin level.

## Current state (pre-fix)

- `templates/cares/_form.plush.{,fr.,de.,nl.}html` (issue #158 feature): the
  "Note template" row (visible on "suivi" cares only) has a select + **Insert**
  button, but also **Save as template** (`#careTemplateSave`, POST
  `/care_templates`) and **Delete** (`#careTemplateDelete`, POST
  `/care_templates/{id}/delete`). Any care-giving user can create/delete
  templates inline.
- `actions/care_support.go`: `CareTemplatesIndex` (JSON, own-user only),
  `CareTemplatesCreate` (JSON POST, any user), `CareTemplatesDestroy` (owner or
  admin). No HTML pages, no management UI.
- Routes (`actions/app.go` ~202-204): `GET /care_templates`,
  `POST /care_templates`, `POST /care_templates/{id}/delete`.

## Root cause

The note-template feature (#158) was implemented as a per-user inline convenience
instead of a managed shared resource. Management (create/list/edit/delete) must
be admin-only; the care form must only consume templates.

## Design

- **Shared pool**: templates become a shared, admin-managed resource. Every
  logged-in user may *select and insert* any template in the care form
  (`GET /suggestions/care_templates`, JSON, authenticated users). `user_id`
  column is kept and records the admin who created the template (additive, no
  schema change; production data preserved).
- **Admin CRUD** at `/care_templates` (guarded by `careTemplatesAdminGuard`:
  unauthenticated → 401; non-admin → flash `care_templates.admin.restricted` +
  redirect home):
  - `GET  /care_templates` — list ALL templates with owner login
  - `GET  /care_templates/new` + `POST /care_templates` — create
    (validation errors → 422 re-render)
  - `GET  /care_templates/{id}/edit` + `PUT /care_templates/{id}` — update
    (buffalo `_method` override; same mechanism as entry_causes)
  - `DELETE /care_templates/{id}` — delete (jquery-ujs `data-method` link,
    same convention as entry_causes resource rows)
- **Care form** (`templates/cares/_form*.plush.html`, all 4 locale variants):
  remove `#careTemplateSave` / `#careTemplateDelete` buttons and their JS
  handlers; `refreshTemplates()` now fetches `/suggestions/care_templates`.
  Select + Insert remain.
- **Menu**: "Note templates" entry in the Administration ▸ Configuration
  submenu (`templates/application.plush.{,fr.,de.,nl.}html`), following the
  `/feeding_guides` hardcoded-href pattern.
- **i18n**: new `locales/care_templates.{en-us,fr,de,nl}.yaml` with
  `care_templates.admin.restricted`, `care_templates.created.success`,
  `care_templates.updated.success`, `care_templates.destroyed.success`.
  Admin pages themselves are hardcoded-English single files (no locale
  variants → no template-parity debt), following the entry_causes convention.
- **Tests** (`actions/care_support_test.go`): rewrite
  `TestCareTemplatesLifecycle158` → `TestCareTemplatesAdminCRUD158` (admin
  HTML flow: list, create 303 + row owned by admin, 422 on empty name,
  edit/update, delete) and `TestCareTemplatesOwnership158` →
  `TestCareTemplatesAdminOnly158` (non-admin: HTML/POST/DELETE redirected,
  template untouched; `GET /suggestions/care_templates` 200 JSON returns the
  shared pool of all users' templates).
  Note: `GO_ENV=test` disables CSRF middleware (mw-csrf), so form POSTs in
  tests need no authenticity_token.

## Validation steps

1. Unit tests: `GO_ENV=test go test ./actions/ -run 'CareTemplates' -count=1`.
2. Full gates (each run separately): `go vet ./...`, `staticcheck ./...`,
   `gocognit -over 15 .`, `gocyclo -over 12 .`,
   `GO_ENV=test go test -count=1 -race -cover ./...`.
3. E2E with agent-browser against `buffalo dev` on :3000 (admin/admin):
   - `/cares/new` (suivi type): note-template row shows select + Insert only —
     no Save/Delete buttons; selecting a template inserts its content.
   - `/care_templates`: list shows all templates + owner, Create new, Edit,
     Delete all work (captured output).
   - Non-admin check (unit tests cover the 303 redirect; e2e optional).

## Issues found during the work

- **`buffalo.Context#Redirect` returns `nil` after writing the response**
  (verified in buffalo v0.18.9 `default_context.go`). An initial guard helper
  that did `return c.Redirect(303, "/")` therefore did NOT stop the wrapped
  handler: the list kept rendering and non-admin POSTs kept creating rows
  (redirect status written, then "headers already written" override warnings).
  Fix: `careTemplatesAdminDenied(c) bool` only decides; each handler returns
  `c.Redirect(http.StatusSeeOther, "/")` itself as its terminal return value,
  the standard repo pattern.
- **Stale test rows**: failed runs left `Hax` template rows in `creaves_test`
  (no cleanup had been registered before the failing assert), breaking the
  exact-count assert on re-runs. The test now deletes stale `Hax` rows at the
  start.
