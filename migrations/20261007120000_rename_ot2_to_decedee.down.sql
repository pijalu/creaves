-- Intentionally partial (see the up header): the folded duplicate rows
-- (uncoded orphans and coded OT2 duplicates) and their merged translations
-- cannot be reconstructed. Only the canonical display name is reverted.
UPDATE outtaketypes SET name = 'DCD', updated_at = NOW()
  WHERE code = 'OT2' AND name = 'Décédé';
