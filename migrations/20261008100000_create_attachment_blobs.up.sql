-- Issue #34 (reopened): store attachment binaries in the database. The
-- database is the only storage for attachment content (no disk files).
--
-- Raw SQL instead of fizz because the fizz "blob" type maps to MySQL BLOB
-- (64 KB max), far too small for the allowed 100 MB videos.
--
-- NOTE for deployments: MariaDB/MySQL `max_allowed_packet` must be larger than
-- the biggest allowed upload (recommend 256M) for inserts into this table.

CREATE TABLE IF NOT EXISTS `attachment_blobs` (
  `attachment_id` char(36) NOT NULL,
  `data` longblob NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`attachment_id`),
  CONSTRAINT `fk_attachment_blobs_attachment` FOREIGN KEY (`attachment_id`) REFERENCES `attachments` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
