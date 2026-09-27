-- 0004_create_world
-- 原著世界观（规格书 §13）：世界、世界规则、地点、势力。
-- 一个原著对应一个世界（软删除不占用名额）；规则/地点/势力挂在世界下。

CREATE TABLE IF NOT EXISTS original_worlds (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_work_id UUID         NOT NULL REFERENCES original_works(id) ON DELETE CASCADE,
    name             VARCHAR(200) NOT NULL DEFAULT '',
    description      TEXT         NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_original_worlds_work
    ON original_worlds (original_work_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS world_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id    UUID         NOT NULL REFERENCES original_worlds(id) ON DELETE CASCADE,
    category    VARCHAR(60)  NOT NULL DEFAULT '',
    name        VARCHAR(200) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    importance  SMALLINT     NOT NULL DEFAULT 3,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    CONSTRAINT world_rules_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT world_rules_importance_range CHECK (importance BETWEEN 1 AND 5)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_world_rules_name
    ON world_rules (world_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_world_rules_world
    ON world_rules (world_id, importance DESC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS locations (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id           UUID         NOT NULL REFERENCES original_worlds(id) ON DELETE CASCADE,
    name               VARCHAR(200) NOT NULL,
    type               VARCHAR(60)  NOT NULL DEFAULT '',
    description        TEXT         NOT NULL DEFAULT '',
    parent_location_id UUID REFERENCES locations(id),
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at         TIMESTAMPTZ,
    CONSTRAINT locations_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT locations_no_self_parent CHECK (parent_location_id IS NULL OR parent_location_id <> id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_locations_name
    ON locations (world_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_locations_world
    ON locations (world_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS factions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id     UUID         NOT NULL REFERENCES original_worlds(id) ON DELETE CASCADE,
    name         VARCHAR(200) NOT NULL,
    type         VARCHAR(60)  NOT NULL DEFAULT '',
    description  TEXT         NOT NULL DEFAULT '',
    goals        TEXT         NOT NULL DEFAULT '',
    relationships TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    CONSTRAINT factions_name_not_blank CHECK (length(btrim(name)) > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_factions_name
    ON factions (world_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_factions_world
    ON factions (world_id) WHERE deleted_at IS NULL;

COMMENT ON TABLE original_worlds IS '原著世界观（一个原著一个世界）';
COMMENT ON TABLE locations      IS '地点，支持 parent_location_id 形成层级';
