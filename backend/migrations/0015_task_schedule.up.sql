-- 0015_task_schedule
-- 任务调度补两个洞（代码质量审查 P1-1 / P2）：
--   1) RUNNING 任务永不回收：worker 崩溃或进程重启后，任务会永久卡在 RUNNING；
--      配合代码里的回收器（启动时 + 周期性），把超时未推进的任务放回 PENDING。
--   2) 失败重试没有退避：失败立刻回 PENDING，worker 2 秒轮询马上重领，
--      确定性失败会在几秒内烧完重试次数，瞬时故障则反复锤上游。
--      新增 next_run_at：未到点不领取，退避由代码按尝试次数计算。

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;

-- 领取任务时按 (status, next_run_at) 过滤，部分索引让队列查询保持轻量
CREATE INDEX IF NOT EXISTS idx_tasks_claim
    ON tasks (status, next_run_at, created_at)
    WHERE deleted_at IS NULL;

COMMENT ON COLUMN tasks.next_run_at IS '下次可领取时间（失败退避）；NULL 表示立即可领取';
