-- Item 9 of the 2026-10-07 day-plan review: the 4 composite seed matchers
-- (SR1*/SR4*/SR6*/SR12* — the implementation artifact of composite rules,
-- since care_rules references exactly one matcher) must not surface in the
-- matcher UI. They get a `derived` flag, are renamed WITHOUT the
-- "(dérivé …)" suffix (the label read as noise in /care_matchers and the
-- rule forms), and their localized display names lose the suffix too.
--
-- The rename MUST land before the boot seed library runs: seeding dedupes
-- by name, so renamed rows match SeedMatchers() and no duplicate is
-- created. Fresh databases (PRD) skip the rename naturally — the seeds
-- insert the suffix-free names directly.

ALTER TABLE care_matchers ADD COLUMN derived BOOL NOT NULL DEFAULT 0;

-- Mark the 4 composite matchers (exact seeded names, pre-rename).
UPDATE care_matchers SET derived = 1 WHERE name IN (
  'Hérisson bébé < 300 g (dérivé SM1)',
  'Colombidés bébé (dérivé SM4+SM11)',
  'Canidés bébé (dérivé SM6+SM11)',
  'Hérisson bébé ou juvénile (dérivé SM1 OU SM2)'
);

-- Rename to the suffix-free seed names (actions/care_plan_seeds.go).
UPDATE care_matchers SET name = 'Hérisson bébé < 300 g' WHERE name = 'Hérisson bébé < 300 g (dérivé SM1)';
UPDATE care_matchers SET name = 'Colombidés bébé' WHERE name = 'Colombidés bébé (dérivé SM4+SM11)';
UPDATE care_matchers SET name = 'Canidés bébé' WHERE name = 'Canidés bébé (dérivé SM6+SM11)';
UPDATE care_matchers SET name = 'Hérisson bébé ou juvénile' WHERE name = 'Hérisson bébé ou juvénile (dérivé SM1 OU SM2)';

-- Localized display names of the derived matchers lose the suffix too
-- (seeded locales: en-US "derived", de "abgeleitet", nl "afgeleid").
UPDATE translations t
  JOIN care_matchers m ON m.id = t.record_id
  SET t.value = REGEXP_REPLACE(t.value, ' [(](derived|abgeleitet|afgeleid) [^)]+[)]$', '')
  WHERE t.table_name = 'care_matchers' AND m.derived = 1;
