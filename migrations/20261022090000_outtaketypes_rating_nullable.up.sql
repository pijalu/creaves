-- Issue #199-11: outtake-type rating becomes optional. OT7
-- (6be11a46-07c1-43c6-95d3-ae3630c8e5c0, "Doublon") must carry no rating:
-- make the column nullable (NULL = no rating) and clear OT7's rating.
-- Additive/backward-compatible: existing ratings keep their value, a NULL
-- rating simply hides the outcome badge/word in the UI.
ALTER TABLE `outtaketypes`
  MODIFY COLUMN `rating` int NULL DEFAULT NULL;
UPDATE `outtaketypes`
  SET `rating` = NULL
  WHERE `id` = '6be11a46-07c1-43c6-95d3-ae3630c8e5c0';
