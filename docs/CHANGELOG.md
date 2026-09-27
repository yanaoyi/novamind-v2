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

### 2026-09-27 · P2-6（第四片）后端：事件 / 时间线 / 剧情弧

**数据模型**（迁移 `0005_create_events_timeline_plot`，共 4 张表）

- `original_events`：标题、描述、`chapter_no`（可空）、`time_order`（手工排序号）、参与者（JSONB 人物 ID 数组）、`location_id`（可挂到世界观地点）+ `location_text`、后果/影响、重要度 1-5
- `original_timelines`：一部原著一条（部分唯一索引）
- `timeline_events`：时间线条目 = 事件 + `sequence` + 时间标签 + 持续时长；同一事件在一条时间线只出现一次
- `plot_arcs`：类型（主线/支线/人物线/感情线/世界线）、标题、摘要、起止事件

**后端**：11 个 API

```
GET|POST       /original/{id}/events      事件列表 / 新增
GET|PUT|DELETE /events/{id}
GET|PUT        /original/{id}/timeline    时间线详情 / 设置顺序
GET|POST       /original/{id}/plot-arcs   剧情弧列表 / 新增
PUT|DELETE     /plot-arcs/{id}
```

三个关键设计：

1. **时间线用"整体替换"实现排序**（`PUT /timeline` 传入的顺序即最终顺序），单事务、天然幂等，前端拖拽/上下移动只需一次请求，不用处理"移动时序号冲突"。
2. **跨原著引用一律拦住**：事件参与者必须属于同一部原著（否则 400）；事件地点必须属于该原著的世界；时间线里不能排入别原著的事件；剧情弧的起止事件也要属于该原著。
3. 删除事件会**连带清理它在时间线上的条目**（同一事务），不留悬挂条目。

**验证**：端到端冒烟扩展到 **72 项全过**，事件部分新增 21 项 —— 新增事件（重要度/参与者/章节号回读）、跨原著参与者 400、空标题 400、事件列表、时间线自动创建为空、加入时间线（时间标签与序号 1 回读）、**重复设置幂等**、**排入别原著事件 404**、剧情弧新增/类型校验 400/列表/更新/删除、**删除事件后时间线条目自动清空**。

### 2026-09-27 · P2-6（第五片）前端：时间线页 + 剧情页

**时间线页** `/original/timeline`

- 上半「事件库」：表格（事件 / 章节 / 排序号 / 重要度 / 参与者标签 / 操作），支持新增（标题、描述、章节号、排序号、重要度、**参与者多选**、地点下拉 + 文字补充、后果）、编辑、删除
- 下半「时间线」：本地编排——从事件库「加入时间线」，用上移/下移调整顺序，行内填写**时间标签**与持续时长，一次「保存时间线」整体落库；序号实时重排

**剧情页** `/original/plot`：剧情弧表格（类型标签 / 标题 / 摘要 / 起止事件）+ 新增编辑弹窗（类型下拉、起止事件选择）

**顺手修的无障碍问题**：时间线行的上移/下移/移出是纯图标按钮，屏幕阅读器读不出用途 —— 加了 `aria-label`（如「上移：拆迁通知贴出」），顺带让测试可以稳定定位。

**测试**：新增 `originalTimeline.test.tsx` 3 例 —— 事件入列 → **上移后断言保存顺序真的是 e2 在第一位**、时间标签一起提交；新增事件出现在事件库；剧情弧新增并展示。前端累计 **11 个用例（4 个文件）全绿**，构建通过。

### 待办（Phase 2 剩余）

- **PDF 解析**（唯一剩余项）

### 2026-09-27 · Phase 3 开工：P3-1 Model Gateway + Prompt Engine

> BOSS 决定：先进 Phase 3，PDF 解析暂时搁置。

**模型接入配置**（迁移 `0006_create_model_providers`）

