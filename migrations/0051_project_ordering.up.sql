-- Global manual ordering of project cards (/projects and the dashboard),
-- set by site admins via drag-and-drop. Kept in its own table so nothing
-- else changes shape: projects without a row sort after ordered ones, by
-- name — which is also exactly the old order when no row exists at all.
CREATE TABLE project_ordering (
    project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    position   INT NOT NULL
);
