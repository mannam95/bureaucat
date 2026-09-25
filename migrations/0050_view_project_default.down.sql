DROP INDEX IF EXISTS idx_project_views_project_default;

ALTER TABLE project_views
    DROP COLUMN IF EXISTS is_default;
