# Archive — #199-15 Media upload — no visible feedback of selected file

**Status**: ✅ DONE — commit `333e4d2` (branch `feature/open-issues-2026-10`, 2026-09-21)

## Observed
In attachments ("Choisir un fichier (photo ou vidéo)"), the chosen filename was not shown
until upload — the user couldn't tell a file was selected.

## Fix
The Bootstrap 4 `custom-file` label on the animal media tab (`templates/animals/show.plush*.html`,
all 4 locales) now displays the selected filename immediately:
- inline `onchange` on `#attachmentFile` writes `this.files[0].name` into the label and
  removes the `text-muted` style;
- the label carries a localized `data-placeholder` restored when the selection is cleared
  (text-muted re-applied).
No server change; the input keeps `required` so empty submits are still blocked by the browser.

## Tests
- e2e (agent-browser, admin, `/animals/10213#nav-media`): dispatched `change` with a
  DataTransfer file → label showed `e2e-photo.jpg` (EN) and `photo-fr.png` (FR), muted style
  removed; clearing the input restored the localized placeholder.
- Gates: `go vet` ✓, `staticcheck` ✓, gocognit 80 / gocyclo 66 (baselines), full suite: only
  the 2 pre-existing grifts failures. Template parity: animals/show stays at its KNOWN frozen
  drift (offset 2549, unchanged — identical edit in all 4 variants).
