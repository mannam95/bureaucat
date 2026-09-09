-- Soft delete for users, plus audit of when/who for deactivate and delete.
-- Nothing is ever hard-deleted: a deleted user keeps their row (and all their
-- history stays valid), they just can't sign in and are hidden from the normal
-- listings. This is what makes "delete" work despite the RESTRICT foreign keys
-- that reference users(id) all over the schema.
--
-- State is derived: deleted_at IS NOT NULL -> Deleted; else is_active = false
-- -> Deactivated; else Active.
ALTER TABLE users
    ADD COLUMN deactivated_at TIMESTAMPTZ,
    ADD COLUMN deactivated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at     TIMESTAMPTZ,
    ADD COLUMN deleted_by     UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_users_deleted_at ON users(deleted_at);
