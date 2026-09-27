#!/usr/bin/env bash
# 启动前端开发服务器（vite，默认 127.0.0.1:5173，/api 代理到后端 8080）
#
# 用法：bash scripts/dev-frontend.sh
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${PROJECT_ROOT}/frontend"

if [ ! -d node_modules ]; then
  echo "首次运行，安装依赖…"
  npm install --no-audit --no-fund
fi

exec npm run dev
