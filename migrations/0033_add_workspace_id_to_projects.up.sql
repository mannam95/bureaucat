-- Attach every project to a workspace. Existing projects are backfilled into a
-- single "Default" workspace so nothing breaks after the upgrade.
ALTER TABLE projects ADD COLUMN workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;

-- Create the Default workspace, owned by the oldest admin (falling back to the
-- oldest user). Runs only when at least one user exists; on a brand-new install
-- there are no users and no projects, so nothing is backfilled and admins create
-- their first workspace from the UI.
INSERT INTO workspaces (workspace_key, name, description, created_by)
SELECT 'DEFAULT', 'Default', 'Default workspace', u.id
FROM users u
ORDER BY (u.user_type = 'admin') DESC, u.created_at ASC
LIMIT 1;

-- Assign all existing projects to the Default workspace.
UPDATE projects
SET workspace_id = (SELECT id FROM workspaces WHERE workspace_key = 'DEFAULT')
WHERE workspace_id IS NULL;

-- Make every existing user a member of the Default workspace so they keep seeing
-- their projects.
INSERT INTO workspace_members (workspace_id, user_id)
SELECT (SELECT id FROM workspaces WHERE workspace_key = 'DEFAULT'), u.id
FROM users u
WHERE EXISTS (SELECT 1 FROM workspaces WHERE workspace_key = 'DEFAULT')
ON CONFLICT (workspace_id, user_id) DO NOTHING;

-- Every project must now live in a workspace.
ALTER TABLE projects ALTER COLUMN workspace_id SET NOT NULL;

CREATE INDEX idx_projects_workspace_id ON projects(workspace_id);
