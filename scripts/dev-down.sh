#!/usr/bin/env bash
# 停止 scripts/dev-up.sh 启动的前后端进程。
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_DIR="${PROJECT_ROOT}/.run"

for name in frontend backend; do
  pidfile="${RUN_DIR}/${name}.pid"
  if [ ! -f "${pidfile}" ]; then
    echo "[${name}] 没有 PID 文件，跳过"
    continue
  fi
  pid="$(cat "${pidfile}")"
  if kill -0 "${pid}" 2>/dev/null; then
    # setsid 启动的进程是会话/进程组组长，连整组一起停（含子进程）
    kill -TERM -"${pid}" 2>/dev/null || kill -TERM "${pid}" 2>/dev/null || true
    sleep 1
    kill -KILL -"${pid}" 2>/dev/null || true
    echo "[${name}] 已停止 (pid ${pid})"
  else
    echo "[${name}] 进程已不存在 (pid ${pid})"
  fi
  rm -f "${pidfile}"
done
