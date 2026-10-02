# Care plan / treatment / dashboard UX — Round 2 bugs

Tracking document for the care-plan / treatment / dashboard UX round-2 bugs
(treatment view, dashboard medication view, care plan view). All work is in
the `creaves/` project.

**Guideline** (same as `/Users/muaddib/dev/creaves.project/bugs.md`):
1. Create a detailed fix plan for each bug - the plan must contain test approach and validation steps - execute the plan and validate the fix when all elements are in place.
2. Any issues found must be fixed and the fix plan must be updated accordingly.
3. Issues found during testing must be fixed and the fix plan must be updated accordingly.
4. Each bug should be moved to docs/archive when tested and closed as the associated plan.
5. **All changes must be tested, including e2e testing using the [agent-browser skill](../.agents/skills/agent-browser/SKILL.md)** — no fix is complete without e2e evidence (commands, URL, captured output).
6. Use interactive shell/filmstrip to validate the output of the tool - you must verify the actual terminal output using agent-browser skill
7. Check code quality with each tool run separately (do not chain them with `;` or `&&`):
- `go vet ./...`
- `staticcheck ./...`
- `gocognit -over 15 .`
- `gocyclo -over 12 .`
- `go test -count=1 -race -cover ./...`
Fix any issues.
8. Commit each fix with a clear and descriptive commit message

### Session constraints
- **creaves contains real production data**: never drop/destroy data; schema changes must be additive/backward compatible (no destructive migrations).

At the end of the session - the bug list must be empty, all changes committed and resolved entries archived in `docs/archive/`. If new items are added, restart the process.

---

<!-- Previous rounds fixed & archived:
- docs/archive/2026-10-02-care-plan-review-regressions-1-7.md
- docs/archive/2026-10-01-care-plan-ux-round2-bugs.md -->


<!-- Round 3 fixed & archived: docs/archive/2026-10-02-care-plan-round-3-bugs.md
     (R3-1..R3-7 + follow-ups: engine window clamp, feeding-only plan entries,
     measured navbar collapse, in-page detail popup, TestUsersListFilters) -->

_Round 3 empty — all bugs fixed, verified (go tests + agent-browser e2e in
en/fr/de/nl), quality gates green and archived._
