-- Watchers: users who follow a ticket to receive its updates without being an
-- assignee or originator. Zero or more per task, mirroring task_assignees.
-- user_id is ON DELETE CASCADE: if a user is removed they simply stop watching.
CREATE TABLE task_watchers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE(task_id, user_id)
);

CREATE INDEX idx_task_watchers_task_id ON task_watchers(task_id);
CREATE INDEX idx_task_watchers_user_id ON task_watchers(user_id);
