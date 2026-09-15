-- DATA BACKFILL -- outtaketype rules from the canonical bugs.md table,
-- keyed by the stable OT codes assigned in 20260912120000_add_outtaketype_code.
-- Additive: only the two new rule columns are touched.
UPDATE outtaketypes SET excluded_native_statuses = 'NS2,NS3,NS4', location_mode = 'free' WHERE code = 'OT1';
UPDATE outtaketypes SET excluded_native_statuses = 'NS3', location_mode = 'list' WHERE code = 'OT4';
UPDATE outtaketypes SET excluded_native_statuses = 'NS1,NS3', location_mode = 'list' WHERE code = 'OT6';
