-- Workspaces table: an organizing layer above projects. Every project belongs
-- to exactly one workspace (see 0033).
CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_key VARCHAR(10) NOT NULL UNIQUE,  -- e.g., "ENG"
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ  -- soft delete
);

CREATE INDEX idx_workspaces_workspace_key ON workspaces(workspace_key);
CREATE INDEX idx_workspaces_created_by ON workspaces(created_by);
CREATE INDEX idx_workspaces_deleted_at ON workspaces(deleted_at) WHERE deleted_at IS NULL;
