-- ==================== WORKSPACES ====================

-- name: CreateWorkspace :one
INSERT INTO workspaces (workspace_key, name, description, created_by)
VALUES ($1, $2, $3, $4)
RETURNING id, workspace_key, name, description, created_by, created_at, updated_at, deleted_at;

-- name: GetWorkspaceByID :one
SELECT id, workspace_key, name, description, created_by, created_at, updated_at, deleted_at
FROM workspaces
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetWorkspaceByKey :one
SELECT id, workspace_key, name, description, created_by, created_at, updated_at, deleted_at
FROM workspaces
WHERE workspace_key = $1 AND deleted_at IS NULL;

-- name: UpdateWorkspace :one
UPDATE workspaces
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, workspace_key, name, description, created_by, created_at, updated_at, deleted_at;

-- name: SoftDeleteWorkspace :exec
UPDATE workspaces
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- name: WorkspaceKeyExists :one
SELECT EXISTS (
    SELECT 1 FROM workspaces
    WHERE workspace_key = $1 AND deleted_at IS NULL
) AS exists;

-- name: ListUserWorkspaces :many
SELECT w.id, w.workspace_key, w.name, w.description, w.created_by, w.created_at, w.updated_at, w.deleted_at
FROM workspaces w
JOIN workspace_members wm ON w.id = wm.workspace_id
WHERE wm.user_id = $1 AND w.deleted_at IS NULL
ORDER BY w.name ASC
LIMIT $2 OFFSET $3;

-- name: CountUserWorkspaces :one
SELECT COUNT(*)
FROM workspaces w
JOIN workspace_members wm ON w.id = wm.workspace_id
WHERE wm.user_id = $1 AND w.deleted_at IS NULL;

-- name: ListUserWorkspacesFiltered :many
SELECT w.id, w.workspace_key, w.name, w.description, w.created_by, w.created_at, w.updated_at, w.deleted_at
FROM workspaces w
JOIN workspace_members wm ON w.id = wm.workspace_id
WHERE wm.user_id = $1 AND w.deleted_at IS NULL
  AND (sqlc.narg('search')::text IS NULL OR w.name ILIKE '%' || sqlc.narg('search') || '%' OR w.workspace_key ILIKE '%' || sqlc.narg('search') || '%' OR w.description ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY w.name ASC
LIMIT $2 OFFSET $3;

-- name: CountUserWorkspacesFiltered :one
SELECT COUNT(*)
FROM workspaces w
JOIN workspace_members wm ON w.id = wm.workspace_id
WHERE wm.user_id = $1 AND w.deleted_at IS NULL
  AND (sqlc.narg('search')::text IS NULL OR w.name ILIKE '%' || sqlc.narg('search') || '%' OR w.workspace_key ILIKE '%' || sqlc.narg('search') || '%' OR w.description ILIKE '%' || sqlc.narg('search') || '%');

-- name: ListAllWorkspaces :many
SELECT w.id, w.workspace_key, w.name, w.description, w.created_by, w.created_at, w.updated_at, w.deleted_at
FROM workspaces w
WHERE w.deleted_at IS NULL
ORDER BY w.name ASC
LIMIT $1 OFFSET $2;

-- name: CountAllWorkspaces :one
SELECT COUNT(*)
FROM workspaces w
WHERE w.deleted_at IS NULL;

-- name: ListAllWorkspacesFiltered :many
SELECT w.id, w.workspace_key, w.name, w.description, w.created_by, w.created_at, w.updated_at, w.deleted_at
FROM workspaces w
WHERE w.deleted_at IS NULL
  AND (sqlc.narg('search')::text IS NULL OR w.name ILIKE '%' || sqlc.narg('search') || '%' OR w.workspace_key ILIKE '%' || sqlc.narg('search') || '%' OR w.description ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY w.name ASC
LIMIT $1 OFFSET $2;

-- name: CountAllWorkspacesFiltered :one
SELECT COUNT(*)
FROM workspaces w
WHERE w.deleted_at IS NULL
  AND (sqlc.narg('search')::text IS NULL OR w.name ILIKE '%' || sqlc.narg('search') || '%' OR w.workspace_key ILIKE '%' || sqlc.narg('search') || '%' OR w.description ILIKE '%' || sqlc.narg('search') || '%');

-- ==================== WORKSPACE MEMBERS ====================

-- name: AddWorkspaceMember :one
INSERT INTO workspace_members (workspace_id, user_id)
VALUES ($1, $2)
RETURNING id, workspace_id, user_id, joined_at;

-- name: RemoveWorkspaceMember :exec
DELETE FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2;

-- name: ListWorkspaceMembers :many
SELECT wm.id, wm.workspace_id, wm.user_id, wm.joined_at,
       u.username, u.email, u.first_name, u.last_name, u.avatar_url
FROM workspace_members wm
JOIN users u ON wm.user_id = u.id
WHERE wm.workspace_id = $1
ORDER BY wm.joined_at ASC;

-- name: GetWorkspaceMember :one
SELECT wm.id, wm.workspace_id, wm.user_id, wm.joined_at,
       u.username, u.email, u.first_name, u.last_name, u.avatar_url
FROM workspace_members wm
JOIN users u ON wm.user_id = u.id
WHERE wm.workspace_id = $1 AND wm.user_id = $2;

-- name: IsWorkspaceMember :one
SELECT EXISTS (
    SELECT 1 FROM workspace_members
    WHERE workspace_id = $1 AND user_id = $2
) AS is_member;
