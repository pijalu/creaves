# Plan — Schedule repeat mode ("start/end + every X") — 2026-10-09

Requirement: [e2e/bugs.md BUG-36](../e2e/bugs.md). Qualification E2E:
[e2e/E2E-PROTOCOL-EDITOR.md](../e2e/E2E-PROTOCOL-EDITOR.md) §P9.
Tracked as session goals G1–G5 (queue order = schedule).

## Context

Today the §4.3 schedule document only has explicit `times[]`. The protocol
editor offers a free-text range generator (BUG-35) that pre-expands slots
client-side; the repeat definition itself is lost at save. Users think in
"from 09:00 to 19:00 every 5 h", and the legacy converter derives exactly
that triple from `feeding_start`/`feeding_end`/`feeding_period` — then
flattens it and (for NULL rows) flags " (à vérifier)".

Change: persist the repeat definition alongside the expanded `times[]`.

## Locked decisions (user, 2026-10-09)

1. Overnight windows allowed (`to` < `from` wraps +24 h).
2. "Every" = hours:minutes duration picker (flatpickr-style), not free text.
3. Repeat is source of truth: server re-derives `times[]` from repeat at
   save/update; client-supplied times are ignored in repeat mode.
4. Repeat mode available for ALL protocol kinds (feeding, medication, care,
   observation, weighing).
5. Qualification on scratch DB `creaves_qualif` restored from
   `/Users/muaddib/dev/creaves.project/creaves-db-2026-10-12.gz`
   (pre-care-expert snapshot).

## Data model (no DB migration — JSON document change)

```json
{
  "times": ["09:00", "14:00", "19:00"],
  "repeat": { "from": "09:00", "to": "19:00", "every_minutes": 300 },
  "every_days": 1, "anchor": "intake"
}
```

- `repeat` optional. Absent ⇒ slots mode (unchanged behavior).
- `times[]` always present and remains the engine contract
  (`occurrences.go` untouched).
- Expansion semantics = existing `DeriveFeedingTimes`: start + n×period ≤
  end, start included, `to < from` ⇒ end += 24 h, cap 24 slots.

## Phases ↔ goals

### G1 — `models/careplan`: repeat block + expansion (foundation)

- `schedule.go`:
  - `type Repeat struct { From, To TimeOfDay; EveryMinutes int }`;
    `Schedule.Repeat *Repeat`.
  - `scheduleJSON.Repeat *repeatJSON` with
    `repeatJSON{from,to string; every_minutes int}` (pointers for
    present/absent, consistent with existing style).
  - `ParseScheduleJSON`: parse + validate repeat (HH:MM strict, every ≥ 1,
    expansion consistency: when both `repeat` and `times` are present, times
    MUST equal `ExpandRepeat` output — stored docs are always consistent
    because the server re-derives).
  - `Validate()`: repeat invariants; keep every existing invariant.
  - `func ExpandRepeat(r Repeat) []TimeOfDay` (wrap-aware, cap 24) — shared
    by validator, server normalization, and the converter.
  - `func NormalizeScheduleJSON(raw []byte) ([]byte, error)`: parse; if
    repeat present → re-derive Times from repeat; re-marshal canonical doc.
- Serialization audit: find every writer of schedule JSON
  (`actions/care_plan_support.go` marshal site, seeds, tests) — all go
  through the struct or are raw-string fixtures to extend.
- Unit tests `models/careplan/schedule_test.go`: parse/validate/round-trip,
  overnight, cap, inconsistency rejection, absent-repeat compatibility.
- Verify: `go test ./models/careplan/`

### G2 — Server save/update expansion (care_animal_plans + care_rules)

- `actions/care_animal_plans.go` Create/Update and the care_rules create/
  update handlers: run `careplan.NormalizeScheduleJSON` on the submitted
  schedule before validation/save. Repeat present ⇒ stored times are
  re-derived (decision 3). Errors → the BUG-33 translated field-error path
  (editor reopens on the schedule tab with a clear message, never raw JSON
  errors, never §/BUG references).
- HTTP tests: repeat create → stored JSON has block + expanded times; times
  tampered client-side are overwritten by expansion; invalid repeat → 422 +
  translated message; slots mode unchanged; PUT round-trip preserves repeat.
- Verify: `go test ./actions -run Repeat`

### G3 — Converter: legacy feeding → repeat form + DB qualification

