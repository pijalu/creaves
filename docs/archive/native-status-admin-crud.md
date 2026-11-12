# Bug 1 — Native statuses: maintainer-only admin CRUD + delete-with-replacement

## Problem (bugs.md)

- Native statuses table needs an ADMIN CRUD available **only to maintainers**.
- The CRUD existed (`NativeStatusesResource`) but had no auth guard, no nav entry,
  and an unguarded `Destroy` that would leave species rows with dangling
  `native_status` references.

## Changes

| Area | File(s) | Change |
|------|---------|--------|
| Auth guard | `actions/native_statuses.go` | `requireMaintainer` on List/Show/New/Create/Edit/Update/Destroy + new delete flow handlers |
| Cleanup | `actions/native_statuses.go` | Removed duplicated `setTranslationValues` call in `New` |
| Delete flow | `actions/native_statuses.go` | `Destroy` now refuses (422) when species use the record; new `NativeStatusDeleteNew` (GET `.../delete`) + `NativeStatusDeleteCreate` (POST `.../delete`) implement delete-with-replacement: species referencing the record are remapped (`UPDATE species SET native_status = ?`) before the row is deleted; replacement is mandatory when usage > 0, must exist and differ from source |
| Helper | `actions/typehelper.go` | `nativeStatusesToSelectables()` — translated labels via `translateIDs`/`ResolveName` |
| Routes | `actions/app.go` | `GET/POST /native_statuses/{native_status_id}/delete` |
| Templates | `templates/native_statuses/delete.plush.{html,fr,de,nl}.html` | New confirm/remap form (usage warning or unused notice, replacement select, cancel) in all 4 locales |
| Templates | `templates/native_statuses/{index,show}.plush.*.html` | Trash buttons now link to the `/delete` flow instead of raw `data-method=DELETE` |
| Nav | `templates/application.plush.{html,fr,de,nl}.html` | "Native statuses" link in Administration → Configuration, inside `current_user.Maintainer` block, all 4 locales |

## Tests

`actions/native_statuses_test.go`:
- `TestNativeStatusesMaintainerOnly` — plain admin → 403 on list/new/show/edit/destroy/delete-new; maintainer → 200 on list
- `TestNativeStatusDestroyBlockedWhenUsed` — DELETE on in-use record → 422, record kept
- `TestNativeStatusDeleteCreateRequiresReplacement` — POST without/invalid/same replacement → 422/400/400
- `TestNativeStatusDeleteWithReplacement` — species remapped to replacement, source deleted
- `TestNativeStatusDeleteUnusedWithoutReplacement` — unused record deletable without replacement

## Quality gates (run separately, from `creaves/`)

- `go vet ./...` — clean
- `staticcheck ./...` — clean
- `gocognit -over 15 .` — no new entries (all flagged functions pre-existing)
- `gocyclo -over 12 .` — no new entries
- `go test -count=1 -race -cover ./...` — all packages ok (`creaves/actions` ok, `creaves/models` ok)

## E2E validation (agent-browser, http://127.0.0.1:3000)

1. Login admin/admin → OK (`/`)
2. Administration → Configuration submenu: link "Native statuses" present next to "Species" — OK
3. `GET /native_statuses` — 5 rows NS1..NS5 with translated status/indication — OK
4. Create NSE2E (Status "E2E Status", Indication "e2e indication", Freeable) → redirect `/native_statuses/NSE2E`, flash "NativeStatus was successfully created." — OK
5. Edit NSE2E indication → "e2e indication v2", flash "NativeStatus was successfully updated.", value persisted — OK
6. `GET /native_statuses/NS1/delete` — warning "This native status is used by **446 species**. Choose a replacement..." with replacement select — OK
7. `GET /native_statuses/NSE2E/delete` — notice "not used by any species", submit with no replacement → redirect `/native_statuses`, NSE2E gone (0 mentions) — OK
