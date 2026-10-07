-- #205 item 15: normalize corrupt legacy feeding windows to NULL.
--
-- Animals fed through the pre-care-plan UI stored their feeding window with
-- a Go zero-time date ('0001-01-01 HH:MM:SS' — year 1 is outside the MySQL
-- DATETIME range, which starts at 1000-01-01). Any later UPDATE of such a
-- row fails with "Error 1292 (22007): Incorrect datetime value" — the
-- outtake crash reported in the ticket.
--
-- The columns are frozen legacy data (bugs.md H3 / Phase 4: read-only form,
-- Update ignores crafted values, the day plan is driven by care rules), and
-- FeedingStartFmt/FeedingEndFmt already render the 07:00 / 22:00 defaults
-- (models.DEF_FEEDING_START/END) for NULL windows, exactly like unset ones.
-- NULL therefore restores writability without changing the default-window
-- display.
--
-- Range comparison (< '1000-01-01') instead of a zero-date literal: strict
-- mode rejects '0000-00-00' as a value on NO_ZERO_DATE servers.
UPDATE animals
SET feeding_start = NULL,
    feeding_end   = NULL
WHERE feeding_start < '1000-01-01'
   OR feeding_end   < '1000-01-01';
