-- 0012_tasks_creative_work
-- 写作 / 一致性检查任务的归属对象是「二创作品」，而 tasks.work_id 外键指向原著 original_works。
-- 这里补一列 creative_work_id，两类作品互不干扰。

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS creative_work_id UUID REFERENCES creative_works(id);

CREATE INDEX IF NOT EXISTS idx_tasks_creative_work
    ON tasks (creative_work_id, created_at DESC) WHERE deleted_at IS NULL;
