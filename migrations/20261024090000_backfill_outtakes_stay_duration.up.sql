-- Issue #204: stay-length badge (-12H / 12-24H / 24-48H / +48H) on animal and
-- outtake records reads outtakes.stay_duration. That column is only filled by
-- Outtake.ComputeStayDuration at creation time, so every outtake recorded
-- before the column existed (issue #175 work) stays NULL and renders an empty
-- badge. Backfill it for historical rows using the same computation as the
-- runtime path: whole hours between the animal intake date (intakes.date, the
-- source of truth — animals.IntakeDate is a denormalized copy with known
-- drift) and the outtake date, negatives clamped to 0, missing intake left
-- NULL. Additive/idempotent in effect: rows already carrying a value are
-- untouched.
UPDATE `outtakes` o
JOIN `animals` a ON a.`outtake_id` = o.`id`
JOIN `intakes` i ON i.`id` = a.`intake_id`
SET o.`stay_duration` = GREATEST(0, TIMESTAMPDIFF(HOUR, i.`date`, o.`date`))
WHERE o.`stay_duration` IS NULL;
