-- #205 item 15: normalize corrupt legacy feeding windows onto a valid date.
--
-- Animals fed through the pre-care-plan UI stored their feeding window with
-- a Go zero-time date ('0001-01-01 HH:MM:SS' — year 1 is outside the MySQL
-- DATETIME range, which starts at 1000-01-01). Any later UPDATE of such a
-- row fails with "Error 1292 (22007): Incorrect datetime value" — the
-- outtake crash reported in the ticket.
--
-- The TIME component is live data: the §8.1 care-plan converter expands the
-- legacy window (feeding_start + n×feeding_period ≤ feeding_end) into the
-- animal's exact feeding slots (docs/care-expert.md §8.1 step 2), and the
-- legacy day plan (calculateFeeding) reads only Hour()/Minute(). An earlier
-- version of this migration set the columns to NULL; that destroyed the
-- window before the converter could read it, degrading every converted
-- feeding plan to a single 09:00 slot flagged "à vérifier".
--
-- Fix: rebase the corrupt date onto 1970-01-01, KEEPING the time-of-day.
-- 1970-01-01 is a valid MySQL DATETIME, so the outtake UPDATE crash is gone;
-- the time component survives, so the converter derives the exact schedule.
-- FeedingStartFmt/FeedingEndFmt and calculateFeeding read only the time —
-- rebasing the date part is behavior-preserving.
--
-- TIMESTAMP(date, time-expr) builds the rebased datetime; MAKETIME extracts
-- the time from the stored '0001-01-01 HH:MM:SS'. Each column is rebased
-- only when IT is corrupt: a valid companion is left byte-identical (the
-- CASE keeps the current value when the column is already ≥ 1000-01-01 or
-- NULL). Strict mode rejects '0000-00-00' literals, but the stored value
-- reads back fine and HOUR()/MINUTE()/SECOND() extract from it.
UPDATE animals
SET feeding_start = CASE
        WHEN feeding_start IS NOT NULL AND feeding_start < '1000-01-01'
        THEN TIMESTAMP('1970-01-01', MAKETIME(HOUR(feeding_start), MINUTE(feeding_start), SECOND(feeding_start)))
        ELSE feeding_start END,
    feeding_end   = CASE
        WHEN feeding_end IS NOT NULL AND feeding_end < '1000-01-01'
        THEN TIMESTAMP('1970-01-01', MAKETIME(HOUR(feeding_end), MINUTE(feeding_end), SECOND(feeding_end)))
        ELSE feeding_end END
WHERE (feeding_start IS NOT NULL AND feeding_start < '1000-01-01')
   OR (feeding_end   IS NOT NULL AND feeding_end   < '1000-01-01');
