-- 0021_chunks_embedding
-- 向量检索的落脚点（Phase 9 §9.1.1 / 修订清单 P0-1）。
--
-- 背景：0019 建 chunks 时本机 PostgreSQL 15.19 装不了 pgvector，所以只上了 BM25 路。
-- 2026-10-05 已用源码编译安装 pgvector 0.8.0（apt 源里没有 postgresql-15-pgvector）：
--   apt-get install postgresql-server-dev-15 build-essential
--   git clone -b v0.8.0 https://github.com/pgvector/pgvector && make && make install
--   sudo -u postgres psql -d novamind -c 'CREATE EXTENSION IF NOT EXISTS vector'
--
-- 注意：`CREATE EXTENSION` 需要超级用户，因此**不放在本迁移里**（迁移以业务账号运行）；
-- 它是部署前置步骤，命令写在上面与 README 的部署说明中。缺扩展时本迁移会明确报错，
-- 不会静默跳过 —— 这是刻意的 fail-loud。

ALTER TABLE chunks ADD COLUMN IF NOT EXISTS embedding vector(1024);

-- 向量索引（ivfflat）暂不建：任务书的验收口径是"数据量上来再加"，
-- 且 ivfflat 需要先有数据才能训练出合理的 lists 参数。届时单独一条迁移补：
--   CREATE INDEX idx_chunks_embedding ON chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

COMMENT ON COLUMN chunks.embedding IS '向量检索用（1024 维，BGE-large-zh 量级）；BM25 路不依赖它';
