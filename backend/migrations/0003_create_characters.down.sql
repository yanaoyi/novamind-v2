-- 回滚 0003_create_characters
DROP INDEX IF EXISTS idx_character_relationships_work;
DROP INDEX IF EXISTS uq_character_relationships;
DROP TABLE IF EXISTS character_relationships;
DROP INDEX IF EXISTS idx_original_characters_work;
DROP INDEX IF EXISTS uq_original_characters_name;
DROP TABLE IF EXISTS original_characters;
