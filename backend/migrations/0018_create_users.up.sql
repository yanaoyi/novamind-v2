-- 0018_create_users
-- Phase 9 前置（BOSS 2026-10-05 选 A）：最小多用户骨架。
--
-- 为什么需要：Phase 9 任务书要求本期所有新表必须带
--   owner_user_id UUID NOT NULL REFERENCES users(id)（"检索结果必须按用户隔离，不可省略"），
-- 而当时的库里既没有 users 表、也没有任何 owner_user_id 列。
--
-- 本期只做"最小骨架"：一张 users 表 + 一个默认账号，
-- 既有业务表**不加** owner 列（那属于多用户 Phase A/B，另行排期）；
-- 只有 Phase 9 的新表带 owner_user_id，将来接真正的登录时可以直接沿用。

CREATE TABLE IF NOT EXISTS users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username     VARCHAR(64)  NOT NULL,
    display_name VARCHAR(120) NOT NULL DEFAULT '',
    -- admin：可跨用户读取（检索的 ownerScope 对 admin 豁免），user：只看自己的
    role         VARCHAR(20)  NOT NULL DEFAULT 'user',
    status       VARCHAR(20)  NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    CONSTRAINT users_username_not_blank CHECK (length(btrim(username)) > 0),
    CONSTRAINT users_role_valid CHECK (role IN ('admin', 'user')),
    CONSTRAINT users_status_valid CHECK (status IN ('active', 'disabled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_username
    ON users (lower(username)) WHERE deleted_at IS NULL;

-- 单用户默认账号：当前所有数据都归它；将来接入多用户后，它可作为初始管理员
INSERT INTO users (username, display_name, role)
SELECT 'default', '默认用户', 'admin'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE lower(username) = 'default');

COMMENT ON TABLE users IS '用户（Phase 9 最小骨架）：检索等新数据按 owner_user_id 隔离；默认账号 default 为 admin';
