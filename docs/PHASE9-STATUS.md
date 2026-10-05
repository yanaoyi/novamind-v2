# Phase 9 进度与交接（Context Engine + Retrieval + Memory）

> 更新：2026-10-05 ｜ 分支：`phase9`（main 未受影响）｜ 冻结基线：tag `v0.9.0-mvp`
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
| §9.2 接线（写作主链路 + 落快照） | ⏳ **下一步** | 需改 `GenerateChapterDraft`/一致性检查 → `AssembleForChapter`，prompt 模板 version+1，**以 e2e 37/37 不回归为验收门槛** |
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
