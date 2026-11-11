# E2E-PROTOCOL-EDITOR.md — Protocol editor & animal care-tab end-to-end browser test plan

**Created:** 2026-10-09 · **Status:** PLANNED (not yet run) · **Tester:** automated agent (agent-browser CLI, black-box GUI testing)
**Scope:** the care-protocol editor ("Nouveau protocole") on all its host pages, the animal
care tab (feeding banner), timeslot creation/generation, validation errors, icons, tab
navigation — covering open bugs **BUG-31 … BUG-35** in [bugs.md](./bugs.md).

| App | URL | Login | DB |
|---|---|---|---|
| creaves | http://127.0.0.1:3000 | `admin` / `admin` | MySQL `creaves` (dev, animal 10372 available) |

**Supported languages:** `fr`, `en-US`, `de`, `nl` — every UI assertion runs in **all 4**
(switch via `/lang/?lang=<L>&url=<path>`). Browser tool: `agent-browser`
(`.agents/skills/agent-browser/SKILL.md`). Evidence = command output + URLs, never
screenshots alone.

**Editor host pages under test** (all embed `templates/care_plan/_editor.plush.html` via
`window.planEditor.attach`):

| Host | URL | Form submit |
|---|---|---|
| Animal protocol modal | `/animals/10372#nav-plan` → *New protocol* button | `POST /animals/10372/care_animal_plans` |
| Animal protocol edit | `/animals/10372#nav-plan` → plan row *edit* | `PUT /animals/10372/care_animal_plans/{id}` |
| Global care rule new | `/care_rules/new` | `POST /care_rules` |
| Global care rule edit | `/care_rules/{id}/edit` | `PUT /care_rules/{id}` |

**Result legend:** ⬜ not run · ✅ pass · ❌ fail (bug id in bugs.md) · 🔁 retested pass after fix.

---

## P0 — Preconditions

| # | Step | Expected | Result |
|---|---|---|---|
| 0.1 | `buffalo dev` running on :3000, dev DB migrated | login page served | ✅ 2026-10-12 — `buffalo dev` on :3000, `/auth/new` 200 |
| 0.2 | Login `admin/admin` via `/auth/new` | landing renders, no 5xx | ✅ 2026-10-12 — login → `/` 302, console clean |
| 0.3 | Animal 10372 exists with a converted feeding plan (name contains `(conversion)`, schedule `{"times":["09:00"],…}`) | confirmed via `/animals/10372#nav-plan` | ✅ 2026-10-12 (adjusted) — animal 10372 has active feeding plan `Alimentation — 6 poussins et 2 souris - eau`, schedule `{"times":["09:00"]}`; name carries **no** `(conversion)`/`(à vérifier)` flag (already user-reviewed; P5 flag-clearing end-state already reached on this animal — P5 rows note this). Converted-plan repeat behavior qualified separately in P9b (9.13–9.18). |
| 0.4 | Browser console logging enabled for the whole run | every page load checked for JS errors + FontAwesome warnings | ✅ 2026-10-12 — `agent-browser console` checked on every page load of the run |

## P1 — Feeding banner & icons (BUG-31, BUG-34)

