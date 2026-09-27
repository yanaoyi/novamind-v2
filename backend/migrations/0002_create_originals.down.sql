-- 回滚 0002_create_originals
DROP INDEX IF EXISTS idx_original_chapters_work;
DROP INDEX IF EXISTS uq_original_chapters_no;
DROP TABLE IF EXISTS original_chapters;
DROP INDEX IF EXISTS uq_original_works_project;
DROP TABLE IF EXISTS original_works;
DROP INDEX IF EXISTS idx_files_project;
DROP TABLE IF EXISTS files;
