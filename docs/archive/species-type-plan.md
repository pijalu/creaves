# Species–Animal Type Relationship Plan

## 1. Objective and first deliverable

The first deliverable is the complete, reviewed species-to-animal-type mapping table. No schema migration, reception change, or runtime relationship implementation starts before this deliverable is accepted.

The first phase must produce clear results for the complete species list:

- enumerate all 484 species currently supplied by the application;
- assign exactly one animal type to every species;
- document evidence and confidence for every assignment;
- resolve every unmapped, duplicate, conflicting, or ambiguous species;
- produce a final report proving complete coverage;
- obtain domain approval for every non-exact classification.

Required first-phase result:

```text
species_total              484
mapped_total               484
approved_high_confidence   484
unmapped                   0
ambiguous                  0
duplicate_assignments      0
unknown_types              0
```

Only after these results are achieved should implementation proceed with reception filtering, species-to-type completion, database migration, and transactional animal-type deletion remapping.

Improve reception data entry so species and animal type remain consistent:

- Selecting an animal type filters species suggestions to matching species.
- Selecting a species automatically fills its animal type when the type is blank.
- Conflicting species/type combinations are rejected server-side.
- Existing reference data is migrated safely across installations whose UUIDs differ.
- Animal-type deletion requires transactional remapping of all dependent records.

The species-to-type mapping is a migration/backfill artifact. After links are populated, runtime behavior uses database foreign keys, not the mapping file.

## 2. Phase 1 — create and approve complete mapping table

This phase is mandatory and blocks all later implementation.

1. Extract the complete canonical species list from `grifts/create_species.csv`.
2. Extract all current animal-type names and description lists from startup reference data.
3. Generate a candidate mapping for every species using exact description matches first, then documented aliases and taxonomy evidence.
4. Write one mapping row for every species to `grifts/species_animaltype_mapping.csv`.
5. Write all evidence, competing candidates, and review decisions to `grifts/species_animaltype_mapping_review.csv`.
6. Review every non-exact assignment with a domain owner.
7. Resolve every conflict; no `ambiguous`, `medium`, or `unmapped` row may remain in the approved mapping.
8. Run completeness validation and publish the result report.
9. Freeze the approved mapping and checksum it in the same change as the audit file.

Phase 1 acceptance requires:

- exactly 484 approved rows;
- every input species present exactly once;
- exactly one assigned animal type per species;
- zero unmapped species;
- zero ambiguous species;
- zero duplicate assignments;
- zero unknown animal types;
- `confidence=high` and `review_status=approved` for every production row.

No database migration, seed repair, reception autocomplete, or animal-type deletion work begins until all acceptance conditions pass.

## 3. Current state

### Existing relationships

- `animals.animaltype_id` already references `animaltypes.id`.
- `species` has no animal-type foreign key.
- `animaltypes.default_species` is a single text value and cannot represent the complete relationship.
- Reception currently loads all species suggestions and uses `default_species` for type-change hints.
- Existing startup reference data uses UUIDs that may differ between installations.
- Species canonical names are stored in `species.creaves_species`.
- Animal type canonical names are stored in `animaltypes.name`.

### Current reference inventory

- 484 species in `grifts/create_species.csv`.
- 16 current animal types in startup reference data.
- Taxonomy fields available: class, order, family, AGW group, and related species metadata.
- Animal-type descriptions contain existing species lists and are the primary source for initial mapping review.

## 3. Deliverables

### Code and schema

- Add nullable `species.animaltype_id`.
- Add foreign key from `species.animaltype_id` to `animaltypes.id`.
- Add index on `species.animaltype_id`.
- Add `Species.AnimaltypeID` to the Go model.
- Add reverse relationship metadata if required by Pop usage.
- Add shared name-based mapping/resolution code.
- Add migration/backfill validation and reporting.
- Update reception suggestions and validation.
- Replace direct animal-type deletion with remapping workflow.
- Update seed repair behavior for empty species links.

