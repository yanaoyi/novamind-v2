-- 0023_create_memory
-- 长篇记忆（Phase 9 §9.3 Memory / P0-4）：让作品"越写越懂"。
--
-- 三层映射（与 docs/ARCHITECTURE.md 一致）：
--   Canonical = 原著结构化表（**只读，AI 不可改**）
--   Creative  = creative_* 表 + memory_facts
--   Episodic  = chapter_summaries + memory_facts(kind='event')
--
-- supersede 语义：同一 subject + kind 出现新事实时，把旧事实的 superseded_by 指向新行
-- （**只标记不删**）。这样"人物左臂受伤 → 后来痊愈"是可追溯的演变，
-- 而不是把历史抹掉；检索时默认只看未被替代的事实。

CREATE TABLE IF NOT EXISTS memory_facts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id    UUID        NOT NULL REFERENCES users(id),
    creative_work_id UUID        NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    kind             TEXT        NOT NULL,
    subject          TEXT        NOT NULL,
    fact             TEXT        NOT NULL,
    chapter_id       UUID        REFERENCES creative_chapters(id) ON DELETE SET NULL,
    superseded_by    UUID        REFERENCES memory_facts(id),
    embedding        vector(1024),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT memory_facts_kind_valid CHECK (
        kind IN ('character_state', 'world_state', 'event', 'item', 'relationship', 'plot')
    ),
    CONSTRAINT memory_facts_subject_not_blank CHECK (length(btrim(subject)) > 0),
    CONSTRAINT memory_facts_fact_not_blank CHECK (length(btrim(fact)) > 0),
    -- 被替代只能指向"别人"（自己指向自己会让"未被替代"的判定失去意义）
    CONSTRAINT memory_facts_no_self_supersede CHECK (superseded_by IS NULL OR superseded_by <> id)
);

CREATE INDEX IF NOT EXISTS idx_memory_facts_work_kind
    ON memory_facts (creative_work_id, kind);
-- 检索"当前有效的同类事实"用（superseded_by IS NULL 的那部分）
CREATE INDEX IF NOT EXISTS idx_memory_facts_active
    ON memory_facts (creative_work_id, kind, subject) WHERE superseded_by IS NULL;
CREATE INDEX IF NOT EXISTS idx_memory_facts_chapter
    ON memory_facts (chapter_id);

CREATE TABLE IF NOT EXISTS chapter_summaries (
    chapter_id    UUID PRIMARY KEY REFERENCES creative_chapters(id) ON DELETE CASCADE,
    owner_user_id UUID        NOT NULL REFERENCES users(id),
    summary       TEXT        NOT NULL,
    embedding     vector(1024),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chapter_summaries_summary_not_blank CHECK (length(btrim(summary)) > 0)
);

COMMENT ON TABLE  memory_facts    IS '长篇记忆事实（§9.3）：同 subject+kind 的新事实替代旧事实（只标记不删）';
COMMENT ON TABLE  chapter_summaries IS '章节摘要（§9.3）：Episodic 层，供后续章节检索回灌';
COMMENT ON COLUMN memory_facts.embedding IS '向量检索用（1024 维，与 chunks 一致）；BM25 路不依赖它';