- `model_providers`：名称、提供商类型、接口地址、**加密后的密钥**、模型名、用途（chat/embedding/both）、温度、最大 token、超时、启用、是否默认
- 密钥用 **AES-256-GCM** 加密存储，主密钥由环境变量 `NOVAMIND_SECRET` 经 SHA-256 派生；**接口永不返回密钥**（连尾 4 位都不给），只返回 `has_api_key`
- 同一用途只能有一个默认配置（部分唯一索引保证）
- 忘记 `NOVAMIND_SECRET` 的后果已在 `.env.example` 里写清：已保存的 Key 只能重填

**Model Gateway**（`internal/ai`）

- 统一入口 `Gateway.Chat()`，按提供商类型分发，业务层**不认识任何厂商 SDK**
- 支持两类协议：**OPENAI_COMPATIBLE**（覆盖 OpenAI / DeepSeek / 智谱 / Kimi / one-api / vLLM 等所有兼容 `/chat/completions` 的服务）与 **ANTHROPIC**（Messages API，system 消息自动抽到独立字段、自动补 `max_tokens`）
- 重试策略：限流（429）与 5xx 退避重试最多 3 次；**鉴权错误不重试**（重试没意义还会浪费额度）
- 错误语义：`APIError{Provider, StatusCode, Body}` 带 `IsAuthError/IsRateLimited/IsRetryable` 判定
- JSON 模式：OpenAI 兼容协议自动带 `response_format=json_object`（Phase 3 结构化输出要用）

**Prompt Engine**（`internal/ai/prompt.go` + `backend/prompts/`）

- 模板文件名即版本：`<name>.<version>.md`，`Get(name, "")` 自动取最新版本
- 模板用 `text/template` 且开启 `missingkey=error`：**变量缺失直接报错**，不会静默渲染空值
- 7 个 v1 模板已就位：`chapter_summary` / `character_extract` / `world_extract` / `plot_extract` / `outline_generate` / `chapter_generate` / `consistency_check`（Phase 3/5 直接复用）
- 目录调整：模板从仓库根 `prompts/` 移到 **`backend/prompts/`** —— `go:embed` 只能嵌入模块内文件，放外面就没法随二进制分发

**API**（8 个）：`/model-providers` CRUD + `/{id}/default` + `/{id}/test`（真实调用一次上游）+ `/prompts` 模板清单

**验证**

- 单元测试 17 例：加密往返/换密钥失败/篡改检测、模板加载与最新版本选择、渲染缺变量报错；网关用 `httptest` 假上游验证了 **OpenAI 协议拼装（含 response_format 与鉴权头）**、**5xx 重试恰好 2 次**、**401 不重试**、响应非 JSON 时的报错提示、**Anthropic 的 system 拆分与多段文本拼接**、上下文超时
- 端到端冒烟 `scripts/smoke-phase3.sh`：**25 项全过**。它会在本机起一个假的 OpenAI 兼容上游，让网关**真的发一次 HTTP**——验证上游收到 `Authorization: Bearer sk-...`、`model` 正确、回复被解析；同时验证**库里存的是密文**、响应不含密钥字段/明文、更新时密钥留空仍保留、重复名 409、类型/温度校验 400、删除后列表干净

### 待办（Phase 3 剩余）

- P3-2 任务系统（异步任务 + 进度 + 重试）
- P3-3 分阶段原著分析流水线 + **AI 提案与作者审核**（AI 不得直接改原著模型，规格书 §52）
- P3-4 前端：模型配置页 / 任务中心 / 分析提案审核页

### 2026-09-27 · P3-4 前端：模型设置 / 任务中心 / AI 分析审核（Phase 3 完成）

**模型设置页**（左侧「模型设置」）

- 模型配置表格：名称（含默认/停用标记）、模型名、接口地址、密钥状态（只显示"已保存/未填"，**不显示任何密钥片段**）
- 新增/编辑弹窗：提供商类型（OpenAI 兼容 / Anthropic）、接口地址、模型名、API Key（编辑时留空=不改）、温度、最大 token、超时、启用、设为默认、备注
- **测试按钮**：真实调用一次上游，成功显示模型回复与耗时/token，失败弹出原始错误
- 下方展示 Prompt 模板清单（名称 + 版本），让作者知道 AI 用的是哪套模板

**任务中心**（左侧「任务中心」）

