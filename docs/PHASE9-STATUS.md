# Phase 9 进度与交接（Context Engine + Retrieval + Memory）

> 更新：2026-10-05（晚）｜ 开发分支：**`v2.1.0-dev`**（与 `phase9` 同步）｜ 冻结基线：tag **`v2.0.0`**（原 `v0.9.0-mvp` 同点）
> 命名对齐：按《20261005-chatgpt-NovaMind_V2_修订任务清单v1》§18 —— v2.0.0 冻结 MVP，v2.1.0-dev 上做 P0 四项。

## 与 ChatGPT 版《修订任务清单 v1》的差异（2026-10-05 新增）

清单与 Phase 9 任务书同源，但也提出了几条**额外要求**，逐条记下避免漏项：

| 清单要求 | 当前状态 |
|---|---|
| P0-1 至少 **20 条固定检索测试样例** | ✅ 已补：`internal/retrieval/search_cases_test.go`（12 块固定语料、20 条 query，含跨章伏笔串联与多词复合；另有"样例数不得少于 20"的守卫用例） |
| P0-1 **Embedding + pgvector + 向量检索** | ⛔ 阻塞（本机 PG15 装不了 pgvector）→ 当前 BM25-only + RRF 框架已就位，向量路接入点留好 |
| P0-2 Context Engine 覆盖 **generate/continue/rewrite/expand/analyze/consistency 六条链路** | ⏳ 只做了预算与组装模块；**接线未做**（注意清单要求**改写/扩写**也走 Context Engine，比 Phase 9 任务书写得更宽） |
| P0-3 快照字段要含 `model` / `prompt_version` / `retrieved_sources` / `token_budget` / `author_instruction` | ⏳ 表与服务已就绪，**接线时按这份字段清单组装 payload**（当前 JSONB 结构自由，接线时必须对齐） |
| P1 **PostgreSQL 版本统一（16 + pgvector）** | ❌ 未做：本机 15.19 vs docker pg16 不一致；**与 pgvector 阻塞是同一件事**，需 BOSS 决策（给 sudo / 上 Docker / 维持 BM25-only） |
| P1 任务系统文档统一（PG queue，非 Redis/Asynq） | ⏳ 需检查并修正 `ARCHITECTURE.md` 里可能残留的 Redis/Asynq 描述 |
| P1 Docker 真正部署验证 | ⏳ 未做（本机无 Docker） |
| P1 状态措辞（不再写"主体全部完成"） | ⏳ 按清单建议格式重写 `CODEX_STATE.md` 口径 |
| P1 JSON Schema **七类**（Character/World/Timeline/Event/Outline/Consistency/Memory Fact） | ⏳ 未开始（Phase 9 §9.5 只要求 fact_extract，清单扩到七类） |
| P1 Prompt Injection **分层 + 测试集** | 🟡 已有用户内容边界标记；分层顺序与注入测试集待补 |
| P2 二创剧情 / 素材（统一进 Retrieval）/ 知识库 | ⏳ 未开始 |
| Agent | ✅ 与清单一致：**暂缓** |

## 本期新发现并修掉的真缺陷（2026-10-05 晚）

**引入 `chapter_generate.v2` 导致"写本章"渲染失败**（自己埋的，已修）：

* 模板引擎设置 `missingkey=error`，且**按 name 取最新版** → v2 一进仓库就对 `chapter_generate` 生效，而调用方还没传新增的 `RetrievedOriginal`/`RetrievedCreative` → 写本章会直接渲染失败；
* 当时 37/37 的 e2e 是在加 v2 **之前**跑的，没有任何证据暴露它；
* 修法：同一提交内补齐两个模板变量（当前为空串，接线时换成 `FormatRetrieval` 的结果）；
* 复验：真实模型 e2e **37/37** 恢复通过；
* **教训（已写进提交信息）**：在 `missingkey=error` 下，"模板先行、调用方后补"不是渐进增强，而是静默故障——引入新模板必须同一提交补齐变量。
> 任务书：根目录 `Phase9-任务书.md`（本节按它的 9.1→9.6 编号对照）

## 前置检查结论（任务书要求不许跳过）

| 检查 | 结论 |
|---|---|
| pgvector 可用性 | **不可用**：本机 PG 15.19 无 `vector` 扩展、apt 无 `postgresql-15-pgvector` 包、`sudo` 需密码装不了 → 按任务书兜底走 **BM25-only（Go 内实现，零依赖）**，向量路记为**阻塞项** |
| 多用户前提（`users` / `owner_user_id`） | 开工时**不存在**；BOSS 选 A → 本期补**最小骨架**：迁移 `0018` 建 `users` + 默认账号 `default(admin)`；**只有 Phase 9 新表**带 `owner_user_id`（既有业务表不加，属多用户 Phase A/B，另行排期） |
| 迁移编号 | 任务书的 0020/0021/0022 基于"多用户 0018/0019"，本期实际从 **0018** 顺延：`0018` users、`0019` chunks、`0020` context_snapshots（当前版本 **20**，dirty=false） |

