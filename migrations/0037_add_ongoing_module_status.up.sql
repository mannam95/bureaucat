-- Some modules are continuous rather than delivered once (platform upkeep,
-- support, infrastructure). They never legitimately reach 'completed', and
-- parking them in 'in_progress' hides which modules are actually being pushed
-- toward a finish. 'ongoing' sits right after 'in_progress' so the enum's sort
-- order still reads as a lifecycle.
ALTER TYPE module_status ADD VALUE IF NOT EXISTS 'ongoing' AFTER 'in_progress';
