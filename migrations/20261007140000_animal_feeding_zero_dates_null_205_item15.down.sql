-- No down: the zero-time datetimes ('0001-01-01 HH:MM') destroyed by the up
-- migration cannot be told apart from legitimately NULL windows afterwards,
-- and re-creating them would resurrect the outtake crash the cleanup fixes.
SELECT 1;
