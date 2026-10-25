-- Activity types for task areas and contributors. The handlers already write
-- them; without these enum values the inserts are rejected and the feed
-- entries silently lost (LogActivity errors are best-effort by design).
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'area_added';
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'area_removed';
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'contributor_added';
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'contributor_removed';
