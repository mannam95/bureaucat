-- One shared view per project can be marked as the project's default view.
-- It is applied automatically for members who haven't personalised their own
-- view state yet; anyone can still change filters afterwards.

ALTER TABLE project_views
    ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX idx_project_views_project_default
    ON project_views(project_id) WHERE is_default AND deleted_at IS NULL;
