# CODEX_STATE.md — 当前开发状态

> 最后更新：2026-10-04（**基准归位：唯一验收基准 = 根目录 `NovaMind_V2_开发规格说明书.md`（v1=v2，全文 72 节）；按规格补齐缺口进入 Phase 8**）
> **每次 Codex 重启，先读这四份**：`docs/CODEX_STATE.md` → 根目录 `NovaMind_V2_开发规格说明书.md` → `docs/SPEC.md`（实施状态对账表）→ `docs/ARCHITECTURE.md`。

---

## 1. 一句话状态

**主体全部完成。** 规格书 §63 的七个 Phase 全部落地，收尾时如实列出的三项缺口（PDF 解析、富文本编辑器、人物/世界/大纲版本历史）已于 2026-09-28 补齐：PDF 导入走自研解析器（真实样本对照 PyMuPDF：整体字符召回 0.9569 / 准确 0.9595，120 个样本里 67 个可用文件 ≥0.98）；编辑器换成 Tiptap 富文本但正文仍以 Markdown 存储；版本历史从章节扩展到人物/世界观/大纲。

**规格基准（2026-10-04 定，推翻 2026-10-03 的"规格自持"）**：v1 规格书已改名移入本仓库根目录 `NovaMind_V2_开发规格说明书.md`，**v1 与 v2 是同一套规格**，它是唯一验收基准；代码注释里的「规格书 §N」指向它。`docs/SPEC.md` 降级为实施状态对账表，不再充当规格。

**仍未实现（按规格书逐条对账，见 `docs/SPEC.md`）**：二创剧情（§26）、大纲独立模型与 AI 生成大纲（§27）、素材（§30）、Context Engine 完整上下文（§31）、ContextSnapshot（§32）、Memory（§33）、Agent/Tool 层（§34–§35）、AI 端点与编辑器动作补全（§38/§49）、时间线与剧情一致性（§39）、检索系统（§55）、JSON Schema 校验（§57）、版本比较（§59）、`docker-compose.yml`（§50）。**按 §68 验收标准，MVP 目前未通过。**

2026-09-29 补：**二创 · 总览**从占位页变成真页面，二创侧直接提供「导入一本书 → 一键开同人」（原著工程 → 导入原文 → 同人作品 → 继承人物与世界观全自动）。

---

## 2. 已完成

| 项 | 说明 |
|---|---|
| 规格书通读（历史） | v1 规格书 `../../novamind-pro/NovaMind_V1_开发规格说明书.md`（2571 行 / 72 节）已于 Phase 0 逐节读完；**2026-10-03 起 V2 不再引用该文档**，开发基准改为 `docs/SPEC.md` |
| 规格基准归位 | 2026-10-04：v1 规格书改名移入本仓库根目录 `NovaMind_V2_开发规格说明书.md`（2571 行 / 72 节），BOSS 确认 **v1=v2**，它是唯一验收基准；代码注释统一写「规格书 §N」 |
| 现有代码勘查 | v1 项目 `../../novamind-pro/` 已勘查：Next.js 15 + SQLite + React 19，71 个 TS/TSX 文件 10597 行，13 个测试文件，git 最近提交「多用户登录 + 每用户模型 Key（阶段 1–5）」；**结论：不复用** |
| 环境勘查 | Go 1.19.8（系统）/ Node v22.18.0 / npm 10.9.3 / Python 3.11.2；**无 PostgreSQL、无 Redis、无 Docker**；sudo 需密码；网络可达 Go/npm 源；磁盘剩 1.1T |
| 目录建立 | 最终位置 `codex/novamindv2/`（与 v1 平级），`docs/` 已建 |
| 治理文档 | `PRODUCT_SPEC.md`、`ARCHITECTURE.md`、本文件 已落盘；`SPEC.md` 为实施状态对账表（2026-10-04 起重定义为对账表，非规格） |
| Phase 5 / 6 / 7 | 写作系统 + 一致性检查 + 版本与导出：后端 4 个模块（domain/repository/service/task+api）、迁移 `0011`/`0012`、端到端冒烟 30 项全过；前端写作工作台 + 编辑器 + 一致性问题页，测试 19 例全绿 |
| 仓库骨架（P1-1） | 目录结构对齐规格书 §50；`.gitignore` / `README.md` / `scripts/dev-env.sh` 就位；Git 仓库已初始化，首提交 `0bc38a7` |
| Go 工具链（P1-2） | Go **1.26.8** 装在 `~/.local/go`（系统 1.19.8 不动） |
| 后端骨架（P1-2） | `config` / `domain.Project` / `api`（统一响应+中间件+健康检查）/ `cmd/server`；`go build`+`go vet`+`gofmt` 全通过；服务实测 200 与统一 404 正常 |

