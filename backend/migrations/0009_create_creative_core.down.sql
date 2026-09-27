-- 回滚 0009_create_creative_core
DROP INDEX IF EXISTS idx_mappings_original;
DROP INDEX IF EXISTS idx_mappings_work;
DROP TABLE IF EXISTS original_creative_mappings;
DROP INDEX IF EXISTS uq_inheritance_rules_pair;
DROP TABLE IF EXISTS inheritance_rules;
DROP INDEX IF EXISTS idx_creative_characters_work;
DROP INDEX IF EXISTS uq_creative_characters_name;
DROP TABLE IF EXISTS creative_characters;
DROP INDEX IF EXISTS idx_creative_works_original;
DROP INDEX IF EXISTS uq_creative_works_project;
DROP TABLE IF EXISTS creative_works;
