# Issue #199-13 — ready_for_release visible in read mode + landing; FR wording

Date: 2026-09-21
Commit: `d432d70` on `feature/open-issues-2026-10`

## Request (from pijalu/creaves#199)

Item 13: the ready-for-release flag must be visible in consultation (read)
mode and on `/`; the FR helper phrase "L'animal peut être relâché une fois son
parcours de soins terminé" must become "L'animal peut être relâché son
parcours de soins est terminé".

## Implementation

- `templates/animals/show.plush*` (4 locales) — new read-mode list item on the
  general tab (after Cage) showing the localized "Ready for release"/"À
  relâcher"/"Auswilderungsbereit"/"Klaar voor vrijlating" label with the
  success dove badge, rendered only when the flag is set.
- `templates/landing/index.plush*` (4 locales) — dove badge next to the animal
  number button when the flag is set (localized `title`), mirroring the
  existing dashboard badge markup.
- `templates/animals/_form.plush.fr.html` — helper phrase drops "une fois":
  "L'animal peut être relâché son parcours de soins est terminé".

## Tests

- `actions/ready_for_release_test.go` — `TestAnimalReadyForReleaseReadModeVisible`
  toggles `ready_for_release` directly and asserts:
  - flag on → `badge-success` + "Ready for release" on `/animals/{id}` and a
    `badge-success` on `/`;
  - flag off → no flag label on the animal sheet, no
    `title="Ready for release"` badge on `/`.
- Full suite: `GO_ENV=test go test -count=1 -race -cover ./...` passes with
  only the two pre-existing grifts debts. No NEW template-parity drift on any
  touched template pair (animals/show, landing/index, animals/_form).

## e2e verification (agent-browser, dev DB; flag set on animal 10213 then reset)

- `/animals/10213` (EN): "Ready for release" text + 1 dove badge.
- `/` (EN): 1 `title="Ready for release"` badge (animal 10213 row).
- `/animals/10213/edit` (FR): new phrase present ("…relâché son parcours de
  soins est terminé"), old phrase ("une fois…") absent.
- `/` (FR): 1 `title="À relâcher"` badge.
- Flag reset to 0 and language reset to EN after verification.
