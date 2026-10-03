-- 0002_create_originals
-- Phase 2 原著系统三张基础表：上传文件、原著作品、原著章节（规格书 §8.1 / §9）。
-- 约定同 0001：UUID 主键、时间戳、软删除、CHECK 约束、可重复执行。

-- 上传文件登记表（文件管理，规格书 §3.5）
CREATE TABLE IF NOT EXISTS files (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID REFERENCES projects(id),
    original_name VARCHAR(300) NOT NULL,
    stored_path   TEXT         NOT NULL,
    mime_type     VARCHAR(120) NOT NULL DEFAULT '',
    size_bytes    BIGINT       NOT NULL DEFAULT 0,
    sha256        CHAR(64)     NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    CONSTRAINT files_name_not_blank CHECK (length(btrim(original_name)) > 0),
    CONSTRAINT files_size_non_negative CHECK (size_bytes >= 0)
);

CREATE INDEX IF NOT EXISTS idx_files_project ON files (project_id) WHERE deleted_at IS NULL;

-- 原著作品（一个 ORIGINAL 类型工程对应一部原著）
CREATE TABLE IF NOT EXISTS original_works (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     UUID         NOT NULL REFERENCES projects(id),
    title          VARCHAR(200) NOT NULL,
    author         VARCHAR(120) NOT NULL DEFAULT '',
    description    TEXT         NOT NULL DEFAULT '',
    source_type    VARCHAR(20)  NOT NULL DEFAULT 'MANUAL',
    source_file_id UUID REFERENCES files(id),
    status         VARCHAR(20)  NOT NULL DEFAULT 'DRAFT',
    char_count     BIGINT       NOT NULL DEFAULT 0,
    chapter_count  INTEGER      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ,
    CONSTRAINT original_works_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT original_works_source_type_valid CHECK (source_type IN ('MANUAL', 'TXT', 'DOCX', 'PDF')),
    CONSTRAINT original_works_status_valid CHECK (status IN ('DRAFT', 'PARSED', 'ANALYZED')),
    CONSTRAINT original_works_counts_non_negative CHECK (char_count >= 0 AND chapter_count >= 0)
);

-- 一个工程只能有一部原著（软删除的不算）
CREATE UNIQUE INDEX IF NOT EXISTS uq_original_works_project
    ON original_works (project_id) WHERE deleted_at IS NULL;

-- 原著章节（含原文位置，支持回溯；规格书 §9）
CREATE TABLE IF NOT EXISTS original_chapters (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_work_id UUID        NOT NULL REFERENCES original_works(id) ON DELETE CASCADE,
    chapter_no       INTEGER     NOT NULL,
    title            VARCHAR(300) NOT NULL DEFAULT '',
    content          TEXT        NOT NULL DEFAULT '',
    summary          TEXT        NOT NULL DEFAULT '',
    start_position   BIGINT      NOT NULL DEFAULT 0,
    end_position     BIGINT      NOT NULL DEFAULT 0,
    char_count       INTEGER     NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT original_chapters_no_positive CHECK (chapter_no > 0),
    CONSTRAINT original_chapters_range_valid CHECK (end_position >= start_position),
    CONSTRAINT original_chapters_char_count_non_negative CHECK (char_count >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_original_chapters_no
    ON original_chapters (original_work_id, chapter_no) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_original_chapters_work
    ON original_chapters (original_work_id, chapter_no) WHERE deleted_at IS NULL;

COMMENT ON TABLE  original_works    IS '原著作品：project_id 必须是 ORIGINAL 类型工程';
COMMENT ON COLUMN original_chapters.start_position IS '在解析后全文中的 UTF-8 字节起始偏移（用于回溯原文）';
COMMENT ON COLUMN original_chapters.end_position   IS '在解析后全文中的 UTF-8 字节结束偏移（不含）';
