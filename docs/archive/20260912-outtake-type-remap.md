# Outtake type OT remap and translations

## Observed behavior

`bugs.md` documented an incorrect OT1–OT7 mapping and incomplete translations. Existing startup logic mapped OT5–OT7 to `Lost`, `Stolen`, and `Other outcome`, while translation artifacts described the intended rows as release, death, euthanasia, transfer, dead-on-arrival, adoption, and duplicate.

## Fix

- Canonical startup rows now use the documented mapping:
  - OT1 `Relacher`
  - OT2 `DCD`
  - OT3 `Euthanasier`
  - OT4 `Transferer`
  - OT5 `Mort à l'arrivée avant l'encodage`
  - OT6 `Adoption`
  - OT7 `Doublon`
- Canonical descriptions and status fields match the French translation artifacts.
- Added additive migration `20260912130000_remap_outtaketype_codes` to remap existing stable codes, names, descriptions, status fields, and legacy translation record IDs without deleting data.
- Added regression expectations for all seven canonical rows.

## Validation

- `go test ./grifts -run 'TestCanonicalOuttakeTypes|TestStartup' -count=1`
- `go test ./models ./actions ./grifts`
- `git diff --check`
