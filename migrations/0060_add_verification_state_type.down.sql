-- PostgreSQL cannot remove an enum value without recreating the type and
-- rewriting every dependent column, so this migration is not reversible.
SELECT 1;
