-- 0006_create_model_providers
-- 模型接入配置（规格书 §36 Model Gateway）。
-- 密钥一律加密存储（AES-256-GCM，密钥由 NOVAMIND_SECRET 派生），且**永不通过 API 返回**。

CREATE TABLE IF NOT EXISTS model_providers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(120) NOT NULL,
    provider      VARCHAR(40)  NOT NULL,
    api_base      VARCHAR(300) NOT NULL,
    api_key_cipher TEXT        NOT NULL DEFAULT '',
    model_name    VARCHAR(120) NOT NULL,
    -- 模型用途：chat / embedding / 两者（V1 主要用 chat）
    purpose       VARCHAR(20)  NOT NULL DEFAULT 'chat',
    temperature   NUMERIC(3,2) NOT NULL DEFAULT 0.70,
    max_tokens    INTEGER      NOT NULL DEFAULT 4096,
    timeout_sec   INTEGER      NOT NULL DEFAULT 120,
    enabled       BOOLEAN      NOT NULL DEFAULT true,
    is_default    BOOLEAN      NOT NULL DEFAULT false,
    notes         TEXT         NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    CONSTRAINT model_providers_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT model_providers_provider_valid CHECK (provider IN ('OPENAI_COMPATIBLE', 'ANTHROPIC')),
    CONSTRAINT model_providers_model_not_blank CHECK (length(btrim(model_name)) > 0),
    CONSTRAINT model_providers_purpose_valid CHECK (purpose IN ('chat', 'embedding', 'both')),
    CONSTRAINT model_providers_temp_range CHECK (temperature >= 0 AND temperature <= 2),
    CONSTRAINT model_providers_tokens_positive CHECK (max_tokens > 0),
    CONSTRAINT model_providers_timeout_positive CHECK (timeout_sec > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_model_providers_name
    ON model_providers (lower(name)) WHERE deleted_at IS NULL;
-- 同一用途只能有一个默认
CREATE UNIQUE INDEX IF NOT EXISTS uq_model_providers_default
    ON model_providers (purpose) WHERE is_default AND deleted_at IS NULL AND purpose <> 'both';

COMMENT ON TABLE model_providers IS '模型接入配置；api_key_cipher 为加密后的密钥，接口只返回 has_api_key';