- 任务表格：类型、状态（排队中/执行中/已完成/失败/已取消）、**进度条 + 进度文案**、尝试次数、结果或错误
- **有活动任务时每 2 秒自动刷新，任务跑完自动停**（不用手动点）
- 运行中可取消、失败/取消后可重试；展开行看入参/输出/错误详情

**AI 分析审核页**（左侧「原著 · AI 分析」）

- 顶部四个阶段按钮（章节摘要 / 人物提取 / 世界观提取 / 剧情与事件提取），点击前二次确认（会消耗模型额度）
- 统计三块：待审核 / 已通过 / 已驳回
- 提案表格：状态、实体类型、标题 + **模型给出的原文依据**、来源阶段
- **审核弹窗**：显示原文依据；人物提案额外用标签展示 DNA 各维度权重；**内容是可编辑的 JSON**（作者改完再通过，对应规格书 §40）；可填审核备注；两个动作「通过并写入原著」「驳回」

**测试**：新增 `phase3Pages.test.tsx` 3 例 —— 模型设置页（列表/密钥状态/模板清单/测连通/新增配置并断言提交含 api_key）、任务中心（进度与错误展示、取消运行中任务、重试失败任务）、分析审核页（统计与依据渲染、**在弹窗里改 JSON 后通过，并断言提交的 payload 就是改过的内容**）。前端累计 **14 个用例（5 个文件）全绿**，构建通过。

**Phase 3 完成**：模型接入 → 异步任务 → 分阶段分析 → 提案审核 → 写入原著，全程可在界面操作。

### 2026-09-27 · Phase 4 开工：P4-1 二创作品 + 人物继承 + 人物融合 + 映射

**数据模型**（迁移 `0009_create_creative_core`）

| 表 | 说明 |
|---|---|
| `creative_works` | 二创作品：挂在 CREATIVE 工程下，**必须指向一部原著**；一个工程一部 |
| `creative_characters` | 二创人物：来源类型（`ORIGINAL_INHERITED` / `MODIFIED` / `FUSED` / `NEW`）、派生 DNA、融合来源与逐维度归属、锁定标记 |
| `inheritance_rules` | 人物继承权重（规格书 §19）：8 个维度各 0-100 |
| `original_creative_mappings` | 原著↔二创映射（规格书 §23）：显式记录每个二创元素从哪来 |

**继承算法**（`domain.ApplyInheritance`，规格书 §19 的落地）

```
新权重 = 原著该维度权重 × 继承权重 ÷ 100      // 0 表示完全不带过来
```

规格书示例（性格 90%、价值观 80%、语言风格 30%、能力 0%）实测：原著人格 90 → 派生 72；价值观 80 → 64；语言风格 40 → 12；能力不带过来。

**融合算法**（`domain.FuseCharacters`，规格书 §20）

每个 DNA 维度取"来源该维度权重 × 来源整体权重 ÷ 100"最大者的来源，并**逐维度记录来自谁**（`fusion_detail`）；同分保留先出现的来源以保证结果可复现。实测：林默（整体 60%）与陈述（整体 100%）融合，人格取自陈述（60 > 43），价值观取自林默（只有他有）。

**API**（13 个）：`POST /original/{id}/create-creative`、`GET /original/{id}/creative-works`、`GET|PUT /creative/{id}`、`GET /creative/{id}/characters`、`POST /creative/{id}/characters/{inherit,new,fuse}`、`GET /creative-characters/{id}`（含继承权重）、`PUT|DELETE /creative-characters/{id}`、`GET /creative/{id}/mappings`、`DELETE /mappings/{id}`

继承与融合都会**自动写映射**（INHERITED / MODIFIED / FUSED），保证"每个二创元素从哪来"可追溯；人物可**锁定**，锁定后拒绝被重新继承覆盖。

**验证**

