-- Extend the activity_type enum so the change history can capture every task
-- mutation: cycle (sprint) add/remove, module (epic) add/remove, attachment
-- add/remove, and cross-project moves (task_moved was referenced in code but
-- never added to the enum, so those inserts failed silently).
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'task_moved';
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'cycle_added';
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'cycle_removed';
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'module_added';
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'module_removed';
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'attachment_added';
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'attachment_removed';
