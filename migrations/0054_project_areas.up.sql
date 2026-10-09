-- Areas: per-project, admin-configurable classification values; a task can
-- carry several (e.g. which product areas it touches). The product ships no
-- built-in values — each project defines its own in settings. Mirrors the
-- labels tables; kept separate so free-form tagging and controlled
-- classification don't mix.
CREATE TABLE project_areas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    color VARCHAR(7) DEFAULT '#3B82F6',  -- hex color
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, name)
);

CREATE INDEX idx_project_areas_project_id ON project_areas(project_id);

CREATE TABLE task_areas (
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    area_id UUID NOT NULL REFERENCES project_areas(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    PRIMARY KEY (task_id, area_id)
);

CREATE INDEX idx_task_areas_area_id ON task_areas(area_id);
