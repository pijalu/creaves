# Outtakes/new — cage selection hidden behind an extra click (RESOLVED)

**Observed behavior**: On `/outtakes/new`, only the "by animal" picker was shown; the cage
outtake flow required an extra click on the "or select a whole cage" link to reach
`/outtakes/cage`. The cares new page (`/cares/new`) already shows both pickers inline.

**Expected behavior**: `/outtakes/new` shows both pickers inline, like `/cares/new`, in all
supported languages (en-US, fr, de, nl).

## Fix

In `creaves/templates/outtakes/new.plush.html` (+ `.fr`, `.de`, `.nl` variants):
replaced the "or select a whole cage" link block with an inline "Select cage for outtake"
section copied from the working `cage_new.plush*` picker — GET form posting to
`/outtakes/cage` with the `cage` param and the `CageWithAnimalInCare` autocomplete,
mirroring the layout of `cares/new.plush.html`.

Localized headings:
- en: "Select cage for outtake"
- fr: "Sélectionnez la cage pour l'issue"
- de: "Käfig für Ausgang auswählen"
- nl: "Kooi selecteren voor afloop"

No Go/route changes: existing `GET /outtakes/cage?cage=...` and
`GET /outtakes/new?animal_year_number=...` handlers untouched.

## Validation

### E2E (agent-browser, dev server :3000, admin/admin)

- `GET /outtakes/new` shows both headings + inputs (`animal_year_number`, `cage`);
  old link absent.
- Cage flow: `cage=B1` → Next → `/outtakes/cage?cage=B1`, heading
  "Outtake for cage B1 (1 animal(s))".
- Animal flow: `animal_year_number=927/22` → Next →
  `/outtakes/new/?animal_year_number=927%2F22`, heading "Outcome of animal 927/22".
- Console: no JS errors. de/fr localized headings verified.

### Tests & quality

- `go test ./actions -run 'Outtake|OuttakeCage|QuickOuttake'` — pass.
- Full suite `go test -count=1 -race ./...` — 26 failures, identical before/after the
  change (stash baseline comparison) — pre-existing, unrelated (test-DB/env issues in
  attachments/config/corpse tests).
- `go vet ./...`, `staticcheck ./...` — clean.
- `gocognit -over 15` / `gocyclo -over 12` — hits are pre-existing Go hotspots
  (event_producer, startup_seed, webhook_pusher), untouched by this template-only change.

Resolved on branch `feature/open-issues-2026-10`.
