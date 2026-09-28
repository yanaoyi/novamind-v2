-- 0013_create_entity_versions (down)

DROP INDEX IF EXISTS idx_entity_versions_work;
DROP INDEX IF EXISTS idx_entity_versions_entity;
DROP TABLE IF EXISTS entity_versions;
