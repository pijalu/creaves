-- MySQL cannot prepare SIGNAL. Deliberately select from this nonexistent,
-- descriptively named table to abort rollback before any lossy ALTER.
SET @creaves_role_downgrade_sql = IF(
  EXISTS(SELECT 1 FROM `users` WHERE CHAR_LENGTH(`role`) > 20),
  'SELECT 1 FROM `creaves_rollback_refused_users_role_over_20`',
  'ALTER TABLE `users` MODIFY `role` VARCHAR(20) NOT NULL DEFAULT '''''
);
PREPARE creaves_role_downgrade_stmt FROM @creaves_role_downgrade_sql;
EXECUTE creaves_role_downgrade_stmt;
DEALLOCATE PREPARE creaves_role_downgrade_stmt;
