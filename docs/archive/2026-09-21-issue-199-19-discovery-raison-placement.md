# Issue #199-19 — Discovery read mode: "Raison" below "Cause d'entrée"

**Date:** 2026-09-21 · **Commit:** `074bf91` · **Branch:** feature/open-issues-2026-10

## Observed
On `/animals/<id>#nav-discovery` read mode, the condition/reason entry was
displayed at the bottom of the discovery list (after Date) and, in French,
labelled "Condition". In edit mode the same field sits right under
"Cause d'entrée" with the label «Raison».

## Expected
Read mode mirrors edit mode: the reason appears directly below the entry
cause, with the FR label «Raison».

## Fix
- `templates/animals/show.plush{,.fr,.de,.nl}.html`: moved the conditional
  `Discovery.Reason` `<li>` block to directly after the entry-cause `<li>`
  (before postal code). FR label changed "Condition" → «Raison»;
  EN "Reason", DE "Grund", NL "Reden" unchanged.

## Tests
- New `TestAnimalShowDiscoveryReasonPlacement`: loads the fixture animal's
  discovery row explicitly (`tx.Find` does not eager-load associations),
  sets a reason, and asserts per locale that the nav-discovery tab renders
  entry cause < reason < postal code, the FR label «Raison» (and never
  "Condition"), and the reason value; DE/NL order also checked. The reason
  is cleared again in cleanup.
- Parity check: no NEW drift on animals/show (pair stays in frozen
  KNOWN-debt set).

## E2E (agent-browser, dev DB)
- Dev animal 10213 already had a real reason («Pris dans des câbles de
  chemin de fer») — no temp data needed.
- FR read mode labels: `["Cause d'entrée","Raison","Code Postal","Ville",…]`;
  value rendered; no "Condition".
- EN read mode labels: `["Entry Cause","Reason","Postal Code","City",…]`.
- No dev data modified; language reset to en-US.

## Gates
go vet ✓ · staticcheck ✓ · gocognit 80 (baseline) · gocyclo 66 (baseline) ·
`GO_ENV=test go test -count=1 -race -cover ./...`: all packages ok except the
2 documented pre-existing grifts failures; actions 50.8% coverage.
(One transient fixture-collision flake `TestAnimalsIndexEntryCauseAndExitStatus`
from leftover residue of earlier interrupted runs — passes on re-run;
470 orphaned "Testsp" fixture rows and their child rows were cleaned from
creaves_test to stop recurrence.)

## Residual risk
None known — display-only reorder; conditionality (block hidden when reason
empty) unchanged.
