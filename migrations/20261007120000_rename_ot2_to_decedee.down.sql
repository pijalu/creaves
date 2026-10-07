-- Intentionally partial (see the up header): the merged orphan row and its
-- folded translations cannot be reconstructed. Only the canonical display
-- name is reverted.
UPDATE outtaketypes SET name = 'DCD', updated_at = NOW()
  WHERE code = 'OT2' AND name = 'Décédé';
