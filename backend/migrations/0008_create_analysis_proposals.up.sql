-- 0008_create_analysis_proposals
-- AI 分析提案（SPEC.md §22.2 的红线落地）：
-- 模型产出**不直接写原著模型**，先落在这张表里等作者审核；作者通过（可带修改）后才写入正式表。
-- 这样"AI 提取结果必须经过作者确认"就不是一句约束，而是数据库层面的事实。

CREATE TABLE IF NOT EXISTS analysis_proposals (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    work_id      UUID         NOT NULL REFERENCES original_works(id) ON DELETE CASCADE,
    task_id      UUID REFERENCES tasks(id),
    stage        VARCHAR(40)  NOT NULL,
    entity_type  VARCHAR(40)  NOT NULL,
    title        VARCHAR(200) NOT NULL DEFAULT '',
    payload      JSONB        NOT NULL DEFAULT '{}'::jsonb,
    evidence     TEXT         NOT NULL DEFAULT '',
    confidence   SMALLINT     NOT NULL DEFAULT 0,
    status       VARCHAR(20)  NOT NULL DEFAULT 'PENDING',
    review_note  TEXT         NOT NULL DEFAULT '',
    reviewed_at  TIMESTAMPTZ,
    applied_id   UUID,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    CONSTRAINT analysis_proposals_stage_valid CHECK (
        stage IN ('chapter_summary', 'character_extract', 'world_extract', 'plot_extract')
    ),
    CONSTRAINT analysis_proposals_entity_valid CHECK (
        entity_type IN ('chapter_summary', 'character', 'world', 'world_rule', 'location',
                        'faction', 'event', 'plot_arc')
    ),
    CONSTRAINT analysis_proposals_status_valid CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    CONSTRAINT analysis_proposals_confidence_range CHECK (confidence BETWEEN 0 AND 100)
);

CREATE INDEX IF NOT EXISTS idx_proposals_work_status
    ON analysis_proposals (work_id, status, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_proposals_task
    ON analysis_proposals (task_id) WHERE deleted_at IS NULL;
-- 同一次分析任务里，同一个实体的提案只留一条
CREATE UNIQUE INDEX IF NOT EXISTS uq_proposals_task_entity
    ON analysis_proposals (task_id, entity_type, lower(title)) WHERE deleted_at IS NULL AND task_id IS NOT NULL;

COMMENT ON TABLE analysis_proposals IS 'AI 分析提案；只有作者 approve 后才写入原著正式表（SPEC.md §22.2）';
COMMENT ON COLUMN analysis_proposals.applied_id IS '审核通过后写入正式表得到的记录 ID';