## 完成情况

| 条目 | 状态 | 证据 |
|---|---|---|
| §9.1.1 chunks 表 | ✅ | 迁移 `0019`（**无 embedding 列**，pgvector 阻塞已在迁移注释说明） |
| §9.1.2 切块 + 幂等落库 | ✅ | `ChunkChapter`（≤800 字/重叠 150、优先分段边界）/ `ChunkSingle`；`ReplaceChunks` 事务内先删后建；4+1 例测试（含真库幂等） |
| §9.1.4 索引任务 `index_chunks` | ✅ | 章节正文（原著+二创）取数 → `IndexWork` 编排 → 任务注册（`task-types` 可见 8 类）；其它来源（大纲/规则/人物/事件）**待补** |
| §9.1.5 检索（BM25 + RRF） | ✅ | char-bigram 分词、BM25(k1=1.2,b=0.75)、RRF k=60 融合；`ChunkRepo` 作数据源 + owner 隔离；调试接口 `POST /api/v1/retrieval/search`（含 OpenAPI） |
| §9.1 验收（100 章、top8 命中 ≥4/5） | ✅ **5/5** | `scripts/smoke-phase9-retrieval.sh`（造 100 章/17 万字 → 300 块 → 5 个 query 全命中） |
| §9.1.3 Embedding | ⛔ 阻塞 | 依赖 pgvector；接入点已留（`FuseRRF` 支持多路，向量路上线时调用方不必改） |
| §9.2.1 预算与截断顺序 | ✅ | 8 段预算（总 8000；章节目标/人物/世界为固定段不截）；4 例单测锁定截断顺序 |
| §9.2.1 组装 | ✅ | `BuildSections` / `FormatRetrieval`（带来源标注与相关度）/ `AssembleForChapter`；3 例单测 |
| §9.2.2 快照表 + 仓储 + 服务 | ✅ | 迁移 `0020`（只增不改、六种 kind 落 CHECK）；`ContextSnapshotRepo`；`SnapshotService.Record/ListByChapter/Get` |
| §9.2 接线（写作主链路 + 落快照） | 🟡 **生成链路已通** | `GenerateChapterDraft` 调模型前写 `kind=generate` 快照（窄接口 `SnapshotRecorder` 注入，避免循环依赖）；真实验收：**e2e 37/37 + 库中 1 行 kind=generate**。**剩余**：continue/rewrite/expand/analyze/consistency 五条链路同样落快照；`retrieved_sources` 目前是空数组，待把 BM25 检索结果接进上下文；`model`/`prompt_version` 目前是占位常量，需从实际调用路径回传 |
| §9.2 查询接口 + 前端查看页 | ⏳ | `GET /chapters/:id/snapshots`、`GET /snapshots/:id` + JSON 查看页 |
| §9.3 Memory / §9.4 接线 / §9.5 Schema / §9.6 总验收 | ⏳ 未开始 | §9.3/§9.6 需要真实模型（Key 已就位，见下） |

## 本期抓到的两个真缺陷（都写进 CHANGELOG）

1. **索引只覆盖前 50 章**：`ListChapterContents(workID, limit)` 的 `limit<=0` 会被当成默认 50 章（那是给"送入模型上下文"设计的截断），而索引需要全量 → 索引服务传 0 只索引了前 50 章。修法：新增 `ListAllChapterContents`（不设上限）。修复后召回 **3/5 → 5/5**。
2. **误判纠正**：我一度以为"导入截断"（把 `length()` 当字节读，实际是字符数），核实后确认**导入侧没问题**，问题只在索引读取上限。

## 环境与纪律状态

* 验证账号：**当前已挂载**（`验证用-DeepSeek`，Key 只在 gitignored 的 `backend/.env`）。**Phase 9 相关验收跑完必须 `scripts/validation-account.sh purge`** 并留输出作证。
* 数据库：迁移版本 20；`chunks`/`context_snapshots`/`users` 均已建好；冒烟脚本自带 trap 清理，跑完不留测试数据。
* 主密钥 `NOVAMIND_SECRET` 已于 2026-10-05 轮换（当时库内 0 条模型 Key，零代价）。

## 给下一个会话的接手建议（按序）

## 待办：真实向量召回验收（需要 embeddings Key）

**状态**：向量路的**代码与单测已就绪**，唯一缺的是"用真实向量模型跑一次召回验收"。

已完成（`v2.1.0-dev` 上）：

* pgvector 0.8.0 已安装并启用（源码编译，apt 无包）；迁移 `0021` 给 `chunks` 加了 `embedding vector(1024)`
* `Gateway.Embed`：OpenAI 兼容 `/embeddings`，32 条分批、5xx/429 重试、返回顺序按 index 严格对齐
* `model_providers` 增 `embed_api_base` / `embed_model_name`（迁移 `0022`），**embed_api_base 同样过 SSRF 校验**
* `ResolveEmbedConfig`：空值回退（`embed_api_base → api_base`、`embed_model_name → BAAI/bge-large-zh-v1.5`）