| # | Step | Expected | Result |
|---|---|---|---|
| 1.1 | Open `/animals/10372#nav-care`; read console | no JS errors; **zero** FontAwesome "Could not find icon" warnings | ✅ 2026-10-12 — console clean (Live Reload logs only); 0 FA warnings |
| 1.2 | Inspect feeding banner title icon and slot-time icon | utensils + clock glyphs render as SVG/webfont; **no blinking "?"/"!"** placeholder (no `<svg class="…svg-inline--fa…">` with the missing-icon path) | ✅ 2026-10-12 — banner title `data-icon="cutlery"` + slot `data-icon="clock-o"` rendered as real SVGs; 0 `data-icon="question"` |
| 1.3 | Attachment comment save button (care tab) | pencil icon renders, no placeholder | ✅ 2026-10-12 — 21× `data-icon="edit"` SVGs (attachment rows), no placeholder |
| 1.4 | `/sync_targets` → edit a target | retry-undeliverable button shows redo/repeat icon, no placeholder | ✅ 2026-10-12 (adjusted) — list lives on `/sync_configuration` (`/sync_targets` has no index route → 404 is correct); dummy target + DLQ fixture row (attempts=25) inserted; retry button renders `<svg data-icon="repeat">`, no placeholder; fixture + dummy target deleted after the check |
| 1.5 | Repeat 1.1–1.3 in fr / en-US / de / nl | same result in each locale | ✅ 2026-10-12 — fr/de/nl: console clean, 0 question-icons, cutlery+clock-o+edit SVGs present in each |
| 1.6 | **Tab link (BUG-34):** on `#nav-care`, click any other tab manually, then click the banner's "Manage in the Protocol tab" link (`#nav-plan`) | Protocol tab activates immediately (hashchange handler) | ✅ 2026-10-12 — General tab clicked manually, then banner `Manage in the Protocol tab` link clicked → Protocol tab `[selected]` without reload |
| 1.7 | Same on `/animals/10372/edit` form tabs | hash links activate tabs after manual switches | ✅ 2026-10-12 — opened `#nav-intake`, clicked General manually, clicked `#nav-intake` hint link → Intake tab re-selected |
| 1.8 | Reload `/animals/10372#nav-plan` directly | Protocol tab active on load (initial-hash path still works) | ✅ 2026-10-12 — Protocol `[selected]` on direct load; console clean |

## P2 — Timeslot creation: add, type, save (BUG-35)

| # | Step | Expected | Result |
|---|---|---|---|
| 2.1 | Animal modal → *New protocol* → kind = feeding | structured editor attached (`.plan-editor` visible), 1 empty slot row | ✅ 2026-10-12 — editor attached, 1 slot row; slot rows render (BUG-37 found + fixed for en-US alt display) |
| 2.2 | Click *add slot* ×2 | 3 slot rows visible | ✅ 2026-10-12 — 3 `[data-pe-slot]` rows after 2 clicks |
| 2.3 | Enter `09:00`, `14:00`, `19:00` via the time control; set name + caretype + food; Save | plan created; schedule JSON = `{"times":["09:00","14:00","19:00"],…}`; banner/plan list shows 3 times | ✅ 2026-10-12 — DB: `{"times":["09:00","14:00","19:00"]}`; plan list shows the 3 times |
| 2.4 | Enter a slot as `9:00` (no leading zero) and Save | value **normalized to `09:00`** or field flagged with a translated inline error — **never silently dropped** from the saved schedule | ✅ 2026-10-12 — typed `9:00` normalized to `09:00` in the schedule (BUG-35 fix live) |
| 2.5 | Enter garbage (`abc`, `25:99`) in a slot and Save | visible translated per-field error; plan NOT saved with missing slots | 🔁 2026-10-12 — fr: garbage slot flagged `is-invalid`, schedule excludes it; server save rejected with translated `care_plan.editor.schedule_invalid` after BUG-39 fix (previously leaked raw `careplan: schedule.times … (§4.3)`). en-US alt-input garbage: BUG-38 found + fixed (now flagged, text kept, not saved) |
| 2.6 | In en-US locale: enter a slot with the 12 h alt-input (`2:00 PM`) | saved value is `14:00` | ✅ 2026-10-12 — alt shows `2:00 PM`, raw/schedule value `14:00` |
| 2.7 | Delete a slot row (×) and Save | removed time absent from saved schedule | ✅ 2026-10-12 — deleted 09:00 → DB `{"times":["14:00","19:00"]}`; re-added for later steps |
| 2.8 | Repeat 2.1–2.5 on `/care_rules/new` | identical behavior | ✅ 2026-10-12 — editor attached, add/typing/`9:00`→`09:00`/garbage-flag identical; rule saved with `{"times":["09:00","14:00","19:00"]}` |

## P3 — Repeat-range generator (BUG-35)

