-- #205 item 1 (https://github.com/pijalu/creaves/issues/205):
-- rename the canonical dead-outtake type from 'DCD' to 'Décédé', the name
-- used by the production reference dump, and collapse every duplicate
-- dead-type row into exactly one OT2-coded row named 'Décédé'.
--
-- Why a dedicated migration: the startup seed reconciles dump rows against
-- existing ones by normalized name and only absorbs prefix drift of >= 5
-- characters — 'dcd' (3 chars) can never match 'decede' — so centers can
-- carry a second dead-type row next to the coded canonical one. Two shapes
-- exist in the wild:
--   * an UNCODED 'Décédé' row left by older seeds next to the coded OT2
--     row (the "entrée complémentaire" from the ticket), and
--   * TWO OT2-CODED rows: 20261005160100 inserts its canonical 'Décédé'
--     row whenever no row is *named* 'Décédé', so a center whose canonical
--     row was still named 'DCD' ends up with a coded duplicate — the plain
--     rename then collides with the unique outtaketypes_name_idx.
-- This migration folds every duplicate dead-type row (coded OT2 duplicates
-- plus uncoded 'DCD'/'Décédé' rows) into a single survivor, then renames
-- that survivor to exactly 'Décédé'.
--
-- Survivor rule: among OT2-coded rows keep the one actually referenced by
-- outtakes; ties or no references fall back to the oldest created_at, then
-- the lowest id, so production FK history stays stable. Uncoded
-- 'DCD'/'Décédé' rows are always folded into the coded survivor; on the
-- corner case of a center with no OT2-coded row at all, the final safety
-- net just renames the legacy 'DCD' row in place.
--
-- Translation precedence: values of the dump-name row (the row *named*
-- 'Décédé', coded or not) override the survivor's own (mirroring
-- 20261002090000_repair_dcd_translations); every other folded row loses
-- against the survivor. Conflicting values are dropped first because of
-- the unique key table/record/field/locale.

-- Step 1 — dump-name translations win: drop the survivor-side values the
-- row named 'Décédé' is about to replace.
DELETE t FROM translations t
  JOIN (
    SELECT id FROM outtaketypes
    WHERE code = 'OT2'
    ORDER BY EXISTS (SELECT 1 FROM outtakes o
                     WHERE o.outtaketype_id = outtaketypes.id) DESC,
             created_at ASC, id ASC
    LIMIT 1
  ) tgt ON tgt.id = t.record_id
  JOIN outtaketypes src ON src.name = 'Décédé' AND src.id <> tgt.id
  JOIN translations legacy ON legacy.table_name = t.table_name
    AND legacy.field = t.field AND legacy.locale = t.locale
    AND legacy.record_id = src.id
  WHERE t.table_name = 'outtaketypes';

-- Step 2 — re-key the dump-name row's translations onto the survivor.
UPDATE translations t
  JOIN outtaketypes src ON src.id = t.record_id
    AND src.name = 'Décédé'
  JOIN (
    SELECT id FROM outtaketypes
    WHERE code = 'OT2'
    ORDER BY EXISTS (SELECT 1 FROM outtakes o
                     WHERE o.outtaketype_id = outtaketypes.id) DESC,
             created_at ASC, id ASC
    LIMIT 1
  ) tgt ON tgt.id <> src.id
  SET t.record_id = tgt.id
  WHERE t.table_name = 'outtaketypes';

-- Step 3 — the survivor's translations beat every other folded row's, and
-- folded rows never compete with each other: drop a folded translation
-- when the survivor already owns the slot or a lower-translations.id folded
-- row owns it (keeps one deterministic value per slot so step 4 cannot hit
-- the unique key).
DELETE t FROM translations t
  JOIN outtaketypes src ON src.id = t.record_id
  JOIN (
    SELECT id FROM outtaketypes
    WHERE code = 'OT2'
    ORDER BY EXISTS (SELECT 1 FROM outtakes o
                     WHERE o.outtaketype_id = outtaketypes.id) DESC,
             created_at ASC, id ASC
    LIMIT 1
  ) tgt ON tgt.id <> src.id
  JOIN translations rival ON rival.table_name = t.table_name
    AND rival.field = t.field AND rival.locale = t.locale
    AND rival.id < t.id
  JOIN outtaketypes rsrc ON rsrc.id = rival.record_id
  WHERE t.table_name = 'outtaketypes'
    AND ((src.code = 'OT2' AND src.name <> 'Décédé') OR src.name = 'DCD')
    AND ((rsrc.code = 'OT2' AND rsrc.name <> 'Décédé') OR rsrc.name = 'DCD'
         OR rsrc.id = tgt.id);

-- Step 4 — move the surviving folded translations onto the survivor.
UPDATE translations t
  JOIN outtaketypes src ON src.id = t.record_id
  JOIN (
    SELECT id FROM outtaketypes
    WHERE code = 'OT2'
    ORDER BY EXISTS (SELECT 1 FROM outtakes o
                     WHERE o.outtaketype_id = outtaketypes.id) DESC,
             created_at ASC, id ASC
    LIMIT 1
  ) tgt ON tgt.id <> src.id
  SET t.record_id = tgt.id
  WHERE t.table_name = 'outtaketypes'
    AND ((src.code = 'OT2' AND src.name <> 'Décédé') OR src.name = 'DCD');

-- Step 5 — repoint the outtakes foreign key from every folded row to the
-- survivor (the survivor subquery reads outtakes only through a
-- materialized derived table: MySQL forbids referencing the update target
-- in a live subquery).
UPDATE outtakes o
  JOIN outtaketypes src ON src.id = o.outtaketype_id
  JOIN (
    SELECT ot.id FROM outtaketypes ot
      LEFT JOIN (SELECT outtaketype_id FROM outtakes
                 GROUP BY outtaketype_id) refs
        ON refs.outtaketype_id = ot.id
    WHERE ot.code = 'OT2'
    ORDER BY (refs.outtaketype_id IS NOT NULL) DESC,
             ot.created_at ASC, ot.id ASC
    LIMIT 1
  ) tgt ON tgt.id <> src.id
  SET o.outtaketype_id = tgt.id
  WHERE src.code = 'OT2' OR src.name IN ('DCD', 'Décédé');

-- Step 6 — drop the now reference-less folded rows (JOIN form: MySQL
-- forbids selecting from the delete target in a subquery).
DELETE src FROM outtaketypes src
  JOIN (
    SELECT id FROM outtaketypes
    WHERE code = 'OT2'
    ORDER BY EXISTS (SELECT 1 FROM outtakes o
                     WHERE o.outtaketype_id = outtaketypes.id) DESC,
             created_at ASC, id ASC
    LIMIT 1
  ) tgt ON tgt.id <> src.id
  WHERE src.code = 'OT2' OR src.name IN ('DCD', 'Décédé');

-- Step 7 — rename the single remaining OT2-coded row to the dump/
-- production name (no-op when it already carries it).
UPDATE outtaketypes SET name = 'Décédé', updated_at = NOW()
  WHERE code = 'OT2' AND name <> 'Décédé';

-- Safety net: centers whose OT2 row never received the code. Guarded so it
-- can never collide with an existing 'Décédé' row on the unique name index.
UPDATE outtaketypes src
  LEFT JOIN outtaketypes existing ON existing.name = 'Décédé'
  SET src.name = 'Décédé', src.updated_at = NOW()
  WHERE src.name = 'DCD' AND existing.id IS NULL;
