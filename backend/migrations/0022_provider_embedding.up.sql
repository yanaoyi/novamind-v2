-- 0022_provider_embedding
-- 向量模型的独立配置（Phase 9 §9.1.3 / 修订清单 P0-1）。
--
-- 为什么单独两列而不是复用 chat 的 api_base/model_name：
--   chat 与 embedding 常常不是同一个供应商/模型（例如 chat 用 DeepSeek、embedding 用本地 BGE）。
--   两列为空时按约定回退：embed_api_base → api_base；embed_model_name → 配置项 DEFAULT_EMBED_MODEL。

ALTER TABLE model_providers ADD COLUMN IF NOT EXISTS embed_api_base   TEXT        NOT NULL DEFAULT '';
ALTER TABLE model_providers ADD COLUMN IF NOT EXISTS embed_model_name TEXT        NOT NULL DEFAULT '';

COMMENT ON COLUMN model_providers.embed_api_base   IS '向量接口地址（为空回退 api_base）；同样过 SSRF 校验';
COMMENT ON COLUMN model_providers.embed_model_name IS '向量模型名（为空回退 DEFAULT_EMBED_MODEL，默认 BAAI/bge-large-zh-v1.5）';
