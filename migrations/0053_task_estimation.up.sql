-- DEV estimation per task: Difficulty (1-5) and Effort (1-5), multiplied into
-- a complexity score (1-25) computed on read. 0 means "not assessed yet".
ALTER TABLE tasks
    ADD COLUMN difficulty INT NOT NULL DEFAULT 0 CHECK (difficulty BETWEEN 0 AND 5),
    ADD COLUMN effort INT NOT NULL DEFAULT 0 CHECK (effort BETWEEN 0 AND 5);