- 领域单元测试 6 例：继承权重按比例缩放、0 权重彻底不继承、全 0 权重被拒绝、融合逐维度取强者并给出归属、整体权重 0 的来源被忽略、二创人物与映射校验
- 端到端冒烟 `scripts/smoke-phase4.sh`：**33 项全过** —— 建原著人物（带 DNA）→ 建二创作品 → 按权重继承（逐一核对 72 / 64 / 12 / 0 四个派生结果）→ 回读继承权重 → 融合并核对维度归属 → 映射 4 条（2 继承 + 2 融合）→ 原创人物无来源 → 5 类错误场景（跨原著继承 400、全 0 权重 400、单来源融合 400、同工程重复建二创 409、给原著工程建二创 400）→ **锁定后拒绝被重新继承（409），解锁后重新继承权重重新算成 45**
- **冒烟抓到并修复一个静默数据错误**：GORM 对带 `default` 标签的字段会把零值从 INSERT 里省略、交给数据库默认值 —— 继承权重里的「能力 0」（表示不继承）被默认值 100 顶替成「完全继承」。去掉该标签后 0 忠实落库

### 2026-09-27 · P4-2 二创世界 + P4-3 分叉点与二创时间线

**数据模型**（迁移 `0010`）

| 表 | 说明 |
|---|---|
| `creative_worlds` | 二创世界：继承模式 `FULL` / `PARTIAL` / `MODIFIED` / `NEW`，指向来源世界 |
| `creative_world_rules` | 二创世界规则：状态 `INHERITED` / `MODIFIED` / `REMOVED` / `NEW`，保留 `source_rule_id` 追溯 |
| `divergence_points` | 分叉点：指向原著事件或章节 + 时间标签与说明 |
| `creative_timeline_events` | 二创时间线：`INHERITED` / `MODIFIED` / `NEW` / `REMOVED`，记录来源原著事件 |

**关键设计**

1. **继承世界时整套带规则**：`FULL` / `PARTIAL` 会把原著规则逐条复制成 `INHERITED`；`MODIFIED` / `NEW` 先建空世界，由作者自己填。
2. **"删除"继承规则不是物理删除**：在二创里删掉一条原著规则时标记为 `REMOVED` 并写映射（`REMOVED`），这样"作者决定不要这条设定"本身也是可追溯的信息；纯新增的规则才真删。
3. **分叉点必须指向本原著**：设置分叉点时会校验事件属于该二创作品所依据的原著，跨原著直接 400。
4. **时间线按分叉点自动切分**：`POST /creative/{id}/timeline/build` 把**分叉点（含）之前**的原著事件按序继承为 `INHERITED`，分叉点之后的不继承（留给二创自己写）；**重新构建不会覆盖作者已有的二创新事件**（它们被保留在末尾）。

**API**（11 个）：`GET|PUT /creative/{id}/world`、`POST /creative/{id}/world/inherit`、`POST /creative/{id}/world/rules`、`PUT|DELETE /creative-world-rules/{id}`、`GET|PUT /creative/{id}/divergence`、`GET|PUT /creative/{id}/timeline`、`POST /creative/{id}/timeline/build`

**验证**：端到端冒烟 `scripts/smoke-phase4b.sh` **32 项全过** —— 继承世界（规则 2 条 INHERITED）→ 改 1 条（MODIFIED）/ 新增 1 条（NEW）/ 删 1 条（REMOVED，且仍在列表里可追溯、统计 `1,1,1`）→ 分叉点（含"没指向"400 与跨原著 400）→ 自动构建时间线（只继承分叉点及之前 2 条，分叉点之后的"真相揭开"未被继承）→ 手工追加二创新事件后重新构建（**总数 3、二创事件被保留在末尾**）→ 映射覆盖世界/事件/规则删除三类

**冒烟抓到并修复两个真 bug**：① 世界还没建时 `/creative/{id}/world` 空指针 → 500（改为返回空壳）；② 时间线继承时写入映射用了内存里的空 ID（真实 ID 由替换操作生成）→ SQL 报"无效的 uuid"，改为**回读后按来源事件 ID 匹配真实条目 ID**；另外把"查不存在的二创作品"从 200 空壳改成 404

### 2026-09-27 · P4-4 前端二创工作区（Phase 4 完成）

新增「二创工作区」（左侧菜单独立入口，同时接管「二创」子菜单的人物/世界/时间线/映射四项）：

