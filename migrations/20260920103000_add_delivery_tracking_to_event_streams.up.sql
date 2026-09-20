ALTER TABLE event_streams ADD COLUMN delivery_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE event_streams ADD COLUMN last_delivery_error TEXT NULL;