**SUPERSEDED 2026-10-12 by BUG-36 / §P9** — the free-text "generate slots from a
range" UX was replaced by the first-class *Repeat range* mode (duration picker,
live occurrence count, translated `repeat_invalid`/`repeat_cap` errors, confirm
on mode switch). Equivalent coverage: 3.1/3.2 → 9.2/9.3; 3.3 → 9.2 (H:MM
duration picker, no free-text minutes); 3.4 → 9.7; 3.5 → 9.6; 3.6 → 9.9;
3.7 → 9.4; 3.8 → 9.12. Results recorded in §P9.

| # | Step | Expected | Result |
|---|---|---|---|
| 3.1–3.8 | (legacy generator UX) | — | 🔁 superseded — see §P9 rows 9.2–9.12 |

## P4 — Multi-tab editor layout & field-level errors (BUG-33)

| # | Step | Expected | Result |
|---|---|---|---|
| 4.1 | Open *New protocol* | editor fields grouped into tabs (e.g. *Général* / *Horaires*); tab strip visible in the modal and on `/care_rules/new` | ❌→🔀 2026-10-12 — editor groups fields into titled cards (*Schedule* / *Action content*), no tab strip. Deferred as **BUG-40** (error-clarity goal delivered via cards + inline marking; tab strip needs user confirmation) |
| 4.2 | Clear the name field, Save | editor redisplays, **auto-switches to the tab containing the name field**, field marked (`is-invalid`), translated message next to it ("Le nom est obligatoire" in fr) | 🔁 2026-10-12 — field marked `is-invalid` + translated message shown (BUG-33 fixes: `localizePlanVerrs` server-side + client field marking; card layout ⇒ no tab to switch to, see BUG-40) |
| 4.3 | Fix name, break the schedule (e.g. remove all slots if required), Save | editor switches to the Horaires tab with the flagged field + translated message | 🔁 2026-10-12 — translated schedule error ("Horaire invalide : …") + `CarePlanSchedule` marked `is-invalid` (BUG-39 + BUG-33 fixes); card layout, no tab switch (BUG-40) |
| 4.4 | Error audit: submit several invalid combinations | no rendered error contains `§`, `BUG-`, a JSON field path, raw Go text, or English when locale = fr/de/nl | 🔁 2026-10-12 — name-blank + schedule-invalid audited in 4 locales; raw engine text leak fixed (BUG-39) |
| 4.5 | 4.2–4.4 in all 4 locales | every message from `locales/care_plan.*.yaml` | ✅ 2026-10-12 — fr "Le nom est obligatoire" · en-US "Name is required" · de "Der Name ist obligatorisch" · nl "De naam is verplicht", field marked in each |

## P5 — Converted-plan confirm ("à vérifier") lifecycle

| # | Step | Expected | Result |
|---|---|---|---|
| 5.1 | `/animals/10372#nav-care` banner | "à vérifier" warning badge visible while the plan name carries the flag | ✅ 2026-10-12 — plan renamed to `… (conversion) (à vérifier)` (fixture) → banner renders `badge badge-warning` "te controleren" (nl locale) next to the plan |
| 5.2 | Open the flagged plan in the editor, adjust slots (e.g. via generator), Save | name loses ` (à vérifier)`, keeps ` (conversion)`; banner warning gone after reload | ✅ 2026-10-12 — plain edit+Save → DB name `… (conversion)` (flag stripped server-side), banner badge gone, page text has no "à vérifier" |
| 5.3 | `/animals/10372#nav-plan` plan list | plan name shown without the flag; no badge | ✅ 2026-10-12 — plan list shows `… (conversion)` without flag/badge |

## P6 — Animal form: legacy feeding removed, frozen server-side

