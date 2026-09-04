-- Originator/Requester: the person whose need a task represents, distinct from
-- the creator (who typed it in) and the assignee (who works it). Mandatory, so
-- existing rows are backfilled to the creator before the column is made NOT NULL.
ALTER TABLE tasks ADD COLUMN originator_id UUID REFERENCES users(id) ON DELETE RESTRICT;

UPDATE tasks SET originator_id = created_by WHERE originator_id IS NULL;

ALTER TABLE tasks ALTER COLUMN originator_id SET NOT NULL;

CREATE INDEX idx_tasks_originator_id ON tasks(originator_id);
