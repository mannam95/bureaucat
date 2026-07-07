-- name: CreatePage :one
INSERT INTO pages (project_id, page_number, title, content, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, page_number, title, content, created_by, created_at, updated_at, deleted_at;

-- name: GetNextPageNumber :one
SELECT COALESCE(MAX(page_number), 0) + 1 AS next_number
FROM pages
WHERE project_id = $1;

-- name: GetPageByProjectAndNumber :one
SELECT p.id, p.project_id, p.page_number, p.title, p.content, p.created_by, p.created_at, p.updated_at, p.deleted_at,
       u.username as creator_username, u.first_name as creator_first_name, u.last_name as creator_last_name, u.avatar_url as creator_avatar_url
FROM pages p
JOIN users u ON p.created_by = u.id
WHERE p.project_id = $1 AND p.page_number = $2 AND p.deleted_at IS NULL;

-- name: ListProjectPages :many
-- Optional case-insensitive search over the title and the page's visible text
-- (HTML tags stripped from content so markup/attributes don't produce matches).
SELECT p.id, p.project_id, p.page_number, p.title, p.created_by, p.created_at, p.updated_at,
       u.username as creator_username, u.first_name as creator_first_name, u.last_name as creator_last_name, u.avatar_url as creator_avatar_url
FROM pages p
JOIN users u ON p.created_by = u.id
WHERE p.project_id = $1 AND p.deleted_at IS NULL
  AND (
    sqlc.narg('search')::text IS NULL
    OR p.title ILIKE '%' || sqlc.narg('search') || '%'
    OR regexp_replace(p.content, '<[^>]*>', '', 'g') ILIKE '%' || sqlc.narg('search') || '%'
  )
ORDER BY p.updated_at DESC;

-- name: UpdatePage :one
UPDATE pages
SET title = COALESCE(sqlc.narg('title'), title),
    content = COALESCE(sqlc.narg('content'), content),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, project_id, page_number, title, content, created_by, created_at, updated_at, deleted_at;

-- name: SoftDeletePage :exec
UPDATE pages
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;
