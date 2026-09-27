# CODEX_STATE.md — 当前开发状态

> 最后更新：2026-09-27（**Phase 1 / 2 / 3 完成；Phase 4 进行中：P4-1 二创作品与人物继承已完成**）
> **每次 Codex 重启，先读这三份**：`docs/CODEX_STATE.md` → `docs/ARCHITECTURE.md` → `docs/PRODUCT_SPEC.md`。

---

## 1. 一句话状态

**Phase 4 进行中：P4-1 二创作品 + 人物继承（DNA 权重）+ 人物融合 + 映射 已完成。**
Phase 1（基础框架）、Phase 2（原著系统，除 PDF）、Phase 3（AI 分析与审核）均已交付并在界面上可用。
Phase 4 剩余：P4-2 二创世界、P4-3 分叉点与二创时间线、P4-4 前端二创工作区。

---

## 2. 已完成

| 项 | 说明 |
|---|---|
| 规格书通读 | `../../novamind-pro/NovaMind_V1_开发规格说明书.md`（2571 行 / 72 节）已逐节读完，未跳读 |
| 现有代码勘查 | v1 项目 `../../novamind-pro/` 已勘查：Next.js 15 + SQLite + React 19，71 个 TS/TSX 文件 10597 行，13 个测试文件，git 最近提交「多用户登录 + 每用户模型 Key（阶段 1–5）」；**结论：不复用** |
| 环境勘查 | Go 1.19.8（系统）/ Node v22.18.0 / npm 10.9.3 / Python 3.11.2；**无 PostgreSQL、无 Redis、无 Docker**；sudo 需密码；网络可达 Go/npm 源；磁盘剩 1.1T |
| 目录建立 | 最终位置 `codex/novamindv2/`（与 v1 平级），`docs/` 已建 |
| 治理文档 | `PRODUCT_SPEC.md`、`ARCHITECTURE.md`、本文件 已落盘 |
| 仓库骨架（P1-1） | 目录结构对齐规格书 §50；`.gitignore` / `README.md` / `scripts/dev-env.sh` 就位；Git 仓库已初始化，首提交 `0bc38a7` |
| Go 工具链（P1-2） | Go **1.26.8** 装在 `~/.local/go`（系统 1.19.8 不动） |
| 后端骨架（P1-2） | `config` / `domain.Project` / `api`（统一响应+中间件+健康检查）/ `cmd/server`；`go build`+`go vet`+`gofmt` 全通过；服务实测 200 与统一 404 正常 |

---

## 3. 正在进行

**P4-1 完成**：二创作品、人物继承（8 维度权重 → DNA 派生）、人物融合（逐维度来源追溯）、映射表；领域测试 6 例 + `scripts/smoke-phase4.sh` 33 项全过。
下一步 P4-2（二创世界：继承/修改/新增世界规则）。

### Phase 4 任务拆分（二创系统）

| # | 任务 | 状态 |
|---|---|---|
| P4-1 | 二创作品 + 人物继承（DNA 权重）+ 人物融合 + 映射 | ✅ |
| P4-2 | 二创世界（继承/修改/新增世界规则） | ✅ |
| P4-3 | 分叉点 + 二创时间线 | ✅ |
| P4-4 | 前端二创工作区 | 待做 |

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
| — | PDF 解析 | 待做（已知缺口） |

运行现状：PostgreSQL 15.19（集群 `15 main 5432 online`）与 Redis 7.0.15 均 active；
业务账号 `novamind` 可登录；迁移版本 = 1（dirty=false）；`projects` 表 0 行（测试不残留）。

---

## 4. 未完成 / 下一步（Phase 1：基础框架）

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

其他已知限制：

* 本地无 Docker → `docker-compose.yml` 只能作为部署/CI 产物，本地验收必须不依赖它。
* 系统 Go 1.19.8 偏旧 → 已装 **Go 1.26.8** 到 `~/.local/go`，项目专用，系统不动。
* 依赖拉取偶发 TLS 超时 → 使用 `GOPROXY=https://goproxy.cn,direct`（已在开发脚本中说明）。

---

## 6. 各子系统状态

| 子系统 | 状态 |
|---|---|
| 后端 | **P1-5 完成**：config / infra / domain / repository / service / api 六层贯通；gin v1.12.0、gorm v1.31.2、go-redis v9、golang-migrate v4 |
| 数据库 | **P1-4 完成**：`projects` 表 + `schema_migrations`（版本 1）；迁移可 up/down/重建 |
| API | **P1-5 完成**：`/api/v1/projects` CRUD + `/api/v1/health` + `/api/v1/openapi.yaml` + `/swagger/index.html` |
| 原著系统 | **P2-1 ~ P2-6 基本完成**：导入与章节（5 API）、人物/DNA/关系（9 API）、世界观（14 API）、事件/时间线/剧情弧（11 API）；前端全链路可用；**仅剩 PDF 解析（BOSS 决定暂缓）** |
| AI 层 | **Phase 3 P3-1 完成**：Model Gateway（OpenAI 兼容 + Anthropic，含重试与错误语义）、Prompt Engine（7 个版本化模板，编译进二进制）、模型配置 CRUD + 连通性测试；密钥 AES-256-GCM 加密，接口不返回密钥 |
| 任务系统 | **P3-2 完成**：PostgreSQL 队列（`FOR UPDATE SKIP LOCKED` 原子领取）+ worker 池 + 进度节流上报 + panic 兜底 + 自动重试/取消；首个任务 `original_reparse`；任务 API 6 个 |
| 分析流水线 | **P3-3 完成**：4 个分析阶段（章节摘要/人物/世界观/剧情）→ 提案表 → 作者审核（可修改后通过）→ 写入原著；模型输出容错提取；6 个 API |
| 解析与存储 | `internal/parser`（编码/切章/DOCX）、`internal/storage`（本地文件系统 + SHA256 + 路径安全） |
| 前端 | Vite 7 + React 18 + antd 5 + Zustand 5；已实现：工程管理、原著总览/上传、章节目录/阅读、**人物（含 DNA 编辑器）与关系**、**世界观（世界/规则/地点/势力）**；8 个测试 |
| 工程化 | `scripts/dev-backend.sh` / `dev-frontend.sh` / `setup-local-db.sh`；`frontend/.npmrc` 走 npmmirror |
| AI 模型 | 未接入；Phase 1 只做 Model Gateway 骨架与配置表，不接真实 Key（真实接入在 Phase 3） |
| Prompt 库 | 目录规划完成，模板未写 |
| 任务系统 | 未开始（Phase 1 只留结构，Phase 3 实装） |
| 一致性引擎 | 未开始（Phase 6） |

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
