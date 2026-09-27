-- 回滚 0008_create_analysis_proposals
DROP INDEX IF EXISTS uq_proposals_task_entity;
DROP INDEX IF EXISTS idx_proposals_task;
DROP INDEX IF EXISTS idx_proposals_work_status;
DROP TABLE IF EXISTS analysis_proposals;
