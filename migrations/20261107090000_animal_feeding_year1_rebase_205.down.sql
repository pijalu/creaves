-- No down: rebased rows cannot be told apart from windows genuinely saved
-- under the year-2000 sentinel, and restoring '0001-01-01' values would
-- resurrect the outtake crash the up migration fixes.
SELECT 1;
