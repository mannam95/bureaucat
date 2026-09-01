-- A fine-grained "star" priority rating (1-10), separate from the existing
-- coarse `priority` label (0=none..4=urgent). 0 means unset. Five priority
-- labels proved too few to rank a large sprint, so this gives a 1-10 scale the
-- team can sort a cycle by. The old `priority` field is kept as-is.
-- INT (not SMALLINT) to match the existing `priority` column, so it reuses the
-- same int32 / pgtype.Int4 plumbing throughout the codebase.
ALTER TABLE tasks
    ADD COLUMN priority_rating INT NOT NULL DEFAULT 0
        CHECK (priority_rating >= 0 AND priority_rating <= 10);

-- Sorting a sprint/cycle by rating is the primary use, so index it per project.
CREATE INDEX idx_tasks_priority_rating ON tasks(project_id, priority_rating);
