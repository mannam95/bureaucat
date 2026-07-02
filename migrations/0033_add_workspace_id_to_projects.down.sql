DROP INDEX IF EXISTS idx_projects_workspace_id;
ALTER TABLE projects DROP COLUMN IF EXISTS workspace_id;
-- Remove the backfilled Default workspace (its members cascade away).
DELETE FROM workspaces WHERE workspace_key = 'DEFAULT';
