-- #205 (follow-up to animal_feeding_zero_dates_null_205_item15):
-- rebase residual year-1 feeding windows onto the year-2000 sentinel.
--
-- The previous fix NULLed every feeding window with a year-1 date
-- ('0001-01-01 HH:MM:SS'). The animal form still lets users pick feeding
-- times, and midnight (00:00) entered after that migration was written
-- again as '0001-01-01 00:00:00' by the old timeToNullTime sentinel. Such
-- rows scan back as Go's zero time.Time and crash any later UPDATE with
-- Error 1292 "Incorrect datetime value: '0000-00-00'" (outtake crash).
--
-- timeToNullTime now writes a year-2000 sentinel; this migration converts
-- the leftovers, preserving the user-configured time-of-day. Range
-- comparison (< '1000-01-01') instead of a zero-date literal: strict mode
-- rejects '0000-00-00' as a value on NO_ZERO_DATE servers.
UPDATE animals
SET feeding_start = CONCAT('2000-01-01 ', TIME(feeding_start))
WHERE feeding_start IS NOT NULL AND feeding_start < '1000-01-01';

UPDATE animals
SET feeding_end = CONCAT('2000-01-01 ', TIME(feeding_end))
WHERE feeding_end IS NOT NULL AND feeding_end < '1000-01-01';
