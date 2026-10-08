# TODO — Sync v2 (creaves ↔ creaves-console)

Tracks implementation of the sync v2 specification. Do **not** edit this file to change
behavior — it is a checklist only; the authoritative documents are:

- **Spec**: [`creaves-console/docs/specs/SYNC_SPEC.md`](../../creaves-console/docs/specs/SYNC_SPEC.md) — webhook contract v2 (instances, `animal_state`, translations, idempotency, cleanup, resync)
- **Implementation plan**: [`creaves-console/docs/plan/SYNC_IMPLEMENTATION_PLAN.md`](../../creaves-console/docs/plan/SYNC_IMPLEMENTATION_PLAN.md) — review findings, sequencing, traceability
- **Per-phase subplans** (micro-tasks + TDD test cases): [`creaves-console/docs/plan/subplans/`](../../creaves-console/docs/plan/subplans/)

Rollout order is fixed: **console first, then creaves** (spec §7). Work each task
RED → GREEN per its subplan; update checkboxes here as tasks complete.

## Phase 1 — Console hardening

- [ ] T1.1 Fix SQL injection in `ReportsBySpecies` (F1) — [`subplans/phase-1-console-hardening.md`](../../creaves-console/docs/plan/subplans/phase-1-console-hardening.md)

## Phase 2 — Console instance registry, state events & cleanup (R1, R2, R4)

- [x] T2.1 `CreavesInstance` model + migration — [`subplans/phase-2-console-instance-registry-cleanup.md`](../../creaves-console/docs/plan/subplans/phase-2-console-instance-registry-cleanup.md)
- [ ] T2.2 Webhook auto-registration + `instance` envelope block
- [ ] T2.3 `consolidated_animals` v2 columns (`translations`, `state_hash`, `last_state_at`)
- [ ] T2.4 `animal_state` processing — replace semantics
- [ ] T2.5 Instance admin UI + single-action cleanup (`/instances`)

## Phase 3 — Console reporting scoping (R2)

- [ ] T3.1 Report scope helper — [`subplans/phase-3-console-reporting-scope.md`](../../creaves-console/docs/plan/subplans/phase-3-console-reporting-scope.md)
- [ ] T3.2 Scope dashboard + all report pages by `instance_id`; instance selector UI

## Phase 4 — Console multilingual UI (R5/R6)

- [ ] T4.1 French locale (templates + yaml + switcher) — [`subplans/phase-4-console-multilingual.md`](../../creaves-console/docs/plan/subplans/phase-4-console-multilingual.md)
- [ ] T4.2 Localized reference-data display (`LocalizedField` helper + report labels)

## Phase 5 — Creaves payload v2 (R5)

- [ ] T5.1 Translations map (all locales) in payload builder — [`subplans/phase-5-creaves-payload-v2.md`](../../creaves-console/docs/plan/subplans/phase-5-creaves-payload-v2.md)
- [ ] T5.2 Instance block + `contract_version` in pusher envelope

## Phase 6 — Creaves full resync (R3, R4)

- [ ] T6.1 `resync_runs` model + `event_streams` columns — [`subplans/phase-6-creaves-full-resync.md`](../../creaves-console/docs/plan/subplans/phase-6-creaves-full-resync.md)
- [x] T6.2 Deterministic state-event identity (content hash + UUIDv5)
- [ ] T6.3 Background resync service (skip-unchanged, cancel, restart recovery)
- [ ] T6.4 Resync web UI + `status.json` progress endpoint

## Phase 7 — End-to-end verification & docs

- [ ] T7.1 Console e2e tests (idempotent resync, cleanup→resync recovery, scoped reports) — [`subplans/phase-7-e2e-docs.md`](../../creaves-console/docs/plan/subplans/phase-7-e2e-docs.md)
- [ ] T7.2 Cross-project manual test checklist
- [ ] T7.3 Update both `AGENTS.md` + project `TODO.md` files to point at the spec
