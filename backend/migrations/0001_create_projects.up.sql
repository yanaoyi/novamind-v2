-- 0001_create_projects
-- 工程表（规格书 §7.1、§51）。
-- 约定：UUID 主键、created_at/updated_at、软删除 deleted_at、枚举用 CHECK 约束。
-- 本迁移必须可重复执行：使用 IF NOT EXISTS 与约束判空。

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS projects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(200) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    type        VARCHAR(20)  NOT NULL,
    status      VARCHAR(20)  NOT NULL DEFAULT 'ACTIVE',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,

    CONSTRAINT projects_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT projects_type_valid      CHECK (type IN ('ORIGINAL', 'CREATIVE')),
    CONSTRAINT projects_status_valid    CHECK (status IN ('DRAFT', 'ACTIVE', 'ARCHIVED'))
);

-- 列表查询：未删除 + 按类型过滤
CREATE INDEX IF NOT EXISTS idx_projects_type        ON projects (type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_projects_status      ON projects (status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_projects_created_at  ON projects (created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_projects_deleted_at  ON projects (deleted_at);

COMMENT ON TABLE  projects        IS 'NovaMind 工程：type=ORIGINAL 载原著模型，type=CREATIVE 载二创模型';
COMMENT ON COLUMN projects.type   IS 'ORIGINAL | CREATIVE';
COMMENT ON COLUMN projects.status IS 'DRAFT | ACTIVE | ARCHIVED';