- **顶部**：当前二创作品、四个统计（人物 / 世界规则 / 时间线事件 / 映射关系）+ 分叉点状态；可新建二创作品（选 CREATIVE 工程 + 标题）
- **人物标签**：
  - 列表展示姓名、来源类型（继承原著 / 继承后修改 / 多人融合 / 原创）、锁定状态、**DNA 摘要标签**（权重最高的 4 个维度）、融合来源与权重
  - **继承弹窗带实时派生预览**：选原著人物 + 8 个维度滑杆，界面实时显示「原著权重 → 继承后权重」（例如人格 90 → 72），所见即 AI/系统会写进二创的值
  - 人物融合弹窗：可动态增加来源，每个来源给整体权重
  - 支持锁定/解锁（锁定后拒绝被重新继承覆盖）与删除
- **世界标签**：四个继承模式按钮（整套继承 / 部分继承 / 大改 / 全新）+ 规则表格（状态标签：继承 / 已修改 / 已删除 / 新增）+ 统计（继承 n / 已改 n / 已删 n / 新增 n）+ 逐条修改与删除（删除继承规则会标记为「已删除」并保留可追溯性）
- **时间线标签**：下拉选原著事件作为**分叉点**、一键「按分叉点自动构建」、表格展示序号/状态/标题/时间标签；明确提示"分叉点之前继承原著事件，之后由你写"
- **映射标签**：表格展示每个二创元素来自哪条原著设定及映射类型（继承/修改/替换/融合/删除/新增）

**验证**：前端构建通过、**14 个用例（5 个文件）全绿**（新增页面未影响既有用例）。

**Phase 4 完成**：二创作品 → 人物继承（DNA 权重）→ 人物融合 → 映射 → 二创世界 → 分叉点 → 二创时间线，全部可在界面操作。

### 2026-09-27 · P3-3 分析流水线 + AI 提案与作者审核（Phase 3 核心）

**把规格书 §52 那条红线做成了数据库事实**：AI 产出**只写入 `analysis_proposals` 提案表**，作者审核通过后才写进原著正式表；两者在同一事务内完成，不存在"审核过了但没写进去"或反之的中间态。

**数据模型**（迁移 `0008_create_analysis_proposals`）：`work_id`、`task_id`、阶段、实体类型、标题、`payload`（JSONB）、原文依据 `evidence`、置信度、状态（PENDING/APPROVED/REJECTED）、审核备注与时间、`applied_id`（通过后写入正式表的记录 ID）；同一任务内同实体只留一条（部分唯一索引）。

**四个分析阶段**（规格书 §54 的分阶段任务，各自是一个任务类型）

| 阶段 | 任务类型 | 产出提案 |
|---|---|---|
| `chapter_summary` | `analysis_chapter_summary` | 章节摘要（逐章调用，可回写到章节） |
| `character_extract` | `analysis_character_extract` | 人物（含 11 维 DNA 权重与原文依据） |
| `world_extract` | `analysis_world_extract` | 世界设定、规则、地点（含层级 parent）、势力 |
| `plot_extract` | `analysis_plot_extract` | 事件（含参与者/地点/后果）、剧情弧 |