| # | Step | Expected | Result |
|---|---|---|---|
| 6.1 | `/animals/10372/edit` care tab | no Feeding textarea, no diet-guide buttons/modal, no ForceFeed checkbox; feeding summary partial + HeatSource/O2 only | ✅ 2026-10-12 — no Feeding textarea, no diet-guide label/buttons, no ForceFeed input; `.feeding-plan-summary` read-only (2 plans listed, 0 editable inputs, single link `/animals/10372#nav-plan` "Beheren in het tabblad Protocol"); care inputs = HeatSource/Oxygen only |
| 6.2 | `/animals/new` care tab | same; **no `#nav-plan` link/hint rendered** (animal not persisted — BUG-32) | ✅ 2026-10-12 — no Feeding textarea, no ForceFeed, no `.feeding-plan-summary` rendered at all (new animal ⇒ no plans ⇒ BUG-32 hint/link absent); care inputs = HeatSource/Oxygen/search only |
| 6.3 | Crafted POST to `/animals/10372` with `Feeding=…&ForceFeed=1&FeedingStart/End/Period=…` | animal row unchanged for all five legacy columns (frozen) | ✅ 2026-10-12 — POST `_method=PUT` + `Animal.Feeding=LEGACY-HACK-ATTEMPT` + `Animal.ForceFeed=1` → 302 redirect (accepted) but DB `feeding` still `5 poussins et 2 souris - EAU`, `force_feed` still `0`; params ignored at handler/binding level |
| 6.4 | Animal show page | no "Feeding/Alimentation/Fütterung/Voeding" legacy label block | ✅ 2026-10-12 — no standalone legacy label (dt/th/label/legend), no editable Feeding textarea, no ForceFeed input; protocol tab + `.feeding-plan-summary` present |
| 6.5 | 6.1/6.2/6.4 in all 4 locales | parity | ✅ 2026-10-12 — `TestAnimalFormLegacyFeedingFrozenAllLocales` PASS fr/en-US/de/nl (edit+new); browser re-checked fr/de/en-US edit form: `feeding:false force:false summary:true` in all 3 |

## P7 — End-to-end protocol → day plan

