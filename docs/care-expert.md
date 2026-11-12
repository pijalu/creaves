# Care Expert — Current Design

The Care Expert converts protocols and per-animal instructions into scheduled, explainable care work. It supports feeding, medication/treatments, general care, cage cleanup, weighing, and observations. The system is implemented and used by the dashboard, day plan, animal Protocol tab, and legacy conversion workflow.

This document describes current behavior only. The full historical specification and fix-round evidence are in `docs/archive/`.

## Components

| Component | Current responsibility |
| --- | --- |
| `models/careplan` | Pure-Go matcher registry, DSL parser/evaluator, schedules, occurrence generation, status logic, and why-traces |
| `actions/care_plan*` | Database-backed rule/plan resolution, day-plan assembly, apply/defer workflows, conversion, validation, and repair |
| `actions/care_rules.go` | Rule-library administration and preview |
| `actions/care_animal_plans.go` | Per-animal protocol editing, overrides, review flags, and plan details |
| `grifts/careplan_*` | Seed, validation, migration, and controlled maintenance commands |
| `care_matchers`, `care_rules` | Reusable selection and protocol library |
| `care_animal_plans` | Animal-specific plans, including converter-owned and human-authored rows |
| `care_plan_applications` | Applied occurrence records and idempotency/history |
| `treatment_time_entries` | Treatment fulfillment/time details used by treatment views |

## Resolution model

1. Build an `AnimalContext` from the animal and related care state.
2. Evaluate enabled matchers against that context.
3. Resolve matching rules and per-animal plans, including exclusions and explicit overrides.
4. Parse each plan's action payload and schedule.
5. Generate occurrences inside the applicable intake/outtake and start/end bounds.
6. Combine occurrences with existing applications, deferrals, treatment entries, and legacy records.
7. Return the ordered day-plan assembly with an explanation of why each item applies.

The engine package does not require database access. Persistence, authorization, transactions, and rendering remain outside it. Matcher fields and operators are registered explicitly; the DSL is validated at save time so unknown fields, invalid operators, malformed literals, and invalid regular expressions are rejected.

The parser is generated from `models/careplan/dsl.y`. After changing the grammar, run `go generate ./models/careplan` and test the generated parser; do not hand-edit generated output.

## Plans and schedules

A plan associates an action kind, action payload, schedule, applicability bounds, and ownership metadata with an animal. Rules provide reusable defaults; animal plans provide concrete or overridden instructions.

Schedules support daily or every-N-day cadence, intake or fixed-date anchoring, explicit times, repeat windows, grace periods, and finite durations where applicable. Generated work is clamped to the animal's actual care period and the plan's active bounds. Day-level cadence must be preserved by converters and editors; do not silently turn it into an every-day schedule.

Action payloads are validated by kind. Instructions and relevant medication, diet, cleanup, weighing, or observation details remain structured data rather than being inferred from a display name.

## Applying and reviewing work

The day plan groups open work by the normative presentation contract: **Late, Now, Later, Done, and History** where applicable. A user can apply an eligible occurrence or defer it within the allowed behavior. Applications are persisted so reloads and repeated requests do not duplicate completed work.

`needs_confirmation` is a manual review flag on a per-animal plan:

- It is edited from the plan editor or feeding summary.
- `/reports/confirmations` lists flagged plans for administrative review.
- Resolving the flag does not alter the schedule.
- It never suppresses an occurrence.
- Converter output does not invent review flags or append uncertainty suffixes to names.

The normative visual and interaction rules are in [care-presentation-guideline.md](care-presentation-guideline.md).

## Legacy conversion and ownership

Legacy feeding fields, treatment series, and cage-oriented rules have been converted into Care Expert structures. Conversion and repair must distinguish two ownership classes:

- **Converter-owned** rows have no human `created_by` and carry the relevant conversion identity. Controlled repair can rewrite or recreate them.
- **Human-authored** rows have a user creator and must not be overwritten by automatic conversion.

Feeding repair is marker-guarded (`startup_v5`) and runs once per database. It compares converter plans with legacy feeding columns, repairs stale or missing converter plans, and retires converter-owned zombies. Treatment-series refresh is idempotent and runs at every boot so prescriptions created after the initial conversion are discovered.

The converter does not maintain a second runtime source of truth: new and edited work is rendered through Care Expert plan resolution. Legacy rows remain important for history, fulfillment, validation, and controlled migration.

## Operational gates

Use `POP_PATH` to select an explicit isolated `database.yml` and verify its database name before running any command.

| Command | Writes? | Purpose |
| --- | --- | --- |
| `buffalo task careplan:seed` | Idempotent library additions | Install newly added matchers/rules without replacing local edits |
| `buffalo task careplan:validate-feeding` | No | Compare every in-care legacy feeding trace with its expected plan |
| `buffalo task careplan:validate-treatments` | No | Verify active converted coverage for future treatment series |
| `buffalo task careplan:repair-feeding` | Marker-guarded | Run the feeding convergence/validation pass |
| `buffalo task careplan:migrate-cage-rules` | Marker-guarded | Convert eligible legacy cage matchers to per-animal plans |
| `buffalo task careplan:fixnames` | Yes | Rebuild conversion-stamped feeding names from structured diet payloads |
| `buffalo task careplan:reconvert-treatments` | Destructive/recreational | Delete converter-owned non-feeding plans and recreate them in one transaction |

Run both read-only validators before and after conversion work. A feeding `STALE` result means stored converter output differs from legacy columns; `MISSING` means required coverage is absent. Treatment `INACTIVE` can represent a deliberate stop, while `MISSING` requires investigation.

Repair commands require a current backup and staging rehearsal. Never repair a production database merely to clear an unexplained validation message; inspect the affected animal and ownership class first.

## Performance and correctness constraints

- Keep day-plan assembly and reference-data loads batched; do not add per-occurrence queries.
- Preserve the in-care animal load and open-item count caches with post-commit invalidation.
- Preserve complete animal histories. Do not cap or truncate history as an optimization without a separate product decision.
- Apply/defer paths must be transactional and idempotent.
- Tests must cover cadence, intake/outtake bounds, time zones/DST where relevant, ownership protection, repeated startup/refresh, and read-only validator behavior.

## Validation

```bash
GO_ENV=test go test -count=1 ./models/careplan ./actions ./grifts
POP_PATH=/path/to/isolated-config buffalo task careplan:validate-feeding
POP_PATH=/path/to/isolated-config buffalo task careplan:validate-treatments
```

Use authenticated browser checks for the day plan, animal Protocol tab, protocol editor, apply/defer flow, and confirmation report in every affected locale. The fixture checklist is [care-plan-test-data.md](care-plan-test-data.md).
