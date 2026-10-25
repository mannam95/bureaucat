-- Contributors: the people who did the work on a task, independent of who is
-- assigned right now. In a hand-over workflow (build, then verify) the person
-- who built the task is unassigned before the task completes, so "assignees at
-- completion" misses them. Rows accrue automatically when an assignee is
-- removed while the task sits in a started-type state, and can be edited by
-- hand; `source` records which path created the row.
CREATE TABLE task_contributors (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'auto', 'backfill')),
    UNIQUE(task_id, user_id)
);

CREATE INDEX idx_task_contributors_task_id ON task_contributors(task_id);
CREATE INDEX idx_task_contributors_user_id ON task_contributors(user_id);
