-- 0014_create_outlines (down)

ALTER TABLE entity_versions DROP CONSTRAINT IF EXISTS entity_versions_type_valid;
ALTER TABLE entity_versions ADD CONSTRAINT entity_versions_type_valid CHECK (
    entity_type IN ('creative_character', 'creative_world', 'creative_outline')
);

DROP TABLE IF EXISTS outline_nodes;
DROP TABLE IF EXISTS outlines;
