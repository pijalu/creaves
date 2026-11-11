# Fix archive — 2026-09-03 — API key visible in Creaves and Creaves Console

## Original report (bugs.md)
> ## creaves-console / creaves: The API key must be visible for both
> Creaves and creaves-console should show the API key to allow the user to copy it/validate it

## Problem
Pairing a Creaves instance with the Console requires comparing the API key on
both sides. Creaves rendered the key as a `type="password"` input (never
readable, not prefilled), and the Console displayed only the bare 8-char key
prefix without the `creaves_` scheme prefix, making cross-app validation
error-prone.

## Fix

### Creaves (commit `b4ef7b2`, branch `feature/i18n`)
- `templates/config/show.plush.html`: API key row now renders a readonly
  monospace text input (`#webhook-api-key-display`) containing the real key,
  click-to-select, plus a copy button (`navigator.clipboard`) and a prefix-match
  hint. Empty key shows the existing `config.show.api-key-not-set` text.
- `locales/config.{en-us,fr,de,nl}.yaml`: new key
  `config.show.api-key-prefix-hint` explaining the prefix comparison after
  `creaves_`.
- `templates/config/_form.plush.{html,de,nl}.html`: API key input changed
  `type="password"` → `type="text" font-monospace`, prefilled with
  `settings.WebhookAPIKey`, `autocomplete="off"`, help text explains that
  clearing the field keeps the current key (Update handler's
  blank-preserves-current logic at `actions/configs.go` unchanged).

### Creaves Console (commit `0cd8a5a`, branch `main`)
- `templates/webhook_api_keys/{index,show}.plush.{html,fr,de,nl}.html`: prefix
  rendered as `creaves_<prefix>…` so it can be compared directly with the full
  key shown on the Creaves side.

## Test approach and validation
- Templates/locales only — no Go code changed; existing suites cover handlers.
- Quality gates run separately, both repos:
  - Creaves: `go vet ./...` clean; `staticcheck ./...` only pre-existing
    findings (capitalized errors, unused helpers — none in touched files);
    `gocognit -over 15 .` / `gocyclo -over 12 .` only pre-existing hotspots
    (buildEventPayloadWithTranslations etc.); full sqlite-tagged
    `go test -count=1 -race -cover ./...` fails only on the known pre-existing
    set (TestPublishAnimalHelpers, TestBuildEventPayload_*, TestLocalizeAnnual*,
    TestReportsAnnualStats, TestRunResync*, TestStartResyncRunCommittedBeforeReturn,
    grifts TestTemplateVariantStructuralParity). Parity test shows no NEW drift
    for edited config forms; "variant absent" entries for `config/*.fr` are
    pre-existing debt (config templates never had FR variants — FR falls back to
    the EN base form, show page uses `t()` keys so it is translated).
  - Console: `go vet ./...` clean; `staticcheck` pre-existing only;
    gocognit/gocyclo pre-existing hotspots only;
    `go test -count=1 -race -cover -tags sqlite ./...` all green
    (actions 55.3%, models 61.0%).
- agent-browser validation (both apps, live dev servers):
  - Creaves `/config` show: key visible as `creaves_e157db6c-…`, copy button,
    EN + FR hint text rendered.
  - Creaves `/config/{id}/edit/`: input `type=text`, value prefilled with
    `creaves_e157db6c-429e-47ae-bb35-9c7452be4e03`; help text verified in EN,
    DE, NL (FR uses EN base form — pre-existing gap, noted above).
  - Console `/webhook_api_keys/` index and `/webhook_api_keys/{id}` show: prefix
    displayed as `creaves_e157db6c…` in EN, FR, DE, NL — matches the Creaves key
    prefix exactly (live pairing with instance BigMac.local confirmed).

## Residual notes
- Config form FR variants: created as part of the template-variant inventory
  completion (commit `bbc77e4`, `feat(i18n): complete template variant
  inventory`), which fixed all 75 "variant absent" entries and the pre-existing
  guest/new.plush.fr.html drift. TestTemplateVariantStructuralParity now passes.
- Raw console key remains visible only once on the `created` page (by design,
  bcrypt stored); this fix addresses prefix validation, not full-key retrieval.
