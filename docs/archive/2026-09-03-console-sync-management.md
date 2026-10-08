# Fixed: creaves-console animals sync management (2026-09-03)

Fixed in creaves-console commit `6c568a0`. Validated with agent-browser in
en/fr/de/nl.

## creaves-console: Cleanup/delete animal (reopen) — FIXED
There should be an admin screen manage animals sync:
* delete all animals from the database
* delete all animals from a specific instance

**Fix**: new admin screen `/sync_management` (handler
`actions/sync_management.go`, templates in en/fr/de/nl) showing
per-instance animal counts, total and orphan counts, with two destructive
actions: delete ALL consolidated animals (danger zone) and delete all
animals of a specific instance. Both keep events, instance registry and
API keys so a full resync from Creaves rebuilds the data. Admin-gated
(403 for non-admin). Admin dropdown also gained the missing Instances
link.

Tests (`actions/sync_management_test.go`): index access + counts,
non-admin denial on all 3 routes, scoped per-instance delete (other
instance untouched, events/registry kept), delete-all, missing
instance_id no-op. Full suite green (race + cover).
