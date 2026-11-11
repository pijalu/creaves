# Species ↔ Animal Type Misalignment Report & Fix Plan

Source: review of dev DB `creaves` @ 127.0.0.1 (10,142 animals, 494 species).
Comparison: `animals.animaltype_id` vs `species.animaltype_id` (corrected mapping, post Colombidés fix).
Date: 2026-09-16.

## ✅ EXECUTION STATUS (2026-09-16) — DONE on prod-copy `creaves_test`

Executed against `creaves_test` (loaded from `creaves-db-2026-09-12.gz`, users wiped, migrated, seeded; admin/admin recreated by seed):

- **Stage 0–1 done**: migrate + seed + `species:repair_links` (needed code fix: alias-aware type lookup — prod had renamed types "Reptiles et Amphibiens", "Hérissons et mammifères insectivores", "Lapins et Lièvres (Lagomorphes)"; added Lagomorphe alias). 484/484 mapped species linked.
- **Stage 1b done**: mapping CSV + generator rules corrected (commit `67f7e66`); 6 species re-linked (SP103/179/180/364/399/436).
- **Stage 2 done**: 118 high-confidence animal corrections applied (Colombidés ×3, Taupe ×8, Oreillard ×4, Ecureuil ×83, Castor ×4, Chats ×15, Blaireau ×1, Lapin domestique ×3).
- **Stage 3 done**: Lagomorphe alias resolved via name-based lookup (no row merge needed — legacy "Lapins et Lièvres (Lagomorphes)" row reused for both species + animals).
- **Stage 4 done**: Corvidés aligned to reference-data types (Pie bavarde/Corneille noire → Grands Oiseaux; Corbeau freux/Choucas des tours → Moyens Oiseaux) — reference dump descriptions are the authority; 177 animals corrected.
- **§3 borderline done**: aligned to species type for consistency (Merle noir, Grive, Alouette, Perruche ondulée → Moyens; colvert, Bernache, Héron, Faisan, Perdrix → Grands).
- **Stage 5 done**: 11 orphan species linked (Grimpereau des jardins + SP488–SP497); their 3 misaligned animals corrected. "Doublon"/"/"/UUID rows left as-is (data-cleanup, center decision).

**Final state**: 494/494 species linked, **0 animal↔species type mismatches** (was 295), 25 animals with free-text species names (Doublon etc.) untouched.
`go test ./grifts/` passes. Commits: `5f2e545` (Colombidés), `67f7e66` (misalignments + alias repair).

**Production rollout**: apply the same SQL stages (scripts: stage2/3 patterns in git history of this doc) or re-run: migrate → seed → `species:repair_links` → `species:fix_colombides` → stage-2/3 SQL. All statements idempotent.

## 1. High-confidence misalignments

### 1a. Animal-side errors (animal typed wrong, species mapping correct)

| Species | Current animal type | Potential fix type | Animals | In care |
|---|---|---|---|---|
| Pigeon ramier | Moyens Oiseaux | **Colombidés** | 2 | 0 |
| Tourterelle turque | Moyens Oiseaux | **Colombidés** | 1 | 0 |

### 1b. Mapping-side errors (species mapping wrong, animal typing correct)

| Species | Current species type | Potential fix type | Animals affected | Rule to change in `generate_species_animaltype_mapping.py` |
|---|---|---|---|---|
| Ecureuil roux (SP399) | Autres mammifères (…) | **Rongeurs** | 79 (+4 typed Hérissons) | Mammalia: `sciur` in family → Rongeurs |
| Castor d'Europe (SP103) | Autres mammifères (…) | **Rongeurs** | 4 | Mammalia: `castorid` in family → Rongeurs |
| Chat sylvestre (SP180) | Autres mammifères (…) | **Félidés** | 12 | (rule exists but unreachable: Domestic AGW short-circuits before Mammalia branch for SP179/SP180 — check rule order) |
| Chat haret (SP179) | Autres mammifères (…) | **Félidés** | 3 | same as above |
| Taupe (SP436) | Rongeurs | **Hérissons / Insectivore** | 8 | `Micro-mammifère` branch: add `talp` to insectivore family check (`erinace`, `soric`) |
| Oreillard (SP364) | Petits Oiseaux | **chauves-souris** | 4 | Chiroptera must match `chauves-souris` AGW rule before description-membership fallback; description match currently wins and is wrong (name listed under Petits Oiseaux in dump) |

### 1c. Legacy reference-type duplicates (rename/merge, not animal edits)

| Species group | Animal carries (legacy row) | Canonical row (species links) | Animals |
|---|---|---|---|
| Lièvre d'Europe, Lapin de Garenne | Lapins et Lièvres (Lagomorphes) `6392f594…` | Lagomorphe `b693e76e…` | 129 |
| (all old "Grands oiseaux" animals) | Grands oiseaux `1d2bf17b…` | Grands Oiseaux (seed dump name, same id `1d2bf17b…`) | — |

Root cause: startup-seed alias merge created a new "Lagomorphe" row instead of reusing the renamed legacy row; "Grands oiseaux"/"Grands Oiseaux" is the same id with drifted casing. Fix = SQL re-point of `animals.animaltype_id` + delete/merge duplicate type, or accept as semantic equivalents.

## 2. Conflicting evidence — needs domain decision BEFORE any fix

