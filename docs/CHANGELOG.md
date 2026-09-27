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

### 2026-09-27 · P2-5 前端原著页面完成

- `src/api/original.ts`：原著接口封装（按工程取原著时把 404 转成 `null`，便于"还没有原著"分支）
- `src/api/client.ts`：**FormData 不再强行写 `Content-Type: application/json`**（否则 multipart 边界丢失，后端解析不出文件）
- `src/stores/originalStore.ts`：当前原著、章节目录、当前章节；只持久化 `workId`（刷新后仍停在同一部原著），不持久化数据本身
- 页面：
  - **原著总览** `OriginalOverviewPage`：未选原著时列出 ORIGINAL 工程供选择 → 该工程没有原著则引导创建 → 有原著则显示详情 + **拖拽上传**（.txt/.docx，导入后提示识别到的编码与切分章数）
  - **章节目录** `OriginalChaptersPage`：分页表格，点击进入阅读
  - **章节阅读** `ChapterReaderPage`：正文按原文换行展示、显示字数与原文位置、上一章/下一章
- 工程列表新增「原著」入口（ORIGINAL 类型工程可见），带目标工程跳转，总览页自动定位
- 路由：`/original/overview`、`/original/chapters`、`/original/chapters/:no` 接上真实页面，其余原著菜单项仍为占位

**测试**：新增 `originalFlow.test.tsx` —— 在内存假后端上跑完整链路（自动定位工程 → 创建原著 → 选文件 → **断言上传确实是 multipart/FormData** → 详情刷新为已导入 → 目录 4 章 → 阅读正文）；前端用例 6 个全绿，`npm run build` 通过。

顺手修：`ProjectsPage` 引入 `useNavigate` 后，旧测试因缺少 Router 上下文失败 → 测试改为在 `MemoryRouter` 中渲染。

### 2026-09-27 · 运行方式修正（懒猫服务发布）

- 问题：用一次性命令 `npm run dev &` 启的服务，工具会话一结束就被回收，懒猫微服报
  `upstream http://127.0.0.1:5173 is not accepting connections`
- 修复：新增 `scripts/dev-up.sh`（`setsid` + PID 文件 + 就绪检查）与 `scripts/dev-down.sh`；
  前端 `vite.config.ts` 改为 `host: true`（监听 `0.0.0.0:5173`，代理能连上）+ `strictPort`
- 验证：新会话复查，后端 PID 的 PPID = 1（已脱离会话），端口 `*:5173` / `127.0.0.1:8080` 正常，三条链路均 200

### 2026-09-27 · P2-6（第一片）人物 / 人物 DNA / 人物关系

**数据模型**（迁移 `0003_create_characters`）

- `original_characters`：姓名、别名（JSONB）、角色/性别/年龄/外貌、性格/动机/价值观/恐惧/欲望/行为模式/语言风格/能力、首次与最后出场、**人物 DNA（JSONB）**、重要度 1-5、来源（MANUAL/AI）、备注；同原著内**姓名唯一**（大小写不敏感，软删除不占用）
- `character_relationships`：有向边（source → target）、10 种关系类型、强度 0-100、描述、来源；**禁止自环**，同一对人物的同一关系类型唯一

**人物 DNA**（规格书 §11 的核心数据结构）：11 个维度（personality / values / motivation / behavior / speech_style / background / ability / decision_style / conflict_response / emotional_response / relationship_pattern），每维一句描述 + **0-100 权重**；权重越界直接拒绝写入。这是后续二创"人物继承"（§19 InheritanceRule）的计算基础。

**后端**：`repository.OriginalCharacterRepo`（PostgreSQL 唯一/外键冲突 → 领域错误翻译）、`service.OriginalCharacterService`（关系两端必须同属一部原著）、9 个 API：

```
GET|POST       /api/v1/original/{id}/characters
GET|PUT|DELETE /api/v1/characters/{id}
GET|POST       /api/v1/original/{id}/relationships
PUT|DELETE     /api/v1/relationships/{id}
```

删除人物会连同其相关关系一起删除（同一事务内完成）。

**验证**

- 单元测试：DNA 边界值（0/100 合法，101/-1 拒绝）、维度必须为 11 个、别名去空白去重、重要度默认值、关系自环 / 非法类型 / 强度越界
- 端到端冒烟扩展到 **33 项全过**：新增人物（DNA 权重保留）、别名去重、同名 409、DNA 权重 120 → 400、关键字搜索、DNA 更新、建关系（强度 80）、重复关系 409、自环 400、**跨原著建关系 400**、关系更新、删除人物后关系自动消失

### 待办（Phase 2 剩余）

- P2-6（第二片）：世界观（World / WorldRule / Location / Faction）、事件、时间线、剧情弧
- PDF 解析支持

### 2026-09-27 · P2-6（第二片）世界观：世界 / 规则 / 地点 / 势力

