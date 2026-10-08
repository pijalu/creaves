## #88 (enhancement, RAPIDE, OK) — Visites vétérinaires du jour dans dashboard — DONE ✅ commit 2481728
**Source:** https://github.com/pijalu/creaves/issues/88
**Observed:** Dashboard has no today's planned vet visits; "Animaux à gaver" block too long in season.
**Expected:** Dashboard shows today's planned vet visits; gaver block replaced by gavage function.
**Plan:**
1. `actions/dashboard.go`: query veterinaryvisits planned today (date = today, not done), preload animal; add to dashboard vars.
2. `templates/dashboard/`: add "Visites vétérinaires du jour" block (animal link, time, vet).
3. Replace "Animaux à gaver" listing with gavage action/button per animal (existing gavage flow).
**Test:** unit test dashboard query (today filter, excludes done/past); e2e: dashboard renders block with seeded visit.
**Validation:** `go test ./actions/...` + agent-browser on /dashboard/ with seeded visit.
