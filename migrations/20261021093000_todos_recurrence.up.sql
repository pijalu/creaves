-- Recurring todos (issue #199-8): a todo can repeat weekly, monthly or
-- yearly. NULL/'' means one-shot. On completion the handler spawns the
-- next occurrence (todo_date + 1 week/month/year) and closes the current
-- one. Additive, backward-compatible column.
ALTER TABLE `todos`
  ADD COLUMN `recurrence` varchar(16) NOT NULL DEFAULT '' AFTER `done_by_id`;
