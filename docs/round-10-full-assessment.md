# Round 10 — Complete solution assessment (all screens × all languages)

Date: 2026-10-04. Branch `feature/care-expert`, after R9 (`61ebccd`).

## Scope & method

1. **Automated crawl** — 113 GET pages (every resource index/new/show/edit, all
   day-plan kinds, reports, admin/sync pages) × 4 locales (en-us, fr, de, nl) =
   452 requests. Each response checked for: non-200 status, Buffalo error pages,
   unrendered plush (`<%=` in output), leaked i18n keys (every key from
   `locales/*.yaml` matched against the page's visible text), and
   "translation missing" markers.
2. **Visual browser sweep** — 33 key screens × 4 locales = 132 screenshots
   (1440×900, agent-browser), reviewed for content correctness, layout and
   language leaks that a crawler cannot see (valid-but-untranslated strings).

## Issues found → fixed

### R10-1 — `/discoveries/new` and `/discoveries/{id}/edit` returned HTTP 500 (all locales)
`templates/discoveries/_form.plush.*.html` used `f.InputTag("Discovery.PostalCode")`
/ `f.InputTag("Discovery.City")`, but the form is bound to the Discovery model
itself — there is no nested `Discovery` struct, so the tags helper hit
`reflect.Value.Interface on zero Value` and the whole page 500'd. The `Discovery.`
prefix IS correct in `animals/_form` and `reception/new` (those bind `animal`,
which has a nested Discovery) — only the discoveries fork was wrong.
**Fix**: field names → `PostalCode` / `City` (4 forks), matching the `Create`
action's direct `c.Bind(discovery)`. Verified 200 in 4 locales.
Also removed a stray `c.Logger().Debugf("LaMerde: …")` debug leftover in
`actions/discoveries.go`.

### R10-2 — Species names inconsistent across screens
The animals list / dashboard / corpse register route species through the
`tspecies` helper (translations table, 494 species fully covered per locale),
while the day plan, care-schedule report, animal Treatment/Protocol tabs and
matcher previews read the raw canonical-French `animals.species` column — so an
EN user saw "West European Hedgehog" on one screen and "Hérisson" on another.
**Fix**: `localizePlanSpecies(c, plan)` rewrites the per-request plan's display
rows (matcher contexts untouched) — applied in CarePlanIndex (HTML + JSON),
dashboard, care-schedule report; `speciesDisplayOf(c)` localizes the animal row
in `AnimalsResource.Show` (covers Treatment/Protocol tabs) and is threaded into
`previewMatcherExpression` (rule/matcher previews). Verified: day plan now shows
West European Hedgehog / Hérisson / Braunbrustigel / Egel in en/fr/de/nl,
consistent with the animals list.

### R10-3 — Dashboard was never localized (all 4 forks byte-identical English)
"Animals in alert", "Animals with weight loss", "Veterinary visits today",
"Log entries (last 24h)", "Statistics / Animals in cares", every table header,
"None", TODO buttons and the delete-confirm were hardcoded English in every
locale. **Fix**: 30 new `dashboard.*` keys added to `dashboard.{en-us,fr,de,nl}.yaml`
and the template switched to `t()` calls (forks stay identical by design).
Verified: FR dashboard renders "Animaux en alerte / Animaux avec perte de poids /
Espèce / Âge / Poids".

### R10-4 — Animal page DE/NL untranslated labels
`templates/animals/show.plush.{de,nl}.html` kept English "Cage" (subtitle +
field), "Age", "Date" ×5, "Note" ×3, "Contacts", "Parasites", "Type" (DE).
**Fix**: translated per fork (DE: Käfig/Alter/Datum/Notiz/Kontakte/Parasiten/Typ;
NL: Kooi/Leeftijd/Datum/Notitie/Contacten/Parasieten; "Type" is correct Dutch).

### R10-5 — Preferences save: dead success flash + key mismatch (R8 leftover)
`PreferencesResource.Save` flashed `preferences.update.ok/.error` — keys that
exist as `preferences.ok`/`preferences.err` — and the success path sat after an
unconditional return (unreachable; vet flagged it). **Fix**: proper
error/success branches using the existing keys.

### R10-6 — Intakes index was a scaffold stub
Single empty `<th>&nbsp;</th>` header, rows rendered only the action buttons —
no data columns at all (all locales). **Fix**: proper table
(Date / General condition / Wounds / Parasites / Remarks + actions) in the 4
language forks.

### R10-7 — NL data typo "juvéniel"
`grifts/translations_nl.sql` seeded the juvenile-age NL name as "juvéniel"
(French accent). Fixed to "juveniel" in the seed and in the live translations
table.

## Verified, not defects

- **FR navbar collapses to icons at 1440px** (en/de/nl show labels): the R4-7.9
  overflow probe is accurate — without compact mode the FR bar really overflows
  by ~59px and pushes the search box out. Tooltips carry the labels.
- **Zone/entity names like "ACCUEIL", "EXTERIEUR", single letters**: DB data
  with canonical-French fallback when untranslated — managed in /translations,
  not UI chrome.
- **French rule/food/drug text on non-FR screens** (e.g. "gavage graines…"):
  reference data authored in French; localized values where translations exist
  (drugs, species, caretypes, ages…).
- **PUT-over-POST edit forms**: Buffalo v0.18.9 mounts MethodOverride by
  default (`a.MethodOverride` in `New`), contradicting the R8 note; resource
  edit forms are fine.
- Export list names (config.yaml data) and reports column names remain French
  where they are authored data.

## Regression evidence

- Full crawl re-run after fixes: 452/452 requests clean (0 error pages,
  0 leaked keys, 0 raw plush).
- `buffalo test ./...` green (see round log); gofmt clean on touched files.
