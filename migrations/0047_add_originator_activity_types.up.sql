-- Activity types for adding/removing a task requester (originator), so the
-- change history records them like assignee changes.
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'originator_added';
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'originator_removed';
