-- ==================== USER PREFERENCES ====================
--
-- Rows hold only user overrides; a missing row means "use the registry
-- default". Reads return the whole set for a scope so the API can fold in
-- defaults. Writes are revision-aware (optimistic concurrency): the caller
-- passes the revision it last saw, and the update only applies when the stored
-- revision still matches. A brand-new row is created at revision 1 regardless.

-- name: ListGlobalPreferences :many
SELECT preference_key, value, value_version, revision, updated_at
FROM user_preferences
WHERE user_id = $1
  AND workspace_id IS NULL
  AND project_id IS NULL;

-- name: ListProjectPreferences :many
SELECT preference_key, value, value_version, revision, updated_at
FROM user_preferences
WHERE user_id = $1
  AND project_id = $2;

-- name: UpsertGlobalPreference :one
-- On a matching revision the row is bumped; on a stale revision no row is
-- returned (the handler maps that to 409). A first write (no existing row)
-- inserts at revision 1.
INSERT INTO user_preferences (user_id, preference_key, value, value_version, revision)
VALUES (@user_id, @preference_key, @value, @value_version, 1)
ON CONFLICT (user_id, preference_key) WHERE workspace_id IS NULL AND project_id IS NULL
DO UPDATE SET
    value = EXCLUDED.value,
    value_version = EXCLUDED.value_version,
    revision = user_preferences.revision + 1,
    updated_at = NOW()
WHERE user_preferences.revision = @expected_revision
RETURNING preference_key, value, value_version, revision, updated_at;

-- name: UpsertProjectPreference :one
INSERT INTO user_preferences (user_id, preference_key, project_id, value, value_version, revision)
VALUES (@user_id, @preference_key, @project_id, @value, @value_version, 1)
ON CONFLICT (user_id, preference_key, project_id) WHERE project_id IS NOT NULL
DO UPDATE SET
    value = EXCLUDED.value,
    value_version = EXCLUDED.value_version,
    revision = user_preferences.revision + 1,
    updated_at = NOW()
WHERE user_preferences.revision = @expected_revision
RETURNING preference_key, value, value_version, revision, updated_at;

-- name: DeleteGlobalPreference :exec
DELETE FROM user_preferences
WHERE user_id = $1 AND preference_key = $2
  AND workspace_id IS NULL AND project_id IS NULL;

-- name: DeleteProjectPreference :exec
DELETE FROM user_preferences
WHERE user_id = $1 AND preference_key = $2 AND project_id = $3;
