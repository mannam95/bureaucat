-- Multiple originators/requesters per task, mirroring task_assignees. The
-- single tasks.originator_id column becomes a many-to-many join table. Existing
-- rows are copied over first so no task loses its current requester, then the
-- old column is dropped (the join table is the source of truth).
--
-- user_id is ON DELETE RESTRICT like the old column: a requester's identity is
-- preserved. Users are soft-deleted (never hard-deleted), so this never blocks
-- normal admin operations.
CREATE TABLE task_originators (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE(task_id, user_id)
);

CREATE INDEX idx_task_originators_task_id ON task_originators(task_id);
CREATE INDEX idx_task_originators_user_id ON task_originators(user_id);

-- Copy the current single originator of every task into the new table.
INSERT INTO task_originators (task_id, user_id, added_by, added_at)
SELECT id, originator_id, created_by, created_at
FROM tasks;

DROP INDEX IF EXISTS idx_tasks_originator_id;
ALTER TABLE tasks DROP COLUMN originator_id;
