DROP INDEX `event_streams_created_at_idx` ON `event_streams`;
DROP INDEX `todos_done_at_todo_date_idx` ON `todos`;
DROP INDEX `veterinaryvisits_date_idx` ON `veterinaryvisits`;
DROP INDEX `animal_audits_animal_id_created_at_id_idx` ON `animal_audits`;
DROP INDEX `treatments_date_timebitmap_animal_id_idx` ON `treatments`;

-- Some legacy databases rely on the standalone type_id key for the cares
-- foreign key. Restore it when absent before removing the composite key.
SET @cares_type_index_sql = (
  SELECT IF(COUNT(*) = 0,
    'CREATE INDEX `type_id` ON `cares` (`type_id`)',
    'SELECT 1')
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'cares'
    AND index_name = 'type_id'
);
PREPARE cares_type_index_stmt FROM @cares_type_index_sql;
EXECUTE cares_type_index_stmt;
DEALLOCATE PREPARE cares_type_index_stmt;
DROP INDEX `cares_type_id_animal_id_date_idx` ON `cares`;
DROP INDEX `translations_table_field_locale_record_id_idx` ON `translations`;
