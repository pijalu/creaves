# Fix archive — 2026-09-03 — Template variant inventory completion (variant absent debt)

## Trigger
While fixing Bug 5 (API key visibility), the pre-existing
`TestTemplateVariantStructuralParity` reported 75 "variant absent" entries
(missing `.plush.fr/.de/.nl.html` locale variants). Per user instruction these
must be fixed, not accepted as debt.

## What was done (creaves commit `bbc77e4`, branch `feature/i18n`)

### Variants created (75 total)
- **French variants where only de/nl existed** — full CRUD sets for
  `discoverers`, `discoveries`, `entry_causes`, `intakes`, `native_statuses`,
  `subside_groups` (_form/edit/index/new/show), plus `species/_form`+`edit`,
  `traveltypes/show`, `paths/index`, `export/csv`+`excel`,
  `config/_form`+`edit`+`index`+`new`+`show`, `_flash`.
- **de/fr/nl for templates that had no variants at all** — `guest` layout,
  `event_streams/index`+`show`, `webhook_resync/index`+`status`,
  `translations/_fields`+`edit`+`index`+`show`.

### t()-keyed bases → verbatim copies
`config/show`, `event_streams/*`, `translations/_fields` render entirely through
the go-i18n catalog (`t("…")`), so their variants are byte-identical copies —
localization happens at runtime, structural parity is trivially satisfied.

### Additional fixes
- `guest/new.plush.fr.html` — pre-existing drift (missing `<p class="·">` in the
  intro paragraph) that the test flagged as NEW once the inventory was clean.
  Fixed by restoring the missing `<p class="text-muted">` wrapper.
- `species/_form.plush.fr..html` / `species/edit.plush.fr..html` — files with a
  double dot in the suffix (`fr..html`), invisible to Buffalo's variant
  resolver. Renamed to correct `.fr.html` names.
- `traveltypes/show.plush.plush copy.html` — junk file, deleted.

## Key technical discovery (documented for future template work)
`normalizeTemplate`'s `normalizeQuotes` pairs single quotes **positionally
across the whole template**, not per text node. Two ASCII apostrophes in
different text nodes of the same tag gap pair up and swallow the intermediate
markup into a quote literal, which the tag-gap eraser then removes — the
variant registers as structural drift. **French text nodes must therefore use
the typographic apostrophe `’` (U+2019)**, which the eraser does not treat as a
quote delimiter. All new fr variants follow this rule; `guest/new.plush.fr.html`
was converted to it as well.

## Validation
- `go vet ./...` clean.
- `go test -count=1 -race -cover -tags sqlite ./...` — `creaves/grifts` now
  **passes** (TestTemplateVariantStructuralParity green, 0 absent, 0 new drift,
  122 known-frozen debts unchanged). Remaining `creaves/actions` failures are
  the identical pre-existing set (TestPublishAnimalHelpers, TestBuildEventPayload_*,
  TestLocalizeAnnual*, TestReportsAnnualStats, TestRunResync*, …) — unrelated.
- agent-browser (dev server, locale switch): FR/DE/NL verified on
  `/entry_causes/`, `/native_statuses/`, `/subside_groups/`, `/translations/`,
  `/webhook_resync`, `/intakes/`, `/config` — headings and table headers render
  in the selected language; FR `/config/{id}/edit` shows the prefilled visible
  API key with French copy.
- Pre-existing, unrelated: `/discoveries/new` returns 500 in EN too (form
  helper reflect error — present before this change, not variant-related).
