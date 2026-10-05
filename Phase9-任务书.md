# Phase 9 任务书：Context Engine + Retrieval + Memory（AI 大脑）

> 目标读者：Codex。按 9.1→9.6 顺序实施，每节末尾验收标准全部通过才算完成。
> 目标：让 NovaMind 从"能写"变成"越写越懂这部小说"——长篇记忆型写作。
> 非目标：Agent/Tool 层（等 9.x 稳定后再议）、团队共享、二创剧情/素材页（§26/§30 另行排期）。

---

## 0. 前置条件（开工前必须满足）

- **P0 冻结**：给当前 main 打 tag（如 `v0.9.0-mvp`），Phase 9 在新分支开发。
- **P1 多用户 Phase A+B 已合入**：`owner_user_id` 已全表。若未合入，Codex 必须给本期所有新表同样加上 `owner_user_id UUID NOT NULL REFERENCES users(id)`（不可省略，检索结果必须按用户隔离）。
- **P2 pgvector 可用性检查**（不许跳过）：
  ```sql
  SELECT * FROM pg_available_extensions WHERE name = 'vector';
  ```
  本机 apt PG 若无 → 先装 `postgresql-16-pgvector`（或切 docker 的 pgvector 镜像）。装不上 → 9.1 先交付 **BM25-only**（Go 内实现，零依赖），vector 记为阻塞项。**不允许静默跳过检查直接写代码。**
- 迁移编号：本期用 **0020/0021/0022**（以多用户 0018/0019 为基准）；若基准变化，Codex 顺延重编号。

---

## 9.1 Retrieval（P0-1）：让 AI 能"翻旧账"