| # | Step | Expected | Result |
|---|---|---|---|
| 7.1 | Create feeding protocol on 10372 with slots covering now ±2 h | `/care_plan?kind=feeding` lists the occurrence inside the Now window | ✅ 2026-10-12 — created "G5 P7 E2E" (feeding, caretype=Alimentation, slots mode, times=["23:00"], active) at 22:23; day plan lists "● B2 ACCUEIL À faire: 2063/26 À faire ○ 23:00" inside Now window |
| 7.2 | Apply the occurrence from the day plan | green ✓ in place (BUG-22 behavior), feeding banner reflects the applied care | ✅ 2026-10-12 — clicked `.plan-feeding-apply` → `planBatchModal` "Confirmer l'application… 1 entrée(s) cage B2" → Confirmer; row flipped in place to "✓ 23:00" (no navigation, BUG-22); care record persisted (`cares.type_id=Alimentation`, date 22:28); `.feeding-plan-summary` shows active plans |
| 7.3 | Deactivate the plan from `#nav-plan` | occurrences leave the day plan; banner shows no active feeding plan | ✅ 2026-10-12 — Edit plan → unchecked `#CarePlanActive` → Save (modal closed); DB `active=0`; day plan no longer lists the 23:00 occurrence (only other plan's 19:00 remains); `.feeding-plan-summary` no longer mentions "G5 P7 E2E" |

## P9 — Repeat-mode schedule model (BUG-36, supersedes P3 generator UX)

> **BUG-43 update (2026-11-13):** the mode radios are REPLACED by a tab
> strip (`[data-pe-mode-tab="slots|repeat"]` — the active tab IS the mode
> state) and an empty plan now defaults to the **slots** tab. Rows 9.1,
> 9.4, 9.9, 9.11, 9.12 expectations updated below → **RETESTed
> 2026-11-13: all ✅** (9.11 surfaced BUG-46, fixed same day); the other
> rows' behavior is unchanged (same sections, same serialization).

| # | Step | Expected | Result |
|---|---|---|---|
| 9.1 | Open *New protocol* | mode TABS visible: *Slots* / *Repeat*; **Slots tab preselected** on an empty plan (never blank) | ✅ 2026-11-13 — tabs "Horaires précis [active, aria-selected]"/"Plage répétée"; slots section visible, repeat `d-none`; 0 `input[name=peMode]`; 1 blank slot row |
| 9.2 | Radio=Repeat → fields | only from/to (time pickers) + every (H:MM duration picker) + live "(N occurrences)" shown; slot rows hidden | ✅ 2026-10-12 — `[data-pe-slots-section]` → `d-none`; repeatFrom/repeatTo/repeatEvery present; live "(3 occurrences)" shown for 09:00→19:00@05:00; schedule serializes to `{"repeat":{...},"times":[...]}` |
| 9.3 | Repeat: from 09:00, to 19:00, every 05:00 → Save | stored schedule = `"repeat":{"from":"09:00","to":"19:00","every_minutes":300}` + `"times":["09:00","14:00","19:00"]` (verify via DB `JSON_EXTRACT`); day plan shows 3 occurrences | ✅ 2026-10-12 — DB `repeat={"from":"09:00","to":"19:00","every_minutes":300}`, `times=["09:00","14:00","19:00"]`, active=1; day plan lists the 19:00 occurrence for 2063/26 (09:00/14:00 already past at test time 22:30) |
| 9.4 | Re-open saved plan in editor | Repeat TAB active, from/to/every filled, slot rows hidden | ✅ 2026-11-13 — plan "Blah" (repeat 08:00→19:00@60): repeat tab active, from=08:00, to=19:00, every=01:00, slots `d-none` |
| 9.5 | Edit only `to` → 22:00, Save | server re-derives times = 09:00/14:00/19:00/22:00 (repeat is source of truth); any client-tampered times ignored | ✅ 2026-10-12 — crafted PUT with tampered `times:["03:33","07:07","23:59"]` → DB kept server-derived `["09:00","14:00","19:00"]` (repeat source of truth). NOTE: doc's expected "…/22:00" was arithmetically off — 22:00 is not on a 5 h grid from 09:00 (next grid slot 24:00 > to), so correct `TestExpandRepeat` result is 3 slots; server re-derive + tamper-ignore verified |
| 9.6 | Overnight: from 20:00, to 08:00, every 04:00 | times = 20:00/00:00/04:00/08:00, no error | ✅ 2026-10-12 — DB `times=["00:00","04:00","08:00","20:00"]` (sorted §4.3), `repeat={"from":"20:00","to":"08:00","every_minutes":240}`, modal closed no error |
| 9.7 | Invalid repeat (missing every, or every 00:00) | translated inline error on the field, nothing saved; error contains no `§`/BUG/JSON paths | ✅ 2026-10-12 — both cases: modal stays open, two translated errors (client "Plage invalide : vérifiez les heures de début/fin et la période" + server "Horaire invalide…"), DB unchanged (still overnight 240), no `§`/BUG/JSON-path tokens in error text |
| 9.8 | Radio=Slots path | unchanged: explicit slot rows saved, no `repeat` key stored; typed `9:00` normalized to `09:00` (BUG-35 fix) | ✅ 2026-10-12 — typed `9:00` → normalized `09:00` on change (no invalid flag); saved plan DB `repeat=NULL`, `times=["09:00"]` |
| 9.9 | Mode switch with unsaved entries in the other mode | confirmation shown before discarding; cancel keeps the CURRENT tab active (nothing to revert) | ✅ 2026-11-13 — dirtied repeat `to`=20:00 → click Slots tab: confirm "Changer de mode efface la saisie de l'autre mode. Continuer ?"; cancel → repeat tab stays active, 20:00 kept; confirm → slots tab active, repeat fields cleared, serializes `{"times":[]}`; DB untouched after Cancel |
| 9.10 | Repeat mode on medication + care kinds (animal modal AND `/care_rules/new`) | radio available and functional for every kind | ✅ 2026-10-12 — animal modal: medication & care → radio visible, from/to/every present, slots hidden, enabled. `/care_rules/new` (RuleActionKind=medication): repeat serializes `{"repeat":{"from":"08:00","to":"20:00","every_minutes":360},"times":["08:00","14:00","20:00"]}`, slots hidden |
| 9.11 | Raw-JSON fallback (unparseable schedule pasted in Advanced) | mode TABS hidden; raw remains authoritative | ✅ 2026-11-13 (with **BUG-46** found+fixed) — mid-session raw entry silently discarded the paste on next field edit (pre-existing since 3ddc805, unreachable attach-time path because `schedule` is a MySQL `json` column); fixed: unparseable change now enters raw mode symmetrically (tabs hidden, banner "Ce JSON n'a pas pu être lu…", textarea `is-invalid`), field edits leave the raw text untouched, clean parse exits raw with values loaded |
| 9.12 | 9.1–9.9 in fr / en-US / de / nl | full translation, duration picker 24 h everywhere (EN slots still 12 h alt display); tab labels translated; Slots tab active by default everywhere | ✅ 2026-11-13 — fr "Horaires précis"/"Plage répétée" + "Ajouter un horaire"; en "Explicit times"/"Repeat range" + "Add a time"; de "Feste Uhrzeiten"/"Wiederholungszeitraum" + "Uhrzeit hinzufügen"; nl "Vaste tijdstippen"/"Herhaalperiode" + "Tijdstip toevoegen"; Slots tab active everywhere; EN slot input altInput=12 h, `repeatEvery` 24 h no-alt in all locales; BUG-45 Advanced labels verified in all 4 ("Display: max … (0 = all)" etc.) |

### P9b — Converter qualification on the pre-care-expert snapshot

| # | Step | Expected | Result |
|---|---|---|---|
| 9.13 | `mysql … -e "DROP DATABASE IF EXISTS creaves_qualif; CREATE DATABASE creaves_qualif CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;"` then `gunzip -c …/creaves-db-2026-10-12.gz \| mysql … creaves_qualif` | restore OK; record `animals`, `cares` counts | ✅ 2026-10-12 — 10382 animals restored (collation must be `utf8mb4_0900_ai_ci` — `unicode_ci` breaks a join migration with Error 1267) |
| 9.14 | App instance pointed at `creaves_qualif` (`GO_ENV=production DATABASE_URL=mysql://…/creaves_qualif…`); `buffalo db migrate up` (63 applied) then boot | startup converter runs (marker written) | ✅ 2026-10-12 — boot log: `converter: done — feeding 0 rules + 207 plans, treatments 98 plans`; v3 `+0`; v4 `207 considered, 0 created, 0 uncovered`; markers `startup_v2/v3/v4` all written |
| 9.15 | SQL: converted feeding plans joined to legacy animals with non-NULL `feeding_start/end/period` | every such plan has `$.repeat.from/to/every_minutes` exactly matching the legacy columns, `times[]` = expansion, name has NO "à vérifier" | ✅ 2026-10-12 (controlled fixture — see note) — 3/3 exact: `08:00→18:00@120` → 6 slots; `07:30→19:30@360` → 3 slots; overnight `22:00→06:00@240` → `["02:00","06:00","22:00"]` (sorted §4.3), repeat block exact, 0 flagged; automated mismatch query over all 207 plans = **0 mismatches** |
| 9.16 | SQL: converted feeding plans from NULL-legacy rows | single `09:00` time, NO repeat block, name keeps " (à vérifier)" | ✅ 2026-10-12 — 204/204: `$.repeat` NULL, `times=["09:00"]`, flagged |
| 9.17 | Open one converted animal page (from 9.15) → protocol editor | radio=Repeat, legacy from/to/every shown correctly | ✅ 2026-10-12 — converted schedule doc from `creaves_qualif` animal 10117 (`repeat={from:08:00,to:18:00,every_minutes:120}`, times=[08:00…18:00], anchor intake, every_days 1) loaded into editor: repeatChecked=true, from=08:00, to=18:00, every=02:00 (=120 min as H:MM), slotsHidden=true. (Fixture plan copied to dev DB animal 10372 for the UI check, then removed.) |
| 9.18 | Record counts (converted-with-repeat / fallback-flagged) in this table | numbers attached | ✅ 2026-10-12 — **3 with-repeat / 204 fallback-flagged** (of 207 in-stock animals; 0 in-stock exact triples in the real snapshot — see note) |

> **P9b note (2026-10-12, important):** the real snapshot contains **no convertible exact triples in stock** — all 6629 legacy `feeding_start` values are Go zero-dates (`0001-01-01 HH:MM:SS`), and migration `20261007140000_animal_feeding_zero_dates_null` (pre-BUG-36) NULLs them at migrate time, before the converter runs. The 3167 exact triples that survive belong exclusively to already out-taken animals, which the converter deliberately skips (`outtake_id IS NULL`). Repeat-form mechanics were therefore qualified with a controlled fixture: 3 in-stock animals were given authentic zero-date triples (`0001-01-01 …`), their fallback plans deleted, markers cleared, and the boot converter re-run — it converted exactly those 3 to unflagged repeat-form plans (9.15). Consequence for production: on any DB upgraded through the 2026-10-07 migration chain, legacy feeding windows are nulled before conversion, so BUG-36 repeat-form conversion only ever triggers for rows written after that migration (or restored pre-migration dumps converted without it). Fallback+flag for the 204 NULL-legacy rows is the honest, intended outcome (BUG-10 "all replaced actions converted").

## P8 — Automated regression gate

| # | Step | Expected | Result |
|---|---|---|---|
| 8.1 | `go build ./...` | clean | ✅ 2026-10-12 — exit 0 |
| 8.2 | `go test ./actions -run "TestCareAnimalPlanUpdateConfirmsReview|TestAnimalFormLegacyFeedingFrozenAllLocales|TestAnimalShowCareTabHasNoSpeciesDietFrame"` | all pass (BUG-32 fixed) | ✅ 2026-10-12 — `ok creaves/actions` (all 3 pass) |
| 8.3 | `go test ./actions` (full, `CGO_ENABLED=1 -tags sqlite` per AGENTS.md if applicable) | no new failures vs the 4 known pre-existing golden-DOM failures (`TestCarePlanBatchGuardrails`, `TestCarePlanErrorContractDetail`, `TestCarePlanPhase0bDOMEquivalence`, `TestCarePlanLateRecordBounds`) | ✅ 2026-10-12 23:17 — **`go test ./...` FULLY GREEN, zero failures** (run inside the 22:00–24:00 window that triggered the original failures). All 8 pre-existing failures FIXED, not merely documented: (a) 5 tests anchored occurrences yesterday/beyond the seeded R8-5 8h view caps → new `uncapDayPlanForTest` fixture helper seeds NULL caps (documented "show everything" mode) with default-restore cleanup; (b) the phase0b golden baselines had been recorded while the persistent test DB held stale 24h-era preference rows → re-recorded (`PHASE0B_RECORD=1`), diff audited: only capped occurrences/counts change, delta documented in the harness comment; (c) the two medication sharing failures were **BUG-41** — a real production defect: the §10-M1 dedup lookup bound TZ-aware local day bounds while `treatments.date` stores wall-clock digits (DSN without loc), so clicks 22:00–24:00 local missed the existing row → fixed in `writeMedicationFulfillment` + deterministic pin `TestCareMedicationEveningClickSharesRow` (bites with the fix stashed, passes with it applied). |
| 8.4 | New/updated tests for BUG-33 (error contract) and BUG-35 (slot normalization / generator presets) exist and pass | green | ✅ 2026-10-12 — `go test ./actions -run 'Repeat\|Convert\|Normalize\|Schedule'` ok; `go test ./models/careplan ./models` ok (incl. new `actions/care_plan_repeat_schedule_test.go`) |

---

## Traceability

| Bug | Covered by |
|---|---|
| BUG-31 icons | P1 (1.1–1.5) |
| BUG-32 `#nav-plan` on /animals/new | P6 (6.2) + 8.2 |
| BUG-33 multi-tab + errors | P4 |
| BUG-34 hashchange tab links | P1 (1.6–1.8) |
| BUG-35 timeslot creation/generator | P2, P3, P7 |
| Confirm-on-save ("à vérifier") | P5 |
| Legacy feeding removal/freeze | P6 |

## Execution notes

- Run top-to-bottom; record results in the Result column with the same conventions as
  [E2E-TEST.md](./E2E-TEST.md) (✅/❌→bug id/🔁).
- Any failure → new BUG-n entry in [bugs.md](./bugs.md) with repro + evidence, then fix
  and retest in place.
- Console must stay clean for the entire run: any FontAwesome warning or JS error is an
  automatic ❌ for that row.
