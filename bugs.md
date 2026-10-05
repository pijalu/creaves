# Bugs — open list

Running bug list for the current round. Add every new report here, fix it with
a fix plan, validate, then move the resolved entries to `docs/archive/` when the
round is closed.

**Guideline** (same convention as the archived rounds):
1. Create a detailed fix plan for each bug — the plan must contain test approach and validation steps — execute the plan and validate the fix when all elements are in place.
2. Any issues found must be fixed and the fix plan updated accordingly.
3. Issues found during testing must be fixed and the fix plan updated accordingly.
4. Each bug is moved to `docs/archive/` when tested and closed with its plan.
5. **All changes must be tested, including e2e testing using the [agent-browser skill](../../.agents/skills/agent-browser/SKILL.md)** — no fix is complete without e2e evidence (commands, URL, captured output).
6. Use interactive shell/filmstrip to validate the output of the tool — verify the actual terminal output, not the plan.
7. Check code quality with each tool run separately (do not chain them with `;` or `&&`):
   - `go vet ./...`
   - `staticcheck ./...`
   - `gocognit -over 15 .`
   - `gocyclo -over 12 .`
   - `go test -count=1 -race -cover ./...`
8. Commit each fix with a clear and descriptive commit message.

**Session constraints:**
- **creaves contains real production data**: never drop/destroy data; schema changes must be additive/backward compatible (no destructive migrations).
- **Every UI change lands in all four locales** (`en-US`, `fr`, `de`, `nl`) — the `templates/**/*.plush.{html,fr,de,nl}.html` forks stay in sync.

---

## Open items

### B10-1 — "Done" timestamps render in UTC, not the user's browser locale

**Reported:** 2026-10-05 (caregiver feedback).

**Symptom:** every "done" timestamp (applied/completed care entries — history rows, treatment-entry badges, toggle tooltips) is shown in UTC instead of the browser's local time. A care applied at 14:30 local reads `12:30` (or an ISO UTC string), which is actively misleading on the work screen.

**Evidence (code):** the timestamps are formatted SERVER-side with the raw `time.Time` (UTC) instead of the client locale, e.g.:
- `templates/animals/show.plush.html:611` — `entry.AppliedAt.Time.Format("15:04")` (Go/server tz).
- `templates/care_plan/_plan_history_table.plush.html` — `data-due-at` is RFC-3339 UTC but the visible label is pre-formatted server-side (no client re-localization).

**Expected:** any user-visible "done"/applied/history time is rendered in the user's browser locale + timezone (e.g. via `Date.prototype.toLocaleString` on an RFC-3339 data attribute, matching how the toggle `title` already localizes `data-due-at` in the treatment slot JS), in all 4 locales.

**Scope check:** history table, animal-page treatment tab done badges, dashboard done rows, care_plan history section — audit every `Format(` on applied/terminal timestamps.


## Archived rounds

| Round | File |
|---|---|
| Round 9 + 8 (caregiver UX / data quality / i18n + revalidation sweep) | `docs/archive/2026-10-05-care-plan-round-9-bugs.md` |
| Round 7 (care plan / animal page UX) | `docs/archive/2026-10-03-care-plan-round-7-bugs.md` |
| Round 4 (care plan / treatment / dashboard UX) | `docs/archive/2026-10-02-care-plan-round-4-bugs.md` |
| Round 3 (care plan / treatment / navbar UX) | `docs/archive/2026-10-02-care-plan-round-3-bugs.md` |
| Round 2 (care plan UX) | `docs/archive/2026-10-01-care-plan-ux-round2-bugs.md` |

Round-7 companion plan: `docs/care-plan-round-7-fix-plan.md`.
Round-4 companion plan: `docs/care-plan-round-4-fix-plan.md`.