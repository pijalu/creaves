# Issue #199-12 — Admin exception: any outtake type with any species (NS)

Date: 2026-09-21
Commit: `f8e4bd8` on `feature/open-issues-2026-10`

## Request (from pijalu/creaves#199)

Item 12: outtake-type list is filtered by the species' native status (NS) for
everyone. Admin users must be able to use all outtake types with any species,
whatever the NS.

## Implementation

- `actions/outtakes.go`
  - New `outtakeNSFilterBypassed(c)` — true when `GetCurrentUser(c).Admin`.
  - `setFilteredOuttakeFormData` — admins get the **unfiltered** outtake-type
    list on the new-outtake form (regular users keep the NS-filtered list).
  - `rejectOuttakeCreate` — the `ExcludesNativeStatus` → 422 guard is skipped
    for admins. The `location_mode` rule (`enforceOuttakeLocationRule`) still
    applies to everyone.

## Tests

- `actions/outtakes_rules_test.go`
  - `TestOuttakeCreateRejectsTypeForbiddenByNativeStatus` — switched to a
    **non-admin** login (`feedingGuideUser(t, false)`); still asserts 422 + no
    persisted outtake for regular users.
  - `TestOuttakeCreateAdminBypassesNativeStatus` (new) — admin posting an
    outtake whose type excludes the species' native status is **not** 422 and
    the outtake is persisted + linked to the animal.
- Full suite: `GO_ENV=test go test -count=1 -race -cover ./...` passes with
  only the two pre-existing grifts debts. No template changes.

## e2e verification (agent-browser, dev DB)

- New-outtake form for animal 9901 (species "Bernache du Canada", NS2) as
  **admin**: the TypeID radio list includes "Relacher" (excluded NS2,NS3,NS4)
  and "Transferer" (excluded NS3) — both forbidden for NS2, both offered to
  the admin. Full 8-type list present.
