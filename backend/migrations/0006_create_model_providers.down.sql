-- 回滚 0006_create_model_providers
DROP INDEX IF EXISTS uq_model_providers_default;
DROP INDEX IF EXISTS uq_model_providers_name;
DROP TABLE IF EXISTS model_providers;
