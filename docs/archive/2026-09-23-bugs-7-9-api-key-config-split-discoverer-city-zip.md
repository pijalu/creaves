# Bugs 7–9 — sync-target API key visibility, config/sync split, discoverer city+zip cleanup

Resolved 2026-09-23. All work in the `creaves/` project. Commits: `b0a83b2`,
`0831ed5`, `231831e`, `858a881`, `d0fdc4a`.

## Bug 7 — API key must be visible for admin/maintainer

The sync-target edit form masked the stored webhook API key for every
non-maintainer (password input; `SyncTargetsResource.Edit` blanked the value),
so a plain admin could not verify the configured key against the key issued by
the Creaves Console.

Fix: the form is `requireAdmin`-gated, so the stored key is now prefilled for
admin **and** maintainer (`type="text"` whenever
`current_user.Maintainer || current_user.Admin`), in all four languages. The
blank-submit-preserves-key fallback in `bindSyncTarget` remains as a safety
net. Tests: plush fragment test for both roles + handler test proving a plain
admin GETs the key in clear text (`TestSyncTargetEditShowsAPIKeyToAdmin`).

E2E: `/sync_targets/{id}/edit` as admin renders
`<input type="text" ... value="creaves_2ff0d503-…">`.

## Bug 8 — general config vs sync config separation

The edit form was already identity-only and sync lives on
`/sync_configuration` (instance ID + event stream + N sync targets), but the
**show** page still displayed the instance-ID row and an Event Stream section,
and the **new** form still rendered the legacy combined form.

Fix: show page reduced to name/description/active/CREAVES identity/timestamps
(all four languages); new form uses the identity partial; legacy
`_form.plush*` partials deleted; `ConfigsResource.Create` falls back to
`defaultInstanceID()` (shared with `LoadConfig`) with a short random suffix
when the unique `instance_id` is already taken (was a hard 500 on duplicate),
and binds settings identity-scoped on top of `DefaultSettings` so
`EnableEventStream` keeps its default (true). Orphaned `config.show.*` sync
locale keys removed from all four languages. General config = 1 row; sync =
multiple sync targets. Tests: `TestConfigShowHasNoSyncDetails`,
`TestConfigNewIsIdentityOnly`, `TestConfigCreateWithoutInstanceID`.

E2E: `/config/210f9a6e-…` shows identity only; `/config/new` has no
InstanceID/EnableEventStream inputs; `/sync_configuration` still owns both.

## Bug 9 — discoverer search/fill with zip merged into city

Production `discoverers.city` values merge the zip code (two variants:
`"67000 Strasbourg"` / `"4280_Avin"`, ~4400 rows). The discoverer picker
returned raw values — labels duplicated the zip and picking a match filled
the mixed value into City; the discoverer-form city autocomplete offered raw
mixed values. Stored rows are not rewritten (production data); the endpoints
clean what they return.

Fix: `splitPostalCity` helper (leading/trailing zip, optional country prefix,
space **and** underscore separators; adopts the zip when the postal-code
field is empty; always returns the clean city). `SuggestionsDiscovererLookup`
also matches `city`/`postal_code` (searching by zip or city shows the match
with details) and returns cleaned postal/city/label. `SuggestionsDiscovererCity`
LIKE-matches raw values but returns cleaned, de-duplicated names.

Follow-up (user note): selecting a discoverer must fill **all** fields — the
lookup payload now carries the discoverer note; reception and animal forms
gained a discoverer Note textarea; `fillDiscoverer` fills it (and every other
field: address, postal, city, country, email, phone, donation, return-request
checkbox); editing the note detaches the picked record like other fields.
Tests assert note + email round-trip through the lookup.

Tests: `TestSplitPostalCity` (table incl. underscore variants),
`TestDiscovererLookupMatchesMixedCity` (by name, city **and** zip; clean
city + postal code + note + email), `TestSuggestionsDiscovererCityCleaned`.

E2E (reception wizard, real autocomplete): typing "Merckx" shows
"Chantal Merckx — Rue Neuve, 25, 1320 Beauvechain"; selecting fills id,
firstname, lastname, address, postal `1320`, city `Beauvechain` (no zip),
country, email `patale4@hotmail.com`, phone, note, donation, return-request.

## Discovered during testing → bugs.md #10 (still open)

Order-dependent test flakes from shared webhook-worker state: the CSRF fixes
let the sync-target/reset tests actually wake the delivery worker; any later
heavy test (dashboard, reports nav, exports) can then 500 with
`i/o timeout` / `invalid connection` (test DSN `readTimeout=3s` + multi-second
dashboard query + worker/pool churn). Failing set varies run to run (run 1:
event-stream/remap/retry — fixed by the CSRF token header change, commit
`858a881`; run 2: four export tests). Every affected test passes in
isolation; verified pre-existing on the pre-fix baseline (commit `2966833`).
Mitigation applied: orphaned `event_streams`/`event_deliveries` rows purged
from `creaves_test`. See bugs.md #10 for the remaining plan.
