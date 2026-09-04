-- Per-user preferences: durable, cross-device view and UI settings.
--
-- One row per (user, preference_key, scope). Scope is inferred from the
-- nullable foreign keys:
--   global:    workspace_id IS NULL AND project_id IS NULL
--   workspace: workspace_id IS NOT NULL   (reserved; no workspace keys yet)
--   project:   project_id  IS NOT NULL
--
-- The table stores only user overrides. A missing row means "use the backend
-- registry default", so product defaults can evolve without touching stored
-- choices. Resetting a preference deletes its row. `revision` gives optimistic
-- concurrency across tabs and devices; `value_version` records the shape of the
-- JSON in `value` so older shapes can be migrated in a future release.

CREATE TABLE user_preferences (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    preference_key  TEXT NOT NULL,
    workspace_id    UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id      UUID REFERENCES projects(id) ON DELETE CASCADE,
    value           JSONB NOT NULL,
    value_version   INTEGER NOT NULL DEFAULT 1,
    revision        BIGINT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- At most one scope column may be set; both NULL means global scope.
    CONSTRAINT ck_user_preferences_single_scope
        CHECK (workspace_id IS NULL OR project_id IS NULL),
    -- Dotted, lowercase semantic keys (e.g. app.theme, tasks.list.view_state).
    CONSTRAINT ck_user_preferences_key
        CHECK (preference_key ~ '^[a-z][a-z0-9_]*([.][a-z][a-z0-9_]*)+$'),
    -- A JSON null is never a valid stored value; delete the row to reset.
    CONSTRAINT ck_user_preferences_value_not_null
        CHECK (jsonb_typeof(value) <> 'null'),
    CONSTRAINT ck_user_preferences_versions
        CHECK (value_version > 0 AND revision > 0)
);

-- Per-scope uniqueness via partial indexes: the nullable scope columns behave
-- cleanly and each index gives ON CONFLICT a precise arbiter. These also serve
-- as the lookup indexes for the per-scope list queries (all lead with user_id).
CREATE UNIQUE INDEX uq_user_preferences_global
    ON user_preferences (user_id, preference_key)
    WHERE workspace_id IS NULL AND project_id IS NULL;

CREATE UNIQUE INDEX uq_user_preferences_workspace
    ON user_preferences (user_id, preference_key, workspace_id)
    WHERE workspace_id IS NOT NULL;

CREATE UNIQUE INDEX uq_user_preferences_project
    ON user_preferences (user_id, preference_key, project_id)
    WHERE project_id IS NOT NULL;