### Mapping artifacts

Create:

- `grifts/species_animaltype_mapping.csv` — approved production mapping.
- `grifts/species_animaltype_mapping_review.csv` — working/audit evidence for review decisions.
- Optional generated validation report/checksum for release review.

The approved mapping must contain exactly one row for every species.

## 4. Mapping table design

### Approved mapping file

Suggested columns:

```text
species_id,species_name,animal_type_name,match_method,confidence,review_status,evidence,notes
```

Example:

```csv
SP1,Autour des palombes,Rapaces,type_description,high,approved,exact description match,
```

### Column rules

- `species_id`: source/dump identifier for traceability only; never used for FK matching.
- `species_name`: canonical `species.creaves_species` value.
- `animal_type_name`: canonical `animaltypes.name` value.
- `match_method`: exact description, approved alias, taxonomy review, or other documented method.
- `confidence`: release mapping must contain `high` only.
- `review_status`: release mapping must contain `approved` only.
- `evidence`: concise reason supporting classification.
- `notes`: exceptions, spelling decisions, or domain-review context.

### Review/audit file

Use the audit file to retain all candidate and review information:

```text
species_name
candidate_type
source_description_match
taxonomy_signal
conflicting_candidates
reviewer
review_date
decision
notes
```

The audit file may contain rejected, ambiguous, or superseded candidates. Those rows must not enter the approved mapping.

## 5. Mapping completeness and confidence gate

No schema backfill or runtime implementation is production-ready until all checks pass:

```text
species_total              484
mapped_total               484
approved_high_confidence   484
unmapped                   0
ambiguous                  0
duplicate_assignments      0
unknown_types              0
blank_species_names        0
```

Additional invariants:

- Every canonical species occurs exactly once in the approved mapping.
- Every mapped animal type exists by canonical name in the target database.
- No mapping relies on a source UUID.
- Every non-exact match has explicit evidence and reviewer approval.
- Mapping generation is deterministic and produces a reviewable checksum.

Any failed gate blocks migration and deployment.

## 6. Mapping production process

### Step 1 — Extract authoritative input

- Read all species from `grifts/create_species.csv`.
- Read all animal types and descriptions from the startup reference data.
- Record canonical names exactly as stored by the application.

### Step 2 — Normalize comparison values

Normalization is only for matching and diagnostics; it must not rewrite canonical stored names.

Apply:

- Unicode normalization.
- Trim surrounding whitespace.
- Normalize line endings and repeated whitespace.
- Normalize apostrophe variants.
- Case-insensitive comparison.
- Explicit, reviewed spelling aliases only where required.

### Step 3 — Match exact description entries

- Parse species names from animal-type descriptions.
- Match exact normalized species names.
- Record the source type description as evidence.
- Detect species listed under multiple types.

### Step 4 — Classify remaining species

For unmatched species, evaluate:

- `Class`.
- `Order`.
- `Family`.
- `AgwGroup`.
- Existing default species values.
- Existing application/domain naming conventions.

Likely strong signals include:

- `Rapace` → `Rapaces`.
- `Chauve-souris` → `chauves-souris`.
- `Mustélidé` → `Mustélidés`.
- `Ongulé` → `Ongulés`.
- `Amphibia`/`Reptilia` → `Reptiles, Amphibiens`.

These are candidate signals, not automatic approval by themselves.

### Step 5 — Domain review

Every non-exact match requires review by a knowledgeable maintainer/domain owner.

Review especially:

- Micro-mammals versus rodents/insectivores.
- Medium mammals spanning several types.
- Bird size categories.
- Limicoles, larids, anatids, and other waterbirds.
- Domestic or exceptional species.
- Species with conflicting description and taxonomy signals.

No ambiguous row may be forced into the release mapping.

### Step 6 — Freeze approved mapping

- Mark all approved rows `high` and `approved`.
- Generate completeness report.
- Generate checksum.
- Commit mapping and audit artifacts together.
- Treat later mapping changes as reviewed data migrations.

