-- Add a "disabled" flag to projects. A disabled project is read-only:
-- all mutating operations are rejected until it is re-enabled.
ALTER TABLE projects ADD COLUMN disabled BOOLEAN NOT NULL DEFAULT FALSE;
