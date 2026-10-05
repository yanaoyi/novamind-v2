#!/usr/bin/env bash
# 验收/冒烟脚本共用的辅助函数（Phase 9 起多个脚本共用同一套断言与等待逻辑）。
#
# 为什么抽出来：先前每个脚本各写一份 check / post_json / 等待任务，
# 一处改了口径（例如"非 2xx 必须打印响应正文"）另一处不会跟着改 ——
# 而这类差异恰恰会让人误判"功能坏了"。现在只有一份实现。
#
# 使用前调用方需设置：API（接口前缀）、PSQL_URL（由 lib/db-url.sh 导出）。
# 失败计数用调用方的 PASS / FAIL 变量（由 check 递增，需先初始化为 0）。

check() { # check 描述 期望 实际
  if [ "$2" = "$3" ]; then
    echo "  ✓ $1 ($3)"; PASS=$((PASS + 1))
  else
    echo "  ✗ $1：期望 $2，实际 $3"; FAIL=$((FAIL + 1))
  fi
}

field() { python3 -c "import sys,json;d=json.load(sys.stdin)['data'];print($1)"; }

psqlq() { psql "$PSQL_URL" -tAc "$1"; }

# post_json：POST 并原样吐出响应体；非 2xx 时打印 HTTP 状态与响应正文再返回非零。
# 为什么不用 curl -sf：它会把错误响应悄悄吞掉，脚本只会表现成"某个字段是空的"，
# 排障时完全看不出发生了什么（早期脚本就踩过这个坑）。
post_json() {
  local path="$1" body="$2" tmp http
  tmp="$(mktemp)"
  http="$(curl -s -o "${tmp}" -w '%{http_code}' -X POST "${API}${path}" \
    -H 'Content-Type: application/json' -d "${body}")"
  if [[ ! "${http}" =~ ^2 ]]; then
    echo "  ✗ POST ${path} → HTTP ${http}：$(head -c 400 "${tmp}")" >&2
    rm -f "${tmp}"
    return 1
  fi
  cat "${tmp}"
  rm -f "${tmp}"
}

# mkv：POST 建对象并把某个字段写进变量；失败即终止。
#
# 为什么是"写进变量"而不是 `X=$(mk ...)`：命令替换里的函数跑在**子 shell**，
# 里面的 exit 只结束子 shell，主脚本会带着空 ID 继续往下跑，
# 表现成"后面一连串莫名其妙的失败"。printf -v 直接赋值才能真的中断。
mkv() { # mkv VAR_NAME LABEL PATH BODY FIELD_EXPR
  local __var="$1" __label="$2" out
  if ! out="$(post_json "$3" "$4")"; then
    echo "== 中断（${__label} 失败，已清理）" >&2
    exit 1
  fi
  printf -v "${__var}" '%s' "$(echo "${out}" | field "$5")"
}

# wait_auto_task：等"系统自动触发"的某类任务完成（脚本不手工入队 —— 那正是要验的东西）。
# 入参：过滤参数名（work_id / creative_work_id）、值、任务类型。
wait_auto_task() {
  local param="$1" id="$2" kind="$3" limit="${4:-180}" status="" i=0
  while [ "${i}" -lt "${limit}" ]; do
    status="$(curl -s "${API}/tasks?${param}=${id}&type=${kind}&page_size=1" \
      | field "d['items'][0]['status'] if d['items'] else ''" 2>/dev/null || echo '')"
    case "${status}" in COMPLETED|FAILED|CANCELLED) echo "${status}"; return ;; esac
    sleep 1
    i=$((i + 1))
  done
  echo "${status:-NO_TASK}"
}

wait_index() { wait_auto_task "$1" "$2" "index_chunks" "${3:-180}"; }

# wait_task：等某个具体任务结束（调用方已拿到 task_id）。
wait_task() {
  local id="$1" limit="${2:-180}" status="" i=0
  while [ "${i}" -lt "${limit}" ]; do
    status="$(curl -s "${API}/tasks/${id}" | field "d['status']" 2>/dev/null || echo '')"
    case "${status}" in COMPLETED|FAILED|CANCELLED) echo "${status}"; return ;; esac
    sleep 1
    i=$((i + 1))
  done
  echo "${status:-TIMEOUT}"
}

# wait_sql：轮询一个 SQL 标量直到等于期望值。
# 用于"等异步链路把数据写进库"，比等某个任务更稳（多个任务时不知道该等哪一个）。
wait_sql() {
  local sql="$1" want="$2" limit="${3:-180}" got="" i=0
  while [ "${i}" -lt "${limit}" ]; do
    got="$(psqlq "${sql}" 2>/dev/null || echo '')"
    [ "${got}" = "${want}" ] && { echo "${got}"; return; }
    sleep 1
    i=$((i + 1))
  done
  echo "${got:-TIMEOUT}"
}

# wait_sql_ge：轮询直到 SQL 标量 >= 期望值（"至少出现 N 条"这类异步断言更实用）。
wait_sql_ge() {
  local sql="$1" min="$2" limit="${3:-180}" got="" i=0
  while [ "${i}" -lt "${limit}" ]; do
    got="$(psqlq "${sql}" 2>/dev/null || echo '')"
    if [ -n "${got}" ] && [ "${got}" -ge "${min}" ] 2>/dev/null; then
      echo "${got}"
      return
    fi
    sleep 1
    i=$((i + 1))
  done
  echo "${got:-0}"
}
