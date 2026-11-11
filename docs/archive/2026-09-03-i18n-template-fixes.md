# i18n Template Fixes — 2026-09-03

## creaves-console: instances page untranslated in German and Dutch
**Observed:** `templates/instances/index.plush.de.html` and `templates/instances/index.plush.nl.html` were byte-identical to the English base template. All UI strings rendered in English.
**Fix:** Translated all UI strings (headings, table headers, buttons, modal text) following the FR variant pattern. DE: "Creaves-Instanzen", "Registrierte Quell-Installationen...", Name, Instanz-ID, Zuerst gesehen, Zuletzt gesehen, Letztes Ereignis, Tiere, Ereignisse, Schlüssel, Nie, Details, Löschen. NL: "Creaves-instances", "Geregistreerde broninstallaties...", Naam, Instance-ID, Eerst gezien, Laatst gezien, Laatste gebeurtenis, Dieren, Gebeurtenissen, Sleutels, Nooit, Details, Verwijderen.
**Validation:** Console test suite passes (`CGO_ENABLED=1 go test -tags sqlite -count=1 ./...`).
**Commit:** `311b888` in creaves-console.

## creaves-console: login page untranslated in French
**Observed:** `templates/auth/new.plush.fr.html` was byte-identical to the English base: "Login", "Authority Access", "Login", "Password", "Login".
**Fix:** Translated to French: "Connexion", "Accès autorité", "Identifiant", "Mot de passe", "Se connecter".
**Validation:** Console test suite passes.
**Commit:** `414465f` in creaves-console.

## creaves: admin menu missing Translations and Webhook resync entries in German and Dutch layouts
**Observed:** Base `templates/application.plush.html` (EN) and the FR variant contain admin dropdown entries "Translations" (`/translations`) and "Webhook resync" (`/webhook_resync`). The DE and NL layout variants lacked both entries.
**Fix:** Added entries to DE layout ("Übersetzungen" + "Webhook-Neusynchronisation") and NL layout ("Vertalingen" + "Webhook-hersynchronisatie"), mirroring the EN structure exactly.
**Validation:** `TestTemplateVariantStructuralParity` passes; creaves grifts suite passes.
**Commit:** `a7b41dd` in creaves.

## creaves-console: "Users" menu entry untranslated in French layout
**Observed:** `templates/application.plush.fr.html` rendered "Users" in the nav while all other entries were French.
**Fix:** Changed "Users" to "Utilisateurs".
**Validation:** Console test suite passes.
**Commit:** `e866813` in creaves-console.

## creaves: residual English labels in German animal form
**Observed:** `templates/animals/_form.plush.de.html` contained English "Date" labels (lines 176, 346, 541, 593, 830) where German should use "Datum".
**Fix:** Replaced all 5 occurrences of "Date" (3 `<label>` + 2 `<th>`) with "Datum", matching the established DE term used in `discoveries/index.plush.de.html`.
**Validation:** `TestTemplateVariantStructuralParity` passes.
**Commit:** `aab9082` in creaves.
