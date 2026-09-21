-- Revert #199-17: drop the animal-level heat source / oxygen columns.
ALTER TABLE `animals`
  DROP COLUMN `oxygen`,
  DROP COLUMN `heat_source`;
