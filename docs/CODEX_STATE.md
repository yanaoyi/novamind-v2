# CODEX_STATE.md — 当前开发状态

> 最后更新：2026-09-27（Phase 1 · P1-1 / P1-2 已完成并通过验收）
> **每次 Codex 重启，先读这三份**：`docs/CODEX_STATE.md` → `docs/ARCHITECTURE.md` → `docs/PRODUCT_SPEC.md`。

---

## 1. 一句话状态

**Phase 1 进行中：P1-1（骨架）、P1-2（Go 工具链 + 后端可编译）已完成并实测通过。**
下一项 P1-3 需要 PostgreSQL / Redis 就绪（BOSS 执行方案 A 的安装命令）。

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

**P1-1 仓库骨架、P1-2 Go 工具链与后端可编译 —— 已完成并验收。**
**P1-3 起需要 PostgreSQL / Redis 就绪**（BOSS 执行方案 A 安装命令后即可继续）。

---

## 4. 未完成 / 下一步（Phase 1：基础框架）

规格书 §63 定义 Phase 1 = Go Backend + React Frontend + PostgreSQL + Redis + Docker + 基础 API + 项目管理。
本文把它拆成 7 个可验收的任务：

| # | 任务 | 交付物 | 验收方式 |
|---|---|---|---|
| ~~P1-1~~ ✅ | 仓库骨架 | `backend/ cmd+internal+migrations`、`prompts/`、`scripts/`、`docker/`、`.gitignore`、`README.md` | 目录与规格书 §50 一致；已提交 `0bc38a7` |
| ~~P1-2~~ ✅ | Go 工具链与后端可编译 | `~/.local/go`（go1.26.8）+ `backend/go.mod` + `cmd/server/main.go` | `go build`/`go vet`/`gofmt` 通过；实测 health 200、404 统一包、trace_id 贯通 |
| P1-3 | 配置与基础设施层 | 配置加载（env）+ PG 连接池 + Redis 连接 + 结构化日志（含 trace_id）+ `/api/v1/health`（报告 DB/Redis 状态） | `curl /api/v1/health` 返回各依赖真实状态 |
| P1-4 | 数据层 | `projects` 表迁移（UUID、created_at/updated_at、软删除）+ `domain.Project` + `repository.ProjectRepo` | 迁移能从零重建；重复执行不报错；repo 单测通过 |
| P1-5 | 项目管理 API | `GET/POST/GET:id/PUT:id/DELETE:id /api/v1/projects` + 统一响应/错误码 + 分页 + OpenAPI 文档 | API 测试全绿；`/swagger` 可访问 |
| P1-6 | 前端骨架 | Vite+React+TS+AntD+Zustand 工程 + 按 PRODUCT_SPEC §8 的空白路由 + `src/api` 客户端 + **项目管理页可增删改查** | 浏览器里能建项目→列表→改→删 |
| P1-7 | 测试与收尾 | 后端 unit/service/API 测试、前端组件测试、`docs/CHANGELOG.md`、更新本文件 | 全部测试通过；三份文档状态刷新 |

**Phase 1 完成判据**：浏览器可完整操作 `Project` 的增删改查；后端三态健康检查真实；迁移可重建；OpenAPI 可访问；测试全绿。
**Phase 1 不做**：原著导入、AI 调用、二创、编辑器——那是 Phase 2 以后。

---

## 5. 已知问题 / 待决策（阻塞项）

| 编号 | 问题 | 结论 | 状态 |
|---|---|---|---|
| P1 | 本地无 PostgreSQL / Redis，怎么装？ | **方案 A**：BOSS 执行 `sudo apt-get update && sudo apt-get install -y postgresql redis-server` | 已定，待 BOSS 执行 |
| P2 | 项目目录位置 | **`/lzcapp/document/codex/novamindv2/`**（与 v1 平级），已迁移完成 | 已定 |
| P3 | v1（`novamind-pro`）是否复用 | **不复用，全部重新做**；v1 只作为产品交互参考，一行不动 | 已定 |

其他已知限制：

* 本地无 Docker → `docker-compose.yml` 只能作为部署/CI 产物，本地验收必须不依赖它。
* 系统 Go 1.19.8 偏旧，最新 Gin/GORM 依赖链要求更高版本 → 计划装 go1.24.x 到 `~/.local/go`，不动系统。

---

## 6. 各子系统状态

| 子系统 | 状态 |
|---|---|
| 后端 | **骨架完成**（P1-1/P1-2）：config / domain.Project / api（统一响应+中间件+health）/ cmd/server；gin v1.12.0 + uuid v1.6.0 |
| 数据库 | 未开始（无库、无迁移） |
| 前端 | 未开始（0 行） |
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
