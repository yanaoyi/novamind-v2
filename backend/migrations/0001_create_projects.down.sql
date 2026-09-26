-- 回滚 0001_create_projects
DROP INDEX IF EXISTS idx_projects_deleted_at;
DROP INDEX IF EXISTS idx_projects_created_at;
DROP INDEX IF EXISTS idx_projects_status;
DROP INDEX IF EXISTS idx_projects_type;
DROP TABLE IF EXISTS projects;
