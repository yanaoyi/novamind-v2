-- 0015_task_schedule (down)

DROP INDEX IF EXISTS idx_tasks_claim;
ALTER TABLE tasks DROP COLUMN IF EXISTS next_run_at;
