-- 0005_create_events_timeline_plot
-- 原著事件（SPEC.md §8.1）、原著时间线（§8.2）、原著剧情结构（§8.3）。
-- 说明：V1 一部原著一条时间线（original_timelines 在作品上唯一）；
--       时间线只保存"事件 + 顺序 + 时间标签"，事件本体在 original_events。

CREATE TABLE IF NOT EXISTS original_events (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_work_id UUID         NOT NULL REFERENCES original_works(id) ON DELETE CASCADE,
    title            VARCHAR(200) NOT NULL,
    description      TEXT         NOT NULL DEFAULT '',
    chapter_no       INTEGER,
    time_order       INTEGER      NOT NULL DEFAULT 0,
    participants     JSONB        NOT NULL DEFAULT '[]'::jsonb,
    location_id      UUID REFERENCES locations(id),
    location_text    VARCHAR(200) NOT NULL DEFAULT '',
    consequences     TEXT         NOT NULL DEFAULT '',
    importance       SMALLINT     NOT NULL DEFAULT 3,
    source           VARCHAR(20)  NOT NULL DEFAULT 'MANUAL',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT original_events_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT original_events_importance_range CHECK (importance BETWEEN 1 AND 5),
    CONSTRAINT original_events_chapter_positive CHECK (chapter_no IS NULL OR chapter_no > 0),
    CONSTRAINT original_events_source_valid CHECK (source IN ('MANUAL', 'AI'))
);

CREATE INDEX IF NOT EXISTS idx_original_events_work
    ON original_events (original_work_id, time_order ASC, created_at ASC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS original_timelines (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_work_id UUID         NOT NULL REFERENCES original_works(id) ON DELETE CASCADE,
    name             VARCHAR(200) NOT NULL DEFAULT '主线时间线',
    description      TEXT         NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_original_timelines_work
    ON original_timelines (original_work_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS timeline_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timeline_id UUID        NOT NULL REFERENCES original_timelines(id) ON DELETE CASCADE,
    event_id    UUID        NOT NULL REFERENCES original_events(id) ON DELETE CASCADE,
    sequence    INTEGER     NOT NULL,
    time_label  VARCHAR(120) NOT NULL DEFAULT '',
    duration    VARCHAR(120) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    CONSTRAINT timeline_events_sequence_positive CHECK (sequence > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_timeline_events
    ON timeline_events (timeline_id, event_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_timeline_events_order
    ON timeline_events (timeline_id, sequence ASC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS plot_arcs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_work_id UUID         NOT NULL REFERENCES original_works(id) ON DELETE CASCADE,
    type             VARCHAR(30)  NOT NULL DEFAULT 'main',
    title            VARCHAR(200) NOT NULL,
    summary          TEXT         NOT NULL DEFAULT '',
    start_event_id   UUID REFERENCES original_events(id),
    end_event_id     UUID REFERENCES original_events(id),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT plot_arcs_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT plot_arcs_type_valid CHECK (
        type IN ('main', 'subplot', 'character_arc', 'relationship_arc', 'world_arc')
    )
);

CREATE INDEX IF NOT EXISTS idx_plot_arcs_work
    ON plot_arcs (original_work_id, created_at ASC) WHERE deleted_at IS NULL;

COMMENT ON TABLE original_events IS '原著事件；participants 存人物 ID 数组，location_id 可挂到地点';
COMMENT ON TABLE timeline_events IS '时间线条目：事件 + 顺序 + 时间标签（同一事件在一条时间线只出现一次）';
COMMENT ON TABLE plot_arcs     IS '剧情弧：主线 / 支线 / 人物线 / 感情线 / 世界线';
