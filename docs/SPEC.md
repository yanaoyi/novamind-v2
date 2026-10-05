# SPEC.md — 实施状态对账表（不是规格）

> **术语说明（2026-10-05）**：产品界面把 Project 这个实体叫「**文章**」（菜单「文章管理」），
> 早期文档与规格书里叫「工程」——指的是同一个东西（`projects` 表 / `/api/v1/projects`）。
> 本文件的文字已统一为「文章」，规格书原文保持原样以便逐条对照。

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
| §27 | 大纲（Outline / OutlineNode） | ✅ | 2026-10-04：迁移 `0014` 建 `outlines` + `outline_nodes`（卷 → 节 → 章三层，parent_id 自引用）；14 个端点 + 前端「二创 · 大纲」页（手搭 / AI 候选采纳 / 一键落成章节 / 版本与比较）。此前只有「卷 + 章节大纲字段」近似，AI 生成大纲的产物无处落库 |
| §28–§29 | Chapter / Scene | ✅ | 迁移 `0011` |
| **§30** | **素材（Material）** | ❌ **未实现** | 无 `materials` 表与页面 |
| §31 | Context Engine（优先级 10 段） | 🟡 **部分** | 写作上下文拼人物 DNA + 世界规则 + 前 3 章摘要；**一致性检查已扩到五类上下文**（人物/世界/时间线/剧情/原著继承）；仍缺原著检索片段（依赖 §55） |
| **§32** | **ContextSnapshot** | ❌ **未实现** | 无快照表、无落库 |
| **§33** | **Memory 三层** | ❌ **未实现** | `internal/memory` 为空目录 |
| **§34–§35** | **7 个 Agent + Agent Runtime（Agent → Tool → Service → DB）** | ❌ **未实现** | `internal/agent` 为空目录；任务是 handler 直接调 service |
| §36 | Model Gateway | ✅ | OpenAI 兼容 + Anthropic，密钥 AES-GCM |
| §37 | Prompt Engine（版本化模板） | ✅ | `prompts/<域>/<name>.<version>.md`，编译进二进制 |
| §38 | AI 写作（续写/扩写/改写/大纲/人物/剧情/场景生成） | ✅ **已闭环** | 2026-10-04：编辑器操作补齐到 **11 种**（+续写/增加动作/增加对白/调整节奏/改变叙事视角）；新增 AI 生成大纲/人物/剧情/场景（`/ai/generate`，只返回候选、作者确认后走既有写入接口）；新增 AI 问答（`/ai/chat`）与就地分析（`/ai/analyze`，不进问题库） |
| §39 | 一致性检查（Character/World/Timeline/Plot + 原著继承） | ✅ **已闭环** | 2026-10-04：五类上下文全部真实注入（二创时间线按 sequence 排序、章节大纲链做剧情骨架、原著↔二创映射做继承检查）；prompt 升级 `consistency_check.v2.md`；JSON 非法自动重试一次，仍失败则计入 `failed_chapters` 并在任务输出里如实报告 |
| §40–§45 | 分析流程与继承界面 | ✅ | 提案 → 作者审核 → 写入 |
| §46–§48 | 页面结构 / 主界面 / 编辑器 | 🟡 **部分** | 三栏与 Tiptap 已有；6 个菜单仍是占位页（见 §3） |
| §49 | API 设计 | ✅ **已闭环** | 2026-10-04：AI 六个端点全部就位（`/ai/rewrite` + `/ai/continue`、`/ai/expand`、`/ai/generate`、`/ai/chat`、`/ai/analyze`）；版本比较 `GET /versions/compare`；OpenAPI 与路由由防漂移测试守住 |
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
| §68 | MVP 验收（12 项 + 生成二创大纲） | ✅ | 2026-10-04：`scripts/validate-e2e-deepseek.sh` 用**真实模型 37/37 全过**（含「AI 生成大纲 → 采纳落库 → 一键落成 3 卷 18 章」）。交付前须执行 `scripts/validation-account.sh purge` 清掉验证账号 |

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
| ~~4~~ ✅ | AI 端点与编辑器动作补全 | §38 §49 | 2026-10-04 完成（6 端点 + 11 种编辑器操作 + 6 个版本化模板） |
| ~~5~~ ✅ | 大纲独立模型 + AI 生成大纲 | §27 §68 | 2026-10-04 完成：`outlines`/`outline_nodes` + `/ai/generate?kind=outline` 候选采纳 + 前端大纲页 + 一键落成章节 |
| 6 | 检索系统（Chunk/Embedding/混合检索） | §55 §56 | `internal/retrieval` + 索引表 + 写作上下文接入 |
| 7 | ContextSnapshot | §32 | 快照表 + 每次生成前落库 |
| 8 | Memory 三层 | §33 | `internal/memory`：长期/项目/短期 |
| 9 | 7 个 Agent + Tool 层 | §34 §35 | `internal/agent`：Agent → Tool → Service → Repository |
| 10 | 二创剧情 | §26 | `creative_plots` + API + 页面 |
| 11 | 素材 | §30 | `materials` + API + 页面 |
| 12 | 知识库页 | §46 | `/original/knowledge`：检索入口（依赖 #6） |
| 13 | 占位页收尾：二创设定、原著人物关系独立页、AI 助手页 | §46 §48 | 复用既有接口 |
| 14 | JSON Schema 校验 + 自动修复重试 | §57 §58 | 在 Model Invoker 层统一实现 |
| 15 | 交付前清除验证账号 | §36 §69 | 已提供 `scripts/validation-account.sh`（seed / status / purge / check），交付时执行 `purge` 并留存输出作为证据 |

