-- name: CreateUser :one
INSERT INTO users (username, email, password_hash, first_name, last_name, user_type)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, username, email, first_name, last_name, user_type, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, username, email, first_name, last_name, user_type, avatar_url, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserPasswordHash :one
-- Deliberately narrow: only the password hash, so it is never carried around on
-- the general-purpose user row. Used to verify the current password on change.
SELECT password_hash
FROM users
WHERE id = $1;

-- name: GetUserByEmailOrUsername :one
SELECT id, username, email, password_hash, first_name, last_name, user_type,
       avatar_url, auth_provider, provider_user_id, created_at, updated_at
FROM users
WHERE email = $1 OR username = $1;

-- name: UserExistsByEmailOrUsername :one
SELECT EXISTS (
    SELECT 1 FROM users
    WHERE email = $1 OR username = $2
) AS exists;

-- name: GetUserStatusByEmailOrUsername :one
-- The state of the account that owns a conflicting email/username, so create
-- can explain whether it's active, deactivated or deleted.
SELECT is_active, deleted_at
FROM users
WHERE email = $1 OR username = $2
LIMIT 1;

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, token_hash, expires_at, created_at, revoked_at;

-- name: GetRefreshTokenByHash :one
SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
FROM refresh_tokens
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW();

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE id = $1;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: ListUsersPaginated :many
SELECT id, username, email, first_name, last_name, user_type, is_active, created_at, updated_at
FROM users
ORDER BY created_at ASC
LIMIT $1 OFFSET $2;

-- name: SearchUsersPaginated :many
-- Used to find people to add to a project/workspace, so deleted users are
-- excluded: a removed account must never be re-addable.
SELECT id, username, email, first_name, last_name, user_type, is_active, created_at, updated_at
FROM users
WHERE deleted_at IS NULL
  AND (
    username ILIKE '%' || $1 || '%'
    OR email ILIKE '%' || $1 || '%'
    OR first_name ILIKE '%' || $1 || '%'
    OR last_name ILIKE '%' || $1 || '%'
    OR (first_name || ' ' || last_name) ILIKE '%' || $1 || '%'
  )
ORDER BY created_at ASC
LIMIT $2 OFFSET $3;

-- name: CountSearchUsers :one
SELECT COUNT(*) FROM users
WHERE username ILIKE '%' || $1 || '%'
   OR email ILIKE '%' || $1 || '%'
   OR first_name ILIKE '%' || $1 || '%'
   OR last_name ILIKE '%' || $1 || '%'
   OR (first_name || ' ' || last_name) ILIKE '%' || $1 || '%';

-- name: DeleteUserByID :exec
DELETE FROM users WHERE id = $1;

-- name: ListActiveRefreshTokens :many
SELECT
    rt.id,
    rt.user_id,
    rt.expires_at,
    rt.created_at,
    u.username,
    u.email
FROM refresh_tokens rt
JOIN users u ON rt.user_id = u.id
WHERE rt.revoked_at IS NULL AND rt.expires_at > NOW()
ORDER BY rt.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountActiveRefreshTokens :one
SELECT COUNT(*)
FROM refresh_tokens
WHERE revoked_at IS NULL AND expires_at > NOW();

-- name: GetRefreshTokenByID :one
SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
FROM refresh_tokens
WHERE id = $1;

-- name: DeleteExpiredRefreshTokens :execrows
DELETE FROM refresh_tokens
WHERE expires_at <= NOW();

-- name: GetUserByEmail :one
SELECT id, username, email, password_hash, first_name, last_name, user_type,
       auth_provider, provider_user_id, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetUserByProviderID :one
SELECT id, username, email, password_hash, first_name, last_name, user_type,
       auth_provider, provider_user_id, created_at, updated_at
FROM users
WHERE auth_provider = $1 AND provider_user_id = $2;

-- name: CreateSSOUser :one
INSERT INTO users (username, email, first_name, last_name, user_type, auth_provider, provider_user_id, avatar_url)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, username, email, first_name, last_name, user_type, auth_provider, provider_user_id, avatar_url, created_at, updated_at;

