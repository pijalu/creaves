-- Issue #199-17: heat source and O2 are properties of the animal, not of
-- individual care entries. Add them to the animals table (additive,
-- backward-compatible). Existing care rows keep their historical
-- heat_source/oxygen values untouched — no backfill, by decision.
ALTER TABLE `animals`
  ADD COLUMN `heat_source` varchar(200) NULL DEFAULT NULL AFTER `feeding_period`,
  ADD COLUMN `oxygen` tinyint(1) NOT NULL DEFAULT 0 AFTER `heat_source`;
