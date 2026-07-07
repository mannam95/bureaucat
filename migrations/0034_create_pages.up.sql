-- Project documentation pages
CREATE TABLE pages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    page_number INT NOT NULL,  -- Sequential per project
    title VARCHAR(500) NOT NULL,
    content TEXT NOT NULL DEFAULT '',  -- rich tiptap content stored as HTML
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,  -- soft delete
    UNIQUE(project_id, page_number)
);

CREATE INDEX idx_pages_project_page_number ON pages(project_id, page_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_pages_created_by ON pages(created_by);
