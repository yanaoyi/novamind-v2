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
    # setsid 启动的进程是会话/进程组组长，连整组一起停（含 vite/node 子进程）
    kill -TERM -"${pid}" 2>/dev/null || kill -TERM "${pid}" 2>/dev/null || true
    for _ in $(seq 1 20); do
      kill -0 "${pid}" 2>/dev/null || break
      sleep 0.25
    done
    if kill -0 "${pid}" 2>/dev/null; then
      kill -KILL -"${pid}" 2>/dev/null || kill -KILL "${pid}" 2>/dev/null || true
    fi
    echo "[${name}] 已停止 (pid ${pid})"
  else
    echo "[${name}] 进程已不存在 (pid ${pid})"
  fi
  rm -f "${pidfile}"
done

# 兜底：PID 文件丢了或进程被重新挂载过时，按端口找"属于本项目"的孤儿进程清掉。
# 只杀 cmdline 里出现本项目路径的进程，避免误伤 v1 或其他服务。
cleanup_orphan_on_port() {
  local port="$1"
  local pids
  pids="$(ss -tlnp 2>/dev/null | grep ":${port} " | grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u)"
  for pid in ${pids}; do
    local cmdline=""
    [ -r "/proc/${pid}/cmdline" ] && cmdline="$(tr '\0' ' ' <"/proc/${pid}/cmdline")"
    case "${cmdline}" in
      *"${PROJECT_ROOT}"*)
        echo "[端口 ${port}] 发现本项目孤儿进程 pid=${pid}，清理中"
        kill -TERM "${pid}" 2>/dev/null || true
        sleep 1
        kill -KILL "${pid}" 2>/dev/null || true
        ;;
      *)
        echo "提示：端口 ${port} 被非本项目进程占用（pid=${pid}），未处理" >&2
        ;;
    esac
  done
}

for port in 8080 5173; do
  if ss -tln 2>/dev/null | grep -q ":${port} "; then
    cleanup_orphan_on_port "${port}"
  fi
done

# 最终校验
for port in 8080 5173; do
  if ss -tln 2>/dev/null | grep -q ":${port} "; then
    echo "警告：端口 ${port} 仍在监听" >&2
  fi
done
