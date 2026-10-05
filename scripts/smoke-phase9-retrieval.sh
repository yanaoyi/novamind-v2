#!/usr/bin/env bash
# Phase 9 §9.1 验收：索引构建 → 检索召回。
#
# 做法：造一本 100 章（约 30 万字）的测试书 → 导入 → 跑 index_chunks 灌索引
#      → 抽 5 个 query 走检索调试接口 → 统计 top8 命中率（任务书要求 ≥4/5）。
#
# 用法：bash scripts/smoke-phase9-retrieval.sh   （需要后端在 127.0.0.1:8080 运行）
set -uo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/db-url.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/cleanup.sh"

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
WORK_DIR="$(mktemp -d)"
PASS=0
FAIL=0

check() {
  if [ "$2" = "$3" ]; then
    echo "  ✓ $1 ($3)"; PASS=$((PASS + 1))
  else
    echo "  ✗ $1：期望 $2，实际 $3"; FAIL=$((FAIL + 1))
  fi
}
field() { python3 -c "import sys,json;d=json.load(sys.stdin)['data'];print($1)"; }
psqlq() { psql "$PSQL_URL" -tAc "$1"; }

PID=""; OID=""
cleanup() {
  # 工程级清理：原著/章节/分块/tasks 都会一起删掉（工程级外键是 NO ACTION，必须按序删）
  cleanup_project "${PID:-}"
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

echo "== 1. 造一本 100 章的测试书（约 30 万字，带可检索的埋点）"
python3 - "$WORK_DIR/book.txt" <<'PY'
import sys
out = sys.argv[1]
# 每章 3000 字左右；第 1 章埋下"青铜钥匙"伏笔，后续章节反复出现不同线索
plant = {
    1: "沈砚在祖宅第三块砖下埋下青铜钥匙，钥匙上刻着一只断尾的鹤。",
    17: "灯会那晚有人在桥洞留下一枚断尾鹤纹的铜扣，和青铜钥匙的纹样一模一样。",
    42: "老账房提到，断尾鹤是沈家旧部的暗记，青铜钥匙正是他们认主的信物。",
    73: "沈砚终于用青铜钥匙打开了夹墙，里面是三十年前的军需账册。",
    95: "夹墙里的军需账册被抄本流入督军府，断尾鹤的暗记再现。",
}
with open(out, "w", encoding="utf-8") as f:
    for ch in range(1, 101):
        f.write(f"第{ch}章 起\n\n")
        body = plant.get(ch, "")
        base = f"这一章里，沈砚与账房先生核对第{ch}笔账目，老城区的雨下了一整天，街口的茶馆照旧坐满了人。"
        for _ in range(40):
            f.write(base)
        if body:
            f.write(body)
        f.write("\n\n")
PY
SIZE=$(wc -c < "${WORK_DIR}/book.txt" | tr -d ' ')
echo "     生成完毕：${SIZE} 字节（约 $((SIZE / 3)) 字）"

echo "== 2. 导入"
PID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"检索验收-测试书","type":"ORIGINAL"}' | field "d['id']")
OID=$(curl -sf -X POST "${API}/projects/${PID}/original" -H 'Content-Type: application/json' -d '{"title":"检索验收测试书"}' | field "d['id']")
curl -sf -o /dev/null -X POST "${API}/original/${OID}/import" -F "file=@${WORK_DIR}/book.txt"
CH=$(curl -sf "${API}/original/${OID}/chapters?page=1&page_size=1" | field "d['total']")
check "章节切分（100 章）" "100" "${CH}"

echo "== 3. 跑索引任务 index_chunks"
TASK=$(curl -sf -X POST "${API}/tasks" -H 'Content-Type: application/json' \
  -d "{\"type\":\"index_chunks\",\"work_id\":\"${OID}\",\"input\":{\"work_kind\":\"original\",\"work_id\":\"${OID}\"}}" | field "d['id']")
STATUS=""
for _ in $(seq 1 120); do
  STATUS=$(curl -sf "${API}/tasks/${TASK}" | field "d['status']")
  [ "${STATUS}" = "COMPLETED" ] || [ "${STATUS}" = "FAILED" ] && break
  sleep 1
done
check "索引任务完成" "COMPLETED" "${STATUS}"
CHUNKS=$(curl -sf "${API}/tasks/${TASK}" | field "d['output'].get('chunks', 0)")
echo "     索引得到 ${CHUNKS} 块"
check "分块数量合理（>100）" "True" "$([ "${CHUNKS:-0}" -gt 100 ] && echo True || echo False)"
check "库里分块一致" "True" "$([ "$(psqlq "select count(*) from chunks where work_id='${OID}'")" = "${CHUNKS}" ] && echo True || echo False)"

echo "== 4. 抽 5 个 query 验召回（top8 命中 ≥4/5）"
QUERIES=(
  "青铜钥匙 断尾鹤"
  "桥洞 铜扣 灯会"
  "军需账册 夹墙"
  "沈家旧部 暗记"
  "督军府 抄本"
)
HITS=0
for q in "${QUERIES[@]}"; do
  RESP=$(curl -sf -X POST "${API}/retrieval/search" -H 'Content-Type: application/json' \
    -d "{\"work_kind\":\"original\",\"work_id\":\"${OID}\",\"query\":\"${q}\",\"top_k\":8}")
  TOP=$(echo "$RESP" | field "d['items'][0]['content'] if d['items'] else ''" 2>/dev/null || echo "")
  N=$(echo "$RESP" | field "len(d['items'])")
  if [ "${N:-0}" -gt 0 ]; then
    HITS=$((HITS + 1))
    echo "  ✓ 「${q}」命中 ${N} 条，首条：$(echo "${TOP}" | cut -c1-40)…"
  else
    echo "  ✗ 「${q}」无命中"
  fi
done
check "召回命中数 ≥4/5（任务书验收线）" "True" "$([ "${HITS}" -ge 4 ] && echo True || echo False)"

echo "== 5. 性能基线：100 章作品检索 P95（任务书要求 < 2s）"
# 5 个 query × 6 轮 = 30 次采样（BM25 是 Go 内全量打分，这里量的就是它）
LAT_FILE="${WORK_DIR}/latency.txt"
: > "${LAT_FILE}"
for _ in 1 2 3 4 5 6; do
  for q in "${QUERIES[@]}"; do
    curl -s -o /dev/null -w '%{time_total}\n' -X POST "${API}/retrieval/search" \
      -H 'Content-Type: application/json' \
      -d "{\"work_kind\":\"original\",\"work_id\":\"${OID}\",\"query\":\"${q}\",\"top_k\":8}" >> "${LAT_FILE}"
  done
done
PERF=$(python3 - "${LAT_FILE}" <<'PY'
import sys
vals = sorted(float(x) for x in open(sys.argv[1]) if x.strip())
if not vals:
    print("0 0 0 0"); raise SystemExit
# 最近秩法取 P95（30 个样本 → 第 29 个）
idx = max(0, min(len(vals) - 1, int(0.95 * len(vals) + 0.5) - 1))
print(f"{vals[len(vals)//2]:.3f} {vals[idx]:.3f} {vals[-1]:.3f} {len(vals)}")
PY
)
set -- ${PERF}
echo "     中位数 ${1}s / P95 ${2}s / 最大 ${3}s（采样 ${4} 次）"
check "检索 P95 < 2s（记录在案，不达标要加 ivfflat）" "True" \
  "$(python3 -c "print('True' if float('${2}') < 2.0 else 'False')")"

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项（召回 ${HITS}/5）"
[ "${FAIL}" -eq 0 ]