-- name: LinkProviderToUser :exec
UPDATE users
SET auth_provider = $2, provider_user_id = $3, updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserAvatarURL :exec
UPDATE users
SET avatar_url = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserType :exec
UPDATE users
SET user_type = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2, updated_at = NOW()
WHERE id = $1;

-- name: IsUserActive :one
SELECT is_active FROM users WHERE id = $1;

-- name: DeactivateUser :exec
UPDATE users
SET is_active = FALSE, deactivated_at = NOW(), deactivated_by = @deactivated_by, updated_at = NOW()
WHERE id = @id;

-- name: ReactivateUser :exec
UPDATE users
SET is_active = TRUE, deactivated_at = NULL, deactivated_by = NULL, updated_at = NOW()
WHERE id = @id;

-- name: SoftDeleteUser :exec
UPDATE users
SET deleted_at = NOW(), deleted_by = @deleted_by, is_active = FALSE, updated_at = NOW()
WHERE id = @id;

-- name: RestoreUser :exec
-- Restores a deleted account back to Active (usable right away).
UPDATE users
SET deleted_at = NULL, deleted_by = NULL, is_active = TRUE, deactivated_at = NULL, deactivated_by = NULL, updated_at = NOW()
WHERE id = @id;

-- name: CountUsersByState :one
-- The three tab counts in one row, so the admin page shows all of them
-- regardless of which tab is selected.
SELECT
    COUNT(*) FILTER (WHERE deleted_at IS NULL AND is_active)     AS active,
    COUNT(*) FILTER (WHERE deleted_at IS NULL AND NOT is_active) AS deactivated,
    COUNT(*) FILTER (WHERE deleted_at IS NOT NULL)               AS deleted
FROM users;

-- name: CountUsersByStateSearch :one
-- Total rows matching the selected state and search, for pagination.
SELECT COUNT(*)
FROM users
WHERE (
        (@status::text = 'active'      AND deleted_at IS NULL AND is_active)
     OR (@status::text = 'deactivated' AND deleted_at IS NULL AND NOT is_active)
     OR (@status::text = 'deleted'     AND deleted_at IS NOT NULL)
      )
  AND (
        @search::text = ''
     OR username ILIKE '%' || @search || '%'
     OR email ILIKE '%' || @search || '%'
     OR first_name ILIKE '%' || @search || '%'
     OR last_name ILIKE '%' || @search || '%'
     OR (first_name || ' ' || last_name) ILIKE '%' || @search || '%'
      );

-- name: ListUsersByState :many
-- One page of users in the selected state, filtered by an optional search.
-- Actor names for who deactivated/deleted are folded in via LEFT JOINs and
-- COALESCEd to non-null strings ('' when unknown).
SELECT u.id, u.username, u.email, u.first_name, u.last_name, u.user_type, u.is_active,
       u.deactivated_at, u.deleted_at, u.created_at, u.updated_at,
       COALESCE(da.first_name || ' ' || da.last_name, '')::text AS deactivated_by_name,
       COALESCE(dl.first_name || ' ' || dl.last_name, '')::text AS deleted_by_name
FROM users u
LEFT JOIN users da ON u.deactivated_by = da.id
LEFT JOIN users dl ON u.deleted_by = dl.id
WHERE (
        (@status::text = 'active'      AND u.deleted_at IS NULL AND u.is_active)
     OR (@status::text = 'deactivated' AND u.deleted_at IS NULL AND NOT u.is_active)
     OR (@status::text = 'deleted'     AND u.deleted_at IS NOT NULL)
      )
  AND (
        @search::text = ''
     OR u.username ILIKE '%' || @search || '%'
     OR u.email ILIKE '%' || @search || '%'
     OR u.first_name ILIKE '%' || @search || '%'
     OR u.last_name ILIKE '%' || @search || '%'
     OR (u.first_name || ' ' || u.last_name) ILIKE '%' || @search || '%'
      )
ORDER BY u.created_at ASC
LIMIT @lim OFFSET @off;
