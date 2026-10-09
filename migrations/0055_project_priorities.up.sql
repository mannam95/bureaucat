-- Priorities become first-class, per-project entities. Each project owns its
-- priority set (name, description, color, rank, active); tasks reference a
-- priority row by id. Rank drives ordering and "at least/at most" filters —
-- higher rank = more urgent. Admins can add, rename, describe, recolor,
-- reorder and deactivate values in project settings.

CREATE TABLE project_priorities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(50) NOT NULL,
    description TEXT,
    color VARCHAR(7) DEFAULT '#6B7280',
    rank INT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, name)
);

CREATE INDEX idx_project_priorities_project_id ON project_priorities(project_id);

-- Seed the five standard levels into every existing project, keeping the
-- historical 0-4 value as the rank so day one looks exactly like yesterday.
INSERT INTO project_priorities (project_id, name, color, rank)
SELECT p.id, v.name, v.color, v.rank
FROM projects p
CROSS JOIN (VALUES
    ('No priority', '#6B7280', 0),
    ('Low',         '#3B82F6', 1),
    ('Medium',      '#EAB308', 2),
    ('High',        '#F97316', 3),
    ('Urgent',      '#EF4444', 4)
) AS v(name, color, rank);

-- Tasks point at their project's matching priority row.
ALTER TABLE tasks ADD COLUMN priority_id UUID REFERENCES project_priorities(id);

UPDATE tasks t
SET priority_id = pp.id
FROM project_priorities pp
WHERE pp.project_id = t.project_id
  AND pp.rank = LEAST(GREATEST(t.priority, 0), 4);

ALTER TABLE tasks ALTER COLUMN priority_id SET NOT NULL;
CREATE INDEX idx_tasks_priority_id ON tasks(priority_id);

-- The old integer column stays FROZEN for one release as a manual-recovery
-- safety net; nothing reads or writes it any more. A later cleanup migration
-- drops it.
COMMENT ON COLUMN tasks.priority IS
    'DEPRECATED: frozen copy of the pre-entity 0-4 priority. Use priority_id.';

-- Rewrite stored priority filters from integers to the new ids, in both saved
-- views (filter_tree) and each user''s remembered per-project filter state
-- (user_preferences value->filter for the two view-state keys). The helper
-- only touches predicates whose field is exactly "priority":
--   in / not_in : [3, 4]  -> ["<id of rank 3>", "<id of rank 4>"]
--   gte / lte   : 3       -> "<id of rank 3>"   (compared by rank at runtime)
CREATE OR REPLACE FUNCTION _migrate_priority_tree(tree jsonb, pid uuid) RETURNS jsonb AS $$
DECLARE
    result jsonb := '[]'::jsonb;
    child jsonb;
    pred jsonb;
    newval jsonb;
    elem jsonb;
    mapped jsonb;
BEGIN
    IF tree IS NULL OR jsonb_typeof(tree->'children') <> 'array' THEN
        RETURN tree;
    END IF;
    FOR child IN SELECT * FROM jsonb_array_elements(tree->'children') LOOP
        pred := child->'predicate';
        IF pred IS NOT NULL AND pred->>'field' = 'priority' THEN
            IF pred->>'op' IN ('in', 'not_in') AND jsonb_typeof(pred->'value') = 'array' THEN
                newval := '[]'::jsonb;
                FOR elem IN SELECT * FROM jsonb_array_elements(pred->'value') LOOP
                    IF jsonb_typeof(elem) = 'number' THEN
                        SELECT to_jsonb(pp.id::text) INTO mapped
                        FROM project_priorities pp
                        WHERE pp.project_id = pid AND pp.rank = (elem #>> '{}')::int
                        LIMIT 1;
                        newval := newval || COALESCE(mapped, elem);
                    ELSE
                        newval := newval || elem;
                    END IF;
                END LOOP;
                pred := jsonb_set(pred, '{value}', newval);
            ELSIF pred->>'op' IN ('gte', 'lte') AND jsonb_typeof(pred->'value') = 'number' THEN
                SELECT to_jsonb(pp.id::text) INTO mapped
                FROM project_priorities pp
                WHERE pp.project_id = pid AND pp.rank = (pred->>'value')::int
                LIMIT 1;
                pred := jsonb_set(pred, '{value}', COALESCE(mapped, pred->'value'));
            END IF;
            child := jsonb_set(child, '{predicate}', pred);
        END IF;
        result := result || jsonb_build_array(child);
    END LOOP;
    RETURN jsonb_set(tree, '{children}', result);
END;
$$ LANGUAGE plpgsql;

UPDATE project_views pv
SET filter_tree = _migrate_priority_tree(filter_tree, pv.project_id)
WHERE filter_tree::text LIKE '%"priority"%';

UPDATE user_preferences up
SET value = jsonb_set(value, '{filter}', _migrate_priority_tree(value->'filter', up.project_id)),
    revision = revision + 1,
    updated_at = NOW()
WHERE up.project_id IS NOT NULL
  AND up.preference_key IN ('tasks.list.view_state', 'board.view_state')
  AND jsonb_typeof(value->'filter') = 'object'
  AND (value->'filter')::text LIKE '%"priority"%';

DROP FUNCTION _migrate_priority_tree(jsonb, uuid);
