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

### 2026-09-27 · Phase 2 开工：原著导入闭环（P2-1 ~ P2-4）

按规格书 §63 Phase 2 拆分，先交付"上传 → 解析 → 切章 → 入库 → 查询"这条闭环。

**P2-1 数据模型**（迁移 `0002_create_originals`）：`files`（上传登记）、`original_works`（原著，一工程一部，部分唯一索引）、`original_chapters`（章节，含原文字节偏移 start_position/end_position，支持回溯）；全部沿用 UUID + 时间戳 + 软删除 + CHECK 约束。

**P2-2 存储与解析**

- `internal/storage`：`Store` 接口 + `LocalStore`（本地文件系统，按 `<根>/<project_id>/<uuid><ext>` 落盘，写入时计算 SHA256 与 MIME；路径穿越与扩展名白名单校验）
- `internal/parser`：编码探测（UTF-8 / UTF-8 BOM / UTF-16 LE·BE / GB18030 / Big5，按"乱码最少"打分选择）、章节识别（`第X章/回/节/卷/篇`、`Chapter N`、`序章/楔子/尾声/番外`）、无标题时的长度兜底切分；DOCX 用标准库解 zip + 扫描 `word/document.xml`（零第三方依赖）
- **PDF 暂未支持**（`ParseByFilename` 返回明确错误），属已知缺口，见 CODEX_STATE

**P2-3 仓储**：`repository.OriginalRepo` —— 创建原著/文件登记；`ReplaceChapters` 单事务"先软删后批量插入 + 回写章节数与字数"，配合"未删除行的唯一索引"保证**重复导入幂等**。

**P2-4 API**（5 个端点，含 OpenAPI 与类型）：`POST /projects/{id}/original`、`GET /original/{id}`、`POST /original/{id}/import`（multipart）、`GET /original/{id}/chapters`、`GET /original/{id}/chapters/{no}`。

**顺带修掉的两个基础设施缺陷**

1. `scripts/dev-up.sh` 用 `$!` 记录的是 `setsid` 的 PID，派生后即失效 → `dev-down` 停不干净、新实例抢不到端口。改为"子进程自己写 `$$` 再 exec"，并给 `dev-down` 加了按端口清理本项目孤儿的兜底。
2. 所有业务路由改为**恒定注册**，服务未就绪时返回 503 而非 404 —— 避免"数据库没连上"被误读成"接口不存在"。

**验证**

- 单元/集成测试 7 个测试文件：parser 11 例（含 GB18030 往返、BOM、DOCX 抽取、无标题兜底）、storage 4 例（含路径穿越拒绝）、domain/repository/service/api 既有用例全绿
- 新增可重复执行的端到端脚本 `scripts/smoke-phase2.sh`：**15 项全过**，含 GBK 中文原文导入识别为 GB18030、5 章切分、重复导入幂等（库里仍 5 章）、PDF→400、重复建原著→409、给 CREATIVE 工程建原著→400、不存在资源→404，且用 trap 自动清理（按外键顺序）

### 待办（Phase 2 剩余）

- P2-5 前端：原著总览页 / 章节列表 / 章节阅读 / 上传入口
- P2-6 原著其余模型与接口：OriginalCharacter、CharacterRelationship、World/WorldRule/Location/Faction、OriginalEvent、Timeline、PlotArc（AI 提取在 Phase 3）
- PDF 解析支持

### 2026-09-27 · 运行方式修正（懒猫服务发布）

- 问题：用一次性命令 `npm run dev &` 启的服务，工具会话一结束就被回收，懒猫微服报
  `upstream http://127.0.0.1:5173 is not accepting connections`
- 修复：新增 `scripts/dev-up.sh`（`setsid` + PID 文件 + 就绪检查）与 `scripts/dev-down.sh`；
  前端 `vite.config.ts` 改为 `host: true`（监听 `0.0.0.0:5173`，代理能连上）+ `strictPort`
- 验证：新会话复查，后端 PID 的 PPID = 1（已脱离会话），端口 `*:5173` / `127.0.0.1:8080` 正常，三条链路均 200
