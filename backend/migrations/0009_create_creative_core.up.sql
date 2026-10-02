-- 0009_create_creative_core
-- 二创核心（SPEC.md §9.1 二创作品、§9.2 二创人物、§9.3 人物继承规则、§9.5 原著↔二创映射）。
-- 铁律：二创数据独立成表，原著表只读引用；继承关系用映射表显式记录，不靠"隐含推断"。

CREATE TABLE IF NOT EXISTS creative_works (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          UUID         NOT NULL REFERENCES projects(id),
    original_work_id    UUID         NOT NULL REFERENCES original_works(id),
    title               VARCHAR(200) NOT NULL,
    description         TEXT         NOT NULL DEFAULT '',
    status              VARCHAR(20)  NOT NULL DEFAULT 'DRAFT',
    divergence_point_id UUID,                     -- 分叉点（P4-3 建表后回填）
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,
    CONSTRAINT creative_works_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT creative_works_status_valid CHECK (status IN ('DRAFT', 'WRITING', 'FINISHED'))
);

-- 一个二创工程只能有一部二创作品
CREATE UNIQUE INDEX IF NOT EXISTS uq_creative_works_project
    ON creative_works (project_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_creative_works_original
    ON creative_works (original_work_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS creative_characters (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creative_work_id    UUID         NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    name                VARCHAR(120) NOT NULL,
    description         TEXT         NOT NULL DEFAULT '',
    -- ORIGINAL_INHERITED=整人继承；MODIFIED=继承后改；FUSED=多人融合；NEW=原创
    source_type         VARCHAR(25)  NOT NULL,
    source_character_id UUID REFERENCES original_characters(id),
    -- 派生后的 DNA（按继承权重算出来的结果，作者可继续改）
    dna                 JSONB        NOT NULL DEFAULT '{}'::jsonb,
    -- 融合说明：每个维度取自哪个来源（SPEC.md §9.4）
    fusion_sources      JSONB        NOT NULL DEFAULT '[]'::jsonb,
    fusion_detail       JSONB        NOT NULL DEFAULT '[]'::jsonb,
    modifications       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    importance          SMALLINT     NOT NULL DEFAULT 3,
    is_locked           BOOLEAN      NOT NULL DEFAULT false,  -- 锁定后拒绝被重新继承覆盖
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,
    CONSTRAINT creative_characters_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT creative_characters_source_valid CHECK (
        source_type IN ('ORIGINAL_INHERITED', 'MODIFIED', 'FUSED', 'NEW')
    ),
    CONSTRAINT creative_characters_importance_range CHECK (importance BETWEEN 1 AND 5)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_creative_characters_name
    ON creative_characters (creative_work_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_creative_characters_work
    ON creative_characters (creative_work_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS inheritance_rules (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creative_character_id  UUID        NOT NULL REFERENCES creative_characters(id) ON DELETE CASCADE,
    source_character_id    UUID        NOT NULL REFERENCES original_characters(id),
    personality_weight     SMALLINT    NOT NULL DEFAULT 100,
    value_weight           SMALLINT    NOT NULL DEFAULT 100,
    motivation_weight      SMALLINT    NOT NULL DEFAULT 100,
    behavior_weight        SMALLINT    NOT NULL DEFAULT 100,
    speech_weight          SMALLINT    NOT NULL DEFAULT 100,
    background_weight      SMALLINT    NOT NULL DEFAULT 100,
    ability_weight         SMALLINT    NOT NULL DEFAULT 100,
    relationship_weight    SMALLINT    NOT NULL DEFAULT 100,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at             TIMESTAMPTZ,
    CONSTRAINT inheritance_rules_weights_range CHECK (
        personality_weight BETWEEN 0 AND 100 AND value_weight BETWEEN 0 AND 100 AND
        motivation_weight BETWEEN 0 AND 100 AND behavior_weight BETWEEN 0 AND 100 AND
        speech_weight BETWEEN 0 AND 100 AND background_weight BETWEEN 0 AND 100 AND
        ability_weight BETWEEN 0 AND 100 AND relationship_weight BETWEEN 0 AND 100
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inheritance_rules_pair
    ON inheritance_rules (creative_character_id, source_character_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS original_creative_mappings (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creative_work_id UUID         NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    original_type    VARCHAR(30)  NOT NULL,
    original_id      UUID         NOT NULL,
    creative_type    VARCHAR(30)  NOT NULL,
    creative_id      UUID         NOT NULL,
    mapping_type     VARCHAR(20)  NOT NULL,
    description      TEXT         NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT mappings_original_type_valid CHECK (
        original_type IN ('character', 'world', 'world_rule', 'location', 'faction', 'event', 'chapter')
    ),
    CONSTRAINT mappings_creative_type_valid CHECK (
        creative_type IN ('creative_character', 'creative_world', 'creative_world_rule',
                          'creative_timeline_event', 'chapter')
    ),
    CONSTRAINT mappings_type_valid CHECK (
        mapping_type IN ('INHERITED', 'MODIFIED', 'REPLACED', 'FUSED', 'REMOVED', 'NEW')
    )
);

CREATE INDEX IF NOT EXISTS idx_mappings_work
    ON original_creative_mappings (creative_work_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_mappings_original
    ON original_creative_mappings (original_id) WHERE deleted_at IS NULL;

COMMENT ON TABLE creative_works  IS '二创作品：挂在 CREATIVE 类型工程下，必须指向一部原著';
COMMENT ON TABLE creative_characters IS '二创人物：DNA 由继承权重派生，作者可继续修改';
COMMENT ON TABLE inheritance_rules IS '人物继承权重（SPEC.md §9.3）：各维度 0-100，0 表示不继承';
COMMENT ON TABLE original_creative_mappings IS '原著↔二创映射（SPEC.md §9.5）：显式记录每个二创元素从哪来';
