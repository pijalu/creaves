# Fix archive — 2026-09-12 — Animal types / outcome types are maintainer-only

## Original report (bugs.md)
> ## Animal types / outcome types reachable by plain admins
> **Observed**: `AnimaltypesResource` and `OuttaketypesResource` guard every handler with `GetCurrentUser(c).Admin` only.
> **Expected**: Both resources accessible to **maintainers only** (`Admin && Maintainer`, via `requireMaintainer`), consistent with species; menu entries hidden for non-maintainer admins.

## Fix

- `actions/animaltypes.go` + `actions/outtaketypes.go`: all 7 handlers each (List, Show,
  New, Create, Edit, Update, Destroy) now use `requireMaintainer(c)` (same guard the
  species resource already used) instead of the admin-only check.
- `templates/application.plush{,.de,.fr,.nl}.html`: the "Animal types" and "Outcome
  types" Administration-menu links are wrapped in `<%= if (current_user.Maintainer) { %> … <% } %>`,
  matching the existing Species link gating.

## Tests
- `actions/reference_guard_test.go::TestAnimaltypeAndOuttaketypeMaintainerOnly`:
  plain admin (Admin without Maintainer) → 403 on `GET /animaltypes` and
  `GET /outtaketypes`; maintainer → 200. PASS.
- `go vet ./...` clean; `staticcheck ./...` — no findings in touched files.
- E2E (agent-browser, authenticated admin/maintainer): Administration → Configuration
  dropdown lists "Animal types" and "Outcome types"; `/animaltypes/` renders 17 rows.
  (Plain-admin menu hiding is enforced by the template conditional; server-side 403
  is covered by the Go regression test.)

## Verification
```
go test ./actions -run 'TestAnimaltypeAndOuttaketypeMaintainerOnly' -count=1   # ok
# browser: dropdown shows both links for maintainer; /animaltypes/ 200
```
