-- Revert #205 item 2 species reference integration.
-- The animal-type assignments are intentionally NOT reverted: rows already
-- carried assignments from earlier mapping work that cannot be distinguished
-- from the ones this migration applied.

-- 1) restore the creaves_species typo state (base column + fr mirror)
UPDATE species SET creaves_species = 'Grimperau des jardins' WHERE ID = 'SP104';
UPDATE translations SET value = 'Grimperau des jardins' WHERE table_name = 'species' AND record_id = 'SP104' AND field = 'creaves_species' AND locale = 'fr';

-- 2) remove the three reference rows added by the up migration
DELETE FROM translations WHERE table_name = 'species' AND record_id IN ('SP498', 'SP499', 'SP500');
DELETE FROM species WHERE ID IN ('SP498', 'SP499', 'SP500');
