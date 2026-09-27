# CHANGELOG

## [Unreleased] Phase 1 · 基础框架

### 2026-09-27 · P1-1 仓库骨架

- 建成目录结构（对齐规格书 §50）：`backend/{cmd,internal,migrations,testdata}`、`prompts/{original,character,world,plot,outline,writing,review}`、`docs/`、`scripts/`、`docker/`
- `internal` 预置分层：`api / config / domain / repository / service / agent / ai / context / memory / task / retrieval / consistency`（Phase 2–6 逐层填充）
- 新增 `.gitignore`（忽略 `.env`、`node_modules`、`data/`、构建产物、覆盖率）、`README.md`、`scripts/dev-env.sh`
- Git 仓库初始化，首个提交 `0bc38a7`

### 2026-09-27 · P1-2 Go 工具链与后端可编译

- 安装 Go **1.26.8** 到 `~/.local/go`（系统自带 1.19.8 不动，项目只用前者）
- `backend/go.mod`：module `github.com/yanaoyi/novamindv2/backend`，依赖 gin v1.12.0、google/uuid v1.6.0
- 落地代码：
  - `internal/config`：env 配置加载 + 轻量 `.env` 读取（已存在的环境变量优先）
  - `internal/domain/project.go`：Project 实体、ProjectType/ProjectStatus 枚举与领域校验（不依赖任何框架）
  - `internal/api`：统一响应包 `{data,error,trace_id}`、错误码、RequestID/Logger/Recovery 中间件、`GET /api/v1/health`（依赖未接入时如实返回 `not_configured`）
  - `cmd/server/main.go`：启动 + 优雅关闭 + 结构化日志（dev 用 text、prod 用 json）
- 验收证据：`go build ./...` 通过、`go vet ./...` 通过、`gofmt -l` 无输出；服务实测 `GET /api/v1/health` → 200 且 checks 显示 `postgres/redis: not_configured`；未知路径 → 统一 404 包；`X-Request-Id` 透传并在日志中贯通

### 待办（下一步）

- P1-3 配置与基础设施层：代码已完成（`internal/infra`），**阻塞于本机 PostgreSQL/Redis 服务未启动**

### 2026-09-27 · P1-3 基础设施连接（代码就绪，待实测）

- 新增 `internal/infra/postgres.go`：GORM + pgx 连接池，连接参数（最大连接 20 / 空闲 5 / 空闲超时 10m / 寿命 1h），带 3 次重试的连通性探测，`Health()` 供健康检查
- 新增 `internal/infra/redis.go`：go-redis v9 客户端 + 重试探测 + `Health()`
- `cmd/server` 接入：配置了就连（连不上直接启动失败，不"假装健康"）；未配置则跳过并告警
- `backend/.env`（本机开发配置，已被 .gitignore 忽略）
- 依赖新增：gorm v1.31.2、gorm.io/driver/postgres v1.6.3、redis/go-redis/v9 v9.22.0
- 迁移 `0001_create_projects.up/down.sql`：UUID 主键、枚举 CHECK、软删除、4 个部分索引，可重复执行（`IF NOT EXISTS`）

### 2026-09-27 · P1-3 实测通过

- BOSS 启动服务并执行 `scripts/setup-local-db.sh`（幂等创建角色/库）
- 实测：`GET /api/v1/health` → `status=ok`，`checks.postgres=ok`、`checks.redis=ok`；`novamind @ novamind | PG 15.19`

### 2026-09-27 · P1-4 数据层完成

- `migrations/embed.go`：`go:embed` 把 SQL 迁移打进二进制
- `cmd/migrate`：`up / down / down-all / version / force`；回滚到空库时正确打印"当前版本: 无"（修正了把 `ErrNilVersion` 当失败的 CLI bug）
- `internal/repository/project_repo.go`：持久化模型与领域实体分离；创建（自动 UUIDv7）、按 ID 查（非法 UUID 直接判不存在而非 500）、分页列表（类型/状态/关键字过滤 + page_size 钳制）、更新（type 不可变）、软删除
- 测试：domain 纯单测 8 例 + repository 集成测试 5 例（真实 PG，事务内执行并回滚，不污染开发库）
- 验收证据：`up` → 版本 1；再 `up` → "没有需要执行的迁移"；`down` → 表删除、版本归无；`up` → 表重建；`go test ./...` 全绿；测试后 `projects` 行数 = 0