**模型输出容错**（`service/analysis_json.go`）：模型即使被要求"只输出 JSON"，也常包 ```json 代码块或带解释文字 —— 这里做容错提取（去代码块 → 取首个 `{` 到末个 `}`），单章解析失败只跳过该条而不拖垮整批。结构化校验按实体类型要求最小字段（人物必须有名字、事件必须有标题…），不合法直接跳过。

**审核通过即写入**：`ApproveProposal` 支持**作者修改后再通过**（payload override，规格书 §40），写入时按实体类型分派：人物→人物表（DNA 一起落）、规则/地点/势力→世界观、事件→**参与者按姓名解析成人物 ID、地点按名称关联**、剧情弧→**起止事件按标题匹配**、章节摘要→回写章节。所有写入都在 `Approve` 的事务里，`apply` 闭包接收 `tx` 复用同一事务。

**API**（6 个）：`POST /original/{id}/analysis`（入队）、`GET /original/{id}/proposals`、`GET /original/{id}/analysis/summary`、`GET /proposals/{id}`、`POST /proposals/{id}/approve`、`POST /proposals/{id}/reject`。

**验证**

- 单元测试：JSON 容错提取（纯 JSON / 代码块 / 夹带解释文字 / 各种非法输入）、DNA 解析与工具函数、提案校验（14 个用例覆盖各实体类型的必填字段与剧情弧类型）
- 端到端冒烟 `scripts/smoke-phase3-analysis.sh`：**24 项全过** —— 起假模型上游 → 触发人物提取 → **AI 产出 2 条提案但原著人物数仍为 0**（红线验证）→ 作者改名后通过 → 原著出现「林默（已校对）」且 `source=AI`、DNA 权重 95 保留、`applied_id` 非空 → 另一条驳回后**没有写进原著** → 统计 pending0/approved1/rejected1 → 重复审核 409 → 无章节时触发分析 400
- **冒烟抓到两个真 bug 并修复**：① 提案统计恒为 0 —— gorm 链式调用复用同一个 statement 导致 Where 条件叠加，改为三次独立查询；② 无章节时触发分析返回 500 —— 缺哨兵错误，补 `ErrNoChapters` 并映射 400

### 2026-09-27 · P3-2 任务系统（异步 + 进度 + 重试 + 取消）

**数据模型**（迁移 `0007_create_tasks`）：`tasks` 表 —— 类型、状态（PENDING/RUNNING/PAUSED/COMPLETED/FAILED/CANCELLED）、进度 0-100、进度文案、input/output（JSONB）、错误、`attempts`/`max_attempts`、开始与结束时间；`(status, created_at)` 部分索引供 worker 领取。

**队列实现的选择**：规格书给的是「Asynq 或等价任务队列」。我用 **PostgreSQL 自身做队列**（`UPDATE ... WHERE id = (SELECT ... FOR UPDATE SKIP LOCKED)`），理由是：任务状态与业务数据同库同事务，**重启不丢任务**，部署不需要再依赖一个中间件；将来要换 Asynq 只需替换 worker 的取任务方式，上层接口不变。

**执行框架**（`internal/task`）

- `Registry`：任务类型 → 处理函数；`Worker`：可配置并发数与轮询间隔的 worker 池
- `Reporter`：handler 用它上报进度（**800ms 节流**，避免高频写库）并检查取消
- **panic 兜底**：handler 崩了会被记为任务失败，而不是把 worker 打死（这类故障最难查）
- 失败自动回到队列重试，达到 `max_attempts` 才置 FAILED；**取消的任务不会被 worker 改写成"完成"**

**第一个真实任务** `original_reparse`：用已保存的源文件重新解析原著章节（切章规则升级后不用让作者重传）。新增 `POST /original/{id}/reparse`（返回 202 + task_id），以及任务 API：列表/详情/取消/重试/手工入队 + `/task-types`。

**验证**

- `internal/task` 单元测试 9 例（内存假仓储，确定性）：成功完成并写 output、空队列返回 (false,nil)、**失败→回队列→重试成功**、**超过上限才 FAILED**、**panic 被兜住并记为失败**、未知类型失败、**已取消任务不被改写**、注册表排序与查找、入参取值
- `internal/repository` 集成测试 6 例（真实 PG，事务回滚）：领取后 RUNNING 且 attempts=1、队列空返回 nil、**失败回到 PENDING→达上限 FAILED→重试清零**、**终态不可取消/不可重试**、不存在报 404、列表过滤与倒序
- 端到端冒烟 `scripts/smoke-phase3-tasks.sh`：**19 项全过** —— 真实走了一遍「建工程→建原著→导入 3 章→入队 reparse→worker 领取执行→轮询到 COMPLETED」，校验进度 100、attempts=1、output 里章节数与编码正确、章节数未变；再验证完成任务的取消/重试都返回 409、非法原著的任务会 FAILED 且带 error、**失败任务可重试并重新排队**、未知类型 400、按 work_id 过滤生效
