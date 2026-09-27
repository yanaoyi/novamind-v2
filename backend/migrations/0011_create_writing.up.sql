-- 0011_create_writing
-- 写作系统（规格书 §27 大纲、§28 章节、§29 场景、§59 版本管理）+ 一致性检查（§39）。

CREATE TABLE IF NOT EXISTS creative_volumes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creative_work_id UUID         NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    title            VARCHAR(200) NOT NULL,
    summary          TEXT         NOT NULL DEFAULT '',
    sequence         INTEGER      NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT creative_volumes_title_not_blank CHECK (length(btrim(title)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_creative_volumes_work
    ON creative_volumes (creative_work_id, sequence ASC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS creative_chapters (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creative_work_id UUID         NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    volume_id        UUID REFERENCES creative_volumes(id),
    chapter_no       INTEGER      NOT NULL,
    title            VARCHAR(200) NOT NULL,
    summary          TEXT         NOT NULL DEFAULT '',
    content          TEXT         NOT NULL DEFAULT '',
    status           VARCHAR(10)  NOT NULL DEFAULT 'DRAFT',
    word_count       INTEGER      NOT NULL DEFAULT 0,
    -- 大纲信息（规格书 §27 OutlineNode）
    purpose          TEXT         NOT NULL DEFAULT '',
    conflict         TEXT         NOT NULL DEFAULT '',
    outcome          TEXT         NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT creative_chapters_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT creative_chapters_status_valid CHECK (status IN ('DRAFT', 'REVIEW', 'FINAL')),
    CONSTRAINT creative_chapters_no_positive CHECK (chapter_no > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_creative_chapters_no
    ON creative_chapters (creative_work_id, chapter_no) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_creative_chapters_work
    ON creative_chapters (creative_work_id, chapter_no ASC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS creative_scenes (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chapter_id     UUID         NOT NULL REFERENCES creative_chapters(id) ON DELETE CASCADE,
    sequence       INTEGER      NOT NULL DEFAULT 1,
    title          VARCHAR(200) NOT NULL DEFAULT '',
    location       VARCHAR(200) NOT NULL DEFAULT '',
    characters     JSONB        NOT NULL DEFAULT '[]'::jsonb,
    purpose        TEXT         NOT NULL DEFAULT '',
    conflict       TEXT         NOT NULL DEFAULT '',
    emotional_goal TEXT         NOT NULL DEFAULT '',
    content        TEXT         NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_creative_scenes_chapter
    ON creative_scenes (chapter_id, sequence ASC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS chapter_versions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chapter_id UUID        NOT NULL REFERENCES creative_chapters(id) ON DELETE CASCADE,
    version_no INTEGER     NOT NULL,
    content    TEXT        NOT NULL DEFAULT '',
    word_count INTEGER     NOT NULL DEFAULT 0,
    note       VARCHAR(200) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_chapter_versions_no
    ON chapter_versions (chapter_id, version_no);

CREATE TABLE IF NOT EXISTS consistency_issues (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creative_work_id UUID         NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    chapter_id       UUID REFERENCES creative_chapters(id),
    severity         VARCHAR(10)  NOT NULL DEFAULT 'medium',
    type             VARCHAR(30)  NOT NULL,
    description      TEXT         NOT NULL,
    evidence         TEXT         NOT NULL DEFAULT '',
    suggestion       TEXT         NOT NULL DEFAULT '',
    status           VARCHAR(10)  NOT NULL DEFAULT 'OPEN',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT consistency_issues_severity_valid CHECK (severity IN ('high', 'medium', 'low')),
    CONSTRAINT consistency_issues_type_valid CHECK (
        type IN ('character', 'world', 'timeline', 'plot', 'language')
    ),
    CONSTRAINT consistency_issues_status_valid CHECK (status IN ('OPEN', 'RESOLVED', 'IGNORED'))
);

CREATE INDEX IF NOT EXISTS idx_consistency_issues_work
    ON consistency_issues (creative_work_id, status, created_at DESC) WHERE deleted_at IS NULL;

COMMENT ON TABLE creative_chapters IS '二创章节；content 为正文，另存大纲信息（purpose/conflict/outcome）';
COMMENT ON TABLE chapter_versions IS '章节版本（规格书 §59）：每次保存正文生成一版，可查看/恢复/比较';
COMMENT ON TABLE consistency_issues IS '一致性检查问题（规格书 §39）';
