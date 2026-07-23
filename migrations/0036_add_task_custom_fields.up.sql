-- Fixed custom fields on a task: where the design lives, which branch the work
-- is on, and the pull request that delivers it. Sub-tasks are rows in the same
-- table, so they get these fields too. All optional and free-text.
ALTER TABLE tasks
    ADD COLUMN figma_link   TEXT,
    ADD COLUMN branch       TEXT,
    ADD COLUMN pull_request TEXT;
