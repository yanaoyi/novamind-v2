-- 0020_create_context_snapshots
-- 上下文快照（Phase 9 §9.2.2）：每次 AI 调用**先落快照再调模型**，
-- 目的是"当时到底给了 AI 什么"可追溯 —— 生成质量出问题时这是第一手证据。
--
-- 注意 snapshot 是**不可变**的：只插入、不更新（没有 update API），
-- 所以表里没有 updated_at。

CREATE TABLE IF NOT EXISTS context_snapshots (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id    UUID        NOT NULL REFERENCES users(id),
    creative_work_id UUID        NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    chapter_id       UUID        REFERENCES creative_chapters(id) ON DELETE CASCADE,
    kind             TEXT        NOT NULL,
    snapshot         JSONB       NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT context_snapshots_kind_valid CHECK (
        kind IN ('generate', 'continue', 'rewrite', 'expand', 'analyze', 'consistency')
    )
);

CREATE INDEX IF NOT EXISTS idx_context_snapshots_chapter
    ON context_snapshots (chapter_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_context_snapshots_work
    ON context_snapshots (creative_work_id, created_at DESC);

COMMENT ON TABLE context_snapshots IS '上下文快照（§9.2）：8 段组装结果 + 检索来源 + 模型与 prompt 版本；只增不改';
