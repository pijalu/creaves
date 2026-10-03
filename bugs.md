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

Round 8 — revalidation sweep of the Round-7 commits (care-plan-round / fixes),
all findings reproduced live via agent-browser against the local dev instance
(admin session, en-US) on 2026-10-03.

### R8-1 `_apply_toggle` script block is a JS syntax error — every shared apply control dead

**Severity**: critical. **Found**: 2026-10-03, agent-browser on `/care_plan?kind=feeding`.

**Defect**: `templates/care_plan/_apply_toggle.plush.html` line 225
(= all four forks) wraps the `jsString(...)` call in literal quotes:

```js
wBox.textContent = "<%= jsString(t("care_plan.apply.last_weight")) %>: " + ...
```

`jsString` emits a full JS string literal INCLUDING quotes (json.Marshal of
the value), so the rendered script contains `"“Last recorded weight”: " + ...`
— a syntax error (`Unexpected identifier 'Last'`) that kills the ENTIRE
script block. Measured in the browser: `new Function(scriptText)` → parse
error; `window.planApply` === undefined.

**Blast radius** (all measured, en-US):
- `planApply` never defined → the care_plan index script (auto-refresh,
  batch apply, detail popup, late-record, history undo) crashes at
  `var csrf = planApply.csrf` — the whole page-level script dies.
- Group "Apply group (N)" buttons (`.plan-feeding-apply`, `.plan-cage-apply`)
  do nothing when clicked (no batch modal, no request) — verified on the
  VE28 row, 3 pending animals, feeding section.
- Skip/defer buttons, detail popup, batch modal, late-record buttons all
  dead on `/care_plan` for every kind.
- The medication slot toggle (a DIFFERENT script, `_plan_med_toggle`) still
  works — which is why single-slot apply/unapply tested fine.

**Fix**: drop the outer quotes in all four forks:
`wBox.textContent = <%= jsString(t("care_plan.apply.last_weight")) %> + ": " + ...`.

**Validation**: `new Function(scriptText)` parses; `window.planApply` is an
object; clicking a group apply opens the batch modal; apply records in DB.
Locale sweep after fix: fr/de/nl/en all render
`wBox.textContent = "<localized label>" + ": " + ...` and `planApply` is an
object on every locale.

**Status**: fixed 2026-10-03 (templates ×4 + regression pin in
`TestNoTemplateInterpolatesATranslationIntoAScript` rejecting quote-wrapped
`jsString` calls).

### R8-2 care-plan auto-refresh never fires; R4-4.1 30 s floor inverted

**Severity**: high (data staleness: day plan silently goes stale for users
who never click anything; the R4-4.1 protection is also defeated for users
who do).
**Found**: 2026-10-03, agent-browser on `/care_plan` — injected
`window.__probe` marker survived 95 s+ with no reload (period is 60 s),
no console errors, no modal open, `planApply.busy()` false.

**Defect**: `templates/care_plan/index.plush.html` line 626:

```js
if (Date.now() - window.planActionAge() < 30000) { return; } // R4-4.1 floor
```

`planActionAge()` already returns `Date.now() - lastActionAt`. The extra
`Date.now() -` inverts the guard in BOTH directions:

- Fresh page (`lastActionAt = 0`): `planActionAge()` = `Date.now()` →
  `Date.now() - planActionAge()` = 0 < 30000 → **defers on every 5 s tick
  forever** → the §10-CP6c auto-refresh NEVER fires until the user performs
  an action. Measured: marker survives 95+ s.
- Right after an action: `Date.now() - planActionAge()` = `lastActionAt`
  (≈1.79e12) ≥ 30000 → guard passes immediately → the reload can fire 1 s
  after the caregiver's change, wiping it from view — the exact failure
  R4-4.1 was written to prevent.

**Fix**: drop the extra subtraction:
`if (window.planActionAge() < 30000) { return; }`.

**Validation**: after fix, fresh page reloads at 60 s (injected marker
disappears); after `planApply.markAction()` the page must survive the next
30 s+ without reloading (marker persists past `nextReloadAt`).

**Status**: fixed 2026-10-03 (all four `index.plush.*` forks +
`TestAutoRefreshGuardUsesActionAgeDirectly` regression pin). Live-verified:
fresh page marker gone at 50–60 s; action marked at +32 s → page still alive
at +61 s (floor held), reloaded by +62–72 s (≥30 s after the action).

---

## Archived rounds

| Round | File |
|---|---|
| Round 7 (care plan / animal page UX) | `docs/archive/2026-10-03-care-plan-round-7-bugs.md` |
| Round 4 (care plan / treatment / dashboard UX) | `docs/archive/2026-10-02-care-plan-round-4-bugs.md` |
| Round 3 (care plan / treatment / navbar UX) | `docs/archive/2026-10-02-care-plan-round-3-bugs.md` |
| Round 2 (care plan UX) | `docs/archive/2026-10-01-care-plan-ux-round2-bugs.md` |

Round-7 companion plan: `docs/care-plan-round-7-fix-plan.md`.
Round-4 companion plan: `docs/care-plan-round-4-fix-plan.md`.