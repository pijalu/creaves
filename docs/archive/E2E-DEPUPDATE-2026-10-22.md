# E2E — feature/depupdate back-port fixes — 2026-10-22

Baseline: `creaves-db-2026-10-14.gz` restored into dev DB `creaves`, 45 pending
migrations applied (`buffalo pop migrate`). Admin access: promoted the dump's
`admin` login (admin/maintainer/approved=1, password reset to `admin`).

Test animal: id 10421 — Pigeon ramier 2026/2112, cage S24 (in care, no outtake).

## Test flake fix (pre-E2E)

- `TestUsersListSearchSort` root cause: the users listing default-sorts by
  `first_name`; the shared `creaves_test` DB had accumulated ~5967 users, most
  with an empty `first_name` that sorts before the test fixtures, so with
  `per_page=100` the fixture logins were off page 1 (`pos() == -1`).
- Fix: `per_page=10000` for all listing requests in the test (commit 3074ddb).
- Verified: `-count=10` green; full `go test ./actions` green (19.9s).

## Browser E2E matrix (all locales)

| Area | fr | en-US | de | nl |
|---|---|---|---|---|
| Landing renders, navbar localized | ✓ Rapports/Tableau de bord/… | ✓ Reports/Dashboard/… | ✓ Berichte/Übersicht/… | ✓ Rapporten/Overzicht/… |
| Navbar a11y labels (BUG-4 back-port) | ✓ "Basculer la navigation", "Rechercher un animal" | ✓ "Toggle navigation", "Search animal" | ✓ "Navigation umschalten", "Tier suchen" | ✓ "Navigatie in-/uitschakelen", "Dier zoeken" |
| Animal consultation — treatment tab buttons (46f14f5) | ✓ Voir/Éditer/Détruire | ✓ View/Edit/Destroy | ✓ Ansehen/Bearbeiten/Löschen | ✓ Bekijken/Bewerken/Verwijderen |
| Treatment create → redirect (826a691, 6c38832) | ✓ created, lands on `/animals/10421/edit#nav-treatment`, Traitement tab auto-selected, entry listed | (flow validated once in fr; UI identical per locale) | | |
| Edit form renders + localized placeholders | ✓ "Tapez un nom pour pré-remplir…" | ✓ "Type a name to prefill…" | ✓ "Namen eingeben, um das Formular…" | ✓ "Typ een naam om het formulier…" |
| Outtakes listing + naming (605e1b4, 72c8ded) | ✓ "Sorties" / "Créer une nouvelle sortie"; outtaketypes canonical (Décédé, no OT2/DCD dups) | ✓ "Outcomes" | ✓ "Ausgänge" | ✓ "Nieuwe afloop aanmaken" |
| Veterinary visits — vet account label (c5500a3) | ✓ "Vétérinaire (compte)" | ✓ "Veterinary (account)" | ✓ "Tierarzt (Konto)" | ✓ "Dierenarts (account)" |
| Veterinary visits — deactivated users hidden (8c597e8) | ✓ all 8 approved=0 users absent from select | — | — | — |
| Care templates validation errors (8839630) | ✓ "Le nom ne peut pas être vide." / "Le contenu ne peut pas être vide." | ✓ "Name can not be blank." / "Content can not be blank." | ✓ "Der Name darf nicht leer sein." / "Der Inhalt darf nicht leer sein." | ✓ "De naam mag niet leeg zijn." / "De inhoud mag niet leeg zijn." |
| Users admin — distinct creation title (a54e322) + listing | ✓ "Créer un nouvel utilisateur" / "Utilisateurs" | ✓ "Create a new user" / "Users" | ✓ "Neuen Benutzer anlegen" / "Benutzer" | ✓ "Nieuwe gebruiker aanmaken" / "Gebruikers" |

## Observations (not failures)

- bootstrap-table's built-in table search input shows English "Search"
  placeholder in de/nl (JS library default; not part of the back-ported fixes).
- Treatment-create form silently re-renders when no Horaire
  (morning/noon/evening) checkbox is ticked — no visible validation message.
  Pre-existing UX behavior, unchanged by this branch.

## Not covered

- bb9ffc0 "not approved account" login error localization — requires an
  unapproved account login attempt; covered by actions tests instead.

After the run the dev DB was restored from `creaves-dev-backup-pre-e2e-2026-10-22.gz`.
