-- Activity types for adding/removing a task watcher, so the change history
-- records them like assignee changes.
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'watcher_added';
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'watcher_removed';
