-- 0013_create_entity_versions
-- 人物 / 世界观 / 大纲的版本历史（规格书 §59：版本管理覆盖 Chapter / Character / World / Outline）。
-- 章节版本已经在 0011 里用 chapter_versions 单独存了（正文大、读写频繁）；
-- 这里用一张通用表存"结构化实体"的状态快照（payload 是 JSONB）。

CREATE TABLE IF NOT EXISTS entity_versions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type      VARCHAR(40) NOT NULL,
    entity_id        UUID        NOT NULL,
    creative_work_id UUID        NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    version_no       INTEGER     NOT NULL,
    payload          JSONB       NOT NULL,
    note             VARCHAR(200) NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT entity_versions_type_valid CHECK (
        entity_type IN ('creative_character', 'creative_world', 'creative_outline')
    ),
    CONSTRAINT entity_versions_no_positive CHECK (version_no > 0),
    CONSTRAINT entity_versions_unique_no UNIQUE (entity_type, entity_id, version_no)
);

CREATE INDEX IF NOT EXISTS idx_entity_versions_entity
    ON entity_versions (entity_type, entity_id, version_no DESC);

CREATE INDEX IF NOT EXISTS idx_entity_versions_work
    ON entity_versions (creative_work_id, created_at DESC);
