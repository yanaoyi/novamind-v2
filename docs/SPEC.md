# SPEC.md — 实施状态对账表（不是规格）

> **唯一验收基准**：仓库根目录 [`NovaMind_V2_开发规格说明书.md`](../NovaMind_V2_开发规格说明书.md)（72 节 / 2571 行）。
> v1 与 v2 是同一套规格（BOSS 2026-10-04 确认），不存在"窄口径"版本。**本文档不构成规格**，只记录「规格要求 → 代码实现」的对账，随开发推进更新。
> 与本文档冲突时，以根目录规格书为准。

最后更新：2026-10-04

---

## 1. 状态总览

| 规格章节 | 要求 | 状态 | 说明 |
|---|---|---|---|
| §1–§5 | 产品定义、边界 | ✅ | 见 `PRODUCT_SPEC.md` |
| §6 | 技术栈（Go/Gin/GORM + PG + Redis + React/Vite/AntD/Zustand/Tiptap） | ✅ | `backend/go.mod`、`frontend/package.json` |
| §7–§13 | Project / OriginalWork / Chapter / Character / DNA / 关系 / World | ✅ | 迁移 `0001`–`0004` |
| §14–§16 | 事件 / 时间线 / 剧情结构 | ✅ | 迁移 `0005` |
| §17–§23 | 二创作品 / 人物 / 继承 / 融合 / 世界 / 世界规则 / 映射 | ✅ | 迁移 `0009`–`0010` |
| §24–§25 | 分叉点 / 二创时间线 | ✅ | 迁移 `0010` |
| **§26** | **二创剧情** | ❌ **未实现** | 无 `creative_plots` 模型与页面 |
| **§27** | **大纲（Outline / OutlineNode）** | ❌ **未独立建模** | 现用「卷 + 章节大纲字段」近似；`outline_generate.v1.md` 模板已写但无业务调用，无 `outline/generate` 端点 |
| §28–§29 | Chapter / Scene | ✅ | 迁移 `0011` |
| **§30** | **素材（Material）** | ❌ **未实现** | 无 `materials` 表与页面 |
| §31 | Context Engine（优先级 10 段） | 🟡 **部分** | 写作上下文拼人物 DNA + 世界规则 + 前 3 章摘要；**一致性检查已扩到五类上下文**（人物/世界/时间线/剧情/原著继承）；仍缺原著检索片段（依赖 §55） |
| **§32** | **ContextSnapshot** | ❌ **未实现** | 无快照表、无落库 |
| **§33** | **Memory 三层** | ❌ **未实现** | `internal/memory` 为空目录 |
| **§34–§35** | **7 个 Agent + Agent Runtime（Agent → Tool → Service → DB）** | ❌ **未实现** | `internal/agent` 为空目录；任务是 handler 直接调 service |
| §36 | Model Gateway | ✅ | OpenAI 兼容 + Anthropic，密钥 AES-GCM |
| §37 | Prompt Engine（版本化模板） | ✅ | `prompts/<域>/<name>.<version>.md`，编译进二进制 |
| §38 | AI 写作（续写/扩写/改写/大纲/人物/剧情/场景生成） | 🟡 **部分** | 有「写本章」+ 6 种改写（改写/扩写/缩写/润色/增强冲突/增强情绪）；缺 续写、增加动作、增加对白、调整节奏、改变叙事视角；缺 AI 生成人物/剧情/场景 |
| §39 | 一致性检查（Character/World/Timeline/Plot + 原著继承） | ✅ **已闭环** | 2026-10-04：五类上下文全部真实注入（二创时间线按 sequence 排序、章节大纲链做剧情骨架、原著↔二创映射做继承检查）；prompt 升级 `consistency_check.v2.md`；JSON 非法自动重试一次，仍失败则计入 `failed_chapters` 并在任务输出里如实报告 |
| §40–§45 | 分析流程与继承界面 | ✅ | 提案 → 作者审核 → 写入 |
| §46–§48 | 页面结构 / 主界面 / 编辑器 | 🟡 **部分** | 三栏与 Tiptap 已有；6 个菜单仍是占位页（见 §3） |
| §49 | API 设计 | 🟡 **部分** | **版本 `compare` 已补**（`GET /versions/compare`）；仍缺 `POST /ai/chat`、`/ai/generate`、`/ai/continue`、`/ai/expand`、`/ai/analyze` |
| §50 | 文件结构 | ✅ **已补齐** | 2026-10-04：`docker-compose.yml` + `docker/{backend,frontend}.Dockerfile` + `docker/nginx.conf`（本机无 Docker，仅作部署/CI 产物） |
| §51 | 数据库设计原则 | ✅ | UUID / 时间戳 / 软删除 / 外键 / JSONB |
| §52 | 数据权限边界（AI 不改原著） | ✅ | 提案表 + 审核事务 |
| §53–§54 | 任务系统 / 分阶段分析 | ✅ | PostgreSQL 队列（`FOR UPDATE SKIP LOCKED`），非 Redis/Asynq；4 个分析阶段 |
| **§55** | **检索系统（Chunk / Embedding / 向量 + 混合检索）** | ❌ **未实现** | `internal/retrieval` 为空目录，无 pgvector、无 Embedding |
| §56 | 写作 Context Assembly | 🟡 **部分** | 见 §31 |
| §57–§58 | 结构化输出 / 错误处理 | 🟡 **部分** | 只有 JSON 容错提取，无 Schema 校验、无自动修复重试接线（`IsJSONInvalid` 未被调用） |
| §59 | 版本管理（查看 / 恢复 / **比较**） | ✅ **已闭环** | 2026-10-04：`GET /versions/compare` 支持四类实体（人物/世界观/大纲字段级 diff、章节正文行级 LCS diff）+ 前端 `VersionDiff` 组件（版本抽屉里「比较」按钮） |
| §60 | 自动保存 | ✅ | debounce 1.5s + 切章 flush |
| §61 | 导出（TXT / Markdown / DOCX） | ✅ | 自建最小 OOXML |
| §62 | 关键用户流程 | 🟡 **部分** | 流程 A/B 通；流程 C 缺"生成二创大纲" |
| §63 | Phase 1–7 | 🟡 **有缺口** | 已闭环 §39/§50/§59；缺口剩 §26/§27/§30/§32/§33/§34/§35/§38/§49(部分)/§55/§57 |
| **§68** | **MVP 验收（12 项 + 生成二创大纲）** | ❌ **未通过** | 卡在：生成二创大纲、按原著检索写作、时间线/剧情一致性、版本比较 |

