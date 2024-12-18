-- MySQL dump 10.13  Distrib 8.4.3, for Linux (x86_64)
--
-- Host: localhost    Database: creaves
-- ------------------------------------------------------
-- Server version	8.4.3

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
) ENGINE=InnoDB AUTO_INCREMENT=1671 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `animalsvirellesexportimport`
--

DROP TABLE IF EXISTS `animalsvirellesexportimport`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `animalsvirellesexportimport` (
  `id` int DEFAULT NULL,
  `species` text,
  `ring` text,
  `cage` text,
  `animalage_id` text,
  `animaltype_id` text,
  `discovery_id` text,
  `intake_id` text,
  `outtake_id` text,
  `created_at` text,
  `updated_at` text,
  `feeding` text,
  `gender` text,
  `year` int DEFAULT NULL,
  `yearNumber` int DEFAULT NULL,
  `IntakeDate` text
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
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
  PRIMARY KEY (`id`)
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
-- Table structure for table `discoverersexportvirelles`
--

DROP TABLE IF EXISTS `discoverersexportvirelles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `discoverersexportvirelles` (
  `id` text,
  `firstname` text,
  `lastname` text,
  `address` text,
  `city` text,
  `country` text,
  `email` text,
  `phone` text,
  `note` text,
  `created_at` text,
  `updated_at` text
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
  PRIMARY KEY (`id`),
  KEY `discoverer_id` (`discoverer_id`),
  CONSTRAINT `discoveries_ibfk_1` FOREIGN KEY (`discoverer_id`) REFERENCES `discoverers` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `discoveriesexportvirelles`
--

DROP TABLE IF EXISTS `discoveriesexportvirelles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `discoveriesexportvirelles` (
  `id` text,
  `location` text,
  `date` text,
  `reason` text,
  `note` text,
  `discoverer_id` text,
  `created_at` text,
  `updated_at` text
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
  PRIMARY KEY (`id`)
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
  PRIMARY KEY (`id`)
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
  CONSTRAINT `outtakes_ibfk_1` FOREIGN KEY (`outtaketype_id`) REFERENCES `outtaketypes` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `outtakesvirellesimport`
--

DROP TABLE IF EXISTS `outtakesvirellesimport`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `outtakesvirellesimport` (
  `id` text,
  `date` text,
  `outtaketype_id` text,
  `location` text,
  `note` text,
  `created_at` text,
  `updated_at` text
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `outtaketypes_name_idx` (`name`)
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
  PRIMARY KEY (`ID`)
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

-- Dump completed on 2024-12-18 19:57:03
