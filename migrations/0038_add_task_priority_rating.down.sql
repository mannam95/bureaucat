DROP INDEX IF EXISTS idx_tasks_priority_rating;
ALTER TABLE tasks DROP COLUMN IF EXISTS priority_rating;
