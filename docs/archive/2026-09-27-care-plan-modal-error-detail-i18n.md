# BUG: care-plan modal hid server validation details; escaped apostrophe in fr/de/nl JS

**Date found**: 2026-09-27 (functional E2E, feature/care-expert @ b9b6a20)
**Severity**: minor (UX / i18n display)

## Symptom

Saving an invalid animal plan in the "New/Edit plan" modal on the animal page
showed only the generic localized message ("Saving failed — check the values." /
"Échec de l'enregistrement…"), discarding the actionable server payload, e.g.

```json
{"errors":{"action_payload":["careplan: feeding payload needs caretype_id (§4.2)"]}}
```

The user had no way to learn that a feeding plan needs `caretype_id`, or that an
observation plan needs `prompt`. Additionally, in the fr variant the translated
prefix rendered the HTML-escaped apostrophe literally in the error box
("Échec de l&#39;enregistrement…"), because plush HTML-escapes `<%= %>` output
inside the inline `<script>`.

## Root cause

`templates/animals/show.plush{,.fr,.de,.nl}.html`, `save()` handler: the
non-ok branch did `r.json().then(function (j) { showErr("<generic>") })` — the
parsed error object `j` was thrown away. The `&#39;` artifact came from plush
auto-escaping the translated string inside the JS string literal.

## Fix plan

1. In all **four** locale variants (all-language rule): extract the detail from
   the parsed body — `j.errors` (map of field → messages) joined, or `j.error` —
   and append it to the localized prefix (`prefix + ' — ' + detail`).
2. Wrap the translated prefix in `raw(t(...))` inside the script literals so
   apostrophes render correctly (translations contain no `"`; JS strings stay safe).
3. Test approach: HTTP test already covers the 422 payload shape; validation of
   the UI is E2E: per language, submit an invalid feeding payload via the modal
   and read `#carePlanModalError` textContent.

## Evidence of fix

- en-US: `Saving failed — check the values. — careplan: feeding payload needs caretype_id (§4.2)`
- fr: `Échec de l'enregistrement — vérifiez les valeurs. — careplan: feeding payload needs caretype_id (§4.2)` (proper apostrophe)
- de: `Speichern fehlgeschlagen — Werte prüfen. — careplan: feeding payload needs caretype_id (§4.2)`
- nl: same patched code path (identical diff applied; key verified in `locales/care_plan.nl.yaml`).
- Suite: `go test -count=1 -race -cover ./...` all ok (template compile covered by
  `TestCarePlanI18N*` render tests).

**Status**: fixed & verified — closed 2026-09-27.