## 7. Database migration

### Migration A — add nullable relationship

Add:

```text
species.animaltype_id CHAR(36) NULL
```

Then add:

- index `species_animaltype_id_idx`.
- foreign key to `animaltypes.id`.

Keep nullable initially so deployment can safely handle unmapped rows during rollout.

Do not add the new column to the positional startup dump until the dump is intentionally regenerated. The existing dump has no column lists and is sensitive to column-order changes.

### Migration B — backfill from approved mapping

Use the approved mapping by canonical names:

1. Load species canonical name.
2. Find mapping row by normalized `species_name`.
3. Find local animal type by normalized canonical `animaltypes.name`.
4. Set `species.animaltype_id` to the local animal-type UUID.
5. Never use mapping `species_id` or animal-type UUIDs for assignment.
6. Preserve existing non-empty links unless an explicit repair operation is requested.
7. Report missing species, missing types, duplicate matches, and conflicts.
8. Abort transaction if release completeness checks fail.

The backfill must be idempotent and safe when source and destination UUIDs differ.

### Post-backfill validation

Require:

- 484 species rows checked.
- 484 valid links.
- zero null links.
- zero invalid foreign keys.
- zero duplicate mapping rows.
- zero unknown animal types.

Only after this validation should the column become `NOT NULL`, if operational review confirms that all future species creation paths also require a type.

## 8. Shared resolver and mapping code

Implement one resolver used by migration and seed repair:

```text
resolveSpeciesAnimalType(speciesName) -> local animal type ID
```

Rules:

1. Exact canonical species name.
2. Normalized canonical species name.
3. Explicit approved alias.
4. Otherwise unresolved.

Animal type resolution always uses canonical `animaltypes.name` and returns the local database UUID.

The resolver must expose diagnostics rather than silently guessing:

- resolved.
- missing species.
- missing type.
- duplicate mapping.
- conflicting existing link.
- invalid alias.

## 9. Seed and update behavior

### General rule

The mapping table is not runtime configuration and is not consulted for normal linked rows.

### Empty-link repair

Seed/update must detect:

```sql
species.animaltype_id IS NULL
```

For each empty link:

1. Resolve species through the approved mapping artifact/code.
2. Resolve animal type by canonical name in the current database.
3. Set the local UUID.
4. Preserve all existing non-empty links.
5. Report repaired and unresolved rows.

Expected output:

```text
species total: 484
already linked: 470
repaired from mapping: 14
unresolved: 0
conflicts: 0
```

### Strictness

- During transition, provide diagnostic/report mode.
- In production after mapping approval, use strict mode.
- Strict mode fails with non-zero status if any mapped species remains empty or unresolved.
- Seed must never silently classify an unknown species.
- Seed must not restore a deleted animal type solely because it exists in old startup data.

### Existing links

- Existing non-empty links are authoritative.
- Seed does not overwrite them automatically.
- A separate explicit repair command is required to recalculate links.

## 10. Animal-type deletion and remapping

Current direct deletion must be replaced before the FK is enforced.

### Required behavior

Deleting an animal type requires a replacement type. The operation must update every dependent record before deleting the source type.

Known dependent tables:

- `animals.animaltype_id`.
- `dosages.animaltype_id`.
- `species.animaltype_id`.
- Any future table with an animal-type foreign key.

### Transaction flow

1. Admin selects source type.
2. Admin selects replacement type.
3. Server loads and locks both rows where supported.
4. Validate source exists.
5. Validate replacement exists and differs from source.
6. Count dependent rows for confirmation.
7. Update all dependent foreign keys to replacement ID.
8. Verify no dependent row still references source.
9. Delete source type.
10. Commit transaction.
11. Invalidate reference-data caches.

Any failure rolls back every update and leaves source type intact.

### UI/API

Recommended routes:

