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
| Frontend | **React 18 + TypeScript + Vite** | SPA；React 18 为与 antd v5 完全兼容（React 19 需额外补丁包），后续可评估升级 |
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
│   │   ├── parser/            # 文档解析：编码探测、章节切分、DOCX 抽取
│   │   ├── repository/        # 数据访问（GORM）
│   │   ├── service/           # 业务编排
│   │   ├── storage/           # 文件存储抽象（本地实现，可换 OSS/S3）
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
│       ├── api/               # 请求层与类型（与 backend/internal/api/openapi.yaml 对齐）
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

删除顺序必须满足外键依赖（实测踩过）：`original_chapters → original_works → files → projects`。
清理脚本/测试的 teardown 一律按这个顺序写，否则会撞 `original_works.source_file_id_fkey`。

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
* **接口禁止直接返回领域结构体**：handler 一律转成 `xxxResponse` DTO 再返回。领域结构体没有 json tag，直接返回会把 `VersionNo` / `EmotionalGoal` 这类 Go 字段名漏给前端（2026-09-27 实测踩过一次，见 CHANGELOG）。
* 二进制响应（导出）不走统一响应包：直接 `Content-Type` + `Content-Disposition` 返回流。

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

落地实现（Phase 6）：`consistency_check` 异步任务 → `service.WritingService.CheckConsistency` 逐章送审 `prompts/review/consistency_check.v1.md` → 模型输出容错提取（复用 P3-3 的解析思路）→ 校验 severity/type/description 后写 `consistency_issues` 表 → 作者在 `/consistency` 页逐条「已解决 / 忽略 / 重新打开」。**AI 只报告问题，不自动改文**（产品原则 1）。

---

## 9.1 写作与版本（Phase 5 / 7）

* 层级：`creative_works → creative_volumes → creative_chapters → creative_scenes / chapter_versions`。
* **正文变化才留版本**：标题、大纲、状态、归属卷的修改不产生版本噪声；只有 `content` 真正变了才 `snapshot`。
* **恢复版本前先自动备份当前正文**：`RestoreVersion` 的顺序固定为「读旧版 → 备份当前 → 写入旧版内容 → 再留一版」，任何一次恢复都不会让内容凭空消失。
* **列表默认不带正文**：`GET /creative/{id}/chapters` 默认剔掉 `content`（长篇正文一起返回会拖垮列表），需要正文时用 `?full=true` 或逐个取详情。导出走 `GET /creative/{id}/export?format=`，按「卷 → 章」排版。

### 9.1.1 编辑器与存储格式（2026-09-28 定）

* **正文一律以 Markdown 文本存储**（数据库里就是一个 `text` 字段）。富文本编辑器（Tiptap/ProseMirror）只负责「所见即所得」的呈现与编辑，进出都经过 `frontend/src/editor/markdown.ts` 的 Markdown↔HTML 转换。
* 这样做的原因：Markdown 能被 AI 直接读、能 diff、能导出、能进版本快照；换成存 HTML 会让 AI 上下文与导出全部多一层清洗。
* 转换器只覆盖写作子集（标题/加粗/斜体/删除线/引用/有序无序列表/分割线/代码），**子集外的写法按纯文本处理，绝不丢字**；HTML 特殊字符必须转义（有测试守着）。
* 编辑器支持「富文本 / Markdown 源码」双模式；外部改动（切章、AI 改写、恢复版本）通过 `lastEmitted` 比较后同步进编辑器，避免回写环路。

### 9.1.2 版本历史（人物 / 世界观 / 大纲）

* 章节版本用 `chapter_versions`（0011，正文体积大、读写频繁，单独存）。
* 结构化实体（人物/世界观/大纲）用**通用快照表** `entity_versions`（0013）：`entity_type + entity_id + version_no + payload(JSONB)`，唯一约束防重号。
* 语义（**必须坚持，界面上也这么写**）：
  * 快照 = 某一刻该实体的完整状态；**每次改动后存一份**；
  * 与最新一版 payload 相同则**不建版本**（`reflect.DeepEqual` 去重，避免噪声版本）；
  * 恢复 = 把快照里记录的字段写回去，**不回滚删除**——快照之后新建的人物/规则/章节不会因此消失；
  * 恢复前若当前内容与最新版本不同，会先自动留一版。
