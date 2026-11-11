# #199-16 — Align `/outtakes/new` design with `/cares/new`

**Issue**: [pijalu/creaves#199](https://github.com/pijalu/creaves/issues/199), item 16
> «Aligne les deux pages "/cares/new/" et "/outtakes/new/" avec le même design
> "Sélectionnez l'animal pour ... ou choix de la cage pour..." — recopie ce qui
> existe sur "/cares/new/" et transpose le sur "/outtakes/new/"».

**Status**: ✅ fixed — commit `601ab71` (branch `feature/open-issues-2026-10`, 2026-09-21)

## Observed

- `/cares/new` shows two inline pickers built with `formFor` (animal → GET
  `newCaresPath()`, cage → GET `newCaresPath()`), all texts localized per
  variant.
- `/outtakes/new` had the same overall layout (added by `ec5e62d`) but the cage
  picker was a raw `<form method="GET" action="/outtakes/cage">`, headings used
  the pre-rename "outtake" terminology in EN, and the four locale variants had
  structurally drifted (frozen KNOWN debt in `TestTemplateVariantStructuralParity`):
  - FR/NL else-branch displayed the raw `animal.Species` instead of `tspecies(...)`;
  - DE/NL included the EN base form partial instead of their localized ones;
  - FR ring input carried a `class=" form-control"` typo.

## Fix

Rewrote `templates/outtakes/new.plush.html` to mirror `cares/new` structurally,
keeping outtake-specific target actions:

- Animal picker: `formFor(outtake, {action: newOuttakesPath(), method: "GET"})`
  (unchanged flow — the New handler keeps its `!animal` condition so flash
  errors re-render the pickers).
- Cage picker: converted to `formFor(outtake, {action: "/outtakes/cage", method: "GET"})`
  — the batch cage flow (`/outtakes/cage`) is kept as-is.
- All texts go through `t()`; the four variants are byte-identical except the
  form partial reference (`outtakes/form.plush.<loc>.html`), which the parity
  eraser neutralizes.
- Else-branch (outcome form): `tspecies()` everywhere, t()-based
  header/identification/cancel, localized partial for de/nl.
- EN headings adopt the "Outcome" terminology already used by
  `outtakes/index` and the navbar (commit `25c409b`).

### Files

- `templates/outtakes/new.plush{,.fr,.de,.nl}.html` — rewritten (4 variants,
  byte-identical modulo partial reference).
- `locales/outtakes.{en-us,fr,de,nl}.yaml` — 12 new keys `outtakes.new.*`.
- `grifts/locale_key_parity_test.go` — `outtakes/new.plush.html` removed from
  `knownVariantDrift` (debt paid).
- `actions/outtakes_new_design_test.go` — new:
  - `TestOuttakesNewPickerDesign`: both pickers present with correct form
    actions/inputs; EN headings; FR wording exactly as quoted in the issue
    (apostrophes asserted HTML-escaped); de/nl render without raw t() keys.
  - `TestOuttakesNewPickerNotShownWithAnimal`: pickers hidden once an animal
    is selected; "Outcome of animal" header shown.

## Verification

- `GO_ENV=test go test ./actions -run TestOuttakesNew -count=1` → ok.
- `TestTemplateVariantStructuralParity`: no outtakes/new drift (neither NEW
  nor KNOWN); remaining NEW drift = pre-existing config/_sync_form,
  guest.plush.*, sync_targets/_form pairs (untouched).
- Full suite `go test -count=1 -race -cover ./...`: only the two documented
  pre-existing grifts failures (parity debt pairs above,
  TestMigrationsReplayOnEmptyDatabase); actions 50.1% coverage.
- `go vet`, `staticcheck` clean; gocognit 80 / gocyclo 66 (baseline).
- e2e (agent-browser, admin session):
  - `/outtakes/new` EN: h4 "Select animal for outcome" + "Select cage for
    outcome"; forms GET `/outtakes/new/` and GET `/outtakes/cage`; "See all
    outcomes" button — structurally identical to `/cares/new`
    ("Select animal for care" / "Select cage for care", both GET `/cares/new/`).
  - Animal flow: `?animal_year_number=1592/26` → pickers hidden, "Outcome of
    animal 1592/26", species translated ("Canada Goose"), ring input, Cancel.
  - Cage flow: submitting the cage picker with `Enclos Renards` →
    `/outtakes/cage?cage=Enclos+Renards` → "Outtake for cage Enclos Renards
    (8 animal(s))" with all 8 animals listed.
  - FR: «Sélectionnez l'animal pour l'issue» / «Sélectionnez la cage pour
    l'issue» (exact issue wording), labels «Numéro de l'animal»/«Cage»,
    buttons «Suivant», «Voir toutes les issues»; outcome form in French
    («Issue de l'animal», «Bernache du Canada», «Arrivé le», «Annuler»,
    localized form partial with «Adresse, lieu précis»).
  - DE («Tier für Ausgang auswählen»/«Käfig für Ausgang auswählen») and NL
    («Dier selecteren voor afloop»/«Kooi selecteren voor afloop») render
    localized.

No DB changes; no e2e rows created.
