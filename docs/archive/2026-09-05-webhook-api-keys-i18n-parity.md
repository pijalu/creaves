# Fix archive — 2026-09-05 — webhook API keys unsynchronized translation (console)

## Original report (bugs.md)
> ## creaves-console: unsynchronized translation
> creave-console: Not all updates are done on all screens - eg: webhook API show
> "Schlüssel-Präfix:   creaves_6270d0b7…" in german/french/ - but english
> make more sense and does not show it (the full key is shown as expected).
>
> Check for other possible discrepencies

## Root cause
The API-key visibility feature (full key value with prefix fallback for legacy
keys) was applied to the EN templates only. The fr/de/nl variants of
`show.plush.*.html` still showed the obsolete "key prefix" row, and
`index.plush.{fr,de,nl}.html` still listed only the prefix.

## Fix (creaves-console commit 85f6d6b)
- `templates/webhook_api_keys/show.plush.{fr,de,nl}.html`: removed the prefix
  `<tr>` row ("Schlüssel-Präfix" / "Préfixe de clé" / "Sleutelvoorvoegsel");
  the API key value row with its translated "unavailable" fallback remains,
  matching EN.
- `templates/webhook_api_keys/index.plush.{fr,de,nl}.html`: the key column now
  uses the EN markup — full `key.KeyValue.String` when valid, otherwise the
  `creaves_<prefix>…` prefix plus a translated "(value unavailable)" note;
  column header renamed to the translated "API Key" (Clé API / API-Schlüssel /
  API-sleutel).
- Checked other webhook_api_keys templates (created/edit/new): already
  structurally identical across locales — no other discrepancies found.

## Validation
- `go vet ./...` — clean.
- `staticcheck ./...` — clean.
- `gocognit -over 15 .` / `gocyclo -over 12 .` — pre-existing entries only;
  no Go code changed.
- `CGO_ENABLED=1 go test -tags sqlite -count=1 ./...` — all packages pass.
- Browser verification (agent-browser) on http://127.0.0.1:3001 (admin/admin123):
  - German index: header "API-Schlüssel", full key `creaves_6270d0b7-…` shown.
  - German show: no "Schlüssel-Präfix"; "API-Schlüsselwert" shows the full key.
  - French index: header "Clé API", full key shown.
  - French show: no "Préfixe de clé"; "Valeur de la clé API" shows full key.
  - English index/show unchanged and correct.

## Follow-up (creaves-console commit 22b12ab)
  - `templates/webhook_api_keys/show.plush.{de,fr,nl}.html`: the EN
    unavailable-fallback also names the legacy prefix
    ("…(prefix creaves_<prefix>…)"); the locale variants omitted that clause.
    Added the translated "(Präfix …)" / "(préfixe …)" / "(prefix …)" note so
    the fallback markup is byte-identical in logic across all 4 locales
    (verified by diffing the plush `<% %>` tags per locale for index, show,
    new, edit, created — logic identical; only translated labels differ).

## Follow-up validation (2026-09-05, second pass)
  - `go vet ./...` — clean.
  - `staticcheck ./...` — clean.
  - `gocognit -over 15 .` — pre-existing entries only (UpdateFromPayload,
    installSafePopTxLogger, TestOutcomeHelpersAcceptValueAndPointer,
    DashboardIndex, SyncManagementIndex); template-only change.
  - `gocyclo -over 12 .` — pre-existing entries only; template-only change.
  - `CGO_ENABLED=1 go test -tags sqlite -count=1 -race -cover ./...` — all
    packages pass (actions 59.1%, models 58.4%).
  - Browser verification (agent-browser) on http://127.0.0.1:3001 (admin/admin123),
    locale switched via /lang/?lang=<locale>:
    - EN index: header "API Key", full key shown; show: "API key value" row,
      no prefix row.
    - DE index: header "API-Schlüssel", full key shown; show:
      "API-Schlüsselwert" full key, no "Schlüssel-Präfix" row.
    - FR index: header "Clé API", full key shown; show: "Valeur de la clé API :"
      full key, no "Préfixe de clé" row.
    - NL index: header "API-sleutel", full key shown; show: "API-sleutelwaarde"
      full key, no "Sleutelvoorvoegsel" row.
    - No render errors in any locale.
