-- 0014_create_outlines
-- 大纲的独立模型（规格书 §27）：Outline + OutlineNode。
--
-- 为什么不再用「卷 + 章节大纲字段」近似：
--   规格书 §27 要求大纲是**先于章节存在**的结构（作品 → 卷 → 节 → 章 → 场景），
--   作者与 AI 都在这个结构上工作，章节是它的下游产物。旧实现只有「卷 + 章节」两层，
--   没有「节」，也没法在写正文之前先把整本书的结构摆出来。
--
-- 关系：creative_works 1—N outlines 1—N outline_nodes（自引用构成树）。

CREATE TABLE IF NOT EXISTS outlines (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creative_work_id UUID         NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    title            VARCHAR(200) NOT NULL,
    summary          TEXT         NOT NULL DEFAULT '',
    version          INTEGER      NOT NULL DEFAULT 1,
    -- MANUAL = 作者手写；AI = 作者采纳了 AI 候选后落库（来源可追，§52 作者拥有最终控制权）
    source           VARCHAR(10)  NOT NULL DEFAULT 'MANUAL',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT outlines_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT outlines_version_positive CHECK (version > 0),
    CONSTRAINT outlines_source_valid CHECK (source IN ('MANUAL', 'AI'))
);

CREATE INDEX IF NOT EXISTS idx_outlines_work
    ON outlines (creative_work_id, created_at ASC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS outline_nodes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outline_id UUID         NOT NULL REFERENCES outlines(id) ON DELETE CASCADE,
    parent_id  UUID REFERENCES outline_nodes(id) ON DELETE CASCADE,
    -- 1 = 卷，2 = 节，3 = 章（规格书 §27 的「卷 → 节 → 章」）
    level      SMALLINT     NOT NULL,
    sequence   INTEGER      NOT NULL DEFAULT 1,
    title      VARCHAR(200) NOT NULL,
    summary    TEXT         NOT NULL DEFAULT '',
    purpose    TEXT         NOT NULL DEFAULT '',
    characters JSONB        NOT NULL DEFAULT '[]'::jsonb,
    location   VARCHAR(200) NOT NULL DEFAULT '',
    conflict   TEXT         NOT NULL DEFAULT '',
    outcome    TEXT         NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT outline_nodes_level_valid CHECK (level IN (1, 2, 3)),
    CONSTRAINT outline_nodes_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT outline_nodes_sequence_positive CHECK (sequence > 0),
    -- 根节点（卷）必须没有父节点；子节点的层级约束由服务层校验（需要看父节点）
    CONSTRAINT outline_nodes_root_has_no_parent CHECK (level <> 1 OR parent_id IS NULL)
);

CREATE INDEX IF NOT EXISTS idx_outline_nodes_outline
    ON outline_nodes (outline_id, level, sequence ASC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_outline_nodes_parent
    ON outline_nodes (parent_id, sequence ASC) WHERE deleted_at IS NULL;

-- 大纲树的版本历史（规格书 §59 的 Outline）：
-- 旧约束只允许 creative_character / creative_world / creative_outline，
-- 这里加上大纲树自己的类型（entity_id = 大纲 id，而不是作品 id）。
ALTER TABLE entity_versions DROP CONSTRAINT IF EXISTS entity_versions_type_valid;
ALTER TABLE entity_versions ADD CONSTRAINT entity_versions_type_valid CHECK (
    entity_type IN ('creative_character', 'creative_world', 'creative_outline', 'creative_outline_tree')
);

COMMENT ON TABLE outlines IS '二创大纲（规格书 §27）：标题 + 概要 + 版本；一个作品可以有多份大纲';
COMMENT ON TABLE outline_nodes IS '大纲节点（规格书 §27）：level 1=卷 2=节 3=章，parent_id 构成树';
