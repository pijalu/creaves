-- Revert #199-11: restore a NOT NULL rating column (NULLs collapse to 0 =
-- neutral) to keep the rollback schema-valid.
ALTER TABLE `outtaketypes`
  MODIFY COLUMN `rating` int NOT NULL DEFAULT 0;
