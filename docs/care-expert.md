# Care Expert System — Specification

**Status**: Specification — **v2, open questions resolved** (no implementation yet)
**Project**: `creaves/` (no Creaves Console impact — care data is deliberately excluded from webhooks)
**Date**: 2026-09-20 (§10 decisions recorded 2026-09-21)

---

## 1. Need Statement (reviewed)

Build a **rule-driven expert system** that guides caretakers to deliver complete
care for each animal, based on the animal's condition:

1. Admin defines **general feeding / care / medication / cleanup rules**.
2. Rules **match animals** on parameters: age, type, species, weight, parasites
   (flags + free text), and **regex on any text field**.
3. A matcher is a **named, reusable query** over the animal condition
   (boolean expression — AND/OR/NOT); rules reference matchers.
4. A matching rule produces **recurring actions**: explicit times of day,
   repeat parameters, valid from/to dates.
5. Actions are **tracked at animal level** so subsequent planning knows what was done.
6. The system renders **everything due in a future time window**, shows
   **late/missing items**, and tracks **applications** (who did it, when) —
   in **two lenses over the same data**: a center-wide day plan and a
   per-animal **day view** (treatment-style glimpse of the day). The day
   plan is first and foremost a **"what should I do next"** driver —
   like the existing feeding/landing screens, it must answer that question
   at a glance, not merely list a chronological trace.
7. Planning works at **two levels**: generic rules (admin, matcher-driven)
   **and** single-animal schedules created directly on the animal
   (caretaker, treatment-style — no matcher, no rule setup).
8. Admin UI must be **clear to set up and navigate**, with **reuse**
   (e.g. one rule for multiple species/matchings).

This **extends** the existing feeding/treatment/care features and **can replace
their scheduling parts** once proven (see §8, Migration & Coexistence).

---

## 2. Current State (code review)

### 2.1 What exists today — four disconnected scheduling mechanisms

| Feature | Where | Model | Limits |
|---|---|---|---|
| **Feeding plan** | `actions/feeding.go` (`FeedingIndex`, `/feeding`) | per-animal columns `feeding`, `feeding_start`, `feeding_end`, `feeding_period` (minutes) | Single window per animal; period-only recurrence (no fixed times); set **manually per animal**; no rule reuse; no weight/parasite awareness. |
| **Treatments** | `models/treatment.go`, `actions/treatments.go` | `treatments` table: one row **per day** × animal, `timebitmap` (morning=1/noon=2/evening=4), `timedonebitmap` completion | Only 3 fixed slots/day; one row per day (57k rows for 10k animals); created **manually** via `TreatmentTemplate` per animal; dosage free text. |
| **Cares** | `models/care.go`, `actions/cares.go` | `cares`: event log (date, caretype, weight, clean, note, heat/oxygen) | Pure log — **no planning**. `caretypes` seeds: *Soin, Suivi, Alimentation, Alerte, Réponse alerte, Déplacement, Repas*. "Cleanup" = `clean=1` flag. |
| **Landing "clean cage"** | `actions/landing.go` | `cares.clean=1 AND date >= CURDATE()+3h` heuristic | Implicit daily-cleanup expectation, invisible as a rule. |

Adjacent, **not** scheduling: `feeding_guides` (static diet text per species ×
4 hardcoded stages — **0 rows in prod**, failed feature), `care_templates`
(reusable note texts), `todos` (free-text tasks with `recurrence` varchar —
unused for animals), `drugs` + `dosages` (per-animal-type posology),
`veterinaryvisits` (diagnostic log).

### 2.2 Animal condition data available for matching

| Source | Fields |
|---|---|
| `animals` | `species` (free text), `animalage_id` (bébé/juvénile/adulte), `animaltype_id` (15 types), `gender`, `zone`, `cage`, `ring`, `force_feed`, `intakeDate` |
| `intakes` | `has_parasites`, `parasites` (free text: *"puces ++++ - tiques"*, *"miases, puces"*…), `has_wounds`, `wounds`, `general`, `remarks` |
| `species` (resolved by name) | `class`, `order`, `family`, `agw_group`, `subside_group`, `native_status`, `game`, `huntable` |
| Derived | `LastWeight()` from cares; **days-in-care** = now − intakeDate; vet `diagnostic` |

### 2.3 Production data evidence (prod-like DB, 2026-09)

| Metric | Value | Implication |
|---|---|---|
| Animals in care | **223** | Rule evaluation set is small — in-memory matching trivial. |
| Cares total / last 30d | 371,559 / **11,536** | ≈ 385 care entries/day — the day-plan view is the primary work surface. |
| Care activity by hour | 07:00–23:00, peak 09:00–12:00 (44k–48k/hr bucket) | Times-of-day must be **explicit**, not just morning/noon/evening. |
| Animals with feeding plan | 196/223; periods: **600 (101), 240 (65)**, 480, 300… | Coarse periods dominate → fixed time lists (07:00/09:30/…/19:00) express reality better. |
| `force_feed` | 29 | First-class action property (hand-feed flag). |
| Parasites in care | 47 flagged, all with free text | Regex matcher on `intakes.parasites` is a real need (e.g. `tiques?`). |
| Wounds in care | 85 | Wound-care actions already exist (`WoundCareDrugName`). |
| Treatments | 57,459 rows; 281 open slots now; bitmaps mostly `2` (noon-only) or `5` | Row-per-day model scales badly; rule + application log replaces it cleanly. |
| Weight records | 122k/371k cares | "Weigh weekly" is a plausible rule action type. |

---

## 3. Conceptual Model

```
┌──────────────┐       ┌───────────────────┐
│  CareRule    │ n──1  │  CareMatcher      │  named, reusable query —
│  (admin CRUD)│──────►│  (matcher library)│  boolean DSL over animal
│              │       └───────────────────┘  condition; AND/OR/NOT
│              │ 1──1  ┌───────────────────┐  live in the expression
│              │──────►│  CareRuleSchedule │  (times, recurrence,
│              │       └───────────────────┘   from/to)
│              │ 1──1  ┌───────────────────┐
│              │──────►│  CareRuleAction   │  what to do (typed payload)
└──────────────┘       └───────────────────┘
        │  matching (in-memory, per request or on change)
        ▼
┌───────────────────┐    ┌──────────────────────────────────────────────┐
│  CareAnimalPlan   │    │  CarePlanItem  (virtual, per animal × source │
│  animal-level     │───►│  × scheduled occurrence in window)           │
│  schedule, no     │    └──────────────────────────────────────────────┘
│  matcher (action  │              │ fulfillment (user clicks "done" → writes
│  + schedule,      │              ▼  existing records)
│  caretaker CRUD)  │    ┌──────────────────────────────────────────────┐
└───────────────────┘    │  CarePlanApplication  (persisted join row)   │
                         │  plan occurrence → care / treatment / task    │
                         └──────────────────────────────────────────────┘
```

**Key design decision 1 — virtual plan items + persisted applications.**
Occurrences are computed, not stored (rules × animals × days × slots would
explode; a schedule edit must not orphan rows). What is persisted is the
**application**: the link between one occurrence and the real record that
fulfilled it (`cares` row / `treatments` row). Late/missing is derivable for
any occurrence: `now > due AND no application`.

**Key design decision 2 — matcher registry, not a closed whitelist.**
Parasites/wounds are today free-text flags on `intakes`, but will likely gain
structured definitions later (typed parasite list, wound location/severity,
evolution tracking). Matchable fields are therefore defined by a **Go-side
registry of field providers** (see §5.1): each entry knows how to resolve a
value from the enriched animal context, which ops it supports, and how to
render itself in the admin UI. Adding a future structured field (e.g.
`parasite_type`, `wound_severity`) = one registry entry + migration of the
data — **no change to rule storage, evaluation, or existing rules**. Matcher
expressions reference registry keys; unknown fields fail closed (no match)
with a UI warning instead of erroring the whole plan.

**Key design decision 3 — the matcher is a named query (DSL); AND/OR belongs
to the matching process, not the rule.** A matcher is not a bag of predicates
hanging off a rule: it is a first-class, named, reusable **query** over the
animal condition, written in a small boolean DSL (§5.2) — e.g.
`animal_type = "Hérisson" AND weight_g BETWEEN 200 AND 300`. Boolean
composition (AND/OR/NOT, parentheses) lives **inside the expression** — the
rule carries no `and/or` flag, it only references a matcher (`matcher_id`).
Matchers are shared across rules (edit once → every referencing rule sees
the new definition). Admins never need to write the DSL by hand: the UX
compiles a visual builder down to the expression (§7) — **ease of rule
setup is a key requirement**.

**Key design decision 4 — two planning levels, one plan.** Generic rules are
**not** the only way to schedule: caretakers create **animal-level plans**
(`care_animal_plans`, §4.7) directly on the animal — action + the *same*
schedule value object, no matcher, no admin rights needed (like treatments
today). Rules and animal plans feed the **same** occurrence generator,
applications log, day plan and per-animal day view. Collision semantics:
**animal-level wins, keyed on action type** — an animal-plan occurrence
suppresses the generic rule occurrence of the **same `action_kind`** in the
same time slot (an animal medication plan never supersedes a generic
feeding rule), surfaced as *"overridden"* in the why-explanation.
Separately, an animal can be **excluded** from a rule (§4.1
`care_rule_exclusions`).

---

## 4. Data Model (new tables)

All new tables use UUID PKs (app convention), `created_at/updated_at`, Fizz migrations.

### 4.1 `care_rules`

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `name` | varchar(200) NOT NULL | e.g. *"Hérisson bébé — gavage"*, *"Parasites tiques → Ivomec"* |
| `description` | text NULL | admin documentation |
| `action_kind` | varchar(32) NOT NULL | `feeding` \| `medication` \| `care` \| `cleanup` \| `weighing` \| `observation` (extensible, validated in Go) |
| `action_payload` | JSON NOT NULL | typed payload, see §4.2 |
| `schedule` | JSON NOT NULL | see §4.3 |
| `matcher_id` | uuid FK → care_matchers NULL | which named query selects animals; NULL = match all in-care animals. No boolean flags here — AND/OR lives in the matcher's expression |
| `active` | bool NOT NULL DEFAULT true | soft switch |
| `priority` | int NOT NULL DEFAULT 100 | ordering in day plan (lower = first) |
| `valid_from` | date NULL | rule-level activation window |
| `valid_to` | date NULL | |
| `stop_on_outtake` | bool NOT NULL DEFAULT true | auto-stop planning when animal leaves (default behavior anyway: only in-care animals match) |
| `latch_membership` | bool NOT NULL DEFAULT false | **course latch** (§5.4): when `duration_days` is set, an animal that has ≥1 application stays in the rule until the course ends even if it stops matching. Default false = live membership (open-ended rules). Ignored when `duration_days` is NULL. |

Indexes: `(active, action_kind)`.

**Per-animal opt-out — `care_rule_exclusions`:**

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `rule_id` | uuid FK → care_rules ON DELETE CASCADE | |
| `animal_id` | int FK → animals | |
| `reason` | varchar(500) NULL | shown in why-explanation |
| `created_by` | uuid FK → users | audit |

UNIQUE `(rule_id, animal_id)`. An excluded animal never matches the rule,
regardless of the matcher result; the exclusion is visible on the rule page
and on the animal's Plan tab ("excluded from rule X — reason").

### 4.2 `action_payload` JSON per kind