* 快照钩子挂在 API 层的「真实改动」上（继承/新增/融合/修改人物、继承/改世界/增删规则、建卷/建章/改章节大纲），失败只记日志不影响主流程。

---

## 9.2 PDF 解析（2026-09-28 实现）

**为什么不引第三方库**：本机 Go 模块缓存里没有任何 PDF 库，网络拉包不稳；而「文本型 PDF 抽文本」是可以精确做到的。实现放在 `internal/parser/`：

| 文件 | 职责 |
|---|---|
| `pdf.go` | 对象扫描（不依赖 xref）、对象流展开、滤镜与预测器、页树遍历（继承 /Resources）、内容流算子抽文本、Form XObject 递归、字体与编码 |
| `pdf_crypt.go` | 标准安全处理器解密：R2/R3/R4（RC4-40/128、AESV2-128），空用户密码；算法 1/2/4/5 与 `/EncryptMetadata false` 的 4 个 `0xFF` |
| `pdf_ttf.go` | 内嵌 TrueType 的 sfnt/cmap（format 4、12）解析与 GID→Unicode 反查 |
| `pdf_debug.go` | 解析诊断（对象数/页数/每页字体与 ToUnicode 覆盖/加密状态），配合 `PDF_DEBUG=1` |

**必须守住的几条经验**（都是踩出来的）：

1. **同一对象号出现多次 = 增量更新，后写覆盖**；保留旧版本会读到过期对象。
2. **ToUnicode 可能不完整**：缺的码位要用「编码名兜底 → 字节特征猜 CJK → 内嵌字体 cmap 反查」三级兜底。
3. **换行按字号比例判**（≥0.5×字号），不能用固定 pt；OCR 文本层逐字一个 BT/ET，在 ET 处换行会把每个字切成一行。
4. **注释外观流（/Annots → /AP → /N）里也常有正文/水印**，不读会误判成纯扫描件。
5. **字体对象要缓存**：大报表里同一个 870KB 内嵌字体会被解析上千遍。
6. 扫描件（无文本层）**明确报错提示需要 OCR**，不要把空字符串当成功。

**质量守门**：`scripts/check-pdf-extract.sh` 用 PyMuPDF 当基准跑真实样本，按「字符召回率」分桶，并把「基准自己就是乱码」的文件单列（这种文件任何抽取器都只能得到乱码，不该算我们头上）。

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
| Go | 系统 `go1.19.8`（`/usr/lib/go-1.19`），无 root 升级 | ✅ 已另装 **go1.26.8** 到 `~/.local/go`，项目专用；不依赖系统 Go |
| Node/npm | v22.18.0 / 10.9.3 | 直接可用 |
| PostgreSQL | 原为未安装 | ✅ 已装 **15.19**，集群 `15 main` 在线；账号/库用 `scripts/setup-local-db.sh` 创建 |
| Redis | 原为未安装 | ✅ 已装 **7.0.15**，监听 127.0.0.1:6379 |
| Docker / Podman | **未安装，也没有 docker.sock** | `docker-compose.yml` 只作为部署/CI 产物；**本地开发流程不得依赖 Docker** |
| sudo | 需要密码（当前用户属 sudo 组） | 不擅自使用 sudo；需要系统级安装时请 BOSS 执行 |
| Python | 3.11.2（scripts 用） | 可用 |
| 网络 | Go 源可用；**npm 官方源极慢**（实测 13 分钟未完成依赖解析） | Go 用 `GOPROXY=https://goproxy.cn,direct`；npm 用 `frontend/.npmrc` 指向 `registry.npmmirror.com`（18 秒装完 252 包） |
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