| Species | Mapping says (evidence) | Animals typed | n |
|---|---|---|---|
| Pie bavarde (SP355) | Grands Oiseaux (dump description) | Moyens Oiseaux | 162 |
| Corneille noire (SP137) | Grands Oiseaux (dump description) | Moyens Oiseaux | 3 |
| Corbeau freux (SP138) | Moyens Oiseaux (AGW corvidé rule) | Grands oiseaux | 11 |
| Choucas des tours (SP139) | Moyens Oiseaux (AGW corvidé rule) | Grands oiseaux | 1 |

Historical practice contradicts the mapping for all 4 corvids. Decide one consistent Corvidés rule (all Grands or all Moyens), then fix mapping + animals together. **177 animals — do not bulk-fix without sign-off.**

## 3. Borderline size-class judgment calls (low priority, center habit)

| Species | Species type | Animal type | n | In care |
|---|---|---|---|---|
| Merle noir | Moyens Oiseaux | Petits Oiseaux | 12 | 1 |
| Grive musicienne | Moyens Oiseaux | Petits Oiseaux | 1 | 0 |
| Alouette des champs | Moyens Oiseaux | Petits Oiseaux | 2 | 0 |
| Perruche ondulée | Moyens Oiseaux | Petits Oiseaux | 1 | 0 |
| Canard colvert | Grands Oiseaux | Moyens Oiseaux | 2 | 0 |
| Bernache du Canada | Grands Oiseaux | Moyens Oiseaux | 1 | 0 |
| Héron cendré | Grands Oiseaux | Moyens Oiseaux | 1 | 1 |
| Faisan de Colchide | Grands Oiseaux | Moyens Oiseaux | 1 | 1 |
| Perdrix rouge | Grands Oiseaux | Moyens Oiseaux | 1 | 0 |

## 4. Reference-data gaps (species table)

| Issue | Rows |
|---|---|
| Species with `animaltype_id IS NULL` (SP488–SP497: Canard/Poule domestique, Perruche alexandre, Inséparable de Fischer, Conure ×2, Diamant mandarin, Cochon vietnamien, Tortue grecque, Tarente de Maurétanie) — also missing from `create_species.csv` | 10 species |
| Animals with species name not in species table: "Doublon" (15), "/" (5), "Grimpereau des jardins" (3 — real species, missing), "ERREUR CORRECTION A FAIRE" (1), "Furet" (1), "Iguane Vert" (1), "Tourterelle" (1 — ambiguous), test rows (2) | 30 animals |
| Animal 4165: species field = literal type UUID `ad5dcb5a-d7f2-45b8-8dcb-a63722a72d27` (data-entry bug) | 1 animal |

## Fix plan (staged, reversible)

**Stage 0 — backup**: `mysqldump creaves animals species animaltypes > backup-pre-species-fix.sql`. All stages idempotent; each verified by re-running the review query.

**Stage 1 — mapping corrections (1b)**, file `creaves/grifts/species_animaltype_mapping.csv` + generator rules:
1. Add `talp` to insectivore families (Taupe → Hérissons / Insectivore).
2. Add Mammalia rules: `sciur` → Rongeurs, `castorid` → Rongeurs.
3. Fix rule order so Félidés rule catches SP179/SP180.
4. Force Chiroptera/AGW `Chauve-souris` to beat description-membership (Oreillard → chauves-souris).
5. Update CSV rows (approved/high) accordingly; update report counts.
6. Re-link: extend `species:fix_colombides`-style grift (or generalize to `species:fix_links <family>`) to overwrite wrong `species.animaltype_id`.

**Stage 2 — animal corrections for high-confidence rows (1a + animals whose species got re-mapped in Stage 1)**:
```sql
-- example per species; run inside transaction, verify counts first
UPDATE animals a JOIN species s ON s.creaves_species = a.species
JOIN animaltypes at ON at.name = '<fix type>'
SET a.animaltype_id = at.id
WHERE a.species = '<species>' AND a.animaltype_id != at.id;
```
Order: Colombidés (3) → Taupe (8) → Oreillard (4) → Ecureuil (83) → Castor (4) → Chats (15) → Blaireau (1).
Note: historical (outtake) animals change too — acceptable, type is a classification not a care record; confirm with domain owner.

**Stage 3 — Lagomorphe duplicate merge (1c)**: re-point 129 animals `6392f594…` → `b693e76e…`, then delete legacy row (check FK on `animals`, `dosages`, default_species first). Reconcile "Grands oiseaux" casing drift.

**Stage 4 — Corvidés (blocked)**: domain decision required (§2). Then one rule in generator + re-link species + update 177 animals.

**Stage 5 — reference gaps**: add SP488–SP497 to `create_species.csv` + mapping CSV (domestic → Autres mammifères / Domestique policy, reptiles → Reptiles, Amphibiens, psittacids → Petits/Moyens Oiseaux); add "Grimpereau des jardins"; manual cleanup of "Doublon"/"/" /UUID rows (case-by-case, with center).

**Stage 6 — verify**: re-run review queries (§1 join + unmatched-species list) → expect 0 high-confidence mismatches remaining; run `go test ./grifts/`; spot-check UI on an edited in-care animal.

**Out of scope / explicitly not fixed**: §3 judgment calls (leave to center habit), console-side consolidation (payload reads names at event time; next animal_state event per animal propagates corrected type name automatically).
