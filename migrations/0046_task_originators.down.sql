-- Restore the single originator_id column, picking each task's earliest
-- originator (falling back to the creator when a task has none), then drop the
-- join table.
ALTER TABLE tasks ADD COLUMN originator_id UUID REFERENCES users(id) ON DELETE RESTRICT;

UPDATE tasks t SET originator_id = COALESCE(
    (SELECT o.user_id FROM task_originators o WHERE o.task_id = t.id ORDER BY o.added_at ASC LIMIT 1),
    t.created_by
);

ALTER TABLE tasks ALTER COLUMN originator_id SET NOT NULL;
CREATE INDEX idx_tasks_originator_id ON tasks(originator_id);

DROP TABLE task_originators;
