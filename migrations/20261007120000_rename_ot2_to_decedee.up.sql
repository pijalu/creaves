-- #205 item 1 (https://github.com/pijalu/creaves/issues/205):
-- rename the canonical dead-outtake type from 'DCD' to 'Décédé', the name
-- used by the production reference dump.
--
-- Why a dedicated migration: on fresh installs the startup seed reconciles
-- dump rows against existing ones by normalized name and only absorbs
-- prefix drift of >= 5 characters — 'dcd' (3 chars) can never match
-- 'decede', so the dump row 'Décédé' was re-inserted as an orphan next to
-- the coded canonical row (the "entrée complémentaire" from the ticket).
-- Renaming the canonical row to 'Décédé' makes the reconcile an exact
-- match, and this migration folds any orphan that older seeds already
-- created into the coded OT2 row before renaming it.
--
-- Step 1 — fold an uncoded dump 'Décédé' row into the coded OT2 row.
-- Repoint the outtakes foreign key first.
UPDATE outtakes o
  JOIN outtaketypes src ON src.id = o.outtaketype_id
    AND src.name = 'Décédé' AND (src.code IS NULL OR src.code = '')
  JOIN outtaketypes tgt ON tgt.code = 'OT2' AND tgt.id <> src.id
  SET o.outtaketype_id = tgt.id;

-- Re-key the orphan's translations onto the OT2 row, dropping conflicting
-- target-side values first (unique key table/record/field/locale; legacy
-- dump values win, mirroring 20261002090000_repair_dcd_translations).
DELETE t FROM translations t
  JOIN outtaketypes tgt ON tgt.id = t.record_id AND tgt.code = 'OT2'
  JOIN translations legacy ON legacy.table_name = t.table_name
    AND legacy.field = t.field AND legacy.locale = t.locale
    AND legacy.record_id IN (
      SELECT id FROM outtaketypes
      WHERE name = 'Décédé' AND (code IS NULL OR code = ''))
  WHERE t.table_name = 'outtaketypes';

UPDATE translations t
  JOIN outtaketypes src ON src.id = t.record_id
    AND src.name = 'Décédé' AND (src.code IS NULL OR src.code = '')
  JOIN outtaketypes tgt ON tgt.code = 'OT2' AND tgt.id <> src.id
  SET t.record_id = tgt.id
  WHERE t.table_name = 'outtaketypes';

-- Drop the now translation-less orphan row (JOIN form: MySQL forbids
-- selecting from the delete target in a subquery).
DELETE src FROM outtaketypes src
  JOIN outtaketypes tgt ON tgt.code = 'OT2' AND tgt.id <> src.id
  WHERE src.name = 'Décédé' AND (src.code IS NULL OR src.code = '');

-- Step 2 — rename the canonical row to the dump/production name.
UPDATE outtaketypes SET name = 'Décédé', updated_at = NOW()
  WHERE code = 'OT2' AND name = 'DCD';

-- Safety net: legacy centers whose OT2 row never received the code.
UPDATE outtaketypes SET name = 'Décédé', updated_at = NOW()
  WHERE name = 'DCD';