### 9.1.1 表（迁移 0020）

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE chunks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL REFERENCES users(id),
    work_kind     TEXT NOT NULL CHECK (work_kind IN ('original','creative')),
    work_id       UUID NOT NULL,              -- original_work_id 或 creative_work_id（跨表，不设 FK）
    chapter_id    UUID,                       -- 章节级 chunk 回指章节
    ref_kind      TEXT NOT NULL DEFAULT 'chapter'
                  CHECK (ref_kind IN ('chapter','outline_node','world_rule','character','event','memory_fact','chapter_summary')),
    ref_id        UUID,                       -- ref_kind 对应表的主键
    seq           INTEGER NOT NULL DEFAULT 0, -- chunk 在 ref 内的顺序
    content       TEXT NOT NULL,
    token_count   INTEGER NOT NULL DEFAULT 0,
    embedding     vector(1024),               -- 维度由 EMBEDDING_DIM 配置，默认 1024（BGE-large-zh）
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_chunks_work  ON chunks (work_kind, work_id);
CREATE INDEX idx_chunks_owner ON chunks (owner_user_id);
-- 注：ivfflat 向量索引等数据量上来再加，本期用暴力 <=>（验收 9.6 有性能基线，不达标再加）。
```

### 9.1.2 切块（`internal/retrieval/chunker.go`，新建包）

- 章节 `content`：按段落切，每 chunk ≤ 800 汉字（≈1200 tokens），overlap 150 汉字；不足一块不硬凑；空内容返回空。
- 大纲节点 / 世界规则 / 人物 / 事件：单条即一 chunk，不切分。
- 幂等：`ReplaceChunks(work_kind, work_id, ref_kind, ref_id, chunks)` —— 先删后建，包事务。

### 9.1.3 Embedding（`internal/ai/gateway.go` 新增方法）

```go
func (g *Gateway) Embed(ctx context.Context, cfg ProviderConfig, inputs []string) ([][]float32, error)
```

- 调 OpenAI 兼容 `/embeddings`，batch ≤ 32；失败重试复用 `IsRetryable` 语义；响应截断保护同 chat。
- `ProviderConfig` 新增 `EmbedModelName` 字段（chat 模型和 embedding 模型必须独立）。
- `model_providers` 表加两列（nullable，迁移 0020 一并做）：`embed_api_base TEXT`、`embed_model_name TEXT`；为空时回退：api_base 用 chat 的，模型名用配置 `DEFAULT_EMBED_MODEL`（默认 `BAAI/bge-large-zh-v1.5`，`EMBEDDING_DIM` 默认 1024）。
- 现有 `ValidateAPIBase` 同样卡 `embed_api_base`（SSRF 防护不许漏）。

### 9.1.4 索引构建（复用异步任务框架）

- 新任务类型 `index_chunks`，参数 `{work_kind, work_id, ref_kind?, ref_id?}`（ref 为空 = 全量重建）。
- 触发点：原著导入完成、章节内容保存/生成完成、大纲节点变更、世界规则变更、memory_facts/chapter_summaries 写入（增量：只重建受影响的 ref）。
- embedding 突发限速：batch 32，batch 间隔 200ms（30 万字全量回填约 20 万 tokens，费用可忽略，但别打爆上游）。
- 失败走现有任务重试；反复失败记 FAILED，不阻塞写作主流程。

### 9.1.5 检索（`internal/retrieval/search.go`）

```go
func Search(ctx context.Context, ownerID, role string, workKind, workID, query string, topK int) ([]ScoredChunk, error)
```

1. query 做 embedding（无 embedding 配置 → 跳过向量路，日志记一条）。
2. 向量路：`WHERE owner_user_id=$1 AND work_kind=$2 AND work_id=$3 AND embedding IS NOT NULL ORDER BY embedding <=> $4 LIMIT topK*2`。
3. BM25 路（Go 内实现，**char-bigram 分词**，无词典依赖；"长篇记忆"→"长篇/篇记/记忆"；去标点空白）：对同 work 的 chunks 全量打分取 topK*2（单 work 万级 chunk 内可接受；超量先按章节范围裁剪，代码里留 `TODO`）。
4. RRF 融合（k=60）去重，返回 topK（默认 8），带 score 与来源（chunk id / ref_kind / ref_id）。
5. owner 隔离：经 `ownerScope`（admin 豁免，沿用多用户规范）。

- 调试 API：`POST /api/v1/retrieval/search {work_kind, work_id, query, top_k}` → chunks（含 score）。

### 验收 9.1

- [ ] pgvector 可用性检查有明确通过/阻塞结论并记录。
- [ ] 100 章小说（~30 万字）切块 + embedding 入库成功；失败任务可重试。
- [ ] 人工抽 5 个 query（如"第17章的伏笔"），top8 命中 ≥4。
- [ ] 无 embedding 配置时 BM25 单路仍可用（单测 + 手动验证）。
- [ ] `go test` 覆盖：chunker 边界（空内容、超长段落、overlap 正确性）、RRF 融合去重。

---

## 9.2 Context Engine（P0-2）+ ContextSnapshot（P0-3）

### 9.2.1 组装（`internal/context/engine.go`，新建包）

```go
func Assemble(ctx context.Context, ownerID, role, chapterID, authorInstruction string) (*ContextSnapshot, error)
```

预算（默认，可配置；中文 token 粗估 `len([]rune)/1.5`，`internal/context/tokens.go` 集中实现，以后可换 tiktoken）：

```
total 8000
├─ 当前章节目标/大纲节点（含 purpose/conflict/outcome）：800（固定优先，不截）
├─ 人物 DNA（沿用 BuildContext 现有逻辑）：1200（不截）
├─ 世界规则（沿用现有逻辑）：800（不截）
├─ 时间线/分叉点/事件：600
├─ 前 3 章摘要（沿用现有逻辑）：600
├─ 检索·原著片段（9.1.5）：1500
├─ 检索·二创片段（含 memory_facts，9.3）：1500
└─ 作者指令：剩余全部
```

截断顺序（超预算时）：检索片段 → 前情摘要 → 时间线 → 作者指令（人物/世界/大纲/当前目标永不截）。

**接线**：`writing_service.go:378`（`GenerateChapterDraft`）与 `:470` 的 `BuildContext` 调用改为调 `context.Assemble`；`ChapterContext` 结构保留或扩展由 Codex 定，但 **prompt 模板必须 version+1**（`backend/prompts/`），老模板回归不能挂。

### 9.2.2 ContextSnapshot 表（迁移 0021）

```sql
CREATE TABLE context_snapshots (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id    UUID NOT NULL REFERENCES users(id),
    creative_work_id UUID NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    chapter_id       UUID REFERENCES creative_chapters(id) ON DELETE CASCADE,
    kind             TEXT NOT NULL CHECK (kind IN ('generate','continue','rewrite','expand','analyze','consistency')),
    snapshot         JSONB NOT NULL,  -- 8 段组装结果 + 检索 chunk id/score + model + prompt_version + token 用量 + created_at
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_snapshots_chapter ON context_snapshots (chapter_id, created_at);
```

- 每次 AI 调用（generate/continue/rewrite/expand/analyze/一致性检查）：先 `Assemble` → 落 snapshot → 再调 LLM。**snapshot 不可变**（不提供 update API）。
- API：`GET /api/v1/chapters/:id/snapshots`（列表）、`GET /api/v1/snapshots/:id`（详情）；前端 v1 只做 JSON 查看页（"当时给了 AI 什么"追溯）。

### 验收 9.2

- [ ] 生成一章 → snapshot 落库，含 8 段 + 检索来源 id/score + 模型与 prompt 版本。
- [ ] 预算截断顺序单测锁定。
- [ ] 现有 37/37 e2e 不因上下文格式变化失败。

---

## 9.3 Memory（P0-4）+ 写作后回写（P0-5）

### 9.3.1 表（迁移 0022）

```sql
CREATE TABLE memory_facts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id   UUID NOT NULL REFERENCES users(id),
    creative_work_id UUID NOT NULL REFERENCES creative_works(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL CHECK (kind IN ('character_state','world_state','event','item','relationship','plot')),
    subject         TEXT NOT NULL,   -- 如 "人物A"
    fact            TEXT NOT NULL,   -- 如 "左臂受伤，无法用剑"
    chapter_id      UUID REFERENCES creative_chapters(id) ON DELETE SET NULL,
    superseded_by   UUID REFERENCES memory_facts(id),  -- 被新事实替代时指向新 id（只标记不删）
    embedding       vector(1024),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_facts_work ON memory_facts (creative_work_id, kind);

CREATE TABLE chapter_summaries (
    chapter_id    UUID PRIMARY KEY REFERENCES creative_chapters(id) ON DELETE CASCADE,
    owner_user_id UUID NOT NULL REFERENCES users(id),
    summary       TEXT NOT NULL,
    embedding     vector(1024),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

三层映射（同步写进 `docs/ARCHITECTURE.md`）：Canonical = 现有原著结构化表（**只读，AI 不可改**）；Creative = `creative_*` 表 + `memory_facts`；Episodic = `chapter_summaries` + `memory_facts(kind='event')`。

### 9.3.2 事实抽取（`internal/memory/extractor.go`，新建包）

- 触发：章节内容保存（AI 生成完成 **或** 作者手动保存且内容变化）→ 异步任务 `extract_facts {chapter_id}`。
- LLM 抽取：`backend/prompts/` 新增版本化模板 `fact_extract_v1`；输出经 **JSON Schema 校验**（9.5）→ 失败则把校验错误喂回 LLM 修一次 → 仍失败任务记 FAILED（**不阻塞写作**）。
- 输出形如：`{facts: [{kind, subject, fact}], summary: "..."}`。
- 写入：memory_facts 新行（同 subject+kind 的旧事实置 `superseded_by`）；chapter_summaries upsert；chunks 增量索引（`ref_kind='memory_fact'/'chapter_summary'`）。
- 抽取完成后**自动触发一致性检查任务**（现有 `checkConsistency`），检查器读 memory_facts + 检索 —— 这就是"越写越懂"的闭环。

### 验收 9.3

- [ ] 写完一章 → facts/summary 自动入库 → 下次同 work 检索能召回。
- [ ] 人物状态被新章节改变 → 旧 fact 被 supersede（不是删除，可追溯）。
- [ ] 抽取输出非法 → 自动修复一次 → 仍失败任务 FAILED 且章节保存不受影响。

---

## 9.4 一致性检查接入

- `checkConsistency` 的上下文源改为走 `context.Assemble`（`kind='consistency'`，同样落 snapshot），替代现在的"拼一些字段"。
- 检查维度加一条：**与 memory_facts 的冲突检测**（prompt 模板更新 +1 版本）。
- v1 不做"生成前约束拦截"（完整 Consistency Engine 放 Phase 10），只做"生成后检查 + 写回 memory"。

---

## 9.5 JSON Schema 校验

- 新增 `internal/ai/schema.go`：`ValidateJSON(data []byte, schemaName string) error`；schema 文件放 `backend/prompts/schemas/*.json`（本期先写 `fact_extract.json`）。
- 库二选一（Codex 定，文档注明）：`github.com/santhosh-tekuri/jsonschema/v5` 或手写轻量校验。
- 本期强制接入点：事实抽取输出；AI 分析类 JSON 输出后续逐步接（本期不做）。

---

## 9.6 总验收

- [ ] e2e（固定测试剧本）：新开二创 → 写 3 章 → 每章 facts/summary 自动产生 → 第 4 章生成时 snapshot 含检索到的第 1 章伏笔 → 一致性检查能指出预设矛盾（如"人物受伤状态与第 2 章矛盾"）。
- [ ] `go test ./...`、`vitest` 全绿；新增回归覆盖 chunker / RRF / 预算截断 / schema 校验 / 回填幂等。
- [ ] 新增 `scripts/smoke-phase9.sh`：索引构建 → 检索召回 → snapshot 落库 → 事实回写，全链路可复跑。
- [ ] 性能基线：100 章 work 检索 P95 < 2s（记录在案；不达标则加 ivfflat 索引，不许无记录上线）。
- [ ] `docs/ARCHITECTURE.md`、`docs/SPEC.md`（§31/§32/§33/§55 状态）、`openapi.yaml` 同步更新。

---

## 风险与取舍（Codex 必读）

1. **不要在 Phase 9 做 Agent/Tool 层** —— 严格按 9.1→9.6 做。
2. **snapshot 存量膨胀**：JSONB 会越积越大；本期只在文档记 TODO（按 created_at 归档），归档实现放 Phase 10。
3. **embedding 维度一旦定死改起来很贵**：`EMBEDDING_DIM` 默认 1024，换模型必须全量重建索引（`index_chunks` 全量模式就是干这个的）。
4. **中文 BM25 不用词典**：char-bigram 是刻意选择（零依赖、可复现）；以后要上 jieba 级分词另起任务。
