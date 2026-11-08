-- Restore the feeding_guides table (empty) if the removal is rolled back.
CREATE TABLE IF NOT EXISTS `feeding_guides` (
  `id` int NOT NULL AUTO_INCREMENT,
  `species_name` varchar(200) NOT NULL,
  `stage` varchar(50) NOT NULL,
  `text` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_feeding_guides_species_stage` (`species_name`,`stage`)
);