```text
GET  /animaltypes/:id/remap
POST /animaltypes/:id/remap
```

The confirmation page must show:

- source type and replacement type.
- animal count.
- species count.
- dosage count.
- explicit irreversible-operation warning.

Direct `DELETE /animaltypes/:id` must either be removed or reject unless a replacement ID is supplied.

### Audit

Record:

- source ID/name.
- replacement ID/name.
- counts updated per table.
- acting user.
- timestamp.

## 11. Reception implementation

### Type-first flow

Update species suggestions to accept `animaltype_id`:

```text
GET /suggestions/animal_species?q=<term>&animaltype_id=<uuid>
```

Query only species whose `animaltype_id` matches. On type change:

- clear incompatible species.
- reload autocomplete data.
- retain species only if still valid.
- use `default_species` only as an optional initial hint.

### Species-first flow

Add a species lookup returning:

```json
{
  "species": "Hérisson",
  "animaltype_id": "local-uuid",
  "animaltype_name": "Hérissons / Insectivore"
}
```

When exact species is selected and type is blank, populate type.

If a species has no link, show a validation error rather than guessing.

### Server-side consistency

On create/update:

- species + blank type → fill type from species link.
- type + blank species → allowed only if species is not required by form policy.
- both matching → accept.
- both conflicting → reject with validation error.
- unknown species → reject or require explicit review, according to existing form policy.

Apply identical rules to batch reception.

## 12. Translation behavior

- Relationships use canonical names only during migration/repair.
- Localized labels are display values only.
- Translation rows continue to use local record IDs.
- Autocomplete may accept localized input, but it must resolve to canonical species before relationship lookup.
- Webhook payload behavior remains canonical and does not expose source UUID assumptions.

## 13. Testing plan

### Mapping tests

- Exactly 484 input species discovered.
- Exactly 484 approved output rows.
- Duplicate species rejected.
- Unknown type rejected.
- Missing species rejected.
- Normalization collision reported.
- Alias resolution requires explicit approval.
- Ambiguous candidates rejected from release artifact.
- Deterministic output/checksum verified.

### Migration tests

- Local animal-type IDs differ from source IDs.
- Name matching selects local IDs.
- Existing links are preserved.
- Empty links are repaired.
- Unmapped species abort strict migration.
- Migration rerun is idempotent.
- Invalid mappings roll back.

### Deletion/remapping tests

- Deletion without replacement rejected.
- Source equals replacement rejected.
- Animals, dosages, and species all remapped.
- No source references remain before deletion.
- Failure in one dependent update rolls back all updates.
- Existing foreign keys prevent unsafe direct deletion.

### Reception tests

- Type filters species suggestions.
- Changing type clears incompatible species.
- Selecting species fills blank type.
- Conflicting values are rejected server-side.
- Localized species input resolves correctly.
- Batch reception applies identical consistency rules.

## 14. Rollout order

1. Inventory and extract current species/type data.
2. Generate candidate mapping and audit file.
3. Review every non-exact classification.
4. Freeze 484-row approved high-confidence mapping.
5. Add nullable schema relationship and index.
6. Run name-based backfill in staging.
7. Validate complete coverage and differing-UUID behavior.
8. Update seed repair for empty links.
9. Implement transactional animal-type remapping/deletion.
10. Update runtime reception suggestions and server validation.
11. Run full migration, seed, unit, integration, and browser checks.
12. Optionally enforce `species.animaltype_id NOT NULL` after operational confirmation.
13. Deprecate `animaltypes.default_species` as a relationship mechanism.

## 15. Release blockers

Release is blocked by any of the following:

- fewer than 484 approved mappings.
- any ambiguous or medium-confidence mapping in production artifact.
- any empty species link after backfill.
- any mapping based on source UUID equality.
- direct animal-type deletion still possible without remapping.
- seed silently leaves empty links.
- runtime still treats the mapping CSV as the source of truth.
- conflicting species/type values accepted by server-side validation.
