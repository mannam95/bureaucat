-- 1-10 star priority rating on modules (epics), mirroring tasks (migration 0038),
-- so the modules overview can be sorted by priority to reprioritize epics.
-- 0 = unset.
ALTER TABLE modules ADD COLUMN priority_rating INT NOT NULL DEFAULT 0
    CHECK (priority_rating >= 0 AND priority_rating <= 10);

CREATE INDEX idx_modules_priority_rating ON modules(project_id, priority_rating);
