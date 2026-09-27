#!/usr/bin/env bash
# Phase 3 任务系统冒烟：真实入队 → worker 领取执行 → 轮询进度 → 取消/重试语义。
#
# 用法：bash scripts/smoke-phase3-tasks.sh   （需要后端在 127.0.0.1:8080 运行，worker 已启动）
set -uo pipefail

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
PSQL_URL="postgresql://novamind:novamind@127.0.0.1:5432/novamind"
WORK_DIR="$(mktemp -d)"

PASS=0
FAIL=0
check() {
  if [ "$2" = "$3" ]; then
    echo "  ✓ $1 ($3)"
    PASS=$((PASS + 1))
  else
    echo "  ✗ $1：期望 $2，实际 $3"
    FAIL=$((FAIL + 1))
  fi
}
getid() { python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["id"])'; }
field() { python3 -c "import sys,json;d=json.load(sys.stdin)['data'];print($1)"; }
psqlq() { psql "$PSQL_URL" -tAc "$1"; }

# 轮询任务直到终态（最多 40 秒）
wait_task() {
  local task_id="$1" want="$2"
  for _ in $(seq 1 40); do
    local status
    status=$(curl -sf "${API}/tasks/${task_id}" | field "d['status']")
    if [ "$status" = "$want" ]; then
      echo "$status"
      return 0
    fi
    if [ "$status" = "FAILED" ] || [ "$status" = "COMPLETED" ] || [ "$status" = "CANCELLED" ]; then
      echo "$status"
      return 0
    fi
    sleep 1
  done
  echo "TIMEOUT"
}

