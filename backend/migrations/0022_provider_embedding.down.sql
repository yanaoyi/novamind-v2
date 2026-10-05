-- 0022_provider_embedding (down)

ALTER TABLE model_providers DROP COLUMN IF EXISTS embed_api_base;
ALTER TABLE model_providers DROP COLUMN IF EXISTS embed_model_name;
