# Care-plan feature — full validation report (2026-09-27)

Validation of the uncommitted care-expert / care-plan workstream (day plan, rules,
matchers, animal plans, startup converter) against a fresh production-scale dump.

## 1. Database reset

| Step | Result |
|---|---|
| Drop + recreate `creaves` (MySQL 8.4.11) | OK |
| Load `creaves-db-2026-09-26.gz` | OK — 10 242 animals, 57 801 treatments, 6 497 animals with feeding schedules, 100 users |
| Drop admin account | OK — `DELETE FROM users WHERE Login='admin'` → 0 rows left |
| `buffalo db migrate` (GO_ENV=development) | OK — 48 migrations applied (dump was at `20250122`), incl. all 7 `2026102609*` care-plan migrations |
| `buffalo task db:seed` | OK — startup reference seeds + **admin recreated** (`admin`, Admin=1, Approved=1) |

## 2. Boot + startup converter (real data)

App booted via `GO_ENV=development /tmp/creaves-app` (go build of ./cmd/app, full
`go build ./...` green). Converter runs post-migration, pre-HTTP; failure aborts boot —
boot succeeded and server served requests.

Boot log line:

```
care_plan_converter: done — seeds +12 rules /17 matchers, feeding 2 rules + 32 plans, treatments 85 plans
```

Persisted report (`care_plan_conversion`, key `startup_v1`) vs DB counts:

| Metric | Report | DB check |
|---|---|---|
| Seed matchers inserted | 17 | 19 total = 17 seed + 2 feeding-cluster matchers, all converter-tagged ✓ |
| Seed rules inserted | 12 | 14 total = 12 seed + 2 feeding-cluster rules ✓ |
| Feeding rules created | 2 | ✓ |
| Feeding plans created | 32 | 32 animal plans, action_kind=feeding ✓ |
| Treatment series | 86 | — |
| Treatment plans created | 85 | 85 animal plans, action_kind=medication ✓ |
| care_plan_applications | — | 0 before E2E ✓ |
| care_rule_exclusions | — | 0 ✓ |

All converter rows carry the `[source: care_plan_converter]` description tag (§8.2
rollback key). Marker idempotency: single `startup_v1` row, no re-run on restart.

## 3. Browser E2E (agent-browser, http://localhost:3000, admin/admin)

Per language (en-US, fr, de, nl): day plan page + statuses, one apply flow,
care-rules index, rule preview, matcher library index, matcher preview,
animal Plan tab. Locale switched via `/lang/?lang=…&url=…` cookie.

### en-US
- Login: `/` → 302 `/auth/new` → dashboard 200.
- Day plan `/care_plan`: "Day plan" / "Cage chores"; cols Animal/Plan/Due/Status;
  2 862 items — **Late 616, Scheduled 1 974, "Replaced by animal plan" 272**.
- Apply (POST `/care_plan/apply`, CSRF header, JSON ref): **201**;
  `cares` 374 044 → **374 045** (+1 care row, feeding); applications 0 → 1;
  row badge **Late → Done** after reload.
- `/care_rules` 200: 14 rows (= DB), EN headers.
- Rule preview `/care_rules/{id}/preview`: 200 JSON with per-animal match + DSL trace.
- `/care_matchers` 200: 19 rows (= DB).
- Matcher preview POST `/care_matchers/preview`: 200, match_count=4 + traces.
- Animal Plan tab (`#nav-plan-tab`) on animal 8635: `#careAnimalPlansTable` shows its
  2 converted medication plans, "New plan" button present.

### fr
- Day plan: "Plan de la journée" / "Soins des cages"; badges **En retard / Planifié /
  Fait / Remplacé par un plan animal**; localized nav + columns.
- Apply on a converted **medication** plan (animal 326/26): **201**, applications 1 → 2;
  fulfillment = **treatment** (by design §10: feeding/care → care row, medication →
  treatment row), badge shows **Fait**.
- "Règles de soins" / "Bibliothèque de sélecteurs" / "Plans de cet animal" / "Nouveau plan" — all localized.

### de
- Day plan: "Tagesplan"; badges **Erledigt / Überfällig / Geplant / Durch Tierplan ersetzt**.
- Apply (feeding, animal 1340/24): **201**, `cares` → **374 046**, applications → 3, badge **Erledigt**.
- "Pflegeregeln" / "Matcher-Bibliothek" / "Pläne dieses Tieres" / "Neuer Plan".

### nl
- Day plan: "Dagplan"; badges **Klaar / Te laat / Gepland / Vervangen door dierplan**.
- Apply (feeding, animal 224/25): **201**, `cares` → **374 047**, applications → 4, badge **Klaar**.
- "Zorgregels" / "Matcherbibliotheek" / "Plannen van dit dier" / "Nieuw plan".

### Errors
- **0 HTTP 500**, 0 `EROR` log lines, 0 panics across the whole session
  (all "500" log greps are the drug name "amoxiclav sandoz 500 mg/50mg").
- Browser console: benign logs only.

## 4. Console impact — creaves-console needs **no changes**

Evidence:
1. `creaves-console` working tree untouched (git status clean).
2. Console code (actions/, models/) contains **zero references** to the new
   care-plan tables (`care_rules`, `care_matchers`, `care_animal_plans`,
   `care_plan_applications`, `care_plan_conversion`, `care_rule_exclusions`).
3. The feature's HTTP surface is entirely inside the Creaves instance
   (`/care_plan*`, `/care_rules*`, `/care_matchers*`); no new outbound endpoints.
4. `git diff` of the Creaves workstream touches **no webhook/event contract code**
   (no changes to payload keys, event types, or the pusher) — so the receiver-side
   schema is unaffected.
5. The lifecycle events the console already consumes (animal create/update/…)
   are unchanged by this workstream; the day plan / rules / matchers are
   instance-local care management and out of the consolidation scope.

## 5. Verdict

All completion criteria verified: fresh DB from `creaves-db-2026-09-26.gz` with admin
drop/recreate, 48 migrations + seed, boot-time converter on real production-scale data
(counts match persisted report exactly), full browser E2E in en-US/fr/de/nl covering
login, day plan statuses, apply flow (care row created, localized badges), rules +
matchers pages + both preview endpoints, animal Plan tab — with captured evidence and
no 500s. **creaves-console requires no changes.**
