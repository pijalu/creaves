# E2E fixture expectations (plan §7.2)

Fixed numbers produced by `buffalo task db:seed:e2e` in both apps on fresh
databases. Every browser assertion in `run.sh` traces back to this file.

## Creaves (instance A) — animals

| id | year | species | type | age | entry cause | outtake | ring |
|----|------|---------|------|-----|-------------|---------|------|
| 250001 | 2025 | E2E_Hedgehog | E2E_TA | E2E Juvenile | E2E_EC1 | E2E_REL | E2E-RING-001 (city `E2E City; "Nord"`) |
| 250002 | 2025 | E2E_Hedgehog | E2E_TA | E2E Juvenile | E2E_EC1 | E2E_DCD | E2E-RING-002 |
| 250003 | 2025 | E2E_Sparrow | E2E_TB | E2E Adult | E2E_EC2 | E2E_REL | E2E-RING-003 |
| 250004 | 2025 | E2E_Sparrow | E2E_TB | E2E Adult | E2E_EC2 | — | E2E-RING-004 |
| 250005 | 2025 | E2E_Newt | E2E_TB | E2E Juvenile | E2E_EC1 | **E2E_ERR** | E2E-RING-005 |
| 250006 | 2025 | E2E_NOSPEC | E2E_TA | E2E Juvenile | E2E_EC1 | — | NULL ring/gender/city |
| 250007 | 2025 | E2E_Newt | E2E_TB | E2E Adult | E2E_EC2 | — | E2E-RING-007 |
| 240008 | 2024 | E2E_Hedgehog | E2E_TA | E2E Juvenile | E2E_EC1 | — | E2E-RING-008 |
| 240009 | 2024 | E2E_Sparrow | E2E_TB | E2E Adult | E2E_EC2 | E2E_REL | E2E-RING-009 |

Taxonomy rows: Hedgehog = class E2E_Mammalia, agw E2E_G1, subside E2E_SG
("E2E Group A"), native E2E_NS ("E2E Native"); Sparrow = E2E_Aves, E2E_G2, no
subside/native; Newt = E2E_Aves, E2E_G1, E2E_SG, E2E_NS. E2E_NOSPEC has no
species row. Outtake types: E2E_REL rating 1 alive; E2E_DCD rating -1 dead;
E2E_ERR error=1.

## Creaves /reports/annual — 2025 (error-outtake excluded → T=6 / outtake T=3)

| table | rows (category count %) | total |
|---|---|---|
| species | E2E_Hedgehog 2 33.3 · E2E_Sparrow 2 33.3 · E2E_Newt 1 16.7 · Unknown 1 16.7 | 6 |
| class | E2E_Aves 3 50.0 · E2E_Mammalia 2 33.3 · Unknown 1 16.7 | 6 |
| agw_group | E2E_G1 3 50.0 · E2E_G2 2 33.3 · Unknown 1 16.7 | 6 |
| subsidies_group | E2E Group A 3 50.0 · Unknown 3 50.0 | 6 |
| native_status | E2E Native 3 50.0 · Unknown 3 50.0 | 6 |
| entry_age | E2E Adult 3 50.0 · E2E Juvenile 3 50.0 | 6 |
| outtake_type | E2E_REL 2 66.7 · E2E_DCD 1 33.3 | 3 |
| outtake_rating | Dead 1 33.3 · Alive 2 66.7 | 3 |
| outtake_dead_released | Dead 1 33.3 · Released 2 66.7 | 3 |
| entry_cause | E2E_C1 3 50.0 · E2E_C2 3 50.0 | 6 |
| entry_cause_detail | E2E_N1 / E2E_C1 / E2E_D1 3 50.0 · E2E_N1 / E2E_C2 3 50.0 | 6 |
| entry_cause_nature | E2E_N1 6 100.0 | 6 |

## Creaves /reports/annual — 2024 (T=2 / outtake T=1)

species Hedgehog 1 / Sparrow 1; class Mammalia 1 / Aves 1; agw G1 1 / G2 1;
subsidies E2E Group A 1 / Unknown 1; native E2E Native 1 / Unknown 1; age
Adult 1 / Juvenile 1; outtake_type E2E_REL 1; rating Alive 1; dead/released
Released 1; entry_cause C1 1 / C2 1; detail N1/C1/D1 1 / N1/C2 1; nature
E2E_N1 2. All 50.0% (or 100.0% for single-row tables).

## /animals search expectations

- year=2025 → 7 rows; year=2024 → 2 rows; no filter → 9 rows
- animaltype E2E_TA → 4 rows (250001, 250002, 250006, 240008)
- species partial `E2E_Newt` → 2 rows (250005, 250007)
- entry_cause_id E2E_EC2 → 4 rows (250003, 250004, 250007, 240009)
- animalage E2E Adult → 4 rows (250003, 250004, 250007, 240009)
- ring partial `RING-00` → 8 rows (all except 250006); ring `E2E-RING-001` → 1
- outtaketype E2E_REL → 3 rows (250001, 250003, 240009); E2E_DCD → 1;
  E2E_ERR → 0 (error-outtakes excluded from the outtake-type filter)
