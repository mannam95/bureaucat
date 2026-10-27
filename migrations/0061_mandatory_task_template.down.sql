DROP INDEX IF EXISTS idx_task_templates_one_mandatory;
ALTER TABLE task_templates DROP COLUMN IF EXISTS is_mandatory;
