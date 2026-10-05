#!/usr/bin/env bash
# Phase 9 §9.2 验收：上下文快照（Context Engine 接线）。
#
# 链路：原著导入 → 索引 → 二创建作品/卷/章 → 作者写第 1 章 → 索引二创
#      → AI 写第 2 章 → 查第 2 章快照，断言"当时给了 AI 什么"可追溯：
#      真实模型 / 真实模板版本 / 8 段上下文 / 检索来源（chunk id + 分数 + 作品归属）/ token 预算。
#
# 为什么必须真库真模型：快照写入失败只记日志、不打断写作（刻意的），
# 所以"payload 组装正确"的单测不能替代"这一行真的落到库里"的验收。
#
# 用法：bash scripts/smoke-phase9.sh   （需要后端在 127.0.0.1:8080 运行 + 已挂验证账号）
#
# 说明（§9.3 待接）：事实抽取/回写 memory_facts 属于第 4 步，本期未实现；
# 本脚本覆盖它之前的三步（索引构建 → 检索召回 → 快照落库），后续按 §9.6 扩写。
set -uo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/db-url.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/cleanup.sh"

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
WORK_DIR="$(mktemp -d)"
PASS=0
FAIL=0

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
# 排障时完全看不出发生了什么（本脚本第一版就踩了这个坑）。
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

# mk：POST 建对象并取字段；失败即终止（后续步骤都依赖这些 ID，硬撑只会产生连环假失败）。
mk() { # mk PATH BODY FIELD_EXPR
  local out
  if ! out="$(post_json "$1" "$2")"; then
    echo "== 中断（前置调用 ${1} 失败，已清理）" >&2
    exit 1
  fi
  echo "${out}" | field "$3"
}

wait_task() { # wait_task <task_id> [max_seconds]
  local id="$1" limit="${2:-180}" status="" i=0
  while [ "${i}" -lt "${limit}" ]; do
    status="$(curl -sf "${API}/tasks/${id}" | field "d['status']" 2>/dev/null || echo '')"
    case "${status}" in COMPLETED|FAILED|CANCELLED) echo "${status}"; return ;; esac
    sleep 1
    i=$((i + 1))
  done
  echo "${status:-TIMEOUT}"
}