**数据模型**（迁移 `0004_create_world`）

- `original_worlds`：一个原著一个世界（部分唯一索引）
- `world_rules`：分类、名称、描述、重要度 1-5；同世界内**名称唯一**
- `locations`：支持 `parent_location_id` 形成「大陆 → 国家 → 城市」层级；**禁止自环**（CHECK），同世界内名称唯一；删除地点时子地点上级自动置空
- `factions`：类型、描述、目标、与其他势力的关系；同世界内名称唯一

**后端**：`repository.OriginalWorldRepo` + `service.OriginalWorldService`（世界 upsert、统计、地点层级校验）+ **14 个 API**：

```
GET|PUT        /api/v1/original/{id}/world        概览（含 3 个计数）/ 创建或更新
GET|POST       /api/v1/original/{id}/rules        规则列表 / 新增
PUT|DELETE     /api/v1/world-rules/{id}
GET|POST       /api/v1/original/{id}/locations    地点列表 / 新增
PUT|DELETE     /api/v1/locations/{id}
GET|POST       /api/v1/original/{id}/factions     势力列表 / 新增
PUT|DELETE     /api/v1/factions/{id}
```

关键设计：**世界不存在时自动创建空世界**（`ensureWorld`），所以可以直接新增规则/地点而不用先保存世界设定；列表接口在世界不存在时返回空数组而不是 404，前端无需处理两种错误态。

地点层级做了两层防护：数据库 CHECK 拦自环，service **沿上级链向上走查环**（深度上限 100）并校验上级与自身同属一个世界。

**验证**

- 单元测试：规则/地点/势力/世界的清洗与校验（含空白名称、重要度越界、地点自环、空白上级归一为 nil）
- 端到端冒烟扩展到 **51 项全过**，世界观部分新增 18 项：保存世界设定、初始计数、新增规则、重复规则 409、重要度 9 → 400、顶层与子地点、自己当上级 400、**层级成环 400**、**跨世界上级 400**、势力新增与重复 409、三项统计正确、规则更新、势力删除后计数归零

### 待办（Phase 2 剩余）

- 「人物」与「世界观」前端页面（后端已就绪，界面仍是占位页）
- 原著事件、时间线（含分叉点前置数据）、剧情弧
- PDF 解析支持

### 2026-09-27 · P2-6（第三片）前端：人物页 + 世界观页

**人物页** `/original/characters`

- 人物表格：姓名（含别名）、角色、重要度（核心/主要/重要/次要/路人）、**DNA 摘要**（自动列出权重最高的 3 个维度，如「人格 90% · 价值观 80%」）、编辑/删除
- 新增/编辑弹窗：姓名、别名（回车添加标签）、角色、性别、年龄、外貌、性格、动机、价值观、恐惧、欲望、行为模式、语言风格、能力、首次/最后出场、备注
- **人物 DNA 编辑器**：11 个维度各一行 —— 描述输入 + 0-100 滑块 + 数字输入（滑块与数字框绑定同一字段，可拖动也可精确输入）
- **人物关系区**：表格（从 → 关系类型 → 到、强度、说明）+ 建立关系弹窗；少于 2 个人物时按钮禁用并给出提示

**世界观页** `/original/world`（`/original/locations`、`/original/factions` 复用同页并直接落到对应标签）

- 世界设定表单（名称 + 设定说明）+ 保存，下方显示规则/地点/势力三个计数
- 三个标签页：**世界规则**（名称/分类/重要度滑块/说明）、**地点**（名称/类型/**完整层级路径**/说明）、**势力**（名称/类型/目标/与其他势力关系）
- 地点编辑时**自动排除自己与所有下级**作为可选上级，从交互层面就避免用户提交出环（后端仍会拦一道）

**顺手修掉一个真实的可访问性 bug**：同一页面上多个表单的字段都叫 `name`，antd 生成的 DOM id 撞车，导致 `<label for>` 指向错误元素（屏幕阅读器与自动化都受影响）。给每个表单加了 `name` 前缀（`world` / `rule` / `location` / `faction` / `character` / `relation`）后 id 唯一。

**测试**：新增 `originalProfile.test.tsx` —— 人物页（列表与 DNA 摘要渲染 → 新增人物并断言提交的 `dna.personality.weight` 就是界面上设的 85 → 删除人物）与世界观页（保存世界设定 → 新增规则并断言提交内容 → 页面计数与列表更新）；前端用例 **8 个全绿**（3 个文件），构建通过。

另外把 vitest 超时从 20 秒放宽到 45 秒：人物页那张 11 维度 DNA 表单在 jsdom 下单例渲染实测要接近 20 秒，与其它文件并行跑时会误报超时。

### 待办（Phase 2 剩余）

- 原著事件、时间线（含分叉点前置数据）、剧情弧（后端 + 前端）
- PDF 解析支持
