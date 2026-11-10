-- The design tool is not a constant; the link field should not name one.
-- figma_link becomes design_link - a rename, so every stored URL survives.
ALTER TABLE tasks RENAME COLUMN figma_link TO design_link;
