## #100 (bug) — Empêcher les encodages multiples — DONE ✅ commit 03bb6eb
**Source:** https://github.com/pijalu/creaves/issues/100 | prod-impacting
**Observed:** Chrome users get duplicate entries (entrées, visites vétérinaires, traitements) when double-clicking submit / slow response. Seen in prod.
**Expected:** One record per submit; second click no-op.
**Plan:**
1. Disable submit button on first click in shared form JS (`templates/` + application JS) — generic helper applied to all new/edit forms (treatments, veterinaryvisits, cares, animals).
2. Server-side guard: in `Create` actions (`actions/treatments.go`, `actions/veterinaryvisits.go`, `actions/cares.go`), reject exact-duplicate insert within short window (same animal + same type + same created-by + <60s) → flash + redirect to existing record instead of insert.
3. Consider idempotency token in form (hidden UUID) checked server-side; keep additive (new nullable column if persisted).
**Test:** unit test duplicate-guard (create twice via action → 1 row); e2e agent-browser: submit form, double-click, assert single record. Run: `go test -count=1 -race ./actions/...`
**Validation:** manual double-click on /treatments/new/, /veterinaryvisits/new, /cares/new — no duplicates.
