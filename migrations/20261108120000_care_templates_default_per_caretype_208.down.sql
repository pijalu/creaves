ALTER TABLE `care_templates`
  DROP KEY `care_templates_caretype_id_idx`,
  DROP COLUMN `is_default`,
  DROP COLUMN `caretype_id`;
