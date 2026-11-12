# Current E2E Resources

Only reusable testing and recovery material remains in this directory.

| File | Purpose |
| --- | --- |
| [EXPECTATIONS.md](EXPECTATIONS.md) | Fixed fixture identities, credentials, and expected records for scripted E2E runs |
| [RESYNC_RUNBOOK.md](RESYNC_RUNBOOK.md) | Current full-resync and recovery procedure |

Historical browser plans, dated runs, and resolved bug logs are under `../docs/archive/`. They are not substitutes for current validation.

Use isolated databases and authenticated browser sessions. The combined startup and smoke-test procedure is in [../docs/SETUP_AND_TESTING.md](../docs/SETUP_AND_TESTING.md). Creaves automated tests require MySQL:

```bash
GO_ENV=test go test -count=1 ./...
```
