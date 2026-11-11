## #170 — Création de Sortie selon la cage — DONE ✅ commit 4fc42c0
**Source:** https://github.com/pijalu/creaves/issues/170
**Spec (from reporter):**
- Allow setting exit for **all animals in the same cage** in one operation (cages can hold multiple animals).
- User selects **the cage** — cage list shows only **known cages with at least one animal requiring outtake** (no empty cages).
- Outtake **rules (type) chosen from a single (first) animal** of the cage, then applied to the whole cage.
- Outtake is saved **for all animals in the cage that need outtake** (animals already out are skipped).
**Implemented (4fc42c0):**
- `GET /outtakes/cage` — cage picker (autocomplete on `/suggestions/CageWithAnimalInCare`, existing endpoint, only cages with animals needing outtake); `?cage=` renders the shared outtake form with the cage's in-care animals listed (num annu + species).
- First animal = reference: intake date feeds the +24h/+48h/duration helpers; its native status filters the type list (reuses `setFilteredOuttakeFormData`); POST re-checks future date, type validity, native-status exclusion and location rule for the first animal before saving anything.
- Batch save: one outtake per animal (own UUID + own `stay_duration` computed from its own intake), treatments after the date deleted + audited per animal, audits + `animal_released`/`animal_died`/state events per animal. Already-out animals excluded by the `outtake_id IS NULL` query.
- Success flash "N animal(s) of cage X" ×4 locales; rejections flash reason and return to the cage form with the cage preserved; entry link "or select a whole cage" on the single-outtake picker ×4.
- Routes registered BEFORE the `/outtakes` resource so `cage` isn't captured by `/{outtake_id}`.
**Test (GO_ENV=test):** batch create with 2 fixture animals having distinct intakes → 2 outtakes, per-animal durations {0 clamped, 36h} (proves per-animal computation); future date → 303 redirect + nothing persisted + both animals stay in care; suggestions endpoint returns the cage; empty cage → warning + redirect. gocyclo/gocognit clean after extracting `cageOuttakeAccepted`/`createOuttakeForAnimal` helpers.
**E2e (agent-browser):** picker → cage E2ECAGE1 (2 test animals moved in for the run) → form lists 994/26 + 1060/26 → DCD + past date → flash "Outcome saved for 2 animal(s) of cage E2ECAGE1.", both animals outtaken with stay_duration; FR picker heading verified; full rollback applied afterwards (animals restored to VE27, test outtakes deleted).
