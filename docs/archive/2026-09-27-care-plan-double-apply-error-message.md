# BUG: double apply returned misleading "hors délai" message

**Date found**: 2026-09-27 (functional E2E, feature/care-expert @ b9b6a20)
**Severity**: minor (error-message quality; behavior itself was correct/idempotent)

## Symptom

Re-submitting `POST /care_plan/apply` for an occurrence that already had an
application returned `409` with:

```
occurrence is out of its apply window (hors délai, §10-A1)
```

The correct semantics fired (idempotent, no second row — §4.5 UNIQUE held), but
the message pointed the caretaker at the wrong cause: the occurrence is no
longer applicable *because it is already recorded*, not because it left the
apply window.

## Root cause

`actions/care_plan.go` `CarePlanApply`: terminal states (applied / skipped /
deferred — `careplan.status.go` `itemApplicable` returns false for them) fell
through to the generic `!item.Applicable` branch, which carries the hors-délai
message. The §4.5 idempotency message only fired on the UNIQUE-constraint
fallback, which is unreachable for items still present in the day plan.

## Fix plan

1. In `CarePlanApply`, after `findItem` and before the window check, add an
   explicit terminal-state check:
   applied/skipped/deferred → `409 "occurrence already recorded (idempotent, §4.5)"`.
2. Test approach: extend the existing HTTP round-trip test
   `TestCarePlanDayPlanApplyIdempotent` (real MySQL test DB, full app mux) with
   an assertion on the double-submit response body.
3. Validation: `go test ./actions -run TestCarePlanDayPlanApplyIdempotent`;
   then live E2E re-submit against the running server.

## Evidence of fix

- Unit/HTTP: `go test ./actions -run TestCarePlanDayPlanApplyIdempotent -count=1` → ok
  (assertion `require.Contains(string(raw), "already recorded (idempotent")`).
- Live E2E (browser session, already-applied dedicated plan item of animal
  1962/26): re-apply → `409 {"error":"occurrence already recorded (idempotent, §4.5)"}`.
- Full suite with race+cover: `go test -count=1 -race -cover ./...` → all ok.

**Status**: fixed & verified — closed 2026-09-27.