### 2026-09-27 · P1-5 项目管理 API 完成

- `internal/service/project_service.go`：清洗→校验→落库；仓储以接口注入便于测试替身；**工程类型创建后不可变更**
- `internal/api/project.go`：5 端点 + DTO + 领域错误→HTTP 错误码翻译（404 PROJECT_NOT_FOUND / 400 BAD_REQUEST / 500 记日志）
- OpenAPI：`internal/api/openapi.yaml`（OpenAPI 3.0.3，4 path、完整 schema）+ 内嵌规范端点 `/api/v1/openapi.yaml` + Swagger UI `/swagger/index.html`（静态资源内嵌，不依赖 CDN）
- **防漂移测试**：`TestOpenAPICoversAllRoutes` 校验每个已注册路由都在规范中；`TestVerifyRouteCoverageDetectsMissingRoute` 反向证明护栏有效
- 端到端冒烟 10 项全过：创建 201 / 列表分页 / 详情 / 更新（改名+归档、type 不变）/ 删除 / 删除后 404 / 非法 UUID 404 / 空名称 400 / 非法 type 400 / 类型过滤
- 测试：service 4 例（假仓储纯逻辑）+ api 2 例

### 2026-09-27 · P1-6 前端骨架完成

- 工程：Vite 7 + React 18 + TypeScript + Ant Design 5 + Zustand 5 + react-router-dom 6
  - **React 用 18 而非 19**：antd v5 配 React 19 需要额外补丁包，Phase 1 优先稳定
- 路由按 PRODUCT_SPEC §8 建全（dashboard / projects / original×10 / creative×10 / editor / ai / consistency / tasks / settings），未实现模块用统一占位页并标注计划 Phase
- `src/api/`：统一请求层（解析 `{data,error,trace_id}`、抛带后端错误码的 `ApiError`）+ 工程接口封装 + 与 OpenAPI 对齐的类型
- `src/stores/projectStore.ts`：Zustand 状态与动作（load / create / update / remove）
- `src/pages/ProjectsPage.tsx`：列表分页、关键字搜索、类型过滤、新建 / 编辑（类型不可改）/ 删除（二次确认）
- `src/pages/Dashboard.tsx`：工作台展示后端真实健康状态
- 开发脚本：`scripts/dev-backend.sh`（自动 cd 到 backend，缺 .env 直接报错）、`scripts/dev-frontend.sh`
- 网络：`frontend/.npmrc` 指向 npmmirror（官方源实测 13 分钟未完成依赖解析，镜像 18 秒装完 252 个包）
- 测试：`ProjectsPage.test.tsx` 5 例（列表渲染 / 新建 POST / 编辑 PUT 且不提交 type / 删除二次确认 / 后端 500 不白屏），用内存假后端校验前后端契约
- **前后端联调冒烟**：vite 代理链路（5173 → 8080）POST 201、列表、PUT 改状态、DELETE 全通
- 修掉两个真实缺陷：① 缺 `DATABASE_URL` 时"降级启动"导致工程路由不注册、接口静默 404 → 改为 **fail fast**；② jsdom 下 antd 两字中文按钮插入空格导致测试按名字找不到按钮

### 2026-09-27 · P1-7 收尾（Phase 1 完成）

- 测试全景：后端 15 例 + 前端 5 例，全绿；测试不污染开发库（事务回滚）
- 文档刷新：本文件、`CODEX_STATE.md`、`README.md`、`ARCHITECTURE.md`

### 待办（下一步：Phase 2）

- Phase 2 原著系统：文件上传、文本解析、章节识别，以及 OriginalWork / OriginalChapter / OriginalCharacter / World / Timeline / Plot
