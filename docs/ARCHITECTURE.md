# ARCHITECTURE.md — 技术架构与代码规范

> 项目：NovaMind V2
> 本文件是技术层面的唯一裁定文件。开发中如与规格书冲突，以规格书为准；如与本文件冲突，先改本文件再改代码。

---

## 1. 技术栈（已定）

| 层 | 选型 | 说明 |
|---|---|---|
| Backend | **Go + Gin + GORM** | 规格书 §6 |
| 数据库 | **PostgreSQL 16**（+ pgvector） | JSONB 存 AI 半结构化结果 |
| 缓存/队列 | **Redis + Asynq** | 异步任务 |
| Frontend | **React + TypeScript + Vite** | SPA |
| UI | **Ant Design** | 先求稳，不追求视觉定制 |
| 状态 | **Zustand** | 轻量，够 V1 |
| 编辑器 | **Tiptap** | 规格书 §48 |
| API 文档 | **OpenAPI 3 / Swagger** | 规格书 §64.4 强制 |
| 迁移 | **golang-migrate（SQL 文件）** | 必须可重复执行 |
| 文件 | 本地文件系统（抽象成 Storage 接口） | 生产可换 OSS/S3/MinIO |

**模型接入**：只通过统一的 Model Gateway 接入 OpenAI / DeepSeek / 智谱 / Claude / 其它 OpenAI Compatible API。
**禁止**在业务代码里写死任何厂商，**禁止**业务层直接调用厂商 SDK。

---

## 2. 目录结构

```
novamindv2/
├── backend/
│   ├── cmd/server/            # main
│   ├── internal/
│   │   ├── api/               # HTTP handler、路由、中间件、DTO
│   │   ├── config/            # 配置加载（env）
│   │   ├── domain/            # 实体、枚举、领域规则（无外部依赖）
│   │   ├── infra/             # 基础设施连接：PostgreSQL / Redis（只做连接与健康）
│   │   ├── repository/        # 数据访问（GORM）
│   │   ├── service/           # 业务编排
│   │   ├── agent/             # 7 个 Agent 的实现
│   │   ├── ai/                # Model Gateway / Prompt Engine
│   │   ├── context/           # Context Engine / Context Snapshot
│   │   ├── memory/            # 三层 Memory
│   │   ├── task/              # 任务系统（Asynq）
│   │   ├── retrieval/         # 切分 / 向量检索 / 混合检索
│   │   └── consistency/       # 一致性引擎
│   ├── migrations/            # 可重复执行的 SQL 迁移
│   └── testdata/
├── frontend/
│   └── src/
│       ├── pages/             # 路由页面（对齐 PRODUCT_SPEC §8）
│       ├── components/
│       ├── features/          # 按业务域组织
│       ├── stores/            # Zustand
│       ├── api/               # API 客户端（由 OpenAPI 生成类型）
│       └── editor/            # Tiptap
├── prompts/
│   ├── original/ character/ world/ plot/ outline/ writing/ review/
├── docs/                      # PRODUCT_SPEC / ARCHITECTURE / CODEX_STATE / CHANGELOG
├── scripts/
├── docker/
├── docker-compose.yml
└── README.md
```

---

## 3. 分层与依赖方向（硬约束）

```
HTTP → api → service → repository → DB
```

* `domain` 不依赖任何外层（不 import gin/gorm）。
* `api` 只做参数校验 + 调用 service + 组装响应，**不写业务逻辑**。
* `repository` 只管数据存取，**不写业务判断**。
* Agent 调用链（规格书 §35 强制）：

```
Agent → Tool → Service → Repository → DB
```

**Agent 不允许直接操作数据库。** Tool 是 Agent 能触达能力的唯一边界，第一版工具集：
`get_character / get_world / get_timeline / get_plot / get_chapter / search_original / search_creative / create_character / update_character / create_outline / create_chapter / check_consistency`。

* **原著/二创数据隔离**：Original 相关表与 Creative 相关表分开，写路径分开；AI 只能通过"审核确认"写入 Original。

---

## 4. 数据与迁移

强制规则（规格书 §51）：