Validation in model `Validate` per `action_kind`.

```jsonc
// feeding
{ "caretype_id": "<Repas|Alimentation uuid>",   // required, must be a feeding-type caretype
  "food": "Croquettes + 4 VDF + EAU",           // free text / feeding-guide reference
  "force_feed": false,
  "note": "…" }

// medication
{ "drug": "Ivomec 1% (SC)",                     // free text (matches treatments.drug) or drugs FK later
  "dosage": "0.1 ml / 100g",                    // literal, or:
  "dosage_from_dosages_table": true,            // resolve via drugs.dosages × animaltype × LastWeight
  "remarks": "…" }
// ↳ dosage resolution failure (§10-B6): if dosages lookup yields nothing (no row for the
//   animaltype, or no weight on record), the plan item renders with a warning and Apply
//   stays enabled — the caretaker enters the dosage manually at apply time. Never blocked.
//   The warning includes the animal's last recorded weight and its date (§10-L2),

// care (generic, incl. wound care, heat/oxygen check…)
{ "caretype_id": "<Soin uuid>", "note": "Changer bandage", "heat_source_check": true }

// cleanup
{ "note": "Nettoyage cage" }                    // fulfills via cares.clean=1

// weighing
{ "note": "…" }                                  // fulfills via care row carrying a weight

// observation — prompt with an alert outcome (§10.6: alert is v1)
{ "prompt": "Mange seul ?",                     // the question shown at apply time
  "note": "…",                                  // optional default note on the care row
  "alert_on": "no",                             // "no" | "yes" | null. On the alert outcome the apply
                                                 // ALSO writes a care row of the Warning caretype
                                                 // (caretypes.warning=1, "Alerte") → animal row turns red
                                                 // until a ResetWarning ("Réponse alerte") care clears it.
                                                 // Uses the caretype.warning mechanism, NOT the vestigial
                                                 // cares.in_warning column.
  "alert_follow_up_hours": 4 }                  // §10-CP3: on the alert outcome, ALSO auto-create a
                                                 // single-occurrence follow-up observation animal-plan
                                                 // ("Vérifier alerte: <prompt>") due now+N h — closes
                                                 // the alert loop so no Warning stays unreviewed.
```

**`instructions` key on every payload (EN3, pulled into v1)**: any
`action_payload` may carry an optional `"instructions"` rich-text string
(+ optional attachment references, e.g. photos of the gavage technique),
rendered in the apply dialog. This is what makes the system a true *expert*
system: it captures the **how** (procedure knowledge, volunteer training
double-duty), not just the when. Additive JSON key — no schema change.

### 4.3 `schedule` JSON

```jsonc
{
  "times": ["07:00", "09:30", "12:30", "15:30", "19:00"], // exhaustive list, ≥1, sorted, HH:MM 24h
  "every_days": 1,        // every N days from anchor (default 1 = daily)
  "weekdays": [1,2,3,4,5,6,0], // optional restriction (0=Sunday)
  "anchor": "intake",     // "intake" | "fixed" — anchor for every_days
  "anchor_date": null,    // date when anchor=fixed
  "from_offset_days": 0,  // occurrence start = anchor + offset
  "duration_days": null,  // null = open-ended (until rule/animal stops matching); N = bounded course (e.g. 5-day antibiotics)
  "grace_minutes": 60,    // occurrence late after due+grace (DEFAULT, §10.2; per-rule overridable)
  "miss_after_hours": 24, // occurrence missing after due+miss (DEFAULT, §10.2; still displayed)
  "lookahead_minutes": 60 // §10-CP6a: an occurrence is "due" from this long before its instant (default 60)
}
```

**Timezone (§10-B1)**: occurrences are generated and `due_at` stored in **server
local time (`time.Local`)**, matching the existing care/treatment rows and the
single-center deployment (Europe/Brussels). This is a documented dependency, not
a per-rule setting. Because care activity is 07:00–23:00, no `times[]` slot is
ever in a DST gap/overlap; grace/miss arithmetic may span a transition by ±1h,
which is acceptable for a care plan. Add a DST boundary unit test regardless.

**Day-1 anchoring (§10-B3)**: when `anchor="intake"`, "day 1" is the **first
`times[]` slot strictly after the intake instant** — not the intake calendar
day. An animal admitted at 18:00 with a 07:00-slot rule does **not** get a
retroactive 07:00 item on intake day; its first occurrence is 07:00 the next
morning. `duration_days` counts **occurrence days actually generated**, not
calendar days from intake, so a 5-day course always yields 5 dosing days. The
clamp `[intakeDate, outtakeDate)` applies to the *instant*, so pre-intake and
post-outtake slots are dropped.

This covers: exhaustive times of day (explicit list, replacing
`feeding_period`), repeat parameters (`every_days`, `weekdays`, offsets,
duration), from/to (`valid_from/to` on rule + offsets on schedule).

### 4.4 `care_matchers` — named, reusable queries

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `name` | varchar(200) NOT NULL UNIQUE | e.g. *"Hérisson bébé < 300 g"*, *"Tiques à l'admission"* |
| `description` | text NULL | admin documentation |
| `expression` | text NOT NULL | matcher DSL (§5.2) — parsed & semantically validated at save |

- Referenced by `care_rules.matcher_id` (many rules → one matcher).
- Deleting a matcher still referenced by rules is blocked (UI lists the
  referencing rules).
- Runtime registry drift (field provider removed/renamed): evaluation fails
  closed (no match) and the matcher is flagged "broken" in the library UI
  for repair — never an error on the plan view.

A rule with **NULL `matcher_id`** matches every in-care animal (explicitly
displayed as "matches all animals" in UI).

### 4.5 `care_plan_applications`

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `source_type` | varchar(8) NOT NULL | `rule` \| `animal` — which planning level produced the occurrence |
| `source_id` | uuid NOT NULL | `care_rules.id` or `care_animal_plans.id` (no DB FK — dual target; enforced in code) |
| `source_snapshot` | JSON NOT NULL | name + payload at application time (audit against later edits) |
| `animal_id` | int FK → animals | |
| `due_at` | datetime NOT NULL | scheduled occurrence instant (kept for adherence stats) |
| `applied_at` | datetime NOT NULL | when marked done — **click time (now), §10.1**; the created care/treatment row also carries click time, not `due_at` |
| `user_id` | uuid FK → users | who |
| `fulfillment_type` | varchar(16) NOT NULL | `care` \| `treatment` |
| `fulfillment_id` | varchar(36) NOT NULL | uuid of the created `cares`/`treatments` row |
| `status` | varchar(16) NOT NULL | `applied` \| `skipped` \| `deferred` |
| `deferred_until` | datetime NULL | **§10-A2/CP4** — set only when `status=deferred`: the snooze target, **clamped to before the next occurrence of the same source**. NULL for applied/skipped. |
| `fulfillment_deleted` | bool NOT NULL DEFAULT false | **§10-CP1** — set by the care/treatment destroy hooks when the linked fulfillment row is deleted; the application is kept for audit and rendered "record deleted" instead of a dangling link. |
| `note` | text NULL | skip/defer reason (**mandatory for both skipped and deferred**, §10-CP4) / extra note |

**UNIQUE `(source_type, source_id, animal_id, due_at)`** — idempotent
fulfillment, prevents double-apply from two browser tabs. Indexes:
`(due_at, status)`, `(animal_id, due_at)`.

*Skip* is an explicit application (with mandatory reason) so "missing" means
truly forgotten, not consciously waived. **Two distinct actions (§10-A2):**
**Skip** (`skipped`) permanently waives the occurrence; **Defer** (`deferred` +
`deferred_until`) snoozes it — the item leaves the actionable list and
resurfaces when `now ≥ deferred_until`. A defer is an application row (so the
UNIQUE key still blocks a concurrent apply), but creates **no** fulfillment
record.

**Un-apply / correction path (§10-CP1 — admin-only)**: `POST
/care_plan/{item}/unapply` deletes the application row and offers to also
delete the linked fulfillment record (checkbox, default keep). The occurrence
then reverts to its computed status (due/late/missing). Restricted to admins —
caretakers ask an admin to correct a mis-tap, which keeps the care log
authoritative. Separately, `CaresResource.Destroy` /
`TreatmentsResource.Destroy` gain a hook: deleting a care/treatment row marks
any linked application `fulfillment_deleted=true` instead of leaving a
dangling `fulfillment_id` — the plan keeps the audit trail and renders
"record deleted".

### 4.6 Reuse model (no extra tables)

Reuse comes from the matcher library itself: broad queries
(`animal_type = ...`, `species_agw_group = ...`) or explicit lists
(`species IN (...)`) are written once and referenced by many rules. The
builder's species multi-select (494 species in prod) **compiles** selections
into an `IN (...)` clause — admins never type species names or expressions
for the common cases.

### 4.7 `care_animal_plans` — animal-level schedules (treatment-style)

The **single-animal** planning level: created directly from the animal page
by any caretaker, no matcher, no admin rights. Same action payload (§4.2)
and same schedule JSON (§4.3) as rules — full parity.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `animal_id` | int FK → animals ON DELETE CASCADE | the one animal it schedules |
| `name` | varchar(200) NOT NULL | e.g. *"Pansement patte G — 2x/j"* |
| `action_kind` | varchar(32) NOT NULL | same kinds as rules |
| `action_payload` | JSON NOT NULL | same schema as rules |
| `schedule` | JSON NOT NULL | same schema as rules (§4.3) |
| `replaces_kind` | bool NOT NULL DEFAULT false | **§10-A3 override flag**: when true, this plan suppresses **ALL** generic-rule occurrences of the same `action_kind` for this animal (not just slot collisions). When false, only same-slot collisions are suppressed. |
| `active` | bool NOT NULL DEFAULT true | |
| `created_by` | uuid FK → users | which caretaker created it (regular user or admin, §10-A5) |

Index `(animal_id, active)`. Semantics:

- Feeds the **same** occurrence generator, applications log (§4.5), day plan
  and animal day view as rules.
- **Animal-level wins — within the same action type only.** Two suppression
  modes (§10-A3):
  - **Slot-level (default, `replaces_kind=false`)** — a rule occurrence is
    suppressed only when **same animal + same `action_kind` + same time slot**
    (same `times[]` instant ± rule grace). Use for "an extra feeding at a
    different hour."
  - **Kind-level (`replaces_kind=true`)** — suppresses **every** generic-rule
    occurrence of that `action_kind` for this animal, regardless of slot. Use
    for the common "feed/treat this animal *instead of* the generic rule"
    case, preventing accidental double-dosing when slots don't collide.
  Different kinds never interact either way: an animal-level **medication**
  plan does NOT supersede a generic **feeding** rule — both appear even at
  the same hour (08:00 Ivomec from the animal plan *and* 08:00 grains from
  the feeding rule are two separate items). Suppressed occurrences are
  reported as *overridden by animal plan "<name>"* in the why-explanation.
- One-click **"promote to rule"** (admin): copies the plan into a rule draft
  pre-filled with the action/schedule and a matcher scaffold
  (`species = "<X>" AND animal_age = "<Y>"`) for the admin to generalize.

---

## 5. Matching Engine

### 5.1 Matchable fields — extensible registry

Fields are **not** hardcoded: a Go registry
(`models/careplan/matcher_registry.go`) maps field key → provider:

```go
type FieldProvider struct {
    Key      string                       // e.g. "parasites"
    LabelKey string                       // i18n key for admin UI
    Type     string                       // string | number | bool
    Ops      []string                     // allowed ops for this field
    Resolve  func(a *AnimalContext) Value // extraction from enriched animal
}
```

`AnimalContext` is the enriched animal (intake, species, last weight,
days-in-care) already assembled via `EnrichAnimalsOptimized`. Registration is
append-only Go code; the admin UI builds its field dropdown **from the
registry**, so new structured condition data becomes matchable the moment its
provider ships. Versioned: if a provider is renamed/removed, matchers
referencing the old key fail closed (no match) and the matcher library
flags them as "broken" for repair.

**Initial registry entries:**

| Field key | Source | Type | Ops |
|---|---|---|---|
| `species` | animals.species | string | eq, neq, in, regex, contains |
| `species_class` / `species_order` / `species_family` / `species_agw_group` / `species_subside_group` / `species_native_status` | species resolved by name | string | eq, in, regex |
| `species_game` / `species_huntable` | species | bool | eq |
| `animal_type` | animaltypes.name (or ID) | string | eq, in |
| `animal_age` | animalages.name (bébé/juvénile/adulte) | string | eq, in |
| `gender` | animals.gender | string | eq, in |
| `zone` / `cage` | animals | string | eq, in, regex |
| `weight_g` | `LastWeight()` | number g | lt, lte, gt, gte (+ null-safe: no weight → no match, shown in explain) |
| `days_in_care` | now − intakeDate | number days | lt, lte, gt, gte |
| `has_parasites` / `has_wounds` | intakes | bool | eq |
| `parasites` / `wounds` / `intake_general` / `intake_remarks` / `feeding` | intakes / animals | text | regex, contains |
| `force_feed` | animals | bool | eq |
| `vet_diagnostic` | latest veterinaryvisit.diagnostic | text | regex, contains |

**Reserved future entries** (ship when parasites/wounds get structured
definitions — naming chosen now so migration is additive):
`parasite_type` (string, eq/in), `parasite_load` (string/enum, eq/in),
`wound_type`, `wound_location`, `wound_severity` (eq/in/gte on ordered
severity), `wound_open` (bool, eq). These will resolve from the new tables
and can coexist with the current free-text `parasites`/`wounds` regex
matchers (which stay valid for historical intakes).

`regex` = Go RE2 (no backtracking → safe against ReDoS on user-supplied
patterns); pattern validated at rule save, case-insensitive flag `(?i)`
documented for admins.

### 5.2 Matcher DSL — a matcher *is* a query

Stored as text in `care_matchers.expression`, parsed to an AST, evaluated in
memory. **Never translated to SQL** → no injection surface, and evaluation
works on the already-enriched animal context.

Grammar (EBNF):

```
expression := or_expr
or_expr    := and_expr ( OR and_expr )*
and_expr   := unary    ( AND unary )*
unary      := NOT unary | "(" expression ")" | predicate
predicate  := field cmp literal
            | field BETWEEN number AND number
            | field IN "(" literal ("," literal)* ")"
            | field "~"  string        -- regex match   (RE2)
            | field "!~" string        -- regex no-match (RE2)
cmp        := "=" | "!=" | "<" | "<=" | ">" | ">="
literal    := string | number | true | false
field      := [a-z_][a-z0-9_]*         -- registry key (§5.1)
```

- Keywords case-insensitive (`AND`/`and`); strings double-quoted with `\"`
  escapes; `BETWEEN x AND y` consumes its own `AND` (parser rule, SQL-style).
- Precedence: `NOT` > `AND` > `OR`; parentheses override.
- Save-time validation: parse, then semantic check — every field must exist
  in the registry, the op must be declared by that field's provider, the
  literal type must match the field type. Errors reported with token
  position (`weight_g < "abc"` → type error at col 12) for inline UI display.
- Runtime semantics: missing value (e.g. no weight recorded) → predicate
  false + trace reason *"no value"* — fail closed. Regex compiled once at
  parse time.
- Implementation: hand-rolled recursive-descent parser
  (`models/careplan/dsl.go`, ~250 LoC, no new dependency); AST cached by
  expression hash.

Examples:

```
animal_type = "Hérissons / Insectivore" AND animal_age = "bébé" AND weight_g < 300
has_parasites = true AND parasites ~ "(?i)tiques"
animal_type = "Colombidés" OR species IN ("Pigeon biset", "Tourterelle turque", "Ramier")
weight_g BETWEEN 300 AND 800 AND NOT force_feed = true
days_in_care >= 7 AND (has_wounds = true OR wounds ~ "(?i)plaie")
```

**Preview is part of the matcher contract**: the evaluator exposes
`Preview(matcher, limit)` → matching animals + per-predicate trace, used by
the matcher library "test" panel and the rule editor live preview (§7).
Every query is checkable against the current in-care population before it
ever drives a plan.

### 5.3 Evaluation

- Load in-care animals (223 in prod) **once**, enriched (`EnrichAnimalsOptimized`
  already provides intake/species/last weight in bulk — reuse it; no N+1).
- Load active rules + their matchers once (tens of rows); expressions parsed
  once, AST cached.
- For each rule: `match = Eval(matcher.AST, animalContext)`; `matcher_id`
  NULL → match.
- Pure Go, no SQL-per-rule; trivially fast. Same code path serves:
  - the day-plan view (all animals),
  - the animal page ("rules matching this animal" + explanation),
  - the **matcher/rule preview** ("these 37 animals currently match").

### 5.4 Dynamic membership

Matching is evaluated at render/fulfillment time, not stored: an animal
enters a rule automatically (age flip juvénile→adulte, weight crossing a
threshold, new parasite note) and leaves automatically (outtake, condition
change). This is the core "expert system" behavior: **the plan follows the
animal's condition**.

**Course latch (§10-A4)** — the exception that protects bounded treatments.
Live membership is correct for open-ended rules (gavage ends when weight
crosses the threshold). But a **bounded course** (`duration_days` set) must
not evaporate mid-treatment when the condition is edited (e.g. tick text
removed on day 3 of a 5-day Ivomec course). So: a rule with
`latch_membership=true` **and** `duration_days` set keeps an animal
generating occurrences **once it has ≥1 application**, until the course's
last occurrence day — even if the matcher now fails. Default
`latch_membership=false` preserves pure live membership; open-ended rules
(`duration_days` NULL) ignore the flag entirely. The per-rule flag lets the
admin choose which behavior a given bounded rule wants. Latched animals are
shown in the why-explanation as *"in course (latched) — no longer matches"*.

### 5.5 Traceability ("why")

AST evaluation returns, per animal × rule, the per-predicate trace (field,
op, expected, actual, pass, reason). Rendered in:
- animal page → "why this rule applies",
- matcher library test panel + rule editor preview → "why these animals match",
- day plan item tooltip → origin rule name.

---

## 6. Planning & Tracking

### 6.1 Occurrence generation

For window `[window_start, window_end]`, from **both** planning levels
(rules and animal plans feed the same generator):

0. **Gather** — active rules + their matchers + exclusions, **and active
   animal plans**, in a handful of queries (tens of rows each).
1. **Generate per source** — for each animal plan: occurrences for its single
   animal; for each rule: for each matching animal over the window:
   a. Determine occurrence days: anchor + offset, step `every_days`, filter
      `weekdays`, clamp to `valid_from/to` and `duration_days`, and to
      `[intakeDate, outtakeDate)`.
   b. Expand each day × `times[]` → `due_at` instants (local time).
2. **Resolve overrides** — for each animal, a rule occurrence of a given
   `action_kind` is marked `overridden` (with the plan name) instead of
   actionable when an active animal-plan of the **same kind** either (a) has
   `replaces_kind=true`, or (b) has a colliding occurrence `(time slot ±
   grace)` (§4.7, §10-A3). Different-kind collisions are kept untouched.
3. **Load applications** for `(source, animal, due_at ∈ window)` in one query.
4. **Join** → plan items with computed status:

| Status | Condition |
|---|---|
| `scheduled` | due in future |
| `due` | window_start ≤ due ≤ now + lookahead (default 60 min, §10-CP6a) |
| `late` | now > due + grace_minutes, no application |
| `missing` | now > due + miss_after_hours, no application |
| `applied` | application exists (status=applied) |
| `skipped` | application exists (status=skipped) |
| `deferred` | application exists (status=deferred, `deferred_until` in future) — snoozed, resurfaces at `deferred_until` (§10-A2) |
| `overridden` | rule occurrence suppressed by an animal-plan occurrence of the same kind (§4.7) — displayed greyed, not actionable |

