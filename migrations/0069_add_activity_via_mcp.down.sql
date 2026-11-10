-- WARNING: rewriting via on existing rows breaks their v2 activity
-- checksums (via is part of the checksum input), so VerifyChain will
-- report those tasks as tampered after this down migration.
UPDATE activity_log SET via = 'pat' WHERE via = 'mcp';
UPDATE notifications SET via = 'pat' WHERE via = 'mcp';
ALTER TABLE activity_log DROP CONSTRAINT IF EXISTS activity_log_via_check;
ALTER TABLE activity_log ADD CONSTRAINT activity_log_via_check CHECK (via IN ('web', 'assistant', 'pat', 'workflow'));