---

## 3. 正在进行

**无进行中的开发项**。Phase 5/6/7 已收官：`scripts/smoke-phase5.sh` 30 项全过；后端 8 个包测试全绿；前端 6 个文件 19 例全绿；迁移版本 12。

### Phase 5/6/7 任务拆分（写作 / 一致性 / 版本与导出）

| # | 任务 | 状态 |
|---|---|---|
| P5-1 | 数据模型：卷 / 章节 / 场景 / 版本 / 一致性问题（迁移 0011） | ✅ |
| P5-2 | 写作服务：章节 CRUD + 正文变更留版 + 恢复前备份 + 上下文组装 | ✅ |
| P5-3 | AI 写作：`writing_chapter` 任务（写本章）+ 编辑器内同步 AI 操作 | ✅ |
| P5-4 | 前端写作工作台：大纲/卷、章节编辑器（1.5s 自动保存）、版本抽屉、场景登记 | ✅ |
| P6 | 一致性检查：`consistency_check` 任务 → 问题清单 → 逐条处理（前端 `/consistency`） | ✅ |
| P7 | 导出 txt / md / docx（自建最小 OOXML）+ 版本历史 | ✅ |
| — | PDF 解析 | ✅ 已完成（2026-09-28，见第 6 节 PDF 解析行） |

### Phase 4 任务拆分（二创系统）

| # | 任务 | 状态 |
|---|---|---|
| P4-1 | 二创作品 + 人物继承（DNA 权重）+ 人物融合 + 映射 | ✅ |
| P4-2 | 二创世界（继承/修改/新增世界规则） | ✅ |
| P4-3 | 分叉点 + 二创时间线 | ✅ |
| P4-4 | 前端二创工作区 | ✅ |

### Phase 3 任务拆分（AI 原著分析）

| # | 任务 | 状态 |
|---|---|---|
| P3-1 | Model Gateway + Prompt Engine + 模型配置 | ✅ |
| P3-2 | 任务系统（异步任务 + 进度 + 重试） | ✅ |
| P3-3 | 分阶段分析流水线 + AI 提案与作者审核 | ✅ |
| P3-4 | 前端：模型配置 / 任务中心 / 提案审核 | ✅ |

### Phase 2 任务拆分

| # | 任务 | 状态 |
|---|---|---|
| P2-1 | 数据模型：files / original_works / original_chapters（迁移 0002） | ✅ |
| P2-2 | 存储抽象 + 文本解析（编码探测 / 章节切分 / DOCX） | ✅ |
| P2-3 | 仓储：原著 CRUD + 事务化章节替换（幂等） | ✅ |
| P2-4 | 原著 API×5 + OpenAPI + 类型 | ✅ |
| P2-5 | 前端：原著总览 / 章节列表 / 章节阅读 / 上传入口 | ✅ |
| P2-6 | 原著其余模型 | 人物 / DNA / 关系 ✅；世界观 ✅；事件 / 时间线 / 剧情弧 ✅（后端 + 前端全部完成） |
| — | PDF 解析 | ✅ 已完成（2026-09-28） |

运行现状：PostgreSQL 15.19（集群 `15 main 5432 online`）与 Redis 7.0.15 均 active；
业务账号 `novamind` 可登录；**迁移版本 = 12**（dirty=false）；各冒烟脚本用 `trap` 自清理，测试不在库里留数据。

---

## 4. Phase 1 任务拆分（历史归档）

> Phase 1 早已收官，此表保留作为最初的拆分依据，便于回溯。

**剩余工作（全项目）**：Phase 1–7 有缺口，**Phase 8 正在按规格书逐项补齐**，明细见 `docs/SPEC.md` 缺口清单（14 项）：docker-compose、一致性检查做实、版本比较、AI 端点与动作、大纲模型与生成、检索、ContextSnapshot、Memory、Agent/Tool、二创剧情、素材、知识库、占位页收尾、JSON Schema。

规格书 §63 定义 Phase 1 = Go Backend + React Frontend + PostgreSQL + Redis + Docker + 基础 API + 项目管理。
本文把它拆成 7 个可验收的任务：

