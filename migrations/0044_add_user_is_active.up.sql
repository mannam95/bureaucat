-- Soft account deactivation: admins can disable a user without deleting them.
-- Deactivated users cannot sign in or refresh; their password and data are kept
-- so reactivating restores access with the same credentials.
ALTER TABLE users
    ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;
