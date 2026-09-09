DROP INDEX IF EXISTS idx_users_deleted_at;

ALTER TABLE users
    DROP COLUMN deactivated_at,
    DROP COLUMN deactivated_by,
    DROP COLUMN deleted_at,
    DROP COLUMN deleted_by;