完成一项就更新本表状态；每完成一个阶段按 §34 纪律（编译 → 测试 → 启动 → 验证 → 更新 `CODEX_STATE.md` / `CHANGELOG.md`）收尾。

---

## 4. 验证账号纪律（BOSS 2026-10-04 定）

* 开发与验证阶段可以临时挂一个 DeepSeek 账号，用来跑通端到端 AI 链路；
* **系统交付前必须清除**：执行 `scripts/validation-account.sh purge`，把模型配置连同行数据一起物理删除；
* 系统**不内置任何模型账号**，使用者自备 API Key，在「模型设置」页自行配置；
* 密钥只从环境变量读取，绝不写进文件、不进命令行参数（避免进 shell 历史与进程列表）；`scripts/validation-account.sh check` 负责扫描仓库里的硬编码密钥。

**已修的真实缺陷（2026-10-04）**：模型配置的「删除」原先只做软删除，`api_key_cipher` 仍留在表里 —— 等于账号看起来删了、密文还在。现在仓库层删除时会先把密文清空，再打软删除标记保留审计痕迹。

---

## 5. 端到端验证记录（真实模型）

脚本：`scripts/validate-e2e-deepseek.sh`（可复跑；每步失败都会打印后端返回的错误正文）

| 日期 | 模型 | 结果 | 覆盖链路 |
|---|---|---|---|
| 2026-10-04 | DeepSeek（验证期临时账号） | **32/32 通过** | 导入 3 章 → 章节切分 → AI 人物提取（3 提案）→ 作者审核写入 → 建二创文章与作品 → 人物/世界继承 → AI 生成大纲 → 建卷建章 → AI 写本章 → AI 续写 → AI 就地分析 → AI 问答 → 一致性检查 → 导出 TXT |
| 2026-10-04（复跑） | DeepSeek（同一临时账号） | **37/37 通过** | 同上，另加 **采纳 AI 大纲为独立模型（3 卷 / 9 节 / 18 章）→ 一键落成 18 章**；并修掉「正文类提示词被强制 JSON 模式导致空正文」的真缺陷（见 CHANGELOG） |

本轮验证暴露并修掉 3 个真缺陷（大纲模板变量名、写本章空正文静默成功、提案 importance 越界导致审核 400），详见 `CHANGELOG.md`。