| # | 任务 | 交付物 | 验收方式 |
|---|---|---|---|
| ~~P1-1~~ ✅ | 仓库骨架 | `backend/ cmd+internal+migrations`、`prompts/`、`scripts/`、`docker/`、`.gitignore`、`README.md` | 目录与规格书 §50 一致；已提交 `0bc38a7` |
| ~~P1-2~~ ✅ | Go 工具链与后端可编译 | `~/.local/go`（go1.26.8）+ `backend/go.mod` + `cmd/server/main.go` | `go build`/`go vet`/`gofmt` 通过；实测 health 200、404 统一包、trace_id 贯通 |
| ~~P1-3~~ ✅ | 配置与基础设施层 | `internal/infra`（Postgres 连接池 + Redis 客户端 + 健康检查）+ 接入 main | 实测 health：`postgres=ok`、`redis=ok` |
| ~~P1-4~~ ✅ | 数据层 | `projects` 迁移 + `cmd/migrate`(up/down/down-all/version/force) + `repository.ProjectRepo` | up 幂等、down 可回滚、可重建；13 个测试全绿；测试不污染库 |
| ~~P1-5~~ ✅ | 项目管理 API | 5 端点 + 统一响应/错误码 + 分页 + OpenAPI + Swagger UI | 10 项端到端冒烟全过；防漂移测试守住文档与代码一致 |
| ~~P1-6~~ ✅ | 前端骨架 | Vite+React18+TS+AntD5+Zustand + §8 全量路由（占位页标注计划 Phase）+ `src/api` + 工程管理页 | 组件测试 5 例全绿；vite 代理联调 POST/GET/PUT/DELETE 全通 |
| ~~P1-7~~ ✅ | 测试与收尾 | 后端 15 例 + 前端 5 例测试；刷新 `CHANGELOG.md` 与三份文档 | 全绿；文档已刷新 |

**Phase 1 完成判据**：浏览器可完整操作 `Project` 的增删改查；后端三态健康检查真实；迁移可重建；OpenAPI 可访问；测试全绿。
**Phase 1 不做**：原著导入、AI 调用、二创、编辑器——那是 Phase 2 以后。

---

## 5. 已知问题 / 待决策（阻塞项）

| 编号 | 问题 | 结论 | 状态 |
|---|---|---|---|
| P1 | 本地无 PostgreSQL / Redis，怎么装？ | **方案 A**：BOSS 已装 PostgreSQL 15.19 + Redis 7.0.15，并用 `scripts/setup-local-db.sh` 建好账号与库 | ✅ 已完成 |
| P2 | 项目目录位置 | **`/lzcapp/document/codex/novamindv2/`**（与 v1 平级），已迁移完成 | 已定 |
| P3 | v1（`novamind-pro`）是否复用 | **不复用，全部重新做**；v1 只作为产品交互参考，一行不动 | 已定 |
| P4 | PDF 解析 | ~~暂缓~~ → 已于 2026-09-28 实现（自研解析器） | ✅ 已完成 |
| P5 | 「调试服务器」指哪台 | `47.237.18.94`（阿里云轻量，root 免密）；旧 `8.145.62.78` 已废弃 | 已定 |
| P6 | PDF 解析 | ✅ 已实现：自研解析器（ToUnicode CMap / 编码名兜底 / 内嵌字体 cmap 反查 / 注释外观流 / 空密码解密） | ✅ 已完成 |
| P7 | 富文本编辑器 | ✅ 已实现：Tiptap + Markdown↔HTML 双向转换，正文仍存 Markdown | ✅ 已完成 |
| P8 | 人物/世界/大纲版本 | ✅ 已实现：`entity_versions` 通用快照表 + 12 个 API + 通用版本抽屉 | ✅ 已完成 |
| P9 | 二创侧导入书入口 | ✅ 已实现：`/creative/overview` 二创总览页，支持上传原文一键开同人（走既有接口，不绕过原著/二创边界） | ✅ 已完成 |

其他已知限制：

* 本地无 Docker → `docker-compose.yml` 只能作为部署/CI 产物，本地验收必须不依赖它。
* 系统 Go 1.19.8 偏旧 → 已装 **Go 1.26.8** 到 `~/.local/go`，项目专用，系统不动。
* 依赖拉取偶发 TLS 超时 → 使用 `GOPROXY=https://goproxy.cn,direct`（已在开发脚本中说明）。

---

## 6. 各子系统状态

