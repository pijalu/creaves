-- Revert #204 stay-duration backfill: not reversible — the backfill overwrote
-- NULL with computed values and there is no way to distinguish historical rows
-- from newer rows whose runtime-computed value happens to equal the same
-- formula. Rollback therefore leaves the data as-is (schema is unchanged);
-- the badge simply stops being rendered only if the code itself is reverted.
SELECT 1;