1. 所有表 **UUID 主键**；
2. `created_at` / `updated_at`；
3. **软删除**（`deleted_at`）；
4. 外键约束；
5. AI 提取的半结构化数据用 **JSONB**；核心实体不许整块塞进 JSON；
6. 原著与二创数据严格区分。

迁移：`backend/migrations/*.up.sql|down.sql`，用 golang-migrate 执行；
**必须能从零重建库**，且重复执行不报错；每次 schema 变更都要写迁移文件，不允许只靠 `AutoMigrate` 交付。

命名：表名复数蛇形（`creative_characters`），外键 `<单数表名>_id`，索引 `idx_<表>_<列>`。

---

## 5. API 规范

* 前缀固定 `/api/v1`（规格书 §49）。
* REST 风格资源路径，与规格书列举的端点保持一致。
* 统一响应包：

```json
{ "data": {...}, "error": null, "trace_id": "..." }
{ "data": null, "error": { "code": "PROJECT_NOT_FOUND", "message": "...", "details": {} }, "trace_id": "..." }
```

* 分页：`?page=1&page_size=20`，响应带 `total`。
* 所有请求带 `X-Request-Id`（无则生成），贯穿日志。
* **OpenAPI 文档随代码自动生成**（swaggo 注解或 huma），CI/本地可访问 `/swagger`。
* 长任务不阻塞 HTTP：创建任务返回 `202 + task_id`，进度走 `GET /api/v1/tasks/:id`。

---

## 6. AI 层

### 6.1 Model Gateway

```
ModelProvider { id, name, provider, api_base, api_key, model_name, enabled }
统一接口：Generate() / Chat() / Embedding()
```

API Key **只存库（加密）或环境变量**，严禁硬编码、严禁提交 Git。

### 6.2 Prompt Engine

Prompt 模板文件化 + 版本化：`prompts/<域>/<name>.<version>.md`，代码里只引用 `prompt_name + prompt_version`。
已规划的模板：`OriginalCharacterAnalysis / CharacterGeneration / WorldAnalysis / OutlineGeneration / ChapterGeneration / ConsistencyCheck`。

### 6.3 Context Engine（写给 Agent 的上下文）

优先级固定（规格书 §31）：

```
1 用户当前指令 → 2 当前章节/场景 → 3 二创人物 → 4 二创世界 → 5 二创时间线
→ 6 二创剧情 → 7 原著继承设定 → 8 原著人物 DNA → 9 原著世界模型 → 10 原著剧情背景
```

**禁止**把原著全文塞进 Prompt。上下文 = 结构化数据 + 向量检索 + 当前章节上下文 + 作者指令。

每次生成前落一条 **ContextSnapshot**（模型、prompt_version、检索到的原著/二创片段），用于调试与重现。

### 6.4 Memory 三层（禁止混成一坨）

| 层 | 内容 |
|---|---|
| Long-term | 人物、世界、时间线、剧情、作者偏好 |
| Project | 本作品设定 |
| Short-term | 当前章节、当前场景、最近几轮对话 |

### 6.5 结构化输出与容错（规格书 §57、§58）

AI 返回 **JSON** → 后端用 JSON Schema 校验 → 失败自动修复重试 → 仍失败则任务失败。
**绝不把未通过校验的 AI 输出写入数据库。**

---

## 7. 任务系统

所有长任务异步（规格书 §53）：`Task{id, project_id, type, status, progress, input, output, error, created_at, started_at, finished_at}`，
状态 `PENDING/RUNNING/PAUSED/COMPLETED/FAILED/CANCELLED`，进度需可分阶段展示（原著分析示例：章节解析 ✓ / 人物提取 ✓ / 世界观分析 运行中 / 时间线 等待）。

原著分析必须切成 9 个阶段任务，每阶段可单独重试。

---

## 8. 检索

`原著章节 → Chunk → Embedding → pgvector`；检索时组合：语义搜索 + 章节范围 + 人物 + 时间 + 场景。
**不要只靠向量相似度**（规格书 §55）。

---

## 9. 一致性引擎

