-- 回滚 0004_create_world
DROP INDEX IF EXISTS idx_factions_world;
DROP INDEX IF EXISTS uq_factions_name;
DROP TABLE IF EXISTS factions;
DROP INDEX IF EXISTS idx_locations_world;
DROP INDEX IF EXISTS uq_locations_name;
DROP TABLE IF EXISTS locations;
DROP INDEX IF EXISTS idx_world_rules_world;
DROP INDEX IF EXISTS uq_world_rules_name;
DROP TABLE IF EXISTS world_rules;
DROP INDEX IF EXISTS uq_original_worlds_work;
DROP TABLE IF EXISTS original_worlds;
