-- 回滚 0011_create_writing
DROP INDEX IF EXISTS idx_consistency_issues_work;
DROP TABLE IF EXISTS consistency_issues;
DROP INDEX IF EXISTS uq_chapter_versions_no;
DROP TABLE IF EXISTS chapter_versions;
DROP INDEX IF EXISTS idx_creative_scenes_chapter;
DROP TABLE IF EXISTS creative_scenes;
DROP INDEX IF EXISTS idx_creative_chapters_work;
DROP INDEX IF EXISTS uq_creative_chapters_no;
DROP TABLE IF EXISTS creative_chapters;
DROP INDEX IF EXISTS idx_creative_volumes_work;
DROP TABLE IF EXISTS creative_volumes;