图例：✅ 已实现　🟡 部分实现　❌ 未实现

---

## 2. 已交付且可复验的能力

原著导入（TXT/DOCX/PDF）、章节切分、人物与 DNA、人物关系、世界观（世界/规则/地点/势力）、事件/时间线/剧情弧、AI 分析（4 阶段，提案 → 作者审核 → 写入原著）、二创（作品/人物继承与融合/世界继承/分叉点/二创时间线/映射）、写作（卷/章节/场景/富文本编辑器/自动保存/版本/导出）、模型配置、任务中心、一致性问题页（人物/世界维度）。

验证方式：`go test ./...`、`npx vitest run`、`scripts/smoke-*.sh`。

---

## 3. 缺口清单（按规格章节，逐项闭环）

| # | 缺口 | 规格 | 计划 |
|---|---|---|---|
| ~~1~~ ✅ | `docker-compose.yml` 缺失 | §50 | 2026-10-04 完成 |
| ~~2~~ ✅ | 一致性检查：时间线/剧情/原著继承上下文 + JSON 失败重试 | §39 §57 §58 | 2026-10-04 完成（prompt 升 v2，失败计入 `failed_chapters`） |
| ~~3~~ ✅ | 版本比较（Chapter/Character/World/Outline） | §59 | 2026-10-04 完成（后端 diff + 前端比较视图） |
| 4 | AI 端点与编辑器动作补全 | §38 §49 | `/ai/continue`、`/ai/expand`、`/ai/generate`、`/ai/chat`、`/ai/analyze` + 续写/增加动作/增加对白/调整节奏/改变叙事视角 |
| 5 | 大纲独立模型 + AI 生成大纲 | §27 §68 | `outlines`/`outline_nodes` + `outline/generate` + 前端大纲页 |
| 6 | 检索系统（Chunk/Embedding/混合检索） | §55 §56 | `internal/retrieval` + 索引表 + 写作上下文接入 |
| 7 | ContextSnapshot | §32 | 快照表 + 每次生成前落库 |
| 8 | Memory 三层 | §33 | `internal/memory`：长期/项目/短期 |
| 9 | 7 个 Agent + Tool 层 | §34 §35 | `internal/agent`：Agent → Tool → Service → Repository |
| 10 | 二创剧情 | §26 | `creative_plots` + API + 页面 |
| 11 | 素材 | §30 | `materials` + API + 页面 |
| 12 | 知识库页 | §46 | `/original/knowledge`：检索入口（依赖 #6） |
| 13 | 占位页收尾：二创设定、原著人物关系独立页、AI 助手页 | §46 §48 | 复用既有接口 |
| 14 | JSON Schema 校验 + 自动修复重试 | §57 §58 | 在 Model Invoker 层统一实现 |

完成一项就更新本表状态；每完成一个阶段按 §34 纪律（编译 → 测试 → 启动 → 验证 → 更新 `CODEX_STATE.md` / `CHANGELOG.md`）收尾。
