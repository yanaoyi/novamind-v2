# NovaMind V2

面向"基于原著进行二次创作"的专业 AI 写作系统。按 `docs/PRODUCT_SPEC.md` / `docs/ARCHITECTURE.md` 分阶段实现，规格书见 `../novamind-pro/NovaMind_V1_开发规格说明书.md`。

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

## 启动（Phase 1 目标形态）

```bash
# 后端
cd backend && go run ./cmd/server        # 默认 :8080
curl -s localhost:8080/api/v1/health

# 前端（Phase 1 后期）
cd frontend && npm install && npm run dev
```

配置：复制 `backend/.env.example` 为 `backend/.env` 并填写。**`.env` 不提交 Git。**

## 开发纪律

见 `docs/ARCHITECTURE.md` §12 与 `docs/CODEX_STATE.md` §7。要点：原著/二创物理分离、Agent 不碰数据库、AI 输出必过 Schema、长任务异步、Prompt 版本化、每个 Phase 收尾必须编译+测试+启动+更新状态文档。
