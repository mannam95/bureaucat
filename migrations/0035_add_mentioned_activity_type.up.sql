-- The notifications table reuses the activity_type enum, so add a 'mentioned'
-- value to surface an @mention as its own notification kind (the mentioned user
-- may not be a task participant, so it can't ride the normal activity fan-out).
ALTER TYPE activity_type ADD VALUE IF NOT EXISTS 'mentioned';
