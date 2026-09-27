-- 0012_tasks_creative_work (down)

DROP INDEX IF EXISTS idx_tasks_creative_work;
ALTER TABLE tasks DROP COLUMN IF EXISTS creative_work_id;