- combination year=2025 + EC1 + Juvenile → 4 rows (250001, 250002, 250005,
  250006)

Search CSV export (`/animals/search/export.csv?year=2025`): 7 data rows,
`;`-delimited without quoting, no discovery-city column. Error-outtake animal
250005 (RING-005) IS included (error exclusion applies to reports and to the
outtake-type filter only). The `;`+quote escaping fixture (`E2E City; "Nord"`)
is exercised on the console export (see below), which does carry the city.

## run.sh behavior notes

`e2e/run.sh` is self-resetting and repeatable: its first step deletes all
non-fixture animals (anything not in 240008-240009/250001-250007), wipes
console instance A + creaves event_streams for A, fails any stuck resync run,
then triggers a fresh resync. The contract section's wizard-created animal is
removed again before the final resync assertion so counts stay at 9.

## Console — instance B rows (seeded directly)

2025: 950001 E2EB_Fox (Mammalia/G1/SGB/NSB, Adult, C1/D1/N1, REL) ·
950002 Fox (no outtake) · 950003 E2EB_Owl (Aves/G2, Juvenile, C2/N2, city
`E2EB City; "Sud"`, NULL ring) · 950004 Owl (DCD dead) ·
950005 all category fields NULL.
2024: 940006 Fox (REL).

Console /consolidated_animals list filters (all years): A entry cause
`E2E_C1 ⇨ E2E_D1` → 5 rows (250001, 250002, 250005, 250006, 240008).

The “all centers” report (below) buckets by label string, so A and B rows
stay separate (E2E_Aves 4 / E2EB_Aves 2 …); the shorthand sums quoted there
(Aves 6, Mammalia 4, REL 3, DCD 2, Juvenile 6, Adult 5) are cross-instance
totals, not literal row values.

## Console /reports/annual — 2025, scope e2e-instance-a (no error exclusion; T=7 / outtake T=4)

species: Hedgehog 2 · Sparrow 2 · Newt 2 · Unknown 1 → 7
class: Aves 4 · Mammalia 2 · Unknown 1 → 7
agw: G1 4 · G2 2 · Unknown 1 → 7
subsidies: E2E_SG 4 · Unknown 3 → 7 (group id stored, not name)
native: E2E_NS 4 · Unknown 3 → 7
age: Juvenile 4 · Adult 3 → 7
outtake_type: REL 2 · DCD 1 · ERR 1 → 4
rating: Alive 2 · Dead 1 · Unknown 1 → 4
dead/released: Released 3 · Dead 1 → 4 (ERR has NULL dead → Released)
entry_cause: `E2E_C1 ⇨ E2E_D1` 4 · `E2E_C2 ⇨` 3 → 7 (producer Fmt format)
detail: E2E_D1 4 · Unknown 3 → 7
nature: E2E_N1 7 → 7

## Console /reports/annual — 2025, scope e2e-instance-b (T=5 / outtake T=2)

species: Fox 2 · Owl 2 · Unknown 1; class: Mammalia 2 · Aves 2 · Unknown 1;
agw: G1 2 · G2 2 · Unknown 1; subsidies: SGB 2 · Unknown 3; native: NSB 2 ·
Unknown 3; age: Adult 2 · Juvenile 2 · Unknown 1; outtake_type: REL 1 ·
DCD 1 (T=2); rating: Alive 1 · Dead 1; dead/released: Released 1 · Dead 1;
entry_cause: C1 2 · C2 2 · Unknown 1; detail: D1 2 · Unknown 3; nature:
N1 2 · N2 2 · Unknown 1.

## Console /reports/annual — 2025, all centers (T=12 / outtake T=6)

species: 2 each Hedgehog/Sparrow/Newt/Fox/Owl + Unknown 2 → 12
class: Aves 6 · Mammalia 4 · Unknown 2
agw: G1 6 · G2 4 · Unknown 2
subsidies: Unknown 6 · E2E_SG 4 · SGB 2
native: Unknown 6 · E2E_NS 4 · NSB 2
age: Juvenile 6 · Adult 5 · Unknown 1
outtake_type: REL 3 · DCD 2 · ERR 1 → 6
rating: Alive 3 · Dead 2 · Unknown 1 → 6
dead/released: Released 4 · Dead 2 → 6
entry_cause: `E2E_C1 ⇨ E2E_D1` 4 · `E2E_C2 ⇨` 3 · E2EB_C1 2 · E2EB_C2 2 · Unknown 1
detail: E2E_D1 4 · E2EB_D1 2 · Unknown 6
nature: E2E_N1 7 · E2EB_N1 2 · E2EB_N2 2 · Unknown 1

2024 all centers: T=3 (A: 240008, 240009; B: 940006), outtake T=2 (REL ×2).

## Contract e2e

- Resync from Creaves UI (`/webhook_resync`) → 9 `animal_state` events →
  console instance A rows fully populated (incl. age, entry cause, outtake
  type/rating, translations for translated fixture values).
- Create/edit an animal in the Creaves UI → matching console row appears /
  updates with the new v2 fields.
- Rows created before payload v2 (or before resync) show NULL new columns and
  group into "Unknown" buckets until resynced — see `e2e/RESYNC_RUNBOOK.md`.
