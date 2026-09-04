-- New 'archived' workflow state type (mirrors how 0037 added 'ongoing' to
-- module_status). An admin can mark a project state as Archived; archived tasks
-- count as complete in cycle/module progress but stay discoverable by state.
ALTER TYPE state_type ADD VALUE IF NOT EXISTS 'archived';
