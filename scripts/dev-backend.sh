#!/usr/bin/env bash
# 在正确的工作目录启动后端。
# 必须从 backend/ 启动：配置加载按「当前工作目录」查找 .env。
#
# 用法：bash scripts/dev-backend.sh
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export PATH="${HOME}/.local/go/bin:${PATH}"
cd "${PROJECT_ROOT}/backend"

echo "工作目录: $(pwd)"
if [ ! -f .env ]; then
  echo "错误：backend/.env 不存在。请先复制 .env.example 为 .env 并填写 DATABASE_URL。" >&2
  exit 1
fi

exec go run ./cmd/server