- `actions/care_plan_convert_data.go`:
  - Non-NULL `feeding_start`/`feeding_end`/`feeding_period` ⇒ schedule gets
    `repeat` block (from/to = legacy start/end HH:MM, every_minutes =
    feeding_period) + expanded times; name WITHOUT " (à vérifier)" — the
    translation is exact (decision: "Convert MUST correctly translate").
  - NULL start/end ⇒ unchanged fallback (single 09:00 slot, no repeat, keep
    " (à vérifier)" + BUG-13 banner flow).
  - `DeriveFeedingTimes` reimplemented as `careplan.ExpandRepeat` call (keep
    exported shim if tests reference it).
- Converter tests: exact triple → repeat form; NULL → fallback; overnight
  legacy windows; idempotent re-run (marker).
- Qualification (decision 5): scratch DB `creaves_qualif`:
  1. `DROP/CREATE DATABASE creaves_qualif`; `gunzip -c creaves-db-2026-10-12.gz | mysql … creaves_qualif`
  2. Point a second app instance (or `database.yml` overlay env) at it;
     `buffalo db migrate up`; boot → startup converter runs.
  3. SQL verification: converted feeding plans where legacy triple was set
     have `JSON_EXTRACT(schedule,'$.repeat')` matching
     `feeding_start/feeding_end/feeding_period` and no "à vérifier" in the
     name; NULL-legacy rows keep fallback + flag. Counts recorded in the E2E
     result table.
- Verify: `go test ./actions -run Convert` + qualification checklist.

### G4 — Editor UI: mandatory mode radio + repeat helpers (all locales)

**Status: done (2026-10-12)** — see e2e/bugs.md BUG-36 progress for the
implementation record; full E2E deferred to G5.

- `templates/care_plan/_editor.plush.html`:
  - Radio `data-pe-mode="slots|repeat"` (required, default from document:
    repeat present ⇒ repeat; else slots; new empty plan ⇒ no default —
    mandatory choice per requirement).
  - Repeat mode: from/to = time pickers (flatpickr, localized like slot
    inputs), every = H:MM duration picker (flatpickr `enableTime,
    noCalendar`, 24 h, minute steps; value parsed as duration), live
    "(N occurrences)" preview (existing `updateRepeatCount`), slot rows
    hidden; schedule textarea written with `{repeat:…, times:expanded
    preview}` — server re-derives anyway.
  - Slots mode: existing slot rows (with the BUG-35 normalization fix:
    typed `9:00` normalized to `09:00` or inline error — never silent drop).
  - Mode switch with unsaved content in the other mode → confirm before
    discarding.
  - Mode hidden/disabled in raw-JSON fallback mode (raw stays authoritative).
- Re-open round-trip: repeat doc ⇒ radio=repeat, fields filled, slots
  hidden; plain doc ⇒ radio=slots.
- Applies on all host pages: animal modal + care_rules new/edit (all kinds,
  decision 4).
- Locales: new keys in `locales/care_plan.{fr,en-us,de,nl}.yaml`:
  `editor.mode`, `editor.mode_slots`, `editor.mode_repeat`,
  `editor.repeat_invalid`, `editor.repeat_every_hint`, plus discard-confirm
  string. Remove/replace the now-obsolete BUG-35 generator keys
  (`repeat_generate` etc.) if the UI element is superseded — decide during
  implementation; keep template keys used.
- Verify: `go build ./...` + browser smoke (G5 covers full E2E).

### G5 — Full suite + E2E qualification run

- `go test ./...` (care-plan suite green; the 4 known pre-existing
  golden-DOM failures excluded/documented).
- Run [e2e/E2E-PROTOCOL-EDITOR.md](../e2e/E2E-PROTOCOL-EDITOR.md) fully,
  incl. new §P9 (repeat mode) and the restored-DB qualification; record
  results in place; new failures → BUG-n entries.

## Risks / rollback

- **Mixed-version rollback:** old binaries reject `repeat` keys (strict
  parser). Rollback path = SQL/task to strip `repeat` from schedule JSON
  (times[] already materialized ⇒ zero functional loss). Document the
  strip-task in G1.
- **Golden-DOM tests** (`TestCarePlanPhase0bDOMEquivalence` etc.) snapshot
  editor markup — expect updates when the radio lands (G4); they are
  already failing pre-existing, do not chase beyond the editor delta.
- **Humanized schedule strings** (`care_plan_humanize.go`) still read
  times[] — acceptable; a "09:00–19:00 toutes les 5 h" rendering is a
  possible follow-up, not in scope.
- **BUG-35 generator keys**: superseded UI must not leave dead locale keys
  or dead JS (checked in G4 review).

## Out of scope

- Console (receiver) changes — webhook payloads carry animal state, not
  schedules.
- Every-days/weekdays interaction with repeat (orthogonal; repeat expands
  times within a day, every_days still steps days).
