-- #208 item 17: default care note template per care type.
-- A care template can be linked to a care type and flagged as the default:
-- the care form then pre-fills the note with it when that care type is
-- selected. Uniqueness of the default per care type is enforced in code
-- (saving a default clears the other defaults of the same care type).
ALTER TABLE `care_templates`
  ADD COLUMN `caretype_id` char(36) DEFAULT NULL AFTER `content`,
  ADD COLUMN `is_default` tinyint(1) NOT NULL DEFAULT 0 AFTER `caretype_id`,
  ADD KEY `care_templates_caretype_id_idx` (`caretype_id`);