| 子系统 | 状态 |
|---|---|
| 后端 | **全 Phase 完成**：config / infra / domain / repository / service / api / ai / task 各层贯通；gin v1.12.0、gorm v1.31.2、go-redis v9、golang-migrate v4；`go test ./...` 8 个包全绿 |
| 数据库 | **迁移版本 13**：projects → originals/characters/world/events → model_providers → tasks（+creative_work_id）→ analysis_proposals → creative core/world/timeline → writing（卷/章节/场景/版本/一致性问题）→ entity_versions（人物/世界/大纲快照）；全部可 up/down/重建 |
| API | **P1-5 完成**：`/api/v1/projects` CRUD + `/api/v1/health` + `/api/v1/openapi.yaml` + `/swagger/index.html` |
| 原著系统 | **P2-1 ~ P2-6 完成**：导入与章节（5 API）、人物/DNA/关系（9 API）、世界观（14 API）、事件/时间线/剧情弧（11 API）；前端全链路可用；PDF 解析已于 2026-09-28 补齐 |
| AI 层 | **Phase 3 P3-1 完成**：Model Gateway（OpenAI 兼容 + Anthropic，含重试与错误语义）、Prompt Engine（7 个版本化模板，编译进二进制）、模型配置 CRUD + 连通性测试；密钥 AES-256-GCM 加密，接口不返回密钥 |
| 任务系统 | **P3-2 完成**：PostgreSQL 队列（`FOR UPDATE SKIP LOCKED` 原子领取）+ worker 池 + 进度节流上报 + panic 兜底 + 自动重试/取消；首个任务 `original_reparse`；任务 API 6 个 |
| 分析流水线 | **P3-3 完成**：4 个分析阶段（章节摘要/人物/世界观/剧情）→ 提案表 → 作者审核（可修改后通过）→ 写入原著；模型输出容错提取；6 个 API |
| 写作系统 | **P5 完成**：卷 / 章节（含大纲三要素）/ 场景 / 版本；`writing_chapter` 任务（带人物 DNA + 世界规则 + 前几章摘要的上下文组装）；编辑器内同步 AI 操作 6 种 |
| 一致性引擎 | **P6 完成**：`consistency_check` 任务逐章送审 → `consistency_issues`（severity/type/evidence/suggestion）→ 逐条「已解决/忽略/重新打开」 |
| 导出 | **P7 完成**：txt / md / docx（自建最小 OOXML，无第三方依赖），按「卷 → 章」输出；非法格式 400 |
| 解析与存储 | `internal/parser`（编码/切章/DOCX）、`internal/storage`（本地文件系统 + SHA256 + 路径安全） |
| PDF 解析 | **已完成**：`internal/parser/pdf.go`（对象扫描/对象流/滤镜/页树/内容流/Form XObject）、`pdf_crypt.go`（标准安全处理器 RC4/AESV2 解密）、`pdf_ttf.go`（内嵌 TrueType cmap 反查）、`pdf_debug.go`（诊断）；配套 `cmd/pdftext` 与 `scripts/check-pdf-extract.sh`（PyMuPDF 对照） |
| 版本历史 | **已完成**：章节版本（0011）+ 人物/世界观/大纲快照（0013）；`service/version_service.go`、`components/EntityVersions.tsx`；语义=「只回填快照字段，不回滚删除」 |
| 前端 | Vite 7 + React 18 + antd 5 + **Tiptap 3** + Zustand 5；已实现：工程管理、原著总览/上传（TXT/DOCX/**PDF**）、章节目录/阅读、人物（含 DNA 编辑器）与关系、世界观（世界/规则/地点/势力）、事件/时间线/剧情、模型设置、任务中心、AI 分析审核、**二创总览（导入书一键开同人）**、二创工作区、写作工作台与章节编辑器（**富文本/Markdown 双模式**、自动保存、版本、AI、导出）、一致性问题页、通用版本抽屉（人物/世界/大纲）；9 个测试文件 33 例全绿 |
| 工程化 | `scripts/dev-backend.sh` / `dev-frontend.sh` / `setup-local-db.sh`；`frontend/.npmrc` 走 npmmirror |
| AI 模型 | Model Gateway 已实现（OpenAI 兼容 / Anthropic），密钥 AES-GCM 加密；真实 Key 由 BOSS 在「模型设置」页配置 |
| Prompt 库 | 8 个版本化模板已就位（`backend/prompts/`），编译进二进制；`outline_generate.v1.md` 尚无业务调用（缺口） |
| 任务系统 | 已实现：PostgreSQL 队列（`FOR UPDATE SKIP LOCKED`）+ worker 池 + 重试/取消/进度；**非 Redis/Asynq** |
| 一致性引擎 | 部分实现：人物、世界维度真实送审；时间线为固定句占位、剧情与原著继承未查（缺口，见 `docs/SPEC.md`） |

---

## 7. 关键路径与不可妥协项（写给未来的 Codex）

1. 原著模型（Original）与二创模型（Creative）**物理分离**，AI 不能直接改 Original。
2. Agent 只能 `Agent → Tool → Service → Repository`，**不许碰数据库**。
3. AI 输出必须过 JSON Schema；失败的输出不许入库。
4. 长任务必须异步、可重试、有进度。
5. Prompt 必须版本化；模型厂商不许写死在业务里。
6. 每个 Phase 收尾必须：编译 → 测试 → 启动 → 验证核心流程 → 更新 `CODEX_STATE.md` → 更新 `CHANGELOG.md`。
7. 不许顺手重构整个项目；先读、再设计、再改、再测。

---

## 8. 变更记录

| 日期 | 事件 |
|---|---|
| 2026-09-27 | Phase 0 完成：通读规格书、勘查 v1 与环境、建立 `novamindv2/`、落盘三份治理文档；提出 Phase 1 计划与 3 项待决策 |
| 2026-09-27 | 三项决策拍板：P1=A（BOSS 装 PG/Redis）、P2=目录移至 `codex/novamindv2/`（与 v1 平级）、P3=不复用 v1 代码。Phase 1 开工 |
| 2026-09-27 | P1-1 仓库骨架完成（目录对齐规格书 §50，Git 首提交 `0bc38a7`） |
| 2026-09-27 | P1-2 完成：Go 1.26.8 装到 `~/.local/go`；后端骨架编译/vet/gofmt 通过；实测 `/api/v1/health` 200、统一 404、trace_id 贯通 |
| 2026-09-27 | BOSS 执行方案 A：PostgreSQL 15.19 与 Redis 7.0.15 已安装（服务 enabled 但未启动） |
| 2026-09-27 | P1-3 代码落地：`internal/infra` 的 Postgres/Redis 连接与健康检查，接入 main；编译/vet 通过。另写入 P1-4 迁移 `0001_create_projects` |
| 2026-09-27 | P1-3 实测通过：health 返回 `postgres=ok` / `redis=ok`（BOSS 启动服务 + 执行 `scripts/setup-local-db.sh`） |
| 2026-09-27 | P1-4 完成：迁移执行器 `cmd/migrate`、`repository.ProjectRepo`；验证 up 幂等 / down 回滚 / 重建；domain+repository 测试全绿且不污染库 |
| 2026-09-27 | P1-5 完成：servcie+api 层、5 个端点、错误码翻译、DTO；OpenAPI 3.0.3 规范 + 内嵌 Swagger UI + 防漂移测试（含反向用例）；10 项端到端冒烟全过 |
| 2026-09-27 | P1-6 完成：前端骨架（Vite+React18+antd5+Zustand）、§8 全量路由、工程管理页、5 个组件测试；vite 代理联调全链路通过 |
| 2026-09-27 | P1-7 完成：测试与文档收尾。**Phase 1 交付完毕**；顺手修掉两个真实缺陷（缺 DATABASE_URL 静默降级 → fail fast；jsdom+antd 按钮名空格问题） |
| 2026-09-27 | P2-1~P2-4 完成：原著数据模型 + 解析（编码/切章/DOCX）+ 事务化幂等入库 + 5 个 API；`scripts/smoke-phase2.sh` 15 项全过 |
| 2026-09-27 | P2-5 完成：前端原著总览/章节目录/章节阅读 + 上传入口；工程列表加「原著」入口；前端集成测试 1 例（含 multipart 断言），前端用例共 6 个全绿 |
| 2026-09-27 | 修复 dev-up/dev-down 的 PID 记录缺陷（setsid 派生导致停不干净）与路由按服务可用性注册的问题；补齐按端口清理孤儿进程的兜底 |
| 2026-09-27 | P2-6 第二片完成：世界观（世界/规则/地点/势力，14 个 API，含地点层级环检测与跨世界校验）；冒烟扩到 51 项全过 |
| 2026-09-27 | P2-6 第三片完成：前端「人物」（含 11 维度 DNA 编辑器与关系管理）与「世界观」（世界/规则/地点/势力）页面；修复表单 id 撞车导致 label 关联失效的问题 |
| 2026-09-27 | P2-6 第四/五片完成：事件 / 时间线 / 剧情弧（11 个 API + 前端时间线页与剧情页）；冒烟 72 项、前端 11 项全过；Phase 2 仅剩 PDF |
| 2026-09-27 | Phase 3 开工（BOSS 决定 PDF 暂缓）：P3-1 Model Gateway + Prompt Engine 完成；密钥加密存储、协议分发与重试、7 个模板；单元测试 17 例 + 冒烟 25 项全过 |
| 2026-09-27 | P3-2 任务系统完成：PG 队列 + worker 池 + 重试/取消/进度；`original_reparse` 任务打通；单元测试 9 例 + 集成测试 6 例 + 端到端冒烟 19 项全过 |
| 2026-09-27 | P3-3 完成：AI 分析只产提案、作者通过才写入原著（规格书 §52 落地）；4 个阶段 + 6 个 API；冒烟 24 项全过，并修复统计聚合与错误码两个真 bug |
| 2026-09-27 | P3-4 完成：前端模型设置（真实连通测试）、任务中心（自动刷新/取消/重试）、AI 分析审核页（可改 JSON 后通过）；前端测试 14 例全绿。**Phase 3 收官，下一步 Phase 4 二创系统** |
| 2026-09-27 | P4-2/P4-3 完成：二创世界继承（FULL/PARTIAL/MODIFIED/NEW + 规则 INHERITED/MODIFIED/REMOVED/NEW）+ 分叉点 + 二创时间线（按分叉点自动切分、不覆盖二创新内容）；冒烟 32 项全过；修复空指针与空 ID 两个真 bug |
| 2026-09-27 | P4-4 完成：前端二创工作区（人物继承滑杆含实时派生预览、融合、世界规则状态、分叉点与时间线、映射）；**Phase 4 收官**，下一步 Phase 5 写作系统 |
| 2026-09-27 | P5-1~P5-4 / P6 / P7 完成：写作系统（卷·章节·场景·版本·AI 写本章·编辑器 AI 操作）、一致性检查、txt/md/docx 导出；冒烟 30 项、后端 8 包、前端 19 例全绿；**Phase 5/6/7 收官，全项目仅剩 PDF 解析** |
| 2026-09-27 | 修掉两个真 bug：① 写作任务入队 500（`tasks.work_id` 外键指向原著，改用新增的 `tasks.creative_work_id`，迁移 `0012`）；② 版本/场景/问题接口直接返回领域结构体导致 JSON 键名是 Go 字段名（补 4 个响应 DTO + 1 个请求 DTO） |
| 2026-09-28 | 补齐三项缺口：**PDF 解析**（自研解析器 + 空密码解密 + 内嵌字体 cmap 反查；120 个真实样本对照 PyMuPDF：召回 0.9569 / 准确 0.9595）、**富文本编辑器**（Tiptap，正文仍存 Markdown）、**人物/世界/大纲版本历史**（迁移 `0013`，12 个 API + 通用版本抽屉）。9 个冒烟脚本 268 项、后端 8 包、前端 30 例全绿；迁移版本 **13** |
| 2026-09-29 | 二创 · 总览从占位页变真页面：二创侧新增「导入一本书 → 一键开同人」（原著工程 → 导入原文 → 同人作品 → 继承人物 DNA 与世界观全自动，走既有接口不绕边界）；顺带修掉两处指向占位页的死引导。前端 9 文件 33 例全绿 |
| 2026-10-03 | ~~规格自持~~（次日推翻）：曾新建自持规格 `docs/SPEC.md` 并把全仓引用改指它；2026-10-04 已回退为「以根目录全文规格书为唯一基准」 |
| 2026-10-04 | **基准归位 + 缺口对账**：BOSS 确认 v1=v2，唯一验收基准 = 根目录 `NovaMind_V2_开发规格说明书.md`；全仓引用改回「规格书 §N」；`docs/SPEC.md` 重写为实施状态对账表（✅/🟡/❌ 逐章对账 + 14 项缺口清单）；清理本文件里 PDF、AI 模型、任务系统、一致性引擎等自相矛盾的旧行。**按规格书 §68，MVP 尚未通过** |
| 2026-10-04 | Phase 8 起步：补 `docker-compose.yml`（§50）、版本比较（§59）、一致性检查五类上下文与失败重试（§39/§57/§58） |
| 2026-10-04 | **验证账号纪律**：BOSS 定「验证期可用 DeepSeek，交付前必须删除，使用者自备 Key」。新增 `scripts/validation-account.sh`（seed/status/purge/check）；修复模型配置删除只软删、密钥密文残留的真缺陷（改为先清密文再软删）；本机实清 5 条残留 → 0 行 / 0 密文 |
