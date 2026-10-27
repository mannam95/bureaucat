-- New 'verification' workflow state type between 'started' and 'completed'
-- (mirrors how 0042 added 'archived'): work the developer finished building
-- that is still being validated — in testing, on staging, waiting for a
-- production release. Insights count it as its own band: open work, never
-- delivery.
ALTER TYPE state_type ADD VALUE IF NOT EXISTS 'verification' BEFORE 'completed';
