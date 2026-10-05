-- 0019_create_chunks
-- Retrieval 的存储层（Phase 9 任务书 §9.1.1）。
--
-- 与任务书的两处**有意偏离**，都写在这里免得后来人以为漏了：
--   1) **没有 embedding 列**：任务书要用 pgvector 的 vector(1024)，但本机 PostgreSQL 15.19
--      既没有 vector 扩展、apt 也没有对应包、sudo 需密码装不了 → 按任务书兜底走
--      "BM25-only（Go 内实现，零依赖）"，vector 记为阻塞项。等 pgvector 可用时，
--      用一条 ALTER TABLE 加上 embedding 列 + ivfflat 索引即可，不影响已有数据。
--   2) 不设 work_id / chapter_id / ref_id 的外键：它们跨 original_works 与 creative_works
--      两张表（work_kind 决定指哪张），加不了单表 FK —— 与任务书注释一致。

CREATE TABLE IF NOT EXISTS chunks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID        NOT NULL REFERENCES users(id),
    work_kind     TEXT        NOT NULL,
    work_id       UUID        NOT NULL,
    chapter_id    UUID,
    ref_kind      TEXT        NOT NULL DEFAULT 'chapter',
    ref_id        UUID,
    seq           INTEGER     NOT NULL DEFAULT 0,
    content       TEXT        NOT NULL,
    token_count   INTEGER     NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chunks_work_kind_valid CHECK (work_kind IN ('original', 'creative')),
    CONSTRAINT chunks_ref_kind_valid CHECK (
        ref_kind IN ('chapter', 'outline_node', 'world_rule', 'character', 'event', 'memory_fact', 'chapter_summary')
    ),
    CONSTRAINT chunks_content_not_blank CHECK (length(btrim(content)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_chunks_work  ON chunks (work_kind, work_id);
CREATE INDEX IF NOT EXISTS idx_chunks_owner ON chunks (owner_user_id);
-- 增量重建按 (ref_kind, ref_id) 先删后建，这个索引让删除与回溯都快
CREATE INDEX IF NOT EXISTS idx_chunks_ref   ON chunks (ref_kind, ref_id);

COMMENT ON TABLE chunks IS '检索分块（Phase 9 §9.1）：当前 BM25-only；embedding 列待 pgvector 可用后补加';