四类检查：Character / World / Timeline / Plot（外加"原著继承一致性"）。
统一输出结构：

```json
{ "severity": "high", "type": "timeline", "description": "...", "evidence": "...", "suggestion": "..." }
```

---

## 10. 前端规范

* 页面结构严格对齐 PRODUCT_SPEC §8。
* 所有请求走 `src/api/`，**组件内不直接 fetch**；类型由 OpenAPI 生成，保证前后端一致。
* 编辑器自动保存：debounce 1–2 秒；切章/离开前强制 flush。
* AI 操作入口：选中文本 → 续写/扩写/改写/缩写/润色/增强冲突/增加对白/调整节奏…
* 三栏布局为主界面骨架。

---

## 11. 测试策略（规格书 §67）

| 层 | 要求 |
|---|---|
| Backend | Unit（domain/service）+ Service（真实 PG 测试库）+ API（httptest） |
| Frontend | 组件测试（Vitest + Testing Library）+ API Mock |
| AI | JSON Schema 测试、Prompt 回归测试、Context 组装测试 |

核心必测案例：人物提取、人物 DNA、人物融合、世界继承、时间线继承、分叉点、二创事件、章节生成、一致性检查。

每个 Phase 结束必须：编译 → 测试 → 启动 → 验证核心流程。

---

## 12. 代码规范

* 语言：Go 侧 `gofmt` + `go vet`；TS 侧 ESLint + `tsc --noEmit`。
* 命名：Go 用惯用短名，导出符号必须有注释；TS 组件 PascalCase、hooks `useXxx`。
* 错误：Go 侧 `fmt.Errorf("...: %w", err)` 包装，统一在 api 层翻译成错误码；不吞错。
* 日志：结构化（`log/slog`），含 `trace_id / project_id / task_id`；重要操作必须记录。
* 配置：全部走环境变量 + `.env`；**`.env` 不提交 Git**；密钥不得硬编码（规格书 §64.13/14）。
* 纪律：不许"为实现一个功能顺手重构全项目"；改动前先读模块、判断能否复用、先设计再改、再测、再更新状态。

---

## 13. 本机环境实测与由此产生的决策

| 项 | 实测结果 | 决策 |
|---|---|---|
| Go | 系统 `go1.19.8`（`/usr/lib/go-1.19`），无 root 升级 | **另装 go1.24.x 到 `~/.local/go`**，只给本项目用；不依赖系统 Go |
| Node/npm | v22.18.0 / 10.9.3 | 直接可用 |
| PostgreSQL | **未安装** | 见下方"待确认决策 P1" |
| Redis | **未安装** | 同上 |
| Docker / Podman | **未安装，也没有 docker.sock** | `docker-compose.yml` 只作为部署/CI 产物；**本地开发流程不得依赖 Docker** |
| sudo | 需要密码（当前用户属 sudo 组） | 不擅自使用 sudo；需要系统级安装时请 BOSS 执行 |
| Python | 3.11.2（scripts 用） | 可用 |
| 网络 | proxy.golang.org / registry.npmjs.org 均 200 | 依赖可拉 |
| 磁盘 | 剩余 1.1T | 充足 |

### 已定决策（2026-09-27 BOSS 拍板）

| 编号 | 结论 |
|---|---|
| **P1** | **方案 A**：BOSS 执行 `sudo apt-get update && sudo apt-get install -y postgresql redis-server`；本地按系统默认路径使用 PG/Redis |
| **P2** | 项目根目录 = **`/lzcapp/document/codex/novamindv2/`**（与 v1 `novamind-pro/` 平级），已迁移完成 |
| **P3** | **不复用 v1 代码，全部重新做**；v1 保持不动，仅作产品交互参考 |

---

## 14. 与 v1 的关系

v1 是 Next.js + SQLite 的单体实现，与规格书要求（Go + PG + Redis + React）不是同一技术路线。
v2 的目标是把规格书的结构化模型（Original/Creative 分离、Character DNA、分叉点、映射）真正落到数据层。
v1 **保持不变、不动一行**，只作为产品交互与领域经验的参考。
