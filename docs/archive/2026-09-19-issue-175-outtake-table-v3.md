## #175 — Table sortie V3 + vue sortie — DONE ✅ commits 3dab3c9 + 2dc0d47
**Source:** https://github.com/pijalu/creaves/issues/175 | labels: CONFORME SPW
**Implemented (commit 3dab3c9):**
1. **Séjour 24h/48h** — `outtakes.stay_duration` column (additive, nullable, applied to creaves + creaves_test, schema.sql regenerated); `ComputeStayDuration` (whole hours, negative→0) persisted on Create+Update; form buttons +24h/+48h set date = intake+Δ (flatpickr setDate or direct fallback) with live "N h" display; shown on outtake show + animal page outtake pane; Edit form pre-fills display via `intakeDate` context.
2. **ID d'indigénat forbidden** — `Outtake.Validate` rejects any type whose accent-normalized name contains "indigen" (`deaccent` via x/text norm.NFD; "indigénat" would NOT match plain "indigen" — accent bug found+fixed during tests).
3. **Lieu per type** — already existed (outtaketype LocationMode none/free/list + JS + suggestions); verified e2e.
4. **No future exit date** — `Validate` (tolerance 1min) + handler pre-check with localized flash `outtake.date.future` (4 locales) on 422 re-render; flatpickr `maxDate: new Date()` client-side.
5. **Canonical outtake types** — seed migration `20261005160100` inserts missing canonical types (OT1 Relacher, OT2 DCD def, OT3 Euthanasier, OT4 Transferer, OT5 Mort à l'arrivée, OT6 Adoption, OT7 Doublon error) and **stamps `code` on same-name rows without touching their data** (center customizations preserved); OT5 guard matches straight + typographic apostrophes (dev DB had `'`, seed used `’` — duplicate caught+fixed via down/up). Immutability via maintainer-only outtaketypes resource (existing).
6. **Num annu everywhere** — show/edit headings use `YearNumberFormatted()`; index header localized (Animal number / N° annu / Tiernummer / Diernummer).
**Test:** models (ComputeStayDuration table incl. zero-date cases, future-date Validate, deaccent); actions handler tests through the real app+CSRF (indigénat 422 + nothing persisted, future date 422, duration persisted = floor(intake→outtake hours) on 303 create).
**E2e (agent-browser, admin):** +24h/+48h buttons set 2026/09/17+18 21:11 & live display; future date 2030 → re-render with "in the future" message; create → animal pane "Stay duration 24 h", show/edit headings "1862/26", index headers ×4 locales verified (en/fr/de/nl). Test outtake cleaned up afterwards.
**Notes:**
- gocyclo/gocognit flags on `OuttakesResource.Create` (20/33) + `New` (17) are the pre-existing HEAD baseline (verified identical on HEAD before the change).
- Dev DB data note for deployment: `Décédé` (non-canonical, NULL code) and `DCD` both have `def=1` — two defaults; center data, seed migration intentionally does not clobber; maintainer may want to unset one.
- Goal verifyCommand must run `GO_ENV=test go test ./actions/...` — without it the suite runs against the DEV db (data-dependent failures).
- 2dc0d47: TestUsersListSearchSort now requests `per_page=100` (fixture rows fell off page 1 when the shared test DB grew past pop's default 20-row page — caught by the #175 gate).
