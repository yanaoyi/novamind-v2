# NovaMind V2 规格符合性审查（Grok v1）

- 审查日期：2026-10-04
- 审查对象：`/lzcapp/document/codex/novamindv2`
- 对照规格：同目录 `NovaMind_V2_开发规格说明书.md`
- 结论：**不符合。** 主干能走，规格要求的闭环没有做完。

## 对照基准

规格文件名是 V2，正文标题写成了「NovaMind V1 开发规格说明书」。已按确认意见，全文当作 **NovaMind V2** 的开发规格。

不采用下面两份自述作为验收依据：

- `docs/SPEC.md`
- `docs/CODEX_STATE.md`

这两份把范围改窄，并写着「Phase 1–7 全部完成」，和代码对不上。`CODEX_STATE.md` 同一文件里既写「PDF 已完成」，又写「PDF 仍待做」「AI 模型未接入」。

## 总判

可运行的半成品：导入原著、切章、提取人物/世界/事件、作者审核后写入原著、建二创、人物继承和融合、世界继承、分叉点、章节编辑、导出，这些有实现。

按规格第 68 节的验收闭环，卡在：**生成二创大纲、按原著检索来写、时间线一致性、版本比较**。

## 已经落地的

| 规格要求 | 代码位置 |
|---|---|
| Go + Gin + GORM + PostgreSQL + Redis；React + TypeScript + Vite + Ant Design + Zustand + Tiptap | `backend/go.mod`、`frontend/package.json` |
| 原著 / 二创分表；AI 分析只出提案，作者通过才写入原著 | 迁移 `0002`–`0010`，`backend/internal/task/handlers_analysis.go` |
| TXT / DOCX / PDF 导入，章节，人物 DNA，关系，世界规则，地点，势力，事件，原著时间线 | 对应 service 与 `frontend/src/pages/original/` |
| 二创继承、融合、映射、分叉点、二创时间线 | `backend/internal/service/creative_service.go`，`frontend/src/pages/CreativePage.tsx` |
| 卷、章节、场景、Tiptap 编辑器、章节版本恢复 | `backend/internal/service/writing_service.go`，`frontend/src/pages/creative/ChapterEditor.tsx` |
| 导出 TXT / Markdown / DOCX | `GET /creative/:id/export` |
| 模型不写死厂商，密钥加密，Prompt 有版本文件 | `backend/internal/ai/`，`backend/prompts/` |
| 长任务异步，状态含 PENDING / RUNNING / FAILED 等 | PostgreSQL 队列（`FOR UPDATE SKIP LOCKED`），不是 Redis / Asynq |

## 规格要求了、代码没有或是空壳

这些都在说明书的必做范围里，不是文末「以后再做」那一节。

### 1. 检索和记忆没做

`backend/internal/retrieval`、`memory`、`context`、`agent`、`consistency` 都是空目录。

没有 pgvector，没有 Embedding，没有 `ContextSnapshot` 表。

写作上下文只拼了人物 DNA、世界规则和前 3 章摘要（`WritingService.BuildContext`）。原著片段、二创时间线、剧情都没进 Prompt。规格要求的是：结构化数据 + 向量检索 + 当前章节 + 作者指令。

### 2. 七个 Agent 和 Tool 层不存在

实际是任务处理函数直接调 Service。没有 `get_character` / `search_original` 这类工具，也没有规格写的 `Agent → Tool → Service → Database` 边界。

### 3. 大纲不是独立模型，也没有 AI 生成大纲

没有 `Outline` / `OutlineNode`。大纲页复用写作工作台的卷和章节字段。

`outline_generate` 模板只出现在 `backend/internal/ai/prompt_test.go` 和 `docs/CHANGELOG.md`，没有任何业务代码调用它。`POST /creative/:id/outline/generate` 不存在。

### 4. 二创剧情、素材、知识库没有

没有 `creative_plots`、`materials` 表。

以下路由仍是占位页（`frontend/src/App.tsx` 的 `Placeholder`）：

- `/creative/plot`
- `/creative/settings`
- `/creative/materials`
- `/original/knowledge`
- `/original/relationships`（关系数据在人物页里，独立页是占位）
- `/ai`

### 5. 一致性检查是半成品

`WritingService.CheckConsistency` 会送入人物设定和世界规则。

时间线被写成固定句子，没有读二创时间线：

```text
TimelineContext: "（时间线检查在 Phase 4 的二创时间线里维护）"
```

原著继承一致性没有单独检查。某一章的模型输出不是合法 JSON 时，代码直接 `continue` 跳过，不重试、不记失败。`IsJSONInvalid` 已定义，没有接到自动修复重试上。

### 6. 编辑器里的 AI 操作缺一半

已有（`REWRITE_ACTIONS`）：改写、扩写、缩写、润色、增强冲突、增强情绪。

规格还要求、代码没有：续写、增加动作、增加对白、调整节奏、改变叙事视角。

也没有：AI 生成人物、AI 生成剧情、AI 生成场景。

不存在的接口：`POST /ai/chat`、`POST /ai/continue`、`POST /ai/expand`、`POST /ai/generate`、`POST /ai/analyze`。现有的是 `POST /ai/rewrite`、`POST /chapters/:id/generate`、`POST /original/:id/analysis`。

### 7. 版本只有查看和恢复，没有比较

章节、人物、世界观、大纲有快照和恢复。路由里没有 compare / diff。

### 8. 工程项和文档不符

规格第 50 节要求 `docker-compose.yml`。`docker/` 是空目录，仓库里没有这个文件。

`README.md` 写「PostgreSQL(+pgvector) + Redis(Asynq)」。代码里 Redis 只做健康检查，任务队列在 PostgreSQL，pgvector 未启用。

## 和规格第 72 节 12 项核心能力的对照

| 能力 | 判定 |
|---|---|
| 原著 | 有。TXT / DOCX / PDF、切章 |
| 人物 | 有 |
| 人物 DNA | 有。继承权重在 `inheritance_rules` |
| 世界 | 有 |
| 时间线 | 原著时间线和二创时间线有；一致性检查没用上二创时间线 |
| 分叉点 | 有 |
| 二创 | 有。继承、融合、映射 |
| 大纲 | 部分。没有独立大纲模型，没有 AI 生成大纲 |
| 章节 | 有 |
| 编辑器 | 部分。Tiptap 有，AI 操作缺一半 |
| AI 写作 | 部分。能写本章和 6 种改写，缺大纲/人物/剧情/场景生成 |
| 一致性 | 部分。人物和世界有，时间线是空壳，原著继承未查 |

## 审查范围

本次只读代码和规格，没有重跑 `scripts/smoke-*.sh`，也没有改业务代码。
