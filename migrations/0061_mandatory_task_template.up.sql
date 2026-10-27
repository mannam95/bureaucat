-- A project admin can mark one template as mandatory: the create-task dialog
-- then opens pre-filled with it, so everyone sees how a task is expected to be
-- written. A guide, not a lock — the text stays fully editable.
ALTER TABLE task_templates ADD COLUMN is_mandatory BOOLEAN NOT NULL DEFAULT FALSE;

-- At most one mandatory template per project.
CREATE UNIQUE INDEX idx_task_templates_one_mandatory ON task_templates(project_id) WHERE is_mandatory;
