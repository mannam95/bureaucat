DROP INDEX IF EXISTS idx_tasks_originator_id;
ALTER TABLE tasks DROP COLUMN IF EXISTS originator_id;
