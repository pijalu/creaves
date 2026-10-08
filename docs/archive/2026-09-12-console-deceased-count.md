# Fix archive — 2026-09-12 — Console deceased count wrong

## Original report (bugs.md)
> ## console does not show deceased count animals
> Console does not show the correct count of deceased animals - eg: it shows 1 due to a
> delete but does not show the actual deceased count from resync animals:
> http://localhost:3001/ — Décédé 1, En soins 212, Relâché 9928
> Even if resync animals are deceased. http://localhost:3001/reports does show
> "5689 Résultat : négatif (décédés)". Other stats are correct.

## Fix

`actions/dashboard.go` (`DashboardIndex`) built its "Animals by Status" card from
`current_status`, which only reflects the latest *state event*. Deaths recorded via
outtakes (outtake rating < 0 or outtake_dead = 1) never set current_status to 'died'
for resync-imported animals, so the dashboard showed 1 deceased while reports showed
5689.

The dashboard now reuses `tallyOutcomes()` — the same outcome-based classification as
the reports page:
- `in_care`: `current_status = 'in_care'` (new first column of the tally SQL)
- `died`: `sqlOutcomeDied` (negative outtake outcome, or died-in-care without outtake)
- `released`: `sqlOutcomeReleased` (positive/neutral outtake outcome)

Tests: `actions/dashboard_outcome_status_sqlite_test.go` —
`TestTallyOutcomes_InCareAndDeceasedClassification` (classification incl. died-in-care
fallback and error-outtake exclusion) and `TestDashboard_IndexOutcomeStatusCounts`
(the rendered dashboard status section shows the outcome-based counts).

## Tests / quality
- `CGO_ENABLED=1 go test -count=1 -race -cover -tags sqlite ./...` — green (actions
  coverage 57.5%).
- `go vet ./...`, `staticcheck ./...` clean (creaves-console).

## Verification (e2e, agent-browser)
```
http://127.0.0.1:3001/dashboard/  -> Animals by Status: In care 212, Released 4240, Died 5690
http://127.0.0.1:3001/reports/    -> Died 5690 (Outcome negative 5690 = positive 4110
                                     + neutral 130 = released 4240; total 10142)
Dashboard and reports now agree on the deceased count.
```

## Commit
- creaves-console `422e728` fix(dashboard): classify status counts by outcome, not
  current_status
