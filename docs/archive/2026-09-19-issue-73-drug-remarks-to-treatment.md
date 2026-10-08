## #73 (enhancement) — Drugs: Remarks copied to treatment — DONE ✅ commit 2eb742e
**Source:** https://github.com/pijalu/creaves/issues/73
**Observed:** Drug remarks (drug `Description`) not carried into treatment remarks when drug selected.
**Expected:** Selecting drug on /treatments/new/ prefills treatment remarks from drug description (editable after).
**Plan:**
1. `templates/treatments/_form.html` (or new.html): JS on drug select change → fetch drug description (embed descriptions as data-attrs in select options or small JSON endpoint) → prefill remarks field only if empty (never overwrite user text).
2. Server-side untouched (copy happens client-side at encode time).
**Test:** e2e: select drug with description → remarks prefilled; type custom remarks first → not overwritten. Unit: none needed beyond build.
**Validation:** agent-browser on /treatments/new/ with seeded drug having description.
