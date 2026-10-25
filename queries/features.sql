-- name: GetFeatureGateMode :one
-- The gate check on the request path: a missing row reads as disabled (grants
-- cannot exist without the gate row, so the caller can stop there).
SELECT mode FROM feature_gates WHERE feature = $1;

-- name: ListFeatureGates :many
SELECT feature, mode, updated_at
FROM feature_gates
ORDER BY feature;

-- name: SetFeatureGateMode :exec
-- Upsert so a feature registered in code before its migration row exists
-- self-heals on the first save.
INSERT INTO feature_gates (feature, mode, updated_by, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (feature) DO UPDATE SET
    mode = EXCLUDED.mode,
    updated_by = EXCLUDED.updated_by,
    updated_at = NOW();

-- name: HasFeatureGrant :one
SELECT EXISTS (
    SELECT 1 FROM feature_grants
    WHERE feature = $1 AND user_id = $2
) AS has_grant;

-- name: ListFeatureGrants :many
SELECT fg.user_id, fg.created_at,
       u.username, u.email, u.first_name, u.last_name, u.avatar_url
FROM feature_grants fg
JOIN users u ON fg.user_id = u.id
WHERE fg.feature = $1
ORDER BY u.first_name ASC, u.last_name ASC;

-- name: ListFeatureGrantsForUser :many
SELECT feature FROM feature_grants WHERE user_id = $1;

-- name: AddFeatureGrant :exec
INSERT INTO feature_grants (feature, user_id, granted_by)
VALUES ($1, $2, $3)
ON CONFLICT (feature, user_id) DO NOTHING;

-- name: RemoveFeatureGrant :exec
DELETE FROM feature_grants WHERE feature = $1 AND user_id = $2;
