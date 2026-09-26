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

- P1-3 配置与基础设施层：PostgreSQL 连接池、Redis 连接、真实健康检查 —— **阻塞于本机安装 PG/Redis（BOSS 执行方案 A）**
