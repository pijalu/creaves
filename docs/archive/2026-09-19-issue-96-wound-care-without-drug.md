## #96 — Choix d'un traitement "soin de plaie" sans médicament — DONE ✅ commits 9d01095+6b3b272
**Source:** https://github.com/pijalu/creaves/issues/96
**Observed:** /treatments/new/ requires drug + posologie; wound-care treatment type should allow remarks-only.
**Expected:** When treatment type = "soin de plaie", drug + dosage fields hidden/optional, remarks kept.
**Plan:**
1. Identify care-type/wound-care treatment type (`caretypes` or treatment type field).
2. `templates/treatments/new.html` + edit: JS toggle — hide drug/dosage when type=soin de plaie; server-side: `actions/treatments.go` Create/Update skip drug validation for that type, require remarks optional.
3. Schema: ensure drug_id nullable (check migrations — additive only if change needed).
**Test:** unit: create soin-de-plaie treatment without drug → valid; other types still require drug. e2e: form toggle hides fields, submit succeeds, remarks saved.
**Validation:** `go test ./actions/...`; agent-browser both type paths.
