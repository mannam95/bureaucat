-- Feature gates: instance-wide switches for gated features, managed by the
-- super admin. A feature's mode decides who can use it (disabled, admins,
-- everyone); grants add specific people on top of the mode. A feature without
-- a gate row is disabled for everyone, so gated surfaces stay locked until
-- the super admin opens them.
CREATE TABLE feature_gates (
    feature TEXT PRIMARY KEY,
    mode TEXT NOT NULL DEFAULT 'disabled' CHECK (mode IN ('disabled', 'admins', 'everyone')),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE feature_grants (
    feature TEXT NOT NULL REFERENCES feature_gates(feature) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    granted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (feature, user_id)
);

-- Insights ships disabled: nobody can use it until the super admin enables it.
INSERT INTO feature_gates (feature, mode) VALUES ('insights', 'disabled');
