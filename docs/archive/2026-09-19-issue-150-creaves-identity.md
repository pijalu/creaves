## #150 — Fiche d'identité du CREAVES — SPEC RECEIVED, CLEAR
**Source:** https://github.com/pijalu/creaves/issues/150 (body empty on GitHub — spec provided by maintainer in session)
**Spec:** Add to config a table with general CREAVES details:
`Nom du CREAVES`, `Nom de l'ASBL`, `Numéro BCE`, `Adresse`, `Numéro de compte`, `Site`.
Shown on the QR/public page (guest view) in an **"À propos de &lt;nom du creaves&gt;"** entry in the menu bar.
**Plan:**
1. **Storage — no migration**: extend `models.ConfigSettings` (JSON blob in `config.settings`) with 6 additive string fields: `CenterName`, `AsblName`, `BceNumber`, `Address`, `AccountNumber`, `Website`. Zero-value safe for existing rows (missing JSON keys → empty strings).
2. **Config form**: new "Identité du CREAVES" section in `templates/config/_form.plush.{html,fr,de,nl}` (6 text inputs bound `Settings.*`), following the existing webhook section pattern.
3. **Binding**: `actions/configs.go` Update (settings construction ~line 368) + any partial-update path (~line 193 area) read the 6 new `Settings.*` params. Identity fields are NOT secret — no redaction change needed in `MarshalJSON`.
4. **Guest page**: `actions/guest.go` `GuestNew`+`GuestCreate` load `CurrentConfigGet().GetSettings()` and set context (e.g. `centerSettings`). `templates/guest.plush.{html,fr,de,nl}` navbar: right-side "À propos de &lt;CenterName&gt;" link (localized; plain "À propos" fallback when name unset) toggling a Bootstrap collapse card with the identity table — rows rendered only for non-empty fields; whole entry hidden when all 6 fields empty (guest view stays clean by default).
5. **i18n**: `locales/` keys for "About", field labels (×4 locales).
**Test:**
- Unit: `ConfigSettings` JSON round-trip with new fields (extends `models/config_test.go`).
- Handler (GO_ENV=test): configs Update persists the 6 fields; GET /guest/ renders about block when identity set and omits it when empty.
- e2e (agent-browser): fill identity in config → save → open /guest/ → "À propos" entry shows table; verify ×4 locales + hidden-when-empty.
**Validation:** `go test ./...`; agent-browser flows; 4 locales.
**DONE ✅ commit 6bcbe7f** — ConfigSettings 6 identity fields (no migration, zero-value safe, not secret); identity section in config form ×4; bound in BOTH configs.go paths (Create ~373, Update ~475); guest navbar "About <name>" + collapse panel (one row per non-empty field) injected in both guest render helpers (form + status view); hidden entirely when empty; labels hardcoded per locale template (guest pages have no i18n keys). Tests: JSON round-trip, hidden/shown handler, Update persists. e2e: form fill → save → localized "À propos de <name>" + panel + collapse toggle; fields cleared → block gone (dev config restored). Note: identity injected via guestSetCenterIdentity in guestRenderNew/guestRenderShow — no auth, reads CurrentConfigGet cache.
