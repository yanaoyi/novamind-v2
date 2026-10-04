# NovaMind V2

面向"基于原著进行二次创作"的专业 AI 写作系统。开发规格见根目录 **`NovaMind_V2_开发规格说明书.md`**（72 节 / 2571 行，**v1 与 v2 是同一套规格，唯一验收基准**）；[`docs/SPEC.md`](docs/SPEC.md) 是实施状态对账表（记录「规格要求 → 代码实现」的差距），配套 `docs/ARCHITECTURE.md` / `docs/PRODUCT_SPEC.md`。

**当前状态（2026-10-04）**：规格书 §63 的 Phase 1–7 全部完成，Phase 8 正在按规格书逐条补齐缺口 —— 工程管理、原著导入（TXT / DOCX / **PDF**）与结构化、AI 分析（提案→作者审核）、二创（继承/融合/分叉点/时间线）、**大纲独立模型（卷 → 节 → 章，可直接落成章节）**、写作（卷·章节·场景·**富文本编辑器**·版本·AI 写作）、一致性检查（五类上下文）、导出（txt/md/docx）、版本比较、人物/世界观/大纲版本历史、`docker-compose.yml`。尚未实现的 6 个页面（二创设定/剧情/素材、原著人物关系/知识库、AI 助手）见 `docs/SPEC.md` 缺口清单，页面上如实标注"未实现"，不伪装成可用功能。

## 评审入口

| 想看什么 | 看这里 |
|---|---|
| **完整开发规格**（2571 行 / 72 节，v1 = v2，唯一验收基准） | [`NovaMind_V2_开发规格说明书.md`](NovaMind_V2_开发规格说明书.md) |
| 规格逐条对账：哪些已实现、哪些还缺 | [`docs/SPEC.md`](docs/SPEC.md) |
| 产品边界与技术架构 | [`docs/PRODUCT_SPEC.md`](docs/PRODUCT_SPEC.md) · [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) |
| 每一轮改了什么、怎么验证的 | [`docs/CHANGELOG.md`](docs/CHANGELOG.md) |
| 当前开发状态与下一步 | [`docs/CODEX_STATE.md`](docs/CODEX_STATE.md) |

验证方式（都可复跑）：

```bash
bash scripts/dev-up.sh                    # 起服务
cd backend && go test ./... -count=1      # 后端 8 个包
cd frontend && npx vitest run             # 前端 10 个文件 42 例
bash scripts/smoke-phase5.sh              # 各阶段端到端冒烟（含 smoke-phase8-outline.sh 45 项）
bash scripts/check-pdf-extract.sh 120 777 # PDF 解析质量（与 PyMuPDF 对照）
```

详见 `docs/CODEX_STATE.md`。

## 技术栈

Go + Gin + GORM + PostgreSQL(+pgvector) + Redis(Asynq) ｜ React + TypeScript + Vite + Ant Design + Zustand + Tiptap

## 目录

```
backend/    Go 服务（cmd / internal / migrations）
frontend/   React 前端（Phase 1 后期创建）
prompts/    Prompt 模板（版本化）
docs/       PRODUCT_SPEC / ARCHITECTURE / CODEX_STATE / CHANGELOG
scripts/    开发脚本
docker/     Docker 相关（本地无 Docker，仅作部署产物）
```

## 开发环境

本机（懒猫微服容器）实测：

* Go **1.26.8** 装在 `~/.local/go`（系统自带的是 1.19.8，项目不用它）
* Node v22.18.0 / npm 10.9.3
* PostgreSQL / Redis 由系统安装（apt）

进入项目后先加载环境：

```bash
source scripts/dev-env.sh     # 把 ~/.local/go/bin 放到 PATH 最前
```

## 启动

> **首次访问要填访问令牌**：所有业务接口都要求 `Authorization: Bearer <ADMIN_TOKEN>`（安全基线，
> 不让接口在局域网里裸奔）。令牌在服务端 `backend/.env` 里配置：
>
> ```bash
> # 在 backend/.env 中生成（值只留在服务器上，不进前端产物）
> echo "ADMIN_TOKEN=$(openssl rand -hex 32)" >> backend/.env
> grep '^ADMIN_TOKEN=' backend/.env        # 把值粘到浏览器弹出的「需要访问令牌」里
> ```
>
> 生产环境（`APP_ENV=production`）未配置令牌会**拒绝启动**；开发环境未配置会放行并在日志里告警。
> 本地冒烟要用 127.0.0.1 的假模型服务，所以 `backend/.env` 里同时打开 `ALLOW_PRIVATE_MODEL_BASE=true`（生产保持 false）。

**推荐**（后台运行、关掉终端也不会被杀，含就绪检查）：

```bash
bash scripts/dev-up.sh      # 启动前后端
bash scripts/dev-down.sh    # 停止
tail -f .run/backend.log    # 看日志（前端是 .run/frontend.log）
```

也可以各开一个终端跑 `bash scripts/dev-backend.sh` / `bash scripts/dev-frontend.sh`（前台模式，方便看实时输出）。

打开 **http://127.0.0.1:5173** 即可使用；前端开发服务器把 `/api` 代理到后端 8080，无需处理跨域。
前端监听 `0.0.0.0:5173`（懒猫微服把服务发布出去时，代理需要能连上）。

> 注意：不要用 `npm run dev &` 这种一次性写法——会话一结束进程就会被回收，
> 懒猫那边会报 "upstream is not accepting connections"。用 `scripts/dev-up.sh`（内部用 `setsid` 脱离会话）。

常用地址：

| 地址 | 用途 |
|---|---|
| http://127.0.0.1:5173 | 前端界面 |
| http://127.0.0.1:8080/api/v1/health | 健康检查（含 PG / Redis 真实状态） |
| http://127.0.0.1:8080/api/v1/openapi.yaml | OpenAPI 规范 |
| http://127.0.0.1:8080/swagger/index.html | Swagger UI |

首次准备数据库：

```bash
sudo bash scripts/setup-local-db.sh     # 幂等创建 novamind 账号与库
cd backend && go run ./cmd/migrate up   # 建表
```

**注意**：后端必须在 `backend/` 目录下启动——配置按当前工作目录查找 `.env`；
缺 `DATABASE_URL` 会直接拒绝启动（不让服务静默退化成"没有工程接口"的残废状态）。
`scripts/dev-backend.sh` 已经处理了这一点。

## 开发纪律

见 `docs/ARCHITECTURE.md` §12 与 `docs/CODEX_STATE.md` §7。要点：原著/二创物理分离、Agent 不碰数据库、AI 输出必过 Schema、长任务异步、Prompt 版本化、每个 Phase 收尾必须编译+测试+启动+更新状态文档。
