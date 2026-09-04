-- PostgreSQL cannot remove enum values without recreating the type and rewriting
-- every dependent column, so this migration is intentionally not reversible.
SELECT 1;
