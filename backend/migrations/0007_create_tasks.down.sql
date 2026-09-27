-- 回滚 0007_create_tasks
DROP INDEX IF EXISTS idx_tasks_work;
DROP INDEX IF EXISTS idx_tasks_project;
DROP INDEX IF EXISTS idx_tasks_claim;
DROP TABLE IF EXISTS tasks;