Late/missing come **for free** from virtual occurrences — no cron, no status
sweeper. Window default: `[today 00:00 − 24h, today + 2 days]` (shows
yesterday's misses + today + tomorrow), configurable via query params
(`?from=&to=`), hard cap 14 days.

**Apply window (§10-A1)** — `missing` is an urgency badge, **not** a permanent
lock, but it is bounded: an occurrence stays **applicable only until the next
occurrence of the same source becomes due**. Concretely, for a source with
`times[]` slots, occurrence *i* is applicable while `now < due_at(i+1)` (or,
for the last slot of the source in the window, while it remains in the display
window). After that the item becomes read-only history — the caregiver acts on
the *current* occurrence instead. This prevents stacking multiple stale
"make-up" applications for the same recurring duty while still letting a
just-missed slot be completed late.

**Defer expiry (§10-H1)** — a defer is only a *snooze*, never a terminal
state: when `now ≥ deferred_until`, the defer row no longer affects status —
the occurrence's status is **recomputed from `due_at`** as if no application
existed (so it resurfaces as `scheduled`, `due`, `late` or `missing`
depending on `now`). The defer row is kept for audit and still blocks the
UNIQUE key. Since CP4 clamps `deferred_until` to before the next occurrence,
a resurfaced item is always still applicable. Items whose apply window has
closed (above) are rendered **without action buttons**, with a 🔒 *hors
délai* badge (§7.2) — visible history, never actionable (§10-H2).

### 6.2 Fulfillment (apply)

`POST /care_plan/{item}/apply` with per-kind fields (§10-L1): **weight is
required for `weighing`, an answer is required for `observation`**;
note/clean/etc. remain optional:

1. Re-verify the source still produces the occurrence and it is un-applied
   (transaction + UNIQUE key as backstop → two-tab double-click safe). For a
   rule occurrence, re-check the matcher; for an animal-plan occurrence,
   re-check the plan is still active. On failure, refuse with the rendered
   **why-trace** (« règle ne s'applique plus : poids 310 g ≥ 300 g », §5.5)
   instead of a bare error (§10-L4).
2. Create the real record per action kind. **The created record always uses
   click time (`now`) as its `date` (§10.1)** — `due_at` lives only on the
   application row for adherence stats, so the care/treatment log reflects
   when the work actually happened:
   - feeding/care/cleanup/weighing → **one `cares` row** (`date=now`;
     cleanup sets `clean=1`, weighing stores weight),
   - observation → **one `cares` row** with the answer recorded in `note`;
     **if the answer is the alert outcome (`alert_on`, §4.2), also write a
     second `cares` row of the Warning caretype** (`caretypes.warning=1`,
     "Alerte") in the same transaction, turning the animal's row red until a
     "Réponse alerte" (`reset_warning=1`) care clears it (§10.6), **and
     auto-create a single-occurrence follow-up observation animal-plan**
     (name `"Vérifier alerte: <prompt>"`, one occurrence at
     now + `alert_follow_up_hours`, `created_by` = applying user,
     self-deactivates after its occurrence is applied/skipped) so the alert
     loop always closes (§10-CP3). **Follow-up resolution (§10-M3)**: applying
     a *"Vérifier alerte"* occurrence with a non-alert answer also writes a
     **ResetWarning ("Réponse alerte") care** in the same transaction — the
     red state clears and the loop closes. An alert answer on the follow-up
     writes a new Warning care and chains a new follow-up (keep watching);
     skipping it leaves the Warning in place — the landing red row is the
     backstop. Chaining stops at outtake (existing clamp),
   - medication → **one `treatments` row** for that date with the single
     slot marked done in `timedonebitmap` (reuses existing treatment
     rendering/stats on the animal page). **Slot mapping (§10-M1)**: the
     occurrence's due time maps to a bitmap bucket — **<11:00 → morning(1),
     11:00–15:00 → noon(2), >15:00 → evening(4)** (aligned with the legacy
     08:00/12:00/18:00 anchors). Same-bucket collision: the bit is set by the
     first apply; later same-bucket applies append to the treatment row's
     `note` (both application rows are still recorded). Keep medication rules
     to **≤1 slot per bucket (≤3/day)** — the rule editor warns otherwise
     (§7.1); the v2 treatment-model change lifts this ceiling.
3. Insert `care_plan_applications` row (`source_type`, `source_id`, source
   snapshot).
4. Redirect back to plan/animal page; item flips to applied.

**Skip**: same endpoint with `status=skipped` + mandatory reason; no
fulfillment record. **Defer**: same endpoint with `status=deferred` +
mandatory reason + `deferred_until` **clamped to before the next occurrence
of the same source** (§10-A2/CP4); no fulfillment record. The defer dialog
defaults `deferred_until` to **+1 h** (editable, still clamped) (§10-L3).

**Batch apply (§10.5 — in v1, no kind restrictions per §10-CP2)**: `POST
/care_plan/apply_batch` accepts a list of item keys (e.g. all currently-due
items of one cleanup/feeding rule, or a cage-group feeding like *"POUR LES 8
RENARDS"*). A **confirmation screen lists the N animals first**, with per-row
opt-out — for medication items it shows **each animal's resolved dosage**
(weight × dosages table) so per-animal dosing stays visible even in batch.
The handler then applies the **same per-item transaction in a loop**
(re-verify → create record → insert application), collecting per-item
results; items that fail re-verification or the UNIQUE backstop are reported
as already-done, not errors. One click → N care/treatment rows + N
applications, each independently idempotent.

This is why "tracked at animal level": every application materializes as a
**standard care/treatment record** → appears in the animal's existing care
tab, treatment list, PDF/exports, plus the application row keeps the
source-occurrence link for the plan.

### 6.3 Subsequent planning

Because applications carry `due_at` and link to fulfillment, planning can
answer: last application per source per animal (for "next due" display),
adherence rate, streaks of misses → surfacing animals at risk (dashboard
badge).

**Adherence definition (§10-CP5)**: `adherence = applied / (applied + late +
missing)` — **skipped, deferred and overridden occurrences are excluded from
the denominator** (they are conscious decisions, not failures), and
**applied-late is tracked as its own metric** (`applied_at > due_at +
grace_minutes`). This keeps the score defensible: a caretaker who correctly
skips a stressed animal's weighing is not penalized, and the badge measures
what it claims to measure.

---

## 7. UI / UX

**Rule setup ease is a key requirement.** An admin must be able to create a
correct rule in under a minute **without knowing the DSL**. The DSL is the
storage/exchange format; the UI is a guided builder that compiles to it, and
every query is previewable before it drives a plan.

### 7.1 Rule setup principles

1. **Start from presets** — migrations seed a starter library (matchers:
   *bébé / juvénile / adulte*, per animal type, *parasites présents*,
   *blessés*; rules: R1–R5 equivalents incl. daily cleanup). Admin duplicates
   & adjusts instead of starting blank.
2. **Visual matcher builder (default)** — rows of *field → op → value* with
   AND/OR grouping; field dropdown comes **from the registry** (i18n
   labels); op dropdown filtered by field type; value widget typed: Select2
   multi-select for species (494 entries), pickers for caretype/drug,
   number+unit, bool toggle. The builder compiles **live** to the DSL text
   (shown read-only below). "Advanced" toggle switches to raw DSL editing
   with inline validation; switching back re-parses into the builder when
   the expression is representable.
3. **Live preview everywhere** — every edit re-evaluates the query against
   the current in-care animals: match count + first N animals + per-animal
   "why" (pass/fail per predicate, §5.5). Same preview component in the
   matcher library ("test this matcher") and in the rule editor. Saving a
   rule is impossible without having seen the preview — the panel is part
   of the form, not a separate page.
4. **Readable everywhere** — rules list, plan tooltips and animal tab render
   the matcher as a pretty-printed i18n sentence (« type = Hérissons /
   Insectivore ET âge = bébé ET poids < 300 g »), never raw DSL unless
   asked.
5. **Reuse** — matcher library shared across rules (edit once → all
   referencing rules updated); matcher and rule duplication buttons.
6. **Guardrails** — explicit badge "matches **all** N animals" when the rule
   has no matcher; `active` toggle separate from save (preview first,
   activate later); save blocked with token-level error pointers on invalid
   expressions; broken matchers (registry drift) flagged in the library, not
   on the plan; **save warns (but allows) when any `times[]` slot falls
   outside 07:00–23:00** — the observed care-activity window (§2.3) — so a
   typo'd `03:00` can't silently generate daily misses (§10-CP6b).

### 7.2 Pages

Admin navigation: **Care plans** group added next to existing Cares/Treatments/Feeding.

| Page | Route | Audience | Content |
|---|---|---|---|
| **Day plan** | `/care_plan` (also replaces/augments `/feeding`) | caretakers | **"What should I do next"** — primary work surface. Two view modes: **① Action-first** (default): items sorted by urgency — overdue/missing first, then due-now, then upcoming — each item is an actionable card with **Apply**/**Defer**/**Skip** one-click buttons, animal name + cage, action description, and status badge. **② Timeline**: chronological trace grouped by zone (existing `AnimalByZoneMap` convention) then time — for review and shift handover. Status badges (due/late/missing/applied/skipped/deferred); filters by action kind, zone, animal; counts header. **Auto-refresh ~60 s with a visible "updated at HH:MM" indicator** (§10-CP6c) — several caretakers work the same list during the 09:00–12:00 peak; refresh prevents double-work that the UNIQUE constraint would otherwise only catch after the fact. |
| **Animal plan tab** | `/animals/{id}` new tab "Plan" | caretakers | **Day glimpse** — treatment-grid-style per-animal day view: today's occurrences (rules + animal plans merged) as a time grid with apply/skip checkboxes, like the existing treatment schedule; plus: animal's own **animal plans** with **create/edit inline** (caretaker-level, no admin rights — action form + schedule chips, no matcher section); active rules for this animal + *why they match* (incl. overridden/excluded states); upcoming occurrences; history of applications (linked care/treatment rows). |
| **Animal plan editor** | inline in the Plan tab (modal/partial), `POST/PUT /animals/{id}/care_animal_plans` | caretakers | One-animal schedule creation: name, action kind + payload (same sub-forms as rule editor ①), schedule (same widget as rule editor ②). No matcher, no preview panel — trivially scoped to the animal. "Promote to rule" button (admin only). |
| **Rules list** | `/care_rules` | admin | Cards/table: name, kind, schedule summary ("5×/day, daily"), matcher as **readable sentence** (link to matcher), live match count, active toggle, duplicate button. |
| **Rule editor** | `/care_rules/new`, `/care_rules/{id}/edit` | admin | 3 sections: ① Action (kind-select swaps payload sub-form, Select2 for caretype/drug, dosage helper pulling from `dosages` table) ② Schedule (times chip-input, every-days, weekdays, offsets/duration, grace) ③ Matcher — pick existing from library (dropdown shows pretty-printed expression) **or** create inline via visual builder. **Live preview panel**: matching animals + one-line explanation each. |
| **Matcher library** | `/care_matchers` | admin | Named queries: pretty-printed expression, live match count, "used by N rules" usage list, broken-matcher flags, duplicate. Row expands to the **test panel** (full preview vs current animals). |
| **Matcher editor** | `/care_matchers/new`, `/care_matchers/{id}/edit` | admin | Visual builder (default) ⇄ raw DSL (advanced toggle); inline token-level validation; live preview; "used by" warning before editing a shared matcher. |
| **Rule show** | `/care_rules/{id}` | admin | Full definition, matching animals, recent applications, adherence stats. |

**Record links & the consultation view (§7.2a)** — every place the plan
references a concrete record renders it as a **hyperlink to that record**,
not bare text. Applied items link their fulfillment: a `care` fulfillment →
`/cares/{fulfillment_id}?back=<plan-url>`, a `treatment` fulfillment →
`/treatments/{fulfillment_id}?back=<plan-url>`. This applies to the day-plan
item tooltip, the animal Plan tab's *Applications récentes* list
(`soin #15234`, `traitement #8842` in §7.3.3 are links), the rule show page's
recent applications, and any "already applied by X at HH:MM" flash (links the
winning application). These are the existing `Show` pages — **reused as
read-only "consultation" views**, not the edit forms: the plan never drops
the user into an editing context, it only consults. Both show templates
already honor a `back` param, so the plan passes its own URL and the user
returns to the exact scroll position.

One gap vs. today's show pages: they currently render **Edit** to everyone
and **Delete** to admins. For consultation from the plan this is wrong for
restricted roles (§10-A5) — and `spw` is not even whitelisted for `/cares/*`
in `roleAllows`. So: the show templates gain a small role gate — Edit/Delete
buttons render only for users who may actually mutate (regular + admin), and
`roleAllows` is extended to let `spw` GET `/cares/{id}` + `/treatments/{id}`
(read-only consult consistent with its animal-tab access). No separate
"consultation" template is needed — the existing show page *is* the
consultation view once its action buttons are role-gated; the plan always
links to it in read-only intent (`back` set, no edit affordance promised).

**Reuse mechanics**: named matcher library (§4.6, edit once → all rules
updated); rule & matcher duplication; matcher on
`species_agw_group`/`family`/`animal_type` for broad groups; action kinds
reference shared `caretypes`/`drugs` reference data.

**Feeds/integrations**:
- Landing page: per-animal badge "N due / M late" (single grouped query over
  plan service — watch performance, reuse `EnrichAnimalsOptimized` pass).
- Existing `/feeding` page: keep during migration (§8), add banner linking to
  `/care_plan?kind=feeding`.
- Existing treatment schedule (`/treatmentschedule`) remains editable during
  migration; the animal Plan tab's day glimpse is the same data plus rule
  occurrences, and becomes the canonical per-animal day view in Phase 3.

**I18n**: all new UI strings in `locales/*.yaml` for **en-US, fr, de, nl**
(workspace all-language rule); templates via the standard `t()` helper.
**Note**: this project **does** use localized template forks (167 `.plush.de.html`
/ `.fr.html` / `.nl.html` files) — new pages must follow the same convention.
Action kind labels i18n-keyed (`care_expert.kind.feeding`…).

### 7.3 UI Examples — Every Screen with Rules

All examples below use the **same seed rule set** (§7.4) so the reader can
trace one rule's effect across every screen.

#### 7.3.1 Day plan — action-first view (default, 09:40)

The default view answers **"what should I do next?"** — most urgent items
first, one-click apply. Grouped by urgency tier, not by time.

```
┌──────────────────────────────────────────────────────────────────────┐
│  Plan du jour — vendredi 20 septembre          [Vue: Actions|Timeline]│
│  🔴 5 manqués · 🟡 22 en retard · ⚪ 41 à faire · ✅ 96 faits        │
│  Filtres: [tous types ▾] [toutes zones ▾] [statut ▾]                │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  🔴 MANQUÉ (5) ─────────────────────────────────────────────────     │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ 🦔 10472 · Hérisson (A12)    Gavage — Croquettes + 4 VDF     │  │
│  │    Prévu hier 19:00 · R1 Hérisson bébé — gavage 5x/j         │  │
│  │    [✅ Fait] [⏭ Reporter] [🚫 Ignorer]                          │  │
│  ├────────────────────────────────────────────────────────────────┤  │
│  │ 🧹 10502 · Merle (B07)       Nettoyage cage                   │  │
│  │    Prévu hier 09:00 · R-Nettoyage quotidien                   │  │
│  │    [✅ Fait] [⏭ Reporter] [🚫 Ignorer]                          │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  🟡 EN RETARD (22) ────────────────────────────────────────────      │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ 🐦 10455 · Pigeon biset (Volière 1)  Grains pigeons eau       │  │
│  │    Prévu 08:30 · R3 Colombidés — grains 2x/j · 70 min retard  │  │
│  │    [✅ Fait] [⏭ Reporter] [🚫 Ignorer]                          │  │
│  ├────────────────────────────────────────────────────────────────┤  │
│  │ 💊 10398 · Renard (Enclos 2)   Ivomec 1% (SC) — 0.08 ml      │  │
│  │    Prévu 08:00 · R2 Tiques → Ivomec 5 jours · jour 3/5       │  │
│  │    [✅ Fait] [⏭ Reporter] [🚫 Ignorer]                          │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ⚪ À FAIRE MAINTENANT (8) ────────────────────────────────────      │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ 🦔 10472 · Hérisson (A12)    Gavage — Croquettes + 4 VDF     │  │
│  │    Prévu 09:30 · R1 Hérisson bébé — gavage 5x/j              │  │
│  │    [✅ Fait] [⏭ Reporter] [🚫 Ignorer]                          │  │
│  ├────────────────────────────────────────────────────────────────┤  │
│  │ 🧹 Tous les animaux (223)    Nettoyage cage                   │  │
│  │    Prévu 09:00 · R-Nettoyage quotidien · 198 ✅ · 21 🟡 · 4 🔴│  │
│  │    [Voir par animal ▸]                                        │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ⏳ À VENIR AUJOURD'HUI (33) ──────────────────────────────────      │
│  10:00  Pesée juvéniles (5 animaux) · R5                            │
│  12:30  🦔 10472 Gavage · R1                                       │
│  15:30  🦔 10472 Gavage · R1                                       │
│  17:00  🐦 Colombidés grains PM (106 animaux) · R3                  │
│  17:00  🐦 10420 Buse nourrissage · plan animal P1                  │
│  18:00  💊 Médications soir (12 animaux) · divers                  │
│  19:00  🦔 10472 Gavage · R1                                       │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Design notes**: The action-first view follows the same principle as the
existing `/feeding` page (`calculateFeeding` → `NextFeedingCode` 0–3
criticality) and the landing page treatment column (morning/noon/evening
dots colored by status). The plan service computes the same urgency
classification from occurrence status + time distance, so the UI sorts
once and renders.

#### 7.3.2 Day plan — timeline view (same data, chronological)

Same data as §11.3 but generated from the same seed rule set. Grouped by
zone (existing convention), sorted by `due_at`. This is the shift-handover
and review view.

#### 7.3.3 Animal page — Plan tab (Buse 10420)

```
┌──────────────────────────────────────────────────────────────────┐
│  10420 · Buse variable ♀ · juvénile · Enclos 2 · C03            │
│  [Infos] [Soins] [Traitements] [Plan] [Historique]               │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  AUJOURD'HUI — vendredi 20 septembre                             │
│  ┌────────┬──────────────────────────────┬──────────┬─────────┐  │
│  │ Heure  │ Action                       │ Source   │ Statut  │  │
│  ├────────┼──────────────────────────────┼──────────┼─────────┤  │
│  │ 08:00  │ Mélange viande 80 g          │ P1 (plan)│ ✅ 08:12│  │
│  │ 08:00  │ ~~Poussins 1x/j~~            │ R6 (règle│ ⃠ remplacé│ │
│  │ 08:00  │ Meloxicam 0.1 ml             │ P2 (plan)│ ✅ 08:15│  │
│  │ 12:00  │ Meloxicam 0.1 ml             │ P2 (plan)│ ⚪ à faire│ │
│  │ 17:00  │ Mélange viande 80 g          │ P1 (plan)│ ⚪ à faire│ │
│  │ 20:00  │ Meloxicam 0.1 ml             │ P2 (plan)│ ⏳ à venir│ │
│  └────────┴──────────────────────────────┴──────────┴─────────┘  │
│                                                                  │
│  PLANS DE CET ANIMAL                              [+ Nouveau plan]│
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │ P1 · Nourrissage 2x/j · 08:00, 17:00 · actif · par julie   │ │
│  │ P2 · Meloxicam 3x/j · 08:00, 12:00, 20:00 · actif · par julie│ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  RÈGLES ACTIVES POUR CET ANIMAL                                  │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │ R6 Rapaces — nourrissage standard · type = Rapaces          │ │
│  │    ⃠ remplacée par plan animal P1 (même type, replaces_kind) │ │
│  │ R-Nettoyage Nettoyage quotidien · tous les animaux           │ │
│  │    ✅ appliquée aujourd'hui 09:05 par admin                  │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  APPLICATIONS RÉCENTES                                           │
│  08:15  Meloxicam 0.1 ml · par julie · traitement #8842          │
│  08:12  Mélange viande 80 g · par julie · soin #15234            │
│  09:05  Nettoyage cage · par admin · soin #15201                 │
└──────────────────────────────────────────────────────────────────┘
```

#### 7.3.4 Rules list (`/care_rules`)

```
┌──────────────────────────────────────────────────────────────────┐
│  Règles de soins                                [+ Nouvelle règle]│
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │ 🦔 Hérisson bébé — gavage 5x/j                     [actif]  │ │
│  │    Nourrissage · 5×/jour · 07:00→19:00 · 6 animaux          │ │
│  │    Matcher: « type = Hérissons ET âge = bébé ET poids < 300 g»│ │
│  │    [✏️] [📋 Dupliquer] [⏸ Désactiver]                        │ │
│  ├──────────────────────────────────────────────────────────────┤ │
│  │ 💊 Tiques → Ivomec 5 jours                         [actif]  │ │
│  │    Médication · 1×/jour 08:00 · 5 jours · 4 animaux         │ │
│  │    Matcher: « parasites ET texte ~ "tiques" »               │ │
│  │    [✏️] [📋 Dupliquer] [⏸ Désactiver]                        │ │
│  ├──────────────────────────────────────────────────────────────┤ │
│  │ 🐦 Colombidés — grains 2x/j                        [actif]  │ │
│  │    Nourrissage · 2×/jour · 08:30, 17:00 · 119 animaux       │ │
│  │    Matcher: « type = Colombidés OU espèce ∈ (3 espèces) »   │ │
│  │    [✏️] [📋 Dupliquer] [⏸ Désactiver]                        │ │
│  ├──────────────────────────────────────────────────────────────┤ │
│  │ 🧹 Nettoyage quotidien                             [actif]  │ │
│  │    Nettoyage · 1×/jour 09:00 · ⚠️ correspond à TOUS (223)   │ │
│  │    Pas de matcher — s'applique à tous les animaux            │ │
│  │    [✏️] [📋 Dupliquer] [⏸ Désactiver]                        │ │
│  ├──────────────────────────────────────────────────────────────┤ │
│  │ ⚖️ Pesée hebdo juvéniles                           [actif]  │ │
│  │    Pesée · 1×/7j · 10:00 · 156 animaux                      │ │
│  │    Matcher: « âge = juvénile »                               │ │
│  │    [✏️] [📋 Dupliquer] [⏸ Désactiver]                        │ │
│  ├──────────────────────────────────────────────────────────────┤ │
│  │ 🐦 Rapaces — nourrissage standard                  [actif]  │ │
│  │    Nourrissage · 1×/jour 08:00 · 20 animaux                  │ │
│  │    Matcher: « type = Rapaces »                               │ │
│  │    [✏️] [📋 Dupliquer] [⏸ Désactiver]                        │ │
│  └──────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
```

#### 7.3.5 Rule editor (`/care_rules/new` — creating R2)

```
┌──────────────────────────────────────────────────────────────────┐
│  Nouvelle règle                                                  │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ① ACTION                                                        │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │ Nom:    [Tiques → Ivomec 5 jours                           ] │ │
│  │ Type:   [💊 Médication ▾]                                   │ │
│  │                                                              │ │
│  │ Médicament: [Ivomec 1%  (SC) ▾]  ← Select2, from drugs     │ │
│  │ Dosage:     [☑ Calculer depuis la table dosages]            │ │
│  │             → résout 0.1 ml/100g × poids animal              │ │
│  │ Remarques:  [Contrôler tiques restantes au jour 5         ]  │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ② HORAIRE                                                       │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │ Heures:  [08:00] [+ ajouter]                                │ │
│  │ Répéter: tous les [1] jour(s)                               │ │
│  │ Ancre:   [admission ▾]   Début: [+0] jours                  │ │
│  │ Durée:   [5] jours  ← course bornée                         │ │
│  │ Grâce:   [60] min    Manqué après: [24] h                   │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ③ MATCHER                                                       │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │ Source: [Tiques à l'admission ▾] ← bibliothèque existante   │ │
│  │         ou  [+ Créer un nouveau matcher]                     │ │
│  │                                                              │ │
│  │ Expression: has_parasites = true AND parasites ~ "(?i)tiques"│ │
│  │             (lecture seule — éditer dans la bibliothèque)    │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌─ APERÇU EN DIRECT ──────────────────────────────────────────┐ │
│  │  4 animaux correspondent actuellement :                      │ │
│  │  ✓ 10398 · Renard · « puces ++++ - tiques » → regex ✓      │ │
│  │  ✓ 10401 · Hérisson · « tiques, puces » → regex ✓          │ │
│  │  ✓ 10415 · Buse · « Retirer une quinzaine de tiques » → ✓  │ │
│  │  ✓ 10422 · Merle · « Myiases - Tique » → regex ✓           │ │
│  │                                                              │ │
│  │  ✗ 10405 · Hérisson · « Puces +++ » → pas de « tiques »    │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  [💾 Enregistrer]  [💾+ Activer]  [Annuler]                      │
└──────────────────────────────────────────────────────────────────┘
```

#### 7.3.6 Matcher library (`/care_matchers`)

```
┌──────────────────────────────────────────────────────────────────┐
│  Bibliothèque de matchers                    [+ Nouveau matcher] │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │ Hérisson bébé < 300 g                              M1       │ │
│  │ « type = Hérissons / Insectivore ET âge = bébé ET           │ │
│  │   poids < 300 g »                                            │ │
│  │ 6 animaux · utilisé par 2 règles (R1 gavage, R1b pesée)     │ │
│  │ [▸ Tester] [✏️] [📋 Dupliquer]                               │ │
│  ├──────────────────────────────────────────────────────────────┤ │
│  │ Tiques à l'admission                               M2       │ │
│  │ « parasites = oui ET texte parasites ~ "(?i)tiques" »       │ │
│  │ 4 animaux · utilisé par 1 règle (R2 Ivomec)                  │ │
│  │ [▸ Tester] [✏️] [📋 Dupliquer]                               │ │
│  ├──────────────────────────────────────────────────────────────┤ │
│  │ Colombidés (type ou liste)                         M3       │ │
│  │ « type = Colombidés OU espèce ∈ (Pigeon biset,              │ │
│  │   Tourterelle turque, Ramier) »                              │ │
│  │ 119 animaux · utilisé par 1 règle (R3 grains)               │ │
│  │ [▸ Tester] [✏️] [📋 Dupliquer]                               │ │
│  ├──────────────────────────────────────────────────────────────┤ │
│  │ Juvénile                                           M4       │ │
│  │ « âge = juvénile »                                           │ │
│  │ 156 animaux · utilisé par 1 règle (R5 pesée)                │ │
│  │ [▸ Tester] [✏️] [📋 Dupliquer]                               │ │
│  ├──────────────────────────────────────────────────────────────┤ │
│  │ Rapaces                                            M5       │ │
│  │ « type = Rapaces »                                           │ │
│  │ 20 animaux · utilisé par 1 règle (R6 nourrissage)           │ │
│  │ [▸ Tester] [✏️] [📋 Dupliquer]                               │ │
│  └──────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
```

#### 7.3.7 Matcher editor — visual builder (`/care_matchers/new`)

```
┌──────────────────────────────────────────────────────────────────┐
│  Nouveau matcher                                                 │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Nom:  [Hérisson bébé < 300 g                                  ] │
│                                                                  │
│  ┌─ CONSTRUCTEUR VISUEL ───────────────────────────────────────┐ │
│  │                                                              │ │
│  │  [ET ▾]                                                      │ │
│  │  ┌──────────────────────────────────────────────────────┐   │ │
│  │  │ Type d'animal  [est ▾]  [Hérissons / Insectivore ▾] │   │ │
│  │  │                                                     [x]│   │ │
│  │  ├──────────────────────────────────────────────────────┤   │ │
│  │  │ Âge            [est ▾]  [bébé ▾]                    │   │ │
│  │  │                                                     [x]│   │ │
│  │  ├──────────────────────────────────────────────────────┤   │ │
│  │  │ Poids (g)      [< ▾]    [300                       ] │   │ │
│  │  │                                                     [x]│   │ │
│  │  └──────────────────────────────────────────────────────┘   │ │
│  │  [+ Ajouter une condition]                                   │ │
│  │                                                              │ │
│  │  Expression générée (lecture seule):                         │ │
│  │  ┌──────────────────────────────────────────────────────┐   │ │
│  │  │ animal_type = "Hérissons / Insectivore" AND          │   │ │
│  │  │ animal_age = "bébé" AND weight_g < 300               │   │ │
│  │  └──────────────────────────────────────────────────────┘   │ │
│  │  [⚙ Mode avancé]                                             │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌─ APERÇU EN DIRECT ──────────────────────────────────────────┐ │
│  │  6 animaux correspondent :                                   │ │
│  │  ✓ 10472 · Hérisson · bébé · 240 g · il y a 3 jours        │ │
│  │  ✓ 10488 · Hérisson · bébé · 185 g · il y a 1 jour         │ │
│  │  ✓ 10491 · Hérisson · bébé · 290 g · il y a 5 jours        │ │
│  │  ✓ 10495 · Hérisson · bébé · 210 g · il y a 2 jours        │ │
│  │  ✓ 10499 · Hérisson · bébé · 275 g · il y a 4 jours        │ │
│  │  ✓ 10503 · Hérisson · bébé · 260 g · il y a 1 jour         │ │
│  │                                                              │ │
│  │  ✗ 10501 · Hérisson · juvénile · 610 g → âge ✗ poids ✗    │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  [💾 Enregistrer]  [Annuler]                                     │
└──────────────────────────────────────────────────────────────────┘
```

#### 7.3.8 Animal plan editor — inline in Plan tab (caretaker, no admin)

```
┌──────────────────────────────────────────────────────┐
│  Nouveau plan pour 10420 · Buse variable             │
├──────────────────────────────────────────────────────┤
│                                                      │
│  Nom:    [Buse 10420 — nourrissage 2x/j            ] │
│  Type:   [🍽 Nourrissage ▾]                          │
│                                                      │
│  Repas:  [mélange viande 80 g                      ] │
│  Gavage: [☑]                                        │
│                                                      │
│  Heures: [08:00] [17:00] [+ ajouter]                │
│  Répéter tous les [1] jour(s)                       │
│  Ancre:  [admission ▾]   Durée: [illimitée ▾]       │
│                                                      │
│  [💾 Enregistrer]  [Annuler]                         │
│                                                      │
│  ⓘ Ce plan s'applique uniquement à cet animal.       │
│    Un admin peut le promouvoir en règle générale.    │
└──────────────────────────────────────────────────────┘
```

---

### 7.4 Seed Rules — Derived from Production Data

The following seed rules are derived from actual patterns observed in the
production database (§2.3, September 2026). They ship as **inactive drafts**
for the admin to review and activate.

#### Seed matchers

| # | Name | Expression | Rationale (prod evidence) |
|---|---|---|---|
| SM1 | Hérisson bébé | `animal_type = "Hérissons / Insectivore" AND animal_age = "bébé"` | 6 bébés in care; hedgehogs are the #1 treatment consumer (1,436 animals historically) |
| SM2 | Hérisson juvénile | `animal_type = "Hérissons / Insectivore" AND animal_age = "juvénile"` | 31 juvéniles in care |
| SM3 | Hérisson adulte | `animal_type = "Hérissons / Insectivore" AND animal_age = "adulte"` | 19 adults in care |
| SM4 | Colombidés | `animal_type = "Colombidés"` | 119 animals in care (84 juv + 22 adu + 13 bébé) — largest population |
| SM5 | Rapaces | `animal_type = "Rapaces"` | 20 animals in care (11 adu + 6 bébé + 3 juv) |
| SM6 | Canidés | `animal_type = "Canidés"` | 11 animals in care (6 bébé + 4 juv + 1 adu) |
| SM7 | Tiques à l'admission | `has_parasites = true AND parasites ~ "(?i)tiques"` | 4 in-care animals match; regex catches "tiques", "Tiques", "tiques++" |
| SM8 | Puces à l'admission | `has_parasites = true AND parasites ~ "(?i)puces"` | 12 in-care animals match; most common parasite text |
| SM9 | Myiases à l'admission | `has_parasites = true AND parasites ~ "(?i)myiases\|mouches plates\|oeufs de mouche"` | 7 in-care animals; variants: "myiases", "mouches plates", "oeufs de mouche" |
| SM10 | Blessés | `has_wounds = true` | 85 in-care animals with wounds |
| SM11 | Bébé (tous types) | `animal_age = "bébé"` | 40 bébés across all types |
| SM12 | Juvénile (tous types) | `animal_age = "juvénile"` | 162 juvéniles — largest age group |
| SM13 | Gavage forcé | `force_feed = true` | 29 animals flagged force-feed (28 Colombidés + 1 Rapace) |

#### Seed rules

| # | Name | Kind | Matcher | Schedule | Payload | Prod evidence |
|---|---|---|---|---|---|---|
| SR1 | Hérisson bébé — gavage | feeding | SM1 + weight < 300g | 07:00, 09:30, 12:30, 15:30, 19:00 daily | Croquettes + 4 VDF, force_feed=true | feeding_period=600 for hedgehogs; peak care hours 08-10h and 17-18h |
| SR2 | Hérisson juvénile — repas 2x/j | feeding | SM2 | 09:00, 18:00 daily | Croquettes + 4 VDF + EAU | Most common hedgehog diet text (11 animals); feeding_period=240 (65 animals) maps to ~4h intervals |
| SR3 | Colombidés — grains AM/PM | feeding | SM4 | 08:30, 17:00 daily | grains pigeons eau | 26 animals share "grains pigeons eau" text; feeding_period=600 (101 animals); meal peaks at 08-09h and 17-18h |
| SR4 | Colombidés bébé — NB gavage | feeding | SM4 + SM11 | 08:00, 11:00, 14:00, 17:00 daily | NB 1/2 50ml, force_feed=true | "NB 1/2" × 13 animals at period=240; "Animal nourri 50ml" is the most common Repas note (2,505 + 172 + 115) |
| SR5 | Rapaces — nourrissage | feeding | SM5 | 08:00 daily | poussins/souris per diet | "5 POUSSINS 1 SOURIS" × 4 animals; feeding_period=600 or 720 |
| SR6 | Canidés bébé — nourrissage groupe | feeding | SM6 + SM11 | 08:00 daily | per diet text | "POUR LES 8 RENARDS" × 6 animals — group feeding note |
| SR7 | Tiques → Ivomec 5j | medication | SM7 | 08:00 daily × 5 days | Ivomec 1% (SC), dosage from dosages table | 1,273 hedgehogs historically treated with Ivomec; 4 current in-care animals match tiques |
| SR8 | Puces → Sarnacuran | medication | SM8 | 08:00 daily × 3 days | Sarnacuran spray | 14 current in-care treatments; 255 animals historically |
| SR9 | Nettoyage quotidien | cleanup | NULL (all) | 09:00 daily | Nettoyage cage + eau fraîche | Formalizes the `clean=1` landing heuristic; grace 180 min |
| SR10 | Pesée hebdo juvéniles | weighing | SM12 | 10:00 every 7 days | Pesée de contrôle | Hedgehog juvéniles have 45% weight recording rate; weighing is the most common non-meal care |
| SR11 | Blessés — contrôle quotidien | care | SM10 | 09:30 daily | Contrôle plaie + bandage | 85 in-care animals with wounds; "Remplacer bandage" × 268 hedgehogs historically |
| SR12 | Hérisson — Catosal + Réhydratation | medication | SM1 OR SM2 | 08:00 daily × 3 days | Catosal 10% + Réhydratation (SC) | 1,372 hedgehogs historically — the most common hedgehog treatment protocol |

---

## 8. Migration & Coexistence (replace path)

Phase-gated, reversible:

1. **Phase 1 — additive**: ship tables + rules + plan view; existing
   feeding/treatment flows untouched. Seed a few **default rules reproducing
   current implicit behaviors** for admins to copy:
   - *Daily cleanup* (matches all in-care, cleanup @ 09:00) — formalizes the
     `clean=1` landing heuristic.
   - *Feeding ×5* on animaltypes where feeding_period≈600 (times 07/09:30/12:30/15:30/19:00).
   - *Weigh weekly* on bébé.
2. **Phase 2 — replace feeding schedule**: for animals with
   `feeding_start/end/period`, a grift generates equivalent entries — either
   a **per-diet rule** (species+age matcher + times derived from
   window/period; identical `feeding` texts cluster well in prod, e.g.
   "grains pigeons eau" ×30) or, when the diet is unique to one animal, a
   **per-animal plan** (`care_animal_plans`, no matcher needed — the
   natural fit). Once `/care_plan` covers feeding, freeze `feeding_period`
   editing (read-only), keep columns (backfill source + exports).
3. **Phase 3 — replace treatment series**: `TreatmentTemplate` gains "create
   as rule" (from=first date, duration=series length, times from bitmap
   morning→08:00/noon→12:00/evening→18:00 mapping). Row-per-day treatments
   stay for history; new series go through rules. Medication plan items
   fulfill into `treatments` rows — **no rendering changes downstream**.
4. **Phase 4 (optional)**: retire `/feeding` page → redirect to
   `/care_plan?kind=feeding`.

Rollback at every phase: rules are additive rows; legacy paths untouched
until explicitly frozen.

---

## 9. Non-Functional

- **Performance**: evaluation in-memory over ≤ a few hundred in-care animals
  × tens of rules → sub-ms; applications query indexed by `(due_at,status)`.
  No N+1 (reuse `EnrichAnimalsOptimized`). No background jobs required —
  statuses derive at render time.
- **Concurrency**: UNIQUE `(source_type, source_id, animal_id, due_at)` +
  transaction (popmw already wraps requests) = idempotent apply.
- **Security**: all routes behind `Authorize`; rule/matcher CRUD admin-only
  (`role_guard` pattern as in `care_templates.go`). **Animal-plan CRUD and
  apply/skip/defer are restricted to regular users + admins (§10-A5)** — the
  restricted roles (`lecteur`/`scientifique`/`spw`) are already blocked from
  POSTs by `RoleGuard`/`roleAllows`, so they get **read-only** plan views
  (day plan, animal Plan tab, rules/matchers display) but cannot mutate.
  RE2 for regex safety; payload JSON validated server-side per kind.
- **Audit**: `source_snapshot` on every application; rules and animal plans
  can hook `animal_audits`-style logging later (not v1).
- **Webhook/console**: **no impact** — care entries are excluded from event
  payloads by design (AGENTS.md §Event Types); applications create
  cares/treatments which also emit nothing.
- **Testing**: model validation tests; DSL parser tests (grammar,
  precedence, type errors, token-position reporting) + evaluator tests
  (each op, null-safety, regex, fail-closed); builder⇄DSL round-trip tests;
  preview endpoint tests; occurrence generation tests (timezones incl. a
  **DST boundary** case, offsets, duration, outtake clamp, **day-1 intake
  anchoring** §10-B3); **course-latch** tests (§10-A4: latched animal keeps
  course after matcher fails); **override** tests (slot-level + `replaces_kind`
  kind-level, §10-A3); apply idempotency test (double submit → 1 application);
  **apply-window** test (occurrence not applicable once next occurrence is due,
  §10-A1); **defer** test (§10-A2); **batch apply** test (§10.5); **observation
  alert-on-no** writes a Warning caretype row (§10.6); handler tests per project
  convention; E2E via Chrome DevTools MCP per AGENTS.md checklist.
- **Locales**: en-US, fr, de, nl strings complete before UI ships.

---

## 10. Decisions — Open Questions Resolved (2026-09-21)

All open questions are answered; the decisions below are **normative** and are
cited inline throughout the spec as `§10-x`.

### 10.1 Original eight (spec §10 items 1–8)

1. **Record time = click time.** Cares/treatments created by apply use `now()`;
   `due_at` stays on the application row for adherence stats (§6.2, §6.3).
2. **Feeding defaults accepted**: 60 min grace, 24 h miss (§4.3, §6.1).
3. **Legacy slot mapping accepted**: morning=08:00 / noon=12:00 / evening=18:00
   for Phase-2 treatment conversion (§8).
4. **Weight matcher without any weight record → no-match** (fail closed); the
   *why* panel shows "no weight on record" (§5.1, §5.5).
5. **Multi-animal batch apply → v1** (§6.2 Batch apply; refined by CP2 below).
6. **Observation alert-on-no → v1** (§4.2 observation payload; refined by CP3).
7. **All four migration phases committed**, including eventually retiring
   `/feeding` (§8).
8. **Seed library built as proposed** (§7.4); adjusted after real-world use.

### 10.2 Review blockers (A = answers, B = assumptions confirmed)

- **A1 — Apply window**: an occurrence is applicable only until the next
  occurrence of the same source becomes due (§6.1).
- **A2 — Skip AND defer, both in v1**: *Ignorer* (waive, terminal) and
  *Reporter* (snooze, resurfaces) — see §4.5, §6.2, §7.2.
- **A3 — Override suppression modes**: default slot-level collision; animal
  plan with `replaces_kind=true` suppresses **all** rule occurrences of that
  kind for the animal (§4.7, §6.1).
- **A4 — Course latch**: per-rule `latch_membership` — bounded courses stay
  attached to their intake cohort after the first application until
  `duration_days` ends; open-ended rules stay live (§4.1, §5.4).
- **A5 — Authorization**: regular users + admins mutate (apply/skip/defer,
  animal-plan CRUD); `lecteur`/`scientifique`/`spw` get read-only plan views
  (§9, §7.2a role gates).
- **B1 — Timezone**: server `time.Local` is the canonical clock for slots and
  due computation; documented dependency + DST unit test (§4.3).
- **B3 — Day-1 anchoring**: day 1 = first slot strictly after the intake
  instant; `duration_days` counts generated occurrence days (§4.3).
- **B6 — Dosage resolution failure**: never blocks apply — the apply form
  warns and asks for a manual dosage (§4.2, §6.2).

### 10.3 Critical points & enhancements round (CP/EN dispositions)

- **CP1 — Un-apply is admin-only** (`POST /care_plan/{item}/unapply`, §4.5);
  deleting a fulfillment from the care/treatment side marks the application
  `fulfillment_deleted=true` (§4.5).
- **CP2 — No kind restrictions on batch apply**; the confirmation screen lists
  every animal with its resolved dosage (medication) and per-row opt-out
  (§6.2).
- **CP3 — Observation alert in full for v1**: alert answer creates the Warning
  caretype row **and** auto-creates a follow-up single-occurrence animal plan
  "Vérifier alerte: <prompt>" (§4.2, §6.2).
- **CP4 — Defer clamped + reason mandatory**: `deferred_until` clamped before
  the next occurrence; reasons required for skip **and** defer (§4.5, §6.2).
- **CP5 — Adherence definition**: `applied / (applied + late + missing)`;
  skipped/deferred/overridden excluded; applied-late is a separate metric
  (§6.3).
- **CP6 — All three UX guards adopted**: (a) `lookahead_minutes` default 60
  (§4.3, §6.1); (b) off-hours save warning for times outside 07:00–23:00
  (§7.1); (c) day-plan auto-refresh ~60 s with "updated at HH:MM" (§7.2).
- **Consultation view** (user request): applied items link to the existing
  record show pages reused as role-gated read-only views — see §7.2a.
- **EN dispositions**: **EN3 (payload `instructions`) pulled into v1** (§4.2);
  EN1, EN2, EN4–EN10 recorded in the v2 backlog — see **§13 Future
  Directions**.

---

## 11. Worked Examples

All examples use real production data observed in §2.3.

### 11.1 Example rules (as stored)

**R1 — Hedgehog baby force-feeding, 5×/day**

```jsonc
// care_rules
{
  "name": "Hérisson bébé — gavage 5x/j",
  "action_kind": "feeding",
  "action_payload": {
    "caretype_id": "<Repas uuid>",
    "food": "🦔 Croquettes + 4 VDF - Eau",
    "force_feed": true
  },
  "schedule": {
    "times": ["07:00", "09:30", "12:30", "15:30", "19:00"],  // replaces feeding_period=600 baseline
    "every_days": 1,
    "anchor": "intake",
    "grace_minutes": 45,
    "miss_after_hours": 12
  },
  "matcher_id": "<M1 uuid>",   // boolean logic lives in the matcher, not the rule
  "active": true
}
// care_matchers — M1 (named, reusable)
{ "name": "Hérisson bébé < 300 g",
  "expression": "animal_type = \"Hérissons / Insectivore\" AND animal_age = \"bébé\" AND weight_g < 300" }
```

M1 is **also referenced by a second rule** — **R1b** *"Hérisson bébé — pesée
quotidienne"* (weighing @ 10:00) — demonstrating library reuse: one
condition, two actions; tighten the threshold once and both rules follow.

Matches: animal 10472 — *Hérisson*, bébé, last weight 240 g, intake 3 days ago.
Stops matching automatically when weight ≥ 300 g or age flips to juvénile.

**R2 — Tick-positive intake → antiparasitic course (bounded)**

```jsonc
{
  "name": "Tiques → Ivomec 5 jours",
  "action_kind": "medication",
  "action_payload": {
    "drug": "Ivomec 1%  (SC)",                       // 2,084 uses in prod
    "dosage_from_dosages_table": true,
    "remarks": "Contrôler tiques restantes au jour 5"
  },
  "schedule": {
    "times": ["08:00"],
    "every_days": 1,
    "anchor": "intake",
    "from_offset_days": 0,
    "duration_days": 5                                 // bounded course → occurrences stop day 5
  },
  "matcher_id": "<M2 uuid>",
  "active": true
}
// care_matchers — M2
{ "name": "Tiques à l'admission",
  "expression": "has_parasites = true AND parasites ~ \"(?i)tiques\"" }
```

Matches prod intake notes like *"puces ++++ - tiques"*, *"Retirer une
quinzaine de tiques"* — but **not** *"Puces"* alone (regex is specific).
When structured parasite data ships, the admin edits M2 **once** — e.g.
`parasite_type = "tiques" OR parasites ~ "(?i)tiques"` — and every rule
referencing M2 benefits; rule rows and application history untouched
(§5.1 extensibility). The preview panel instantly shows how the population
changes between the old and new expression.

**R3 — Pigeon aviary feeding (multi-species reuse)**

```jsonc
{
  "name": "Colombidés — grains 2x/j",
  "action_kind": "feeding",
  "action_payload": { "caretype_id": "<Alimentation uuid>", "food": "grains pigeons eau" },
  "schedule": { "times": ["08:30", "17:00"], "every_days": 1, "anchor": "intake" },
  "matcher_id": "<M3 uuid>"
}
// care_matchers — M3: type match OR explicit species list
{ "name": "Colombidés (type ou liste)",
  "expression": "animal_type = \"Colombidés\" OR species IN (\"Pigeon biset\", \"Tourterelle turque\", \"Ramier\")" }
```

The `IN (...)` clause is what the species multi-select widget compiles to —
the admin picked 3 species in a dropdown, never typed the expression.

**R4 — Daily cleanup (formalizes the current `clean=1` landing heuristic)**

```jsonc
{ "name": "Nettoyage quotidien",
  "action_kind": "cleanup",
  "action_payload": { "note": "Nettoyage cage + eau fraîche" },
  "schedule": { "times": ["09:00"], "every_days": 1, "anchor": "intake",
                "grace_minutes": 180, "miss_after_hours": 24 },
  "matcher_id": null }
// NULL matcher → matches all in-care animals, UI badge "matches all animals"
```

**R5 — Weight watch on juveniles (weekly)**

```jsonc
{ "name": "Pesée hebdo juvéniles",
  "action_kind": "weighing",
  "action_payload": { "note": "Pesée de contrôle" },
  "schedule": { "times": ["10:00"], "every_days": 7, "anchor": "intake",
                "grace_minutes": 720 },
  "matcher_id": "<M4 uuid>" }
// care_matchers — M4: animal_age = "juvénile"
```

### 11.2 Evaluation trace ("why")

Matcher M1's expression evaluated against animal 10472 (trace from AST eval):

| Predicate | Expected | Actual | Pass |
|---|---|---|---|
| animal_type = … | Hérissons / Insectivore | Hérissons / Insectivore | ✅ |
| animal_age = … | bébé | bébé | ✅ |
| weight_g < 300 | < 300 | 240 | ✅ |

→ **match (AND: all pass)** → occurrences generated from intake date.
In the UI this trace renders under the pretty-printed sentence
« type = Hérissons / Insectivore ET âge = bébé ET poids < 300 g » — the
same preview the admin saw when building M1.

Same rule against animal 10501 (hérisson juvénile, 610 g): the age
predicate fails → no match → no plan items. The animal page shows R1 under
"not matching" with the failed predicates if the caretaker opens the
explanation — useful when a rule *should* have applied (e.g. weight never
recorded → `weight_g` predicate false with reason *"no value"*, shown as
"no weight on record").

### 11.3 Day plan rendering (`/care_plan`, window = yesterday → tomorrow)

Grouped by zone (existing convention), sorted by `due_at` then rule `priority`:

| Time | Animal | Action | Rule | Status |
|---|---|---|---|---|
| hier 19:00 | 10472 · Hérisson (A12) | 🦔 gavage — Croquettes + VDF | R1 | 🔴 **missing** (>12h, never applied) |
| 07:00 | 10472 · Hérisson (A12) | 🦔 gavage — Croquettes + VDF | R1 | ✅ applied 07:12 par *julie* |
| 08:00 | 10398 · Renard (Enclos 2) | Ivomec 1% (SC) — 0.08 ml | R2 | ✅ applied 08:03 par *admin* |
| 08:00 | 10420 · Buse (C03) | Mélange viande 80 g | *animal plan* P1 « nourrissage 2x/j » | ⚪ due (**Apply** / Skip) |
| 08:00 | 10420 · Buse (C03) | ~~Rapaces — nourrissage standard~~ | R6 feeding | ⃠ **overridden** by animal plan P1 (same kind, same slot) — greyed, not actionable |
| 08:30 | 10455 · Pigeon biset (Volière 1) | grains pigeons eau | R3 | 🟡 **late** (now 09:40, grace 60 min) |
| 09:00 | 223 animals | Nettoyage cage | R4 | 198 ✅ · 21 🟡 late · 4 🔴 missing |
| 09:30 | 10472 · Hérisson (A12) | 🦔 gavage — Croquettes + VDF | R1 | ⚪ due (bouton **Apply** / Skip) |
| 10:00 | 10433 · Merle (B07) | Pesée de contrôle | R5 | ⏭ skipped par *julie* — « animal stressé, reporté » |
| 12:30 | 10472 · Hérisson (A12) | 🦔 gavage | R1 | ⚪ scheduled |
| demain 08:00 | 10398 · Renard (Enclos 2) | Ivomec — **jour 5/5, dernier** | R2 | ⚪ scheduled |

Header counters: `🔴 5 missing · 🟡 22 late · ⚪ 41 due today · ✅ 96 applied`.
Filters: action kind, zone, status. Clicking a status badge on an animal
row opens its Plan tab (§7).

### 11.4 Apply flow (feeding occurrence, 09:30 slot)

1. Caretaker clicks **Apply** on the R1/10472 09:30 item; optional note +
   weight field offered (weighing during gavage is common — 122k cares in
   prod carry a weight).
2. `POST /care_plan/{item}/apply` in transaction:
   - re-check R1 still matches 10472 (weight now 245 g — still < 300 ✅);
   - insert `cares` row: `date=now`, `type=Repas`, `note="🦔 Croquettes +
     4 VDF - Eau"`, `weight="245"`;
   - insert `care_plan_applications`:
     `{source_type: "rule", source_id: R1, animal_id: 10472,
       due_at: 2026-09-20 09:30, applied_at: 09:41, user_id: julie,
       fulfillment_type: care, fulfillment_id: <care uuid>,
       status: applied, source_snapshot: {…R1 name+payload…}}`
   - UNIQUE `(source_type, source_id, animal_id, due_at)` backstop:
     colleague's second tab clicking Apply at 09:41:02 → constraint
     violation → friendly flash "already applied by julie at 09:41", no
     duplicate care row.
3. Plan re-renders: item ✅; the care row appears in the animal's existing
   care tab like any manually entered care (weight trend arrow included —
   `AnnotateCaresForDisplay` unchanged).

### 11.5 Dynamic membership in practice

- Day 0: hérisson bébé 240 g admitted with *"puces ++++ - tiques"* → R1 +
  R2 + R4 + R5 all start generating occurrences from intake.
- Day 5: R2 course ends (`duration_days`) — Ivomec disappears from plan;
  history kept in applications.
- Day 12: weight 310 g → M1's weight predicate fails → gavage items stop;
  admin's "adult hedgehog diet" rule (matcher
  `animal_type = "Hérissons / Insectivore" AND weight_g BETWEEN 300 AND 2000`)
  takes over **without anyone editing the animal**.
- Day 20: outtake (released) → all rules stop (in-care filter); applications
  remain for the stay report.

This is the core expert-system value: **the plan is recomputed from the
animal's current condition on every render** — caretakers never maintain
per-animal schedules by hand.

### 11.6 Animal plan + kind-keyed override in practice

Buse 10420 is admitted with a fractured wing; the generic rule set includes:

```jsonc
// R6 — generic raptor feeding rule (admin)
{ "name": "Rapaces — nourrissage standard",
  "action_kind": "feeding",
  "action_payload": { "caretype_id": "<Repas uuid>", "food": "poussins 1x/j" },
  "schedule": { "times": ["08:00"], "every_days": 1, "anchor": "intake" },
  "matcher_id": "<Rapaces matcher uuid>" }
```

The caretaker needs a special 2×/day hand-feeding for this bird only — no
admin involved, no matcher, created in 30 s from the animal's Plan tab:

```jsonc
// P1 — animal-level plan (caretaker), care_animal_plans
{ "animal_id": 10420,
  "name": "Buse 10420 — nourrissage 2x/j",
  "action_kind": "feeding",
  "action_payload": { "caretype_id": "<Repas uuid>", "food": "mélange viande 80 g", "force_feed": true },
  "schedule": { "times": ["08:00", "17:00"], "every_days": 1, "anchor": "intake" },
  "replaces_kind": true,   // top-level care_animal_plans column (§4.7), NOT a schedule key
  "active": true, "created_by": "julie" }
```

Occurrence resolution for 10420 at 08:00:

| Source | Kind | Slot | Result |
|---|---|---|---|
| P1 (animal plan) | feeding | 08:00 | ✅ actionable occurrence |
| R6 (rule) | feeding | 08:00 | ⃠ `overridden` — same animal + same kind + same slot → suppressed, shown greyed with *"overridden by animal plan “Buse 10420 — nourrissage 2x/j”"* |
| R6 (rule) | feeding | *(no 17:00 slot)* | — P1's 17:00 occurrence stands alone |

Note: because P1 sets `replaces_kind=true` (§4.7), the suppression would
equally cover **any** R6 feeding slot (e.g. a hypothetical R6 17:00) — the
08:00 case shown above happens to coincide with a slot collision, which pure
slot-level mode (`replaces_kind=false`) would also catch.

Counter-example — **different kinds never suppress each other**: the same
caretaker also creates an animal-level **medication** plan *« Meloxicam
2x/j »* at 08:00/20:00 for 10420. At 08:00 the plan shows **both** the
medication item (animal plan) and the feeding item (P1) — a medication plan
does NOT supersede a feeding rule, they are independent tracks.

When the bird heals, julie deactivates P1 → R6 occurrences become
actionable again automatically (dynamic membership, §5.4). Admin can
**promote P1 to a rule** if the 2×/day hand-feeding pattern proves useful
for other birds (§4.7).

### 11.7 Migration example (Phase 2)

Animal with `feeding="grains pigeons eau"`, `feeding_start=07:00`,
`feeding_end=22:00`, `feeding_period=600` (101 animals in prod share a 600
min period). Grift generates either:
- **per-diet rule** (preferred — 30 animals share the exact text): one rule
  *"Colombidés — grains 2x/j"* (R3 above) matched by type/species, per-animal
  feeding columns then frozen read-only; or
- **per-animal plan** when the diet text is unique: a `care_animal_plans`
  row for that animal, times derived: 07:00 + n×600 min ≤ 22:00 →
  `[07:00, 17:00]` — no matcher scaffold needed (§4.7).

---

## 12. Glossary / Naming (code)

`CareRule`, `CareMatcher` (named DSL query), `CareAnimalPlan` (animal-level
schedule), `CareRuleExclusion` (per-animal rule opt-out),
`CareRuleSchedule` / `CareRuleAction` (JSON value objects, shared by rules
and animal plans), `CarePlanItem` (virtual), `CarePlanApplication`;
engine package `models/careplan` (field registry, DSL parser + evaluator,
occurrence generator + override resolution); models `models/care_rule*.go`,
`models/care_matcher*.go`, `models/care_animal_plan*.go`; handlers
`actions/care_rules.go`, `actions/care_matchers.go`,
`actions/care_animal_plans.go`, `actions/care_plan.go`; routes `/care_rules`,
`/care_matchers`, `/animals/{id}/care_animal_plans`, `/care_plan`,
`/care_plan/apply_batch` (§6.2, CP2), `/care_plan/{item}/unapply` (§4.5, CP1).

---

## 13. Future Directions (v2 backlog)

Enhancement proposals recorded for post-v1 consideration (§10-EN). All are
**additive** to the v1 schema — none requires a migration of v1 data. **EN3
(payload `instructions`) was pulled into v1** — see §4.2.

1. **EN1 — Shift-handover digest**: end-of-shift summary card (what was
   applied late/missing/skipped + open alerts) on the day plan's timeline
   view; printable.
2. **EN2 — Point-of-action context**: the apply dialog shows last weight +
   ±10 % trend arrow, the resolved dosage computation, and the current diet
   text inline — decisions without leaving the plan.
3. **EN4 — Protocol bundles**: named packs of rules + matchers (e.g.
   "hérisson gavage protocol") installable in one action; versioning of
   bundles.
4. **EN5 — Criticality escalation**: a per-rule `critical` flag; N consecutive
   critical misses raise a landing-page banner (beyond the existing row
   highlight).
5. **EN6 — Vet-visit → plan suggestion**: after a veterinary visit is logged,
   suggest creating a bounded animal plan (medication course / observation
   follow-up) prefilled from the visit.
6. **EN7 — Zone "round mode"**: per-zone filtered day plan optimized for
   walking a round, with per-zone batch apply.
7. **EN8 — QR cage cards**: QR code per cage linking straight to the animal's
   Plan tab (phone at the cage).
8. **EN9 — Data-hygiene panel**: admin report of stale matchers (0 matches for
   30 days), rules never applied, animals with overdue weights — keeps the
   rule base healthy.
9. **EN10 — Adherence report for read-only roles**: a `/reports`-style
   adherence page whitelisted for `spw`/`scientifique` (plan adherence only,
   no care-detail exposure beyond what their role already sees).
