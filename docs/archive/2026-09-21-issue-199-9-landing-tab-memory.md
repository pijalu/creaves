# Issue #199-9 — Landing tab memory on animal-sheet back

## Observed
From `/` (landing, one tab per zone/type), opening an animal sheet then clicking
"Retour aux animaux en cours de soins" landed on the first tab, not the tab the
user came from. The old back link was `/#<animal type name>`, which never
matched any tab anchor (`#t-<sha1>`).

## Expected
Return to the originating landing tab.

## Fix
- `actions/helper.go`: `landingTabAnchor(key)` builds `#t-<sha256(key)>` exactly
  like the landing template; `landingBackTarget(c, animal)` returns the
  sanitized `back` param when present, else the animal's zone tab anchor.
- `actions/animals.go` `Show`: `c.Set("landingBack", landingBackTarget(c, animal))`.
- `templates/animals/show.plush*.html` (×4): back button uses `landingBack`.
- `templates/landing/index.plush*.html` (×4): animal number links append
  `?back=/<v=type?>%23t-<at.ID>` so the originating tab survives the round trip
  (`#` URL-encoded as `%23`; `?v=type` preserved in type view).

## Tests
- `actions/landing_back_test.go`: default back = animal's zone tab; `back`
  param honored (zone and type views); external targets rejected.
- e2e (agent-browser): clicked 2nd landing tab → opened animal → back button
  href `/#t-a60d…` → clicked → 2nd tab active again; direct open without param
  defaults to the animal's zone tab.

## Gates
- go vet, staticcheck: clean.
- gocognit 79 / gocyclo 65: baseline (Show kept at 15 via helper extraction).
- Full `go test -count=1 -race -cover ./...`: green except documented
  pre-existing grifts debt (parity debt pairs, migrations replay).
- Template parity: no NEW drift on landing/show pairs.

## Commit
- `6691f9c` (branch `feature/open-issues-2026-10`)
