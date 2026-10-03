-- 0010_create_creative_world_timeline
-- 二创世界（规格书 §21 二创世界、§22 二创世界规则）、分叉点（§24）、二创时间线（§25）。

CREATE TABLE IF NOT EXISTS creative_worlds (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creative_work_id UUID         NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    source_world_id  UUID REFERENCES original_worlds(id),
    -- FULL=整套继承；PARTIAL=默认继承但逐条可改；MODIFIED=继承基准上大改；NEW=全新世界
    inheritance_mode VARCHAR(10)  NOT NULL DEFAULT 'FULL',
    name             VARCHAR(200) NOT NULL DEFAULT '',
    description      TEXT         NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT creative_worlds_mode_valid CHECK (inheritance_mode IN ('FULL', 'PARTIAL', 'MODIFIED', 'NEW'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_creative_worlds_work
    ON creative_worlds (creative_work_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS creative_world_rules (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creative_world_id UUID         NOT NULL REFERENCES creative_worlds(id) ON DELETE CASCADE,
    source_rule_id    UUID REFERENCES world_rules(id),
    -- INHERITED=原样继承；MODIFIED=继承后改；REMOVED=原著有但二创里删掉；NEW=二创新增
    status            VARCHAR(12)  NOT NULL DEFAULT 'NEW',
    category          VARCHAR(60)  NOT NULL DEFAULT '',
    name              VARCHAR(200) NOT NULL,
    description       TEXT         NOT NULL DEFAULT '',
    importance        SMALLINT     NOT NULL DEFAULT 3,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ,
    CONSTRAINT creative_world_rules_status_valid CHECK (status IN ('INHERITED', 'MODIFIED', 'REMOVED', 'NEW')),
    CONSTRAINT creative_world_rules_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT creative_world_rules_importance_range CHECK (importance BETWEEN 1 AND 5)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_creative_world_rules_name
    ON creative_world_rules (creative_world_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_creative_world_rules_world
    ON creative_world_rules (creative_world_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS divergence_points (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creative_work_id    UUID         NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    original_chapter_id UUID REFERENCES original_chapters(id),
    original_event_id   UUID REFERENCES original_events(id),
    time_label          VARCHAR(120) NOT NULL DEFAULT '',
    description         TEXT         NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_divergence_points_work
    ON divergence_points (creative_work_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS creative_timeline_events (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creative_work_id          UUID         NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    source_original_event_id  UUID REFERENCES original_events(id),
    status                    VARCHAR(12)  NOT NULL DEFAULT 'NEW',
    sequence                  INTEGER      NOT NULL,
    time_label                VARCHAR(120) NOT NULL DEFAULT '',
    title                     VARCHAR(200) NOT NULL,
    description               TEXT         NOT NULL DEFAULT '',
    created_at                TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at                TIMESTAMPTZ,
    CONSTRAINT creative_timeline_events_status_valid CHECK (status IN ('INHERITED', 'MODIFIED', 'NEW', 'REMOVED')),
    CONSTRAINT creative_timeline_events_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT creative_timeline_events_sequence_positive CHECK (sequence > 0)
);

CREATE INDEX IF NOT EXISTS idx_creative_timeline_events_order
    ON creative_timeline_events (creative_work_id, sequence ASC) WHERE deleted_at IS NULL;
-- 同一条原著事件在一条二创时间线上只继承一次
CREATE UNIQUE INDEX IF NOT EXISTS uq_creative_timeline_source
    ON creative_timeline_events (creative_work_id, source_original_event_id)
    WHERE deleted_at IS NULL AND source_original_event_id IS NOT NULL;

COMMENT ON TABLE creative_worlds IS '二创世界：标注继承模式（FULL/PARTIAL/MODIFIED/NEW），规则逐条可改';
COMMENT ON TABLE divergence_points IS '分叉点（规格书 §24）：二创从哪里离开原著';
COMMENT ON TABLE creative_timeline_events IS '二创时间线（规格书 §25）：分叉点之前继承原著事件，之后是二创新事件';
