-- Multi-hub sync: one Creaves instance can push events to several
-- consoles ("sync targets"). Delivery state is tracked per event per
-- target so one failing hub never blocks the others.
CREATE TABLE `sync_targets` (
  `id` char(36) NOT NULL,
  `name` varchar(255) NOT NULL,
  `enabled` tinyint(1) NOT NULL DEFAULT '0',
  `webhook_url` varchar(1024) NOT NULL DEFAULT '',
  `webhook_api_key` varchar(512) NOT NULL DEFAULT '',
  `webhook_batch_size` int NOT NULL DEFAULT '1',
  `webhook_max_per_min` int NOT NULL DEFAULT '60',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE `event_deliveries` (
  `id` char(36) NOT NULL,
  `event_id` char(36) NOT NULL,
  `target_id` char(36) NOT NULL,
  `attempts` int NOT NULL DEFAULT '0',
  `delivered_at` datetime DEFAULT NULL,
  `acknowledged_at` datetime DEFAULT NULL,
  `last_error` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `event_deliveries_event_target_idx` (`event_id`,`target_id`),
  KEY `event_deliveries_target_id_idx` (`target_id`),
  KEY `event_deliveries_delivered_at_idx` (`delivered_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Backfill: the pre-existing single webhook configuration becomes the
-- first sync target (only when a webhook URL was actually configured).
INSERT INTO sync_targets (id, name, enabled, webhook_url, webhook_api_key, webhook_batch_size, webhook_max_per_min, created_at, updated_at)
SELECT UUID(),
  'Default',
  COALESCE(CAST(JSON_EXTRACT(c.settings, '$.webhook_enabled') AS UNSIGNED), 0),
  COALESCE(JSON_UNQUOTE(JSON_EXTRACT(c.settings, '$.webhook_url')), ''),
  COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(c.settings, '$.webhook_api_key')), 'null'), ''),
  COALESCE(JSON_EXTRACT(c.settings, '$.webhook_batch_size'), 1),
  COALESCE(JSON_EXTRACT(c.settings, '$.webhook_max_per_min'), 60),
  NOW(), NOW()
FROM config c
WHERE c.active = 1
  AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(c.settings, '$.webhook_url')), '') <> ''
LIMIT 1;

-- Backfill deliveries: events already delivered to the previous single
-- hub get a delivered row for the migrated target.
INSERT INTO event_deliveries (id, event_id, target_id, attempts, delivered_at, acknowledged_at, last_error, created_at, updated_at)
SELECT UUID(), e.id, t.id, e.delivery_attempts, e.delivered_at, e.acknowledged_at, NULL, NOW(), NOW()
FROM event_streams e
CROSS JOIN sync_targets t
WHERE e.delivered_at IS NOT NULL;
