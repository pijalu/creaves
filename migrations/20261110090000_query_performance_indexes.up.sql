-- Match locale-wide translation map loads without scanning every record ID
-- for the table first.
CREATE INDEX `translations_table_field_locale_record_id_idx`
  ON `translations` (`table_name`, `field`, `locale`, `record_id`);

-- Cover warning/reset-warning care aggregation by care type and animal.
CREATE INDEX `cares_type_id_animal_id_date_idx`
  ON `cares` (`type_id`, `animal_id`, `date`);

-- Serve the paginated treatment list directly in its display order instead
-- of sorting the complete treatment history before applying LIMIT.
CREATE INDEX `treatments_date_timebitmap_animal_id_idx`
  ON `treatments` (`date` DESC, `timebitmap` ASC, `animal_id` DESC);

-- Match the admin audit filter and deterministic pagination order.
CREATE INDEX `animal_audits_animal_id_created_at_id_idx`
  ON `animal_audits` (`animal_id`, `created_at`, `id`);

-- Match dashboard date ranges and the paginated visit list ordering.
CREATE INDEX `veterinaryvisits_date_idx`
  ON `veterinaryvisits` (`date`);

-- One key serves open todos ordered by due date and done todos ordered by
-- completion date (both filter on done_at).
CREATE INDEX `todos_done_at_todo_date_idx`
  ON `todos` (`done_at`, `todo_date`);

-- Avoid sorting the complete event history for every paginated admin page.
CREATE INDEX `event_streams_created_at_idx`
  ON `event_streams` (`created_at`);