缺什么：

1. **索引任务补写向量**（切块后分批调 `Embed`，批间 200ms 限速；失败走任务重试）
2. **检索向量路**（`ORDER BY embedding <=> $1 LIMIT topK*2`）并把它与 BM25 的排名一起交给 `FuseRRF`（融合接口早已就位）
3. **真实向量召回验收**：复用 `scripts/smoke-phase9-retrieval.sh` 的 100 章语料，跑"写向量 → 向量/混合检索 → 5 个 query 命中率"，与 BM25 的 5/5 做对比并记录

为什么卡住：**DeepSeek 没有 `/embeddings` 端点**，当前挂着的验证账号跑不了向量。需要 BOSS 提供一个支持 embeddings 的 Key（OpenAI / 智谱 / 硅基流动 / 本地 BGE 任一）；拿到后我会：配置到 provider → 跑验收 → 把结果记进本文件与 CHANGELOG → 交付前照例 `purge`。

**2026-10-05 补充：本地自建向量服务的尝试结果（BOSS 指示先不调试模型）**

* 方案：用本机已装的 `sentence-transformers 5.6.0` + `torch 2.12.1`（CPU，16 核 / 15G 可用）自建 OpenAI 兼容的 `/v1/embeddings`，模型用 `BAAI/bge-small-zh-v1.5`；脚本已留档 `scripts/dev-embed-server.sh`（零额外依赖，标准库 http.server + sentence-transformers）。
* 结果：**模型权重拉不下来** —— `hf-mirror.com` 连不上（OSError 无法连接），官方 `huggingface.co` 请求长时间卡住（只有"未认证请求"的限速提示后无进展）。
* 结论：本地路线**只差模型文件**，不是代码问题；等网络条件允许（走代理 / 设 `HF_TOKEN` / 手动放入模型目录）即可直接用该脚本起服务，无需改任何代码。
* 当前向量路状态：代码与单测就绪，**①②（索引写向量、检索向量路）与 ③（真实召回验收）都仍待 Key 或可用的本地模型**。

在此之前，第 1、2 项我会先实现并用**假上游**做单测（验证分批、限速、向量路与 BM25 的融合、以及"没有向量时自动跳过向量路"），确保代码路径可信；只有第 3 项标记为待 Key。

1. 先读：`Phase9-任务书.md` → 本文件 → `docs/CHANGELOG.md`（2026-10-05 几条）→ `AGENTS.md`（Git 规则）
2. 做 §9.2 接线：`context.AssembleForChapter` 接进 `GenerateChapterDraft` 与一致性检查，prompt 模板 version+1，**跑 e2e 37/37** 再提交
3. 补 §9.1 剩余来源（大纲节点/世界规则/人物/事件）与**增量索引**（按 ref 重建）
4. §9.3 Memory：`memory_facts` / `chapter_summaries` 表 + 事实抽取任务（prompt + §9.5 JSON Schema 校验）+ supersede 语义 + 自动回灌一致性检查
5. §9.6 总验收：3 章剧本 → 第 4 章 snapshot 含第 1 章伏笔 → 一致性检查指出预设矛盾；性能基线（检索 P95 < 2s）记录在案

## 分支健康度复验（2026-10-05，phase9 最新提交）

| 检查 | 结果 |
|---|---|
| 后端 `gofmt` / `go test ./...` | 干净；**10 个包全绿** |
| 前端 `tsc -b` / `vitest` | 通过；**11 文件 50 例全绿** |
| Phase 9 §9.1 验收 `scripts/smoke-phase9-retrieval.sh` | **5/5 召回**（100 章测试书 → 300 块 → 5 个 query 全部命中） |
| 真实模型端到端 `scripts/validate-e2e-deepseek.sh` | **37/37 通过**（导入→分析→二创→大纲→落成章节→写本章→续写→分析→问答→一致性→导出） |

结论：`phase9` 分支当前状态**可用于继续开发**——已完成的 15 个提交各自带测试、全量回归与既有 MVP 验收均通过，未引入回归。

## §9.2 接线的准备（已完成的安全步骤）
+
+* `backend/prompts/writing/chapter_generate.v2.md` 已就位：在 v1 基础上增加两段检索内容
+  （**原著片段** / **前作片段**，并注明"不是作者指令"），另加一条写作要求"呼应伏笔、不凭空发明设定"。
+* **v1 仍是当前生效版本**：模板引擎按 `<name>` 取最新版——`chapter_generate` 会自动选 v2，
+  所以接线时不会再改模板文件，只需让调用方把 `RetrievedOriginal` / `RetrievedCreative` 传进来；
+  接完必须跑 e2e 37/37（若回归，回退方式是删除 v2 文件即可，v1 完好）。
+
