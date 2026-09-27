#!/usr/bin/env bash
# 后台启动前后端（脱离终端会话，关掉 shell 也不会被杀）。
#
# 用法：bash scripts/dev-up.sh
# 停止：bash scripts/dev-down.sh
# 日志：.run/backend.log / .run/frontend.log
#
# 说明：直接用 `npm run dev &` 或 `go run ./cmd/server &` 在一次性会话里启动，
# 会话结束进程就会被回收——本脚本用 setsid 把进程放进独立会话规避这个问题。
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_DIR="${PROJECT_ROOT}/.run"
BACKEND_DIR="${PROJECT_ROOT}/backend"
FRONTEND_DIR="${PROJECT_ROOT}/frontend"
SERVER_BIN="${RUN_DIR}/novamind-server"

export PATH="${HOME}/.local/go/bin:${PATH}"
mkdir -p "${RUN_DIR}"

start_detached() {
  local name="$1" dir="$2"
  shift 2
  local pidfile="${RUN_DIR}/${name}.pid" logfile="${RUN_DIR}/${name}.log"

  if [ -f "${pidfile}" ] && kill -0 "$(cat "${pidfile}")" 2>/dev/null; then
    echo "[${name}] 已在运行 (pid $(cat "${pidfile}"))"
    return 0
  fi

  # 让子进程自己写 PID：bash 先把 $$ 落盘，再 exec 替换成目标进程，PID 全程不变。
  # 早前版本用 $! 记录 setsid 的 PID —— setsid 派生后这个值就失效了，
  # 结果是 dev-down 停不掉进程、新实例又抢不到端口。这是那次"服务没反应"的真因。
  ( cd "${dir}" && PID_FILE="${pidfile}" setsid nohup bash -c 'echo $$ > "$PID_FILE"; exec "$@"' bash "$@" >"${logfile}" 2>&1 </dev/null ) &

  local waited=0
  while [ ! -s "${pidfile}" ] && [ "${waited}" -lt 25 ]; do
    sleep 0.2
    waited=$((waited + 1))
  done

  local pid=""
  [ -s "${pidfile}" ] && pid="$(cat "${pidfile}")"
  if kill -0 "${pid}" 2>/dev/null; then
    echo "[${name}] 已启动 (pid ${pid})  日志: ${logfile}"
  else
    echo "[${name}] 启动失败，见 ${logfile}" >&2
    tail -20 "${logfile}" >&2 || true
    return 1
  fi
}

echo "==> 编译后端"
( cd "${BACKEND_DIR}" && go build -o "${SERVER_BIN}" ./cmd/server )

echo "==> 启动服务"
start_detached backend "${BACKEND_DIR}" "${SERVER_BIN}"
start_detached frontend "${FRONTEND_DIR}" npm run dev

echo
echo "==> 就绪检查"
for i in $(seq 1 15); do
  if curl -sf -o /dev/null --max-time 3 http://127.0.0.1:8080/api/v1/health \
    && curl -sf -o /dev/null --max-time 3 http://127.0.0.1:5173/; then
    echo "后端  http://127.0.0.1:8080  OK"
    echo "前端  http://127.0.0.1:5173  OK"
    exit 0
  fi
  sleep 1
done

echo "服务未在 15 秒内就绪，请检查 ${RUN_DIR}/*.log" >&2
exit 1
