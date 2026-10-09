-- Restores the integer priority from each task's priority rank (clamped to the
-- legacy 0-4 scale). Stored filter trees rewritten by the up migration are NOT
-- converted back — recreate affected saved views by hand if rolling back.
UPDATE tasks t
SET priority = COALESCE(
    (SELECT LEAST(GREATEST(pp.rank, 0), 4) FROM project_priorities pp WHERE pp.id = t.priority_id),
    0
);

ALTER TABLE tasks DROP COLUMN IF EXISTS priority_id;
DROP TABLE IF EXISTS project_priorities;

COMMENT ON COLUMN tasks.priority IS NULL;