PROJECT_ID=""
WORK_ID=""
cleanup() {
  if [ -n "${WORK_ID}" ]; then
    psqlq "delete from tasks where work_id='${WORK_ID}' or project_id='${PROJECT_ID}'" >/dev/null 2>&1
    psqlq "delete from timeline_events where timeline_id in (select id from original_timelines where original_work_id='${WORK_ID}')" >/dev/null 2>&1
    psqlq "delete from original_timelines where original_work_id='${WORK_ID}'" >/dev/null 2>&1
    psqlq "delete from plot_arcs where original_work_id='${WORK_ID}'" >/dev/null 2>&1
    psqlq "delete from original_events where original_work_id='${WORK_ID}'" >/dev/null 2>&1
    psqlq "delete from character_relationships where original_work_id='${WORK_ID}'" >/dev/null 2>&1
    psqlq "delete from original_characters where original_work_id='${WORK_ID}'" >/dev/null 2>&1
    psqlq "delete from original_chapters where original_work_id='${WORK_ID}'" >/dev/null 2>&1
    psqlq "delete from world_rules where world_id in (select id from original_worlds where original_work_id='${WORK_ID}')" >/dev/null 2>&1
    psqlq "delete from locations where world_id in (select id from original_worlds where original_work_id='${WORK_ID}')" >/dev/null 2>&1
    psqlq "delete from factions where world_id in (select id from original_worlds where original_work_id='${WORK_ID}')" >/dev/null 2>&1
    psqlq "delete from original_worlds where original_work_id='${WORK_ID}'" >/dev/null 2>&1
    psqlq "delete from original_works where id='${WORK_ID}'" >/dev/null 2>&1
  fi
  [ -n "${PROJECT_ID}" ] && psqlq "delete from files where project_id='${PROJECT_ID}'" >/dev/null 2>&1
  [ -n "${PROJECT_ID}" ] && psqlq "delete from projects where id='${PROJECT_ID}'" >/dev/null 2>&1
  [ -n "${PROJECT_ID}" ] && rm -rf "${WORK_DIR}/uploads/${PROJECT_ID}" 2>/dev/null
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

echo "== 1. 任务类型清单"
TYPES=$(curl -sf "${API}/task-types")
check "含 original_reparse" "True" "$(echo "$TYPES" | python3 -c 'import sys,json;print("original_reparse" in json.load(sys.stdin)["data"]["items"])')"

echo "== 2. 准备一部可解析的原著"
PROJECT_ID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' \
  -d '{"name":"任务冒烟","type":"ORIGINAL"}' | getid)
WORK_ID=$(curl -sf -X POST "${API}/projects/${PROJECT_ID}/original" -H 'Content-Type: application/json' \
  -d '{"title":"任务冒烟原著"}' | getid)

python3 - "$WORK_DIR" <<'PY'
import sys
work = sys.argv[1]
parts = []
for i in range(1, 4):
    parts.append(f"第{i}章 测试")
    parts.append("")
    parts.extend(f"这是第{i}章第{j}句正文。" for j in range(1, 11))
    parts.append("")
open(f"{work}/sample.txt", "w", encoding="utf-8").write("\n".join(parts))
PY
IMPORT=$(curl -sf -X POST "${API}/original/${WORK_ID}/import" -F "file=@${WORK_DIR}/sample.txt")
CHAPTERS=$(echo "$IMPORT" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["chapter_count"])')
check "导入章节数" "3" "$CHAPTERS"

echo "== 3. 入队「重新解析」任务"
ACCEPTED=$(curl -s -w '\n%{http_code}' -X POST "${API}/original/${WORK_ID}/reparse")
HTTP_CODE=$(echo "$ACCEPTED" | tail -1)
TASK_JSON=$(echo "$ACCEPTED" | head -n -1)
check "返回 202（异步受理）" "202" "$HTTP_CODE"
TASK_ID=$(echo "$TASK_JSON" | getid)
check "任务初始状态" "PENDING" "$(echo "$TASK_JSON" | field "d['status']")"

echo "== 4. 等 worker 执行完成"
FINAL=$(wait_task "$TASK_ID" "COMPLETED")
check "任务最终状态" "COMPLETED" "$FINAL"
DETAIL=$(curl -sf "${API}/tasks/${TASK_ID}")
check "进度 100" "100" "$(echo "$DETAIL" | field "d['progress']")"
check "尝试次数 1" "1" "$(echo "$DETAIL" | field "d['attempts']")"
check "结果含章节数" "3" "$(echo "$DETAIL" | field "d['output']['chapter_count']")"
check "结果含编码" "UTF-8" "$(echo "$DETAIL" | field "d['output']['encoding']")"
check "有开始/结束时间" "True" "$(echo "$DETAIL" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(bool(d["started_at"]) and bool(d["finished_at"]))')"
check "章节数未变" "3" "$(curl -sf "${API}/original/${WORK_ID}/chapters" | field "d['total']")"

echo "== 5. 终态任务的取消/重试语义"
check "完成任务再取消（应 409）" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/tasks/${TASK_ID}/cancel")"
check "完成任务再重试（应 409）" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/tasks/${TASK_ID}/retry")"
check "不存在的任务（应 404）" "404" "$(curl -s -o /dev/null -w '%{http_code}' "${API}/tasks/00000000-0000-7000-8000-000000000000")"

echo "== 6. 失败与重试"
FAILTASK=$(curl -sf -X POST "${API}/tasks" -H 'Content-Type: application/json' \
  -d '{"type":"original_reparse","input":{"work_id":"00000000-0000-7000-8000-000000000000"},"max_attempts":1}' | getid)
FAILED_STATUS=$(wait_task "$FAILTASK" "FAILED")
check "非法原著的任务应失败" "FAILED" "$FAILED_STATUS"
check "失败原因已记录" "True" "$(curl -sf "${API}/tasks/${FAILTASK}" | python3 -c 'import sys,json;print(bool(json.load(sys.stdin)["data"]["error"]))')"
check "失败任务可重试" "PENDING" "$(curl -sf -X POST "${API}/tasks/${FAILTASK}/retry" | field "d['status']")"
wait_task "$FAILTASK" "FAILED" >/dev/null

echo "== 7. 未注册任务类型（应 400）"
check "未知类型被拒绝" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/tasks" -H 'Content-Type: application/json' -d '{"type":"no_such_task"}')"

echo "== 8. 任务列表过滤"
check "按 work_id 过滤命中" "True" "$(curl -sf "${API}/tasks?work_id=${WORK_ID}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["total"] >= 1)')"

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项"
[ "${FAIL}" -eq 0 ]
