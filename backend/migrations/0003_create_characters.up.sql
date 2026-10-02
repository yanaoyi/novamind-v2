-- 0003_create_characters
-- 原著人物（SPEC.md §6.1）、人物 DNA（§6.2）、人物关系（§6.3）。
-- AI 提取结果与人工录入共用同一张表，用 source 字段区分（Phase 3 用 AI，Phase 2 手工录入）。

CREATE TABLE IF NOT EXISTS original_characters (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_work_id UUID         NOT NULL REFERENCES original_works(id) ON DELETE CASCADE,
    name             VARCHAR(120) NOT NULL,
    aliases          JSONB        NOT NULL DEFAULT '[]'::jsonb,
    role             VARCHAR(40)  NOT NULL DEFAULT '',
    gender           VARCHAR(20)  NOT NULL DEFAULT '',
    age              VARCHAR(40)  NOT NULL DEFAULT '',
    appearance       TEXT         NOT NULL DEFAULT '',
    personality      TEXT         NOT NULL DEFAULT '',
    motivation       TEXT         NOT NULL DEFAULT '',
    values_text      TEXT         NOT NULL DEFAULT '',
    fears            TEXT         NOT NULL DEFAULT '',
    desires          TEXT         NOT NULL DEFAULT '',
    behavior_patterns TEXT        NOT NULL DEFAULT '',
    speech_style     TEXT         NOT NULL DEFAULT '',
    abilities        TEXT         NOT NULL DEFAULT '',
    first_appearance VARCHAR(200) NOT NULL DEFAULT '',
    last_appearance  VARCHAR(200) NOT NULL DEFAULT '',
    -- 人物 DNA：各维度 {text, weight}，weight 0-100（SPEC.md §6.2）
    dna              JSONB        NOT NULL DEFAULT '{}'::jsonb,
    importance       SMALLINT     NOT NULL DEFAULT 3,
    source           VARCHAR(20)  NOT NULL DEFAULT 'MANUAL',
    notes            TEXT         NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT original_characters_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT original_characters_importance_range CHECK (importance BETWEEN 1 AND 5),
    CONSTRAINT original_characters_source_valid CHECK (source IN ('MANUAL', 'AI'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_original_characters_name
    ON original_characters (original_work_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_original_characters_work
    ON original_characters (original_work_id, importance DESC, created_at ASC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS character_relationships (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_work_id    UUID        NOT NULL REFERENCES original_works(id) ON DELETE CASCADE,
    source_character_id UUID        NOT NULL REFERENCES original_characters(id) ON DELETE CASCADE,
    target_character_id UUID        NOT NULL REFERENCES original_characters(id) ON DELETE CASCADE,
    relation_type       VARCHAR(20) NOT NULL,
    strength            SMALLINT    NOT NULL DEFAULT 50,
    description         TEXT        NOT NULL DEFAULT '',
    source              VARCHAR(20) NOT NULL DEFAULT 'MANUAL',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,
    CONSTRAINT character_relationships_no_self CHECK (source_character_id <> target_character_id),
    CONSTRAINT character_relationships_strength_range CHECK (strength BETWEEN 0 AND 100),
    CONSTRAINT character_relationships_type_valid CHECK (
        relation_type IN ('family', 'friend', 'lover', 'enemy', 'mentor',
                          'student', 'colleague', 'rival', 'organization', 'other')
    ),
    CONSTRAINT character_relationships_source_valid CHECK (source IN ('MANUAL', 'AI'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_character_relationships
    ON character_relationships (source_character_id, target_character_id, relation_type)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_character_relationships_work
    ON character_relationships (original_work_id) WHERE deleted_at IS NULL;

COMMENT ON TABLE  original_characters IS '原著人物；dna 为人物 DNA（各维度描述 + 0-100 权重）';
COMMENT ON TABLE  character_relationships IS '人物关系（有向边：source → target，strength 0-100）';
