-- ==================== INSIGHTS ====================
-- Bulk reads for the insights engine, which reconstructs delivery reporting
-- from the activity log. Each query loads one dimension of a whole project in
-- a single round trip; the engine joins them in memory.

-- name: ListInsightsTasks :many
SELECT t.id, t.task_number, t.title, t.parent_task_id, t.state_id,
       t.priority_id, t.difficulty, t.effort,
       t.created_at, t.start_date, t.due_date
FROM tasks t
WHERE t.project_id = $1 AND t.deleted_at IS NULL;

-- name: ListInsightsActivity :many
-- Only the event types the reconstruction needs, oldest first so the engine
-- can replay them without re-sorting.
SELECT a.task_id, a.activity_type::text AS activity_type, a.field_name,
       a.old_value, a.new_value, a.created_at, a.actor_id
FROM activity_log a
JOIN tasks t ON t.id = a.task_id
WHERE t.project_id = $1 AND t.deleted_at IS NULL
  AND a.activity_type::text = ANY(@types::text[])
ORDER BY a.created_at ASC, a.id ASC;

-- name: ListInsightsAssignees :many
SELECT ta.task_id, ta.user_id
FROM task_assignees ta
JOIN tasks t ON t.id = ta.task_id
WHERE t.project_id = $1 AND t.deleted_at IS NULL;

-- name: ListInsightsContributors :many
SELECT tc.task_id, tc.user_id
FROM task_contributors tc
JOIN tasks t ON t.id = tc.task_id
WHERE t.project_id = $1 AND t.deleted_at IS NULL;

-- name: ListInsightsBlockedTasks :many
-- Tasks blocked by at least one blocker that is itself still open. Blocked is
-- a relationship, not a state name, so it works in every project vocabulary.
SELECT DISTINCT tb.task_id
FROM task_blockers tb
JOIN tasks t ON t.id = tb.task_id AND t.deleted_at IS NULL
JOIN tasks bt ON bt.id = tb.blocker_task_id AND bt.deleted_at IS NULL
JOIN project_states ps ON ps.id = bt.state_id
WHERE t.project_id = $1
  AND ps.state_type NOT IN ('completed', 'cancelled', 'archived');

-- name: ListInsightsCycleTasks :many
-- Current cycle membership with its own timestamp: the replay's anchor when a
-- membership predates activity logging (imports, direct store writes).
SELECT ct.cycle_id, ct.task_id, ct.added_at
FROM cycle_tasks ct
JOIN tasks t ON t.id = ct.task_id
WHERE t.project_id = $1 AND t.deleted_at IS NULL;

-- name: ListInsightsCycles :many
SELECT c.id, c.title, c.start_date, c.end_date
FROM cycles c
WHERE c.project_id = $1 AND c.deleted_at IS NULL
ORDER BY c.start_date ASC;

-- name: ListUsersByIDs :many
SELECT u.id, u.username, u.first_name, u.last_name, u.avatar_url
FROM users u
WHERE u.id = ANY(@ids::uuid[]);
