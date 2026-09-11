-- MySQL dump 10.13  Distrib 8.4.11, for macos26.6 (arm64)
--
-- Host: localhost    Database: creaves
-- ------------------------------------------------------
-- Server version	8.4.11

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `animal_audits`
--

DROP TABLE IF EXISTS `animal_audits`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `animal_audits` (
  `id` char(36) NOT NULL,
  `animal_id` int NOT NULL,
  `user_id` char(36) DEFAULT NULL,
  `user_name` varchar(255) NOT NULL,
  `entity` varchar(255) NOT NULL,
  `entity_id` varchar(255) DEFAULT NULL,
  `action` varchar(255) NOT NULL,
  `changes` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `animal_audits_animal_id_idx` (`animal_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `animalages`
--

DROP TABLE IF EXISTS `animalages`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `animalages` (
  `id` char(36) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text,
  `def` tinyint(1) NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `animals`
--

DROP TABLE IF EXISTS `animals`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `animals` (
  `id` int NOT NULL AUTO_INCREMENT,
  `species` varchar(255) NOT NULL,
  `ring` varchar(255) DEFAULT NULL,
  `cage` varchar(255) DEFAULT NULL,
  `animalage_id` char(36) NOT NULL,
  `animaltype_id` char(36) NOT NULL,
  `discovery_id` char(36) NOT NULL,
  `intake_id` char(36) NOT NULL,
  `outtake_id` char(36) DEFAULT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `feeding` varchar(255) DEFAULT NULL,
  `gender` varchar(255) DEFAULT NULL,
  `year` int DEFAULT NULL,
  `yearNumber` int DEFAULT NULL,
  `IntakeDate` datetime NOT NULL,
  `force_feed` tinyint(1) NOT NULL DEFAULT '0',
  `zone` varchar(255) DEFAULT NULL,
  `feeding_start` datetime DEFAULT NULL,
  `feeding_end` datetime DEFAULT NULL,
  `feeding_period` int NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `animals_year_yearNumber_idx` (`year`,`yearNumber`),
  KEY `animaltype_id` (`animaltype_id`),
  KEY `discovery_id` (`discovery_id`),
  KEY `intake_id` (`intake_id`),
  KEY `outtake_id` (`outtake_id`),
  CONSTRAINT `animals_ibfk_1` FOREIGN KEY (`animaltype_id`) REFERENCES `animaltypes` (`id`),
  CONSTRAINT `animals_ibfk_2` FOREIGN KEY (`discovery_id`) REFERENCES `discoveries` (`id`),
  CONSTRAINT `animals_ibfk_3` FOREIGN KEY (`intake_id`) REFERENCES `intakes` (`id`),
  CONSTRAINT `animals_ibfk_4` FOREIGN KEY (`outtake_id`) REFERENCES `outtakes` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=980051 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `animaltypes`
--

DROP TABLE IF EXISTS `animaltypes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `animaltypes` (
  `id` char(36) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text,
  `def` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `has_ring` tinyint(1) NOT NULL DEFAULT '0',
  `default_species` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `animaltypes_name_idx` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `cares`
--

DROP TABLE IF EXISTS `cares`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `cares` (
  `id` char(36) NOT NULL,
  `date` datetime NOT NULL,
  `type_id` char(36) NOT NULL,
  `animal_id` int NOT NULL,
  `weight` varchar(255) DEFAULT NULL,
  `note` text,
  `clean` tinyint(1) DEFAULT NULL,
  `in_warning` tinyint(1) DEFAULT NULL,
  `link_to_id` char(36) DEFAULT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `type_id` (`type_id`),
  KEY `animal_id` (`animal_id`),
  KEY `link_to_id` (`link_to_id`),
  KEY `cares_date_idx` (`date`),
  KEY `cares_date_type_id_animal_id_idx` (`date`,`type_id`,`animal_id`),
  KEY `cares_animal_id_date_idx` (`animal_id`,`date`),
  CONSTRAINT `cares_ibfk_1` FOREIGN KEY (`type_id`) REFERENCES `caretypes` (`id`),
  CONSTRAINT `cares_ibfk_2` FOREIGN KEY (`animal_id`) REFERENCES `animals` (`id`),
  CONSTRAINT `cares_ibfk_3` FOREIGN KEY (`link_to_id`) REFERENCES `cares` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `caretypes`
--

DROP TABLE IF EXISTS `caretypes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `caretypes` (
  `id` char(36) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text,
  `def` tinyint(1) NOT NULL,
  `warning` tinyint(1) NOT NULL,
  `reset_warning` tinyint(1) NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `type` int NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `caretypes_warning_idx` (`warning`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `config`
--

DROP TABLE IF EXISTS `config`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `config` (
  `id` char(36) NOT NULL,
  `instance_id` varchar(255) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text,
  `active` tinyint(1) NOT NULL DEFAULT '1',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `settings` json DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `instance_configs_instance_id_idx` (`instance_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `consolidated_animals`
--

DROP TABLE IF EXISTS `consolidated_animals`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `consolidated_animals` (
  `id` char(36) NOT NULL,
  `instance_id` varchar(255) NOT NULL,
  `animal_id` int NOT NULL,
  `year` int NOT NULL,
  `year_number` int NOT NULL,
  `species` varchar(255) DEFAULT NULL,
  `animal_type` varchar(255) DEFAULT NULL,
  `animal_age` varchar(255) DEFAULT NULL,
  `discovery_location` varchar(255) DEFAULT NULL,
  `discovery_date` datetime DEFAULT NULL,
  `current_status` varchar(255) NOT NULL,
  `intake_date` datetime DEFAULT NULL,
  `intake_general` varchar(255) DEFAULT NULL,
  `intake_wounds` varchar(255) DEFAULT NULL,
  `intake_parasites` varchar(255) DEFAULT NULL,
  `intake_remarks` varchar(255) DEFAULT NULL,
  `outtake_date` datetime DEFAULT NULL,
  `outtake_type` varchar(255) DEFAULT NULL,
  `outtake_location` varchar(255) DEFAULT NULL,
  `last_event_at` datetime NOT NULL,
  `event_count` int NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `consolidated_animals_instance_id_animal_id_idx` (`instance_id`,`animal_id`),
  KEY `consolidated_animals_instance_id_idx` (`instance_id`),
  KEY `consolidated_animals_current_status_idx` (`current_status`),
  KEY `consolidated_animals_species_idx` (`species`),
  KEY `consolidated_animals_discovery_date_idx` (`discovery_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `discoverers`
--

DROP TABLE IF EXISTS `discoverers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `discoverers` (
  `id` char(36) NOT NULL,
  `firstname` varchar(255) DEFAULT NULL,
  `lastname` varchar(255) DEFAULT NULL,
  `address` varchar(255) DEFAULT NULL,
  `city` varchar(255) DEFAULT NULL,
  `country` varchar(255) DEFAULT NULL,
  `email` varchar(255) DEFAULT NULL,
  `phone` varchar(255) DEFAULT NULL,
  `note` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `postal_code` varchar(255) DEFAULT NULL,
  `return_request` tinyint(1) NOT NULL DEFAULT '0',
  `donation` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `discoveries`
--

DROP TABLE IF EXISTS `discoveries`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `discoveries` (
  `id` char(36) NOT NULL,
  `location` varchar(255) DEFAULT NULL,
  `date` datetime NOT NULL,
  `reason` text,
  `note` text,
  `discoverer_id` char(36) NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `postal_code` varchar(255) DEFAULT NULL,
  `city` varchar(255) DEFAULT NULL,
  `return_habitat` tinyint(1) NOT NULL DEFAULT '0',
  `in_garden` tinyint(1) NOT NULL DEFAULT '0',
  `entry_cause_id` varchar(255) NOT NULL DEFAULT '1.1',
  PRIMARY KEY (`id`),
  KEY `discoverer_id` (`discoverer_id`),
  CONSTRAINT `discoveries_ibfk_1` FOREIGN KEY (`discoverer_id`) REFERENCES `discoverers` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `dosages`
--

DROP TABLE IF EXISTS `dosages`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `dosages` (
  `id` char(36) NOT NULL,
  `animaltype_id` char(36) NOT NULL,
  `drug_id` char(36) NOT NULL,
  `enabled` tinyint(1) NOT NULL,
  `description` text,
  `dosage_per_grams` float DEFAULT NULL,
  `dosage_per_grams_unit` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `animaltype_id` (`animaltype_id`),
  KEY `drug_id` (`drug_id`),
  CONSTRAINT `dosages_ibfk_1` FOREIGN KEY (`animaltype_id`) REFERENCES `animaltypes` (`id`),
  CONSTRAINT `dosages_ibfk_2` FOREIGN KEY (`drug_id`) REFERENCES `drugs` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `drugs`
--

DROP TABLE IF EXISTS `drugs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `drugs` (
  `id` char(36) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `entry_causes`
--

DROP TABLE IF EXISTS `entry_causes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `entry_causes` (
  `id` varchar(255) NOT NULL,
  `cause` varchar(255) NOT NULL,
  `detail` varchar(255) NOT NULL,
  `nature` varchar(255) NOT NULL,
  `indication` varchar(255) NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `sort_order` int NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `event_streams`
--

DROP TABLE IF EXISTS `event_streams`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `event_streams` (
  `id` char(36) NOT NULL,
  `instance_id` varchar(255) NOT NULL,
  `animal_id` int NOT NULL,
  `event_type` varchar(255) NOT NULL,
  `payload` json DEFAULT NULL,
  `processed_at` datetime DEFAULT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `delivered_at` datetime DEFAULT NULL,
  `content_hash` varchar(255) DEFAULT NULL,
  `resync_run_id` char(36) DEFAULT NULL,
  `acknowledged_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `event_streams_instance_id_animal_id_created_at_idx` (`instance_id`,`animal_id`,`created_at`),
  KEY `event_streams_processed_at_idx` (`processed_at`),
  KEY `event_streams_event_type_idx` (`event_type`),
  KEY `event_streams_delivered_at_idx` (`delivered_at`),
  KEY `event_streams_event_type_content_hash_idx` (`event_type`,`content_hash`),
  KEY `event_streams_resync_run_id_idx` (`resync_run_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `intakes`
--

DROP TABLE IF EXISTS `intakes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `intakes` (
  `id` char(36) NOT NULL,
  `date` datetime NOT NULL,
  `general` text,
  `wounds` text,
  `parasites` text,
  `remarks` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `has_wounds` tinyint(1) NOT NULL DEFAULT '0',
  `has_parasites` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `intakes_date_idx` (`date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `localities`
--

DROP TABLE IF EXISTS `localities`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `localities` (
  `id` varchar(255) NOT NULL,
  `country` varchar(255) NOT NULL,
  `region` varchar(255) NOT NULL,
  `province` varchar(255) NOT NULL,
  `municipality` varchar(255) NOT NULL,
  `sub_municipality` tinyint(1) NOT NULL,
  `postal_code` varchar(255) NOT NULL,
  `locality` varchar(255) NOT NULL,
  `zoning` varchar(255) NOT NULL,
  `direction` varchar(255) NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `localities_locality_idx` (`locality`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `logentries`
--

DROP TABLE IF EXISTS `logentries`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `logentries` (
  `id` char(36) NOT NULL,
  `user_id` char(36) NOT NULL,
  `description` text NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `user_id` (`user_id`),
  KEY `logentries_updated_at_idx` (`updated_at`),
  CONSTRAINT `logentries_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `native_statuses`
--

DROP TABLE IF EXISTS `native_statuses`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `native_statuses` (
  `id` varchar(255) NOT NULL,
  `status` varchar(255) NOT NULL,
  `indication` varchar(255) NOT NULL,
  `precision` varchar(255) DEFAULT NULL,
  `freeable` tinyint(1) NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `outtakes`
--

DROP TABLE IF EXISTS `outtakes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `outtakes` (
  `id` char(36) NOT NULL,
  `date` datetime NOT NULL,
  `outtaketype_id` char(36) NOT NULL,
  `location` varchar(255) DEFAULT NULL,
  `note` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `outtaketype_id` (`outtaketype_id`),
  KEY `outtakes_date_idx` (`date`),
  CONSTRAINT `outtakes_ibfk_1` FOREIGN KEY (`outtaketype_id`) REFERENCES `outtaketypes` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `outtaketypes`
--

DROP TABLE IF EXISTS `outtaketypes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `outtaketypes` (
  `id` char(36) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text,
  `def` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `dead` tinyint(1) NOT NULL DEFAULT '0',
  `rating` int NOT NULL DEFAULT '0',
  `discoverer_news` text,
  `error` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `outtaketypes_name_idx` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `resync_runs`
--

DROP TABLE IF EXISTS `resync_runs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `resync_runs` (
  `id` char(36) NOT NULL,
  `instance_id` varchar(255) NOT NULL,
  `status` varchar(255) NOT NULL,
  `started_at` datetime NOT NULL,
  `finished_at` datetime DEFAULT NULL,
  `total_animals` int NOT NULL DEFAULT '0',
  `animals_processed` int NOT NULL DEFAULT '0',
  `events_created` int NOT NULL DEFAULT '0',
  `events_skipped_unchanged` int NOT NULL DEFAULT '0',
  `errors` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `events_delivered` int NOT NULL DEFAULT '0',
  `events_failed` int NOT NULL DEFAULT '0',
  `announced_expected_total` int NOT NULL DEFAULT '0',
  `announced_expected_checksum` varchar(255) DEFAULT NULL,
  `announced_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `resync_runs_instance_id_status_idx` (`instance_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `schema_migration`
--

DROP TABLE IF EXISTS `schema_migration`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `schema_migration` (
  `version` varchar(14) NOT NULL,
  UNIQUE KEY `schema_migration_version_idx` (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `species`
--

DROP TABLE IF EXISTS `species`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `species` (
  `ID` varchar(255) NOT NULL,
  `species` varchar(255) NOT NULL,
  `class` varchar(255) NOT NULL,
  `family` varchar(255) NOT NULL,
  `creaves_species` varchar(255) NOT NULL,
  `subside_group` varchar(255) NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `order` varchar(255) NOT NULL,
  `game` tinyint(1) NOT NULL DEFAULT '0',
  `agw_group` varchar(255) NOT NULL,
  `native_status` varchar(255) NOT NULL,
  `huntable` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`ID`),
  KEY `species_creaves_species_idx` (`creaves_species`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `subside_groups`
--

DROP TABLE IF EXISTS `subside_groups`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `subside_groups` (
  `id` varchar(255) NOT NULL,
  `group` varchar(255) NOT NULL,
  `size` int NOT NULL,
  `amount` float NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `translations`
--

DROP TABLE IF EXISTS `translations`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `translations` (
  `id` varchar(36) NOT NULL,
  `table_name` varchar(64) NOT NULL,
  `record_id` varchar(36) NOT NULL,
  `field` varchar(64) NOT NULL,
  `locale` varchar(8) NOT NULL,
  `value` text NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `translations_table_name_record_id_field_locale_idx` (`table_name`,`record_id`,`field`,`locale`),
  KEY `translations_table_name_record_id_locale_idx` (`table_name`,`record_id`,`locale`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `travels`
--

DROP TABLE IF EXISTS `travels`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `travels` (
  `id` char(36) NOT NULL,
  `date` datetime NOT NULL,
  `animal_id` int NOT NULL,
  `user_id` char(36) NOT NULL,
  `traveltype_id` char(36) NOT NULL,
  `type_details` varchar(255) DEFAULT NULL,
  `distance` int NOT NULL,
  `details` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `animal_id` (`animal_id`),
  KEY `user_id` (`user_id`),
  KEY `traveltype_id` (`traveltype_id`),
  CONSTRAINT `travels_ibfk_1` FOREIGN KEY (`animal_id`) REFERENCES `animals` (`id`),
  CONSTRAINT `travels_ibfk_2` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`),
  CONSTRAINT `travels_ibfk_3` FOREIGN KEY (`traveltype_id`) REFERENCES `traveltypes` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `traveltypes`
--

DROP TABLE IF EXISTS `traveltypes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `traveltypes` (
  `id` char(36) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text,
  `def` tinyint(1) NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `treatments`
--

DROP TABLE IF EXISTS `treatments`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `treatments` (
  `id` char(36) NOT NULL,
  `date` datetime NOT NULL,
  `animal_id` int NOT NULL,
  `drug` varchar(255) NOT NULL,
  `dosage` varchar(255) NOT NULL,
  `remarks` text,
  `timebitmap` int NOT NULL,
  `timedonebitmap` int NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `treatments_animals_id_fk` (`animal_id`),
  KEY `treatments_date_idx` (`date`),
  KEY `treatments_date_animal_id_idx` (`date`,`animal_id`),
  KEY `treatments_animal_id_timebitmap_timedonebitmap_idx` (`animal_id`,`timebitmap`,`timedonebitmap`),
  CONSTRAINT `treatments_animals_id_fk` FOREIGN KEY (`animal_id`) REFERENCES `animals` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `users`
--

DROP TABLE IF EXISTS `users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `users` (
  `id` char(36) NOT NULL,
  `login` varchar(255) NOT NULL,
  `admin` tinyint(1) NOT NULL DEFAULT '0',
  `approved` tinyint(1) NOT NULL DEFAULT '0',
  `password_hash` varchar(255) NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  `shared` tinyint(1) NOT NULL DEFAULT '0',
  `maintainer` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `users_login_idx` (`login`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `veterinaryvisits`
--

DROP TABLE IF EXISTS `veterinaryvisits`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `veterinaryvisits` (
  `id` char(36) NOT NULL,
  `date` datetime NOT NULL,
  `user_id` char(36) NOT NULL,
  `animal_id` int NOT NULL,
  `veterinary` text NOT NULL,
  `diagnostic` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `user_id` (`user_id`),
  KEY `animal_id` (`animal_id`),
  CONSTRAINT `veterinaryvisits_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`),
  CONSTRAINT `veterinaryvisits_ibfk_2` FOREIGN KEY (`animal_id`) REFERENCES `animals` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `zones`
--

DROP TABLE IF EXISTS `zones`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `zones` (
  `id` char(36) NOT NULL,
  `zone` varchar(255) NOT NULL,
  `type` varchar(255) NOT NULL,
  `default` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `zones_zone_idx` (`zone`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2026-09-11 15:16:10
