-- Restores the legacy English display names that the up step replaced; see
-- the up header for why this is intentionally partial.
UPDATE outtaketypes SET name = CASE code WHEN 'OT5' THEN 'Lost' WHEN 'OT6' THEN 'Stolen' WHEN 'OT7' THEN 'Other outcome' ELSE name END WHERE code IN ('OT5','OT6','OT7');
