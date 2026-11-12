# Creaves Documentation

## Current documentation

| Document | Scope |
| --- | --- |
| [../README.md](../README.md) | Features, architecture, setup, configuration, tasks, testing, deployment, and troubleshooting |
| [../AGENTS.md](../AGENTS.md) | Concise engineering, safety, localization, and validation rules |
| [care-expert.md](care-expert.md) | Current Care Expert components, resolution model, ownership, and operational gates |
| [care-presentation-guideline.md](care-presentation-guideline.md) | Normative care-plan presentation and interaction behavior |
| [SETUP_AND_TESTING.md](SETUP_AND_TESTING.md) | Combined Creaves/Console setup and integration validation |
| [care-plan-test-data.md](care-plan-test-data.md) | Fixture checklist for Care Expert browser tests |
| [../e2e/EXPECTATIONS.md](../e2e/EXPECTATIONS.md) | Fixed fixture expectations used by E2E tooling |
| [../e2e/RESYNC_RUNBOOK.md](../e2e/RESYNC_RUNBOOK.md) | Current full-resync and recovery procedure |
| [../scripts/README.md](../scripts/README.md) | Embedded translation-seed regeneration and drift checks |
| [../../creaves-console/docs/specs/SYNC_SPEC.md](../../creaves-console/docs/specs/SYNC_SPEC.md) | Canonical webhook v2 contract |

## Historical material

[archive/](archive/) preserves research, model/route catalogs, implementation checklists, completed fix plans, bug logs, dated assessments, and old E2E results. These files are evidence of earlier work, **not current setup or design authority**. Consult them only when investigating why a historical decision was made.

## Maintenance rules

- Keep active pages limited to current features, design, operations, contracts, and runbooks.
- Update an existing current page with behavior changes; avoid duplicating the same contract in several places.
- Move completed research, plans, and dated reports to `docs/archive/` rather than leaving them beside current guides.
- Preserve historical content and repair links when moving files.
- Record validation in commits or a dated archive report; do not accumulate test-result logs in active guides.