PID=""; OID=""; CPID=""; CID=""
cleanup() {
  # 工程级清理（含 chunks / tasks / 原著与二创从属表）—— 以前直接删 projects 会被外键拒绝
  cleanup_project "${PID:-}"
  cleanup_project "${CPID:-}"
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

echo "== 1. 造原著并导入（第 1 章埋伏笔：青铜钥匙 / 左肩箭伤）"
python3 - "$WORK_DIR/book.txt" <<'PY'
import sys
plant = {
    1: "沈砚在祖宅第三块砖下埋下青铜钥匙，钥匙上刻着一只断尾的鹤；他左肩的箭伤也是那年落下的。",
    2: "账房先生提起，断尾鹤是沈家旧部的暗记。",
    3: "督军府设宴，满座盐商都盯着沈砚左肩那道旧伤。",
}
with open(sys.argv[1], "w", encoding="utf-8") as f:
    for ch in range(1, 9):
        f.write(f"第{ch}章 起\n\n")
        f.write(f"梅雨下了整夜。沈砚与账房先生核对第{ch}笔账目，老城区的雨一整天没停。\n\n")
        if ch in plant:
            f.write(plant[ch] + "\n\n")
PY

PID=$(mk /projects '{"name":"Phase9上下文验收-原著","type":"ORIGINAL"}' "d['id']")
OID=$(mk "/projects/${PID}/original" '{"title":"Phase9上下文验收原著"}' "d['id']")
if ! curl -s -o "${WORK_DIR}/import.json" -w '%{http_code}' -X POST "${API}/original/${OID}/import" \
  -F "file=@${WORK_DIR}/book.txt" | grep -q '^2'; then
  echo "  ✗ 原著导入失败：$(head -c 300 "${WORK_DIR}/import.json")" >&2
  exit 1
fi
CH=$(curl -sf "${API}/original/${OID}/chapters?page=1&page_size=1" | field "d['total']")
check "原著切分 8 章" "8" "${CH}"

echo "== 2. 索引原著 + 检索召回（§9.1）"
TASK=$(mk /tasks "{\"type\":\"index_chunks\",\"work_id\":\"${OID}\",\"input\":{\"work_kind\":\"original\",\"work_id\":\"${OID}\"}}" "d['id']")
check "原著索引任务完成" "COMPLETED" "$(wait_task "${TASK}")"

RESP=$(curl -sf -X POST "${API}/retrieval/search" -H 'Content-Type: application/json' \
  -d "{\"work_kind\":\"original\",\"work_id\":\"${OID}\",\"query\":\"青铜钥匙 断尾鹤\",\"top_k\":8}")
HIT=$(echo "${RESP}" | field "d['items'][0]['content'] if d['items'] else ''")
check "原著检索能召回第 1 章伏笔" "True" "$([[ "${HIT}" == *青铜钥匙* ]] && echo True || echo False)"

echo "== 3. 建二创作品：作者先写第 1 章，再索引"
CPID=$(mk /projects '{"name":"Phase9上下文验收-二创","type":"CREATIVE"}' "d['id']")
CID=$(mk "/original/${OID}/create-creative" "{\"project_id\":\"${CPID}\",\"title\":\"Phase9上下文验收同人\",\"description\":\"验收用\"}" "d['id']")
VOL=$(mk "/creative/${CID}/volumes" '{"title":"第一卷","summary":"开局","sequence":1}' "d['id']")
CH1=$(mk "/creative/${CID}/chapters" \
  "{\"volume_id\":\"${VOL}\",\"chapter_no\":1,\"title\":\"雨夜归人\",\"summary\":\"沈砚回到老宅\",\"content\":\"沈砚推开老宅的木门。他左肩的箭伤在雨里隐隐作痛，怀里揣着从祖宅第三块砖下挖出的青铜钥匙。\",\"purpose\":\"交代沈砚与青铜钥匙\",\"conflict\":\"旧伤与旧案\",\"outcome\":\"钥匙到手\"}" \
  "d['id']")
CH2=$(mk "/creative/${CID}/chapters" \
  "{\"volume_id\":\"${VOL}\",\"chapter_no\":2,\"title\":\"账房里的灯\",\"summary\":\"他拿着钥匙去开夹墙\",\"purpose\":\"让沈砚用青铜钥匙打开夹墙，发现三十年前的军需账册\",\"conflict\":\"督军的人开始盯梢\",\"outcome\":\"夹墙打开\"}" \
  "d['id']")

# 注意：二创作品在 tasks 表里要走 creative_work_id —— tasks.work_id 的外键指向 original_works，
# 把二创 id 塞进 work_id 会被外键拒绝（第一版脚本就是在这里静默失败的）。
TASK=$(mk /tasks "{\"type\":\"index_chunks\",\"creative_work_id\":\"${CID}\",\"input\":{\"work_kind\":\"creative\",\"work_id\":\"${CID}\"}}" "d['id']")
check "二创索引任务完成" "COMPLETED" "$(wait_task "${TASK}")"
CCHUNKS=$(psqlq "select count(*) from chunks where work_id='${CID}'")
check "二创索引非空" "True" "$([ "${CCHUNKS:-0}" -gt 0 ] && echo True || echo False)"

echo "== 4. AI 写第 2 章 → 落上下文快照（§9.2）"
TASK=$(mk "/chapters/${CH2}/generate" '{"target_words":300,"instruction":"克制的短句，呼应第 1 章的伤口与钥匙"}' "d['id']")
check "写本章任务完成" "COMPLETED" "$(wait_task "${TASK}")"

SNAPS=$(curl -sf "${API}/chapters/${CH2}/snapshots")
KIND=$(echo "${SNAPS}" | field "d['items'][0]['kind'] if d['items'] else ''")
check "快照 kind=generate" "generate" "${KIND}"
SNAPS_ID=$(echo "${SNAPS}" | field "d['items'][0]['id'] if d['items'] else ''")
DETAIL=$(curl -sf "${API}/snapshots/${SNAPS_ID}")

MODEL=$(echo "${DETAIL}" | field "d['snapshot'].get('model','')")
VERSION=$(echo "${DETAIL}" | field "d['snapshot'].get('prompt_version','')")
check "快照记真实模型（非占位）" "True" "$([[ -n "${MODEL}" && "${MODEL}" != "default" ]] && echo True || echo False)"
check "快照记真实模板版本（chapter_generate）" "True" "$([[ "${VERSION}" == chapter_generate.* ]] && echo True || echo False)"

SRC_COUNT=$(echo "${DETAIL}" | field "len(d['snapshot'].get('retrieved_sources',[]))")
check "快照含检索来源（§9.2 要求 id/score）" "True" "$([ "${SRC_COUNT:-0}" -gt 0 ] && echo True || echo False)"
SRC_OK=$(echo "${DETAIL}" | field "all(('chunk_id' in s and 'score' in s and 'work_kind' in s) for s in d['snapshot'].get('retrieved_sources',[])) and len(d['snapshot'].get('retrieved_sources',[]))>0")
check "每条来源都带 chunk_id / score / 作品归属" "True" "${SRC_OK}"
HAS_CREATIVE=$(echo "${DETAIL}" | field "any(s['work_kind']=='creative' for s in d['snapshot'].get('retrieved_sources',[]))")
check "来源含二创作品（第 1 章伏笔）" "True" "${HAS_CREATIVE}"

RECALL=$(echo "${DETAIL}" | field "d['snapshot'].get('retrieved_creative','')")
check "第 2 章上下文真的带回了第 1 章的青铜钥匙" "True" "$([[ "${RECALL}" == *青铜钥匙* ]] && echo True || echo False)"

LIMIT=$(echo "${DETAIL}" | field "d['snapshot'].get('token_budget',{}).get('limit',0)")
USED=$(echo "${DETAIL}" | field "d['snapshot'].get('token_budget',{}).get('used',0)")
BY_SEC=$(echo "${DETAIL}" | field "len(d['snapshot'].get('token_budget',{}).get('by_section',{}))")
check "快照记 token 预算上限" "8000" "${LIMIT}"
check "快照记用掉的 token（>0）" "True" "$([ "${USED:-0}" -gt 0 ] && echo True || echo False)"
check "快照记每段用量（8 段）" "8" "${BY_SEC}"

SEC=$(echo "${DETAIL}" | field "'character_context' in d['snapshot'] and 'world_context' in d['snapshot'] and 'timeline_context' in d['snapshot'] and 'prev_summary' in d['snapshot'] and 'retrieved_original' in d['snapshot'] and 'author_instruction' in d['snapshot'] and 'outline_context' in d['snapshot']")
check "8 段上下文齐备（§9.2.1）" "True" "${SEC}"

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项"
[ "${FAIL}" -eq 0 ]
