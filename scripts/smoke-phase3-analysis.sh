#!/usr/bin/env bash
# Phase 3 分析闭环冒烟：
#   假模型上游 → 触发人物提取 → AI 产出进"待审核提案" → 作者通过（带修改）/驳回 → 只有通过的才写进原著。
#
# 这是规格书 §52 那条红线的端到端验证：**AI 不能直接改原著模型**。
#
# 用法：bash scripts/smoke-phase3-analysis.sh   （需要后端在 127.0.0.1:8080 运行）
set -uo pipefail

# 接口访问令牌：curl 通过 $CURL_HOME/.curlrc 自动带上 Authorization 头（P0 安全修复配套）
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
FAKE_PORT=19556
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

wait_task() {
  local task_id="$1" want="$2"
  for _ in $(seq 1 40); do
    local status
    status=$(curl -sf "${API}/tasks/${task_id}" | field "d['status']")
    if [ "$status" = "$want" ] || [ "$status" = "FAILED" ] || [ "$status" = "COMPLETED" ]; then
      echo "$status"
      return 0
    fi
    sleep 1
  done
  echo "TIMEOUT"
}

# ---------- 假模型：固定返回两个人物 ----------
cat > "${WORK_DIR}/fake_model.py" <<'PY'
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

CHARACTERS = {
    "characters": [
        {
            "name": "林默", "aliases": ["小默"], "role": "主角", "gender": "女", "age": "26",
            "personality": "外冷内热，习惯把话说一半", "motivation": "查清母亲当年的意外",
            "values": "把承诺看得比利益重", "speech_style": "短句，偶尔带旧城口音",
            "importance": 5,
            "dna": {
                "personality": {"text": "克制、锋利", "weight": 95},
                "values": {"text": "重承诺", "weight": 85},
                "motivation": {"text": "查清真相", "weight": 90}
            },
            "evidence": "林默站在月台上，风把她的头发吹得凌乱。"
        },
        {
            "name": "陈述", "aliases": [], "role": "男主", "gender": "男", "age": "28",
            "personality": "表面圆滑，底线很硬", "importance": 4,
            "dna": {"personality": {"text": "善于周旋", "weight": 80}},
            "evidence": "陈述笑了一下，什么都没说。"
        }
    ]
}

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get('Content-Length') or 0)
        self.rfile.read(length)
        content = json.dumps(CHARACTERS, ensure_ascii=False)
        payload = {
            "model": "fake-analysis-model",
            "choices": [{"message": {"content": content}, "finish_reason": "stop"}],
            "usage": {"prompt_tokens": 100, "completion_tokens": 200, "total_tokens": 300},
        }
        data = json.dumps(payload).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args):
        pass

HTTPServer(("127.0.0.1", int(sys.argv[1])), Handler).serve_forever()
PY

python3 "${WORK_DIR}/fake_model.py" "${FAKE_PORT}" &
FAKE_PID=$!

PROJECT_ID=""
WORK_ID=""
PROVIDER_ID=""
cleanup() {
  [ -n "${WORK_ID}" ] && psqlq "delete from analysis_proposals where work_id='${WORK_ID}'" >/dev/null 2>&1
  [ -n "${WORK_ID}" ] && psqlq "delete from tasks where work_id='${WORK_ID}'" >/dev/null 2>&1
  [ -n "${WORK_ID}" ] && psqlq "delete from character_relationships where original_work_id='${WORK_ID}'" >/dev/null 2>&1
  [ -n "${WORK_ID}" ] && psqlq "delete from original_characters where original_work_id='${WORK_ID}'" >/dev/null 2>&1
  [ -n "${WORK_ID}" ] && psqlq "delete from original_chapters where original_work_id='${WORK_ID}'" >/dev/null 2>&1
  [ -n "${WORK_ID}" ] && psqlq "delete from original_timelines where original_work_id='${WORK_ID}'" >/dev/null 2>&1
  [ -n "${WORK_ID}" ] && psqlq "delete from original_works where id='${WORK_ID}'" >/dev/null 2>&1
  [ -n "${PROJECT_ID}" ] && psqlq "delete from files where project_id='${PROJECT_ID}'" >/dev/null 2>&1
  [ -n "${PROJECT_ID}" ] && psqlq "delete from projects where id='${PROJECT_ID}'" >/dev/null 2>&1
  [ -n "${PROVIDER_ID}" ] && psqlq "delete from model_providers where id='${PROVIDER_ID}'" >/dev/null 2>&1
  kill "${FAKE_PID}" 2>/dev/null || true
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

sleep 1

echo "== 1. 配置模型（指向假上游）"
PROVIDER_ID=$(curl -sf -X POST "${API}/model-providers" -H 'Content-Type: application/json' -d "{
  \"name\":\"冒烟-分析用假模型\",\"provider\":\"OPENAI_COMPATIBLE\",
  \"api_base\":\"http://127.0.0.1:${FAKE_PORT}\",\"api_key\":\"sk-fake\",
  \"model_name\":\"fake-analysis-model\",\"purpose\":\"chat\",\"temperature\":0.2,
  \"max_tokens\":2048,\"timeout_sec\":15,\"enabled\":true,\"is_default\":true}" | getid)
check "模型配置创建" "36" "${#PROVIDER_ID}"

echo "== 2. 准备原著（2 章）"
PROJECT_ID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' \
  -d '{"name":"分析冒烟","type":"ORIGINAL"}' | getid)
WORK_ID=$(curl -sf -X POST "${API}/projects/${PROJECT_ID}/original" -H 'Content-Type: application/json' \
  -d '{"title":"分析冒烟原著"}' | getid)
python3 - "$WORK_DIR" <<'PY'
import sys
work = sys.argv[1]
parts = []
for i in (1, 2):
    parts.append(f"第{i}章 测试")
    parts.append("")
    parts.append(f"这是第{i}章的正文，林默站在月台上，风把她的头发吹得凌乱。")
    parts.append("陈述笑了一下，什么都没说。")
    parts.append("")
open(f"{work}/sample.txt", "w", encoding="utf-8").write("\n".join(parts))
PY
curl -sf -o /dev/null -X POST "${API}/original/${WORK_ID}/import" -F "file=@${WORK_DIR}/sample.txt"
check "章节已导入" "2" "$(curl -sf "${API}/original/${WORK_ID}/chapters" | field "d['total']")"

echo "== 3. 触发人物提取（异步）"
HTTP=$(curl -s -o "${WORK_DIR}/task.json" -w '%{http_code}' -X POST "${API}/original/${WORK_ID}/analysis" \
  -H 'Content-Type: application/json' -d '{"stage":"character_extract"}')
check "返回 202" "202" "$HTTP"
TASK_ID=$(python3 -c 'import json;print(json.load(open("'"${WORK_DIR}"'/task.json"))["data"]["id"])')
check "分析任务完成" "COMPLETED" "$(wait_task "$TASK_ID" "COMPLETED")"
check "产出 2 条提案" "2" "$(curl -sf "${API}/tasks/${TASK_ID}" | field "d['output']['proposals_created']")"

echo "== 4. AI 结果进「待审核」，没有直接写原著"
check "原著人物数仍为 0" "0" "$(curl -sf "${API}/original/${WORK_ID}/characters" | field "d['total']")"
PENDING=$(curl -sf "${API}/original/${WORK_ID}/proposals?status=PENDING")
check "待审核提案 2 条" "2" "$(echo "$PENDING" | field "d['total']")"
check "实体类型为 character" "character" "$(echo "$PENDING" | field "d['items'][0]['entity_type']")"
check "带原文依据" "True" "$(echo "$PENDING" | field "bool(d['items'][0]['evidence'])")"
check "带人物 DNA 权重" "95" "$(echo "$PENDING" | field "max((v['weight'] for v in d['items'][0]['payload']['dna'].values()), default=0)")"

APPROVE_ID=$(echo "$PENDING" | field "[i['id'] for i in d['items'] if i['title']=='林默'][0]")
REJECT_ID=$(echo "$PENDING" | field "[i['id'] for i in d['items'] if i['title']=='陈述'][0]")

echo "== 5. 作者修改后通过（林默→林默（已校对））"
APPROVED=$(curl -sf -X POST "${API}/proposals/${APPROVE_ID}/approve" -H 'Content-Type: application/json' \
  -d '{"payload":{"name":"林默（已校对）","role":"主角","importance":5,"dna":{"personality":{"text":"克制、锋利","weight":95}}},"note":"改了个名字试试"}')
check "提案状态" "APPROVED" "$(echo "$APPROVED" | field "d['status']")"
check "已写入正式表（applied_id 非空）" "True" "$(echo "$APPROVED" | field "bool(d['applied_id'])")"
CHARS=$(curl -sf "${API}/original/${WORK_ID}/characters")
check "原著人物数 1" "1" "$(echo "$CHARS" | field "d['total']")"
check "名字是作者改过的" "林默（已校对）" "$(echo "$CHARS" | field "d['items'][0]['name']")"
check "来源标记为 AI" "AI" "$(echo "$CHARS" | field "d['items'][0]['source']")"
check "DNA 权重保留" "95" "$(echo "$CHARS" | field "d['items'][0]['dna']['personality']['weight']")"

echo "== 6. 驳回另一条"
check "驳回状态" "REJECTED" "$(curl -sf -X POST "${API}/proposals/${REJECT_ID}/reject" -H 'Content-Type: application/json' -d '{"note":"原文依据不足"}' | field "d['status']")"
check "被驳回的没有写进原著" "1" "$(curl -sf "${API}/original/${WORK_ID}/characters" | field "d['total']")"

echo "== 7. 统计与重复审核"
SUMMARY=$(curl -sf "${API}/original/${WORK_ID}/analysis/summary")
check "待审 0" "0" "$(echo "$SUMMARY" | field "d['pending']")"
check "已通过 1" "1" "$(echo "$SUMMARY" | field "d['approved']")"
check "已驳回 1" "1" "$(echo "$SUMMARY" | field "d['rejected']")"
check "已通过的不能再审（应 409）" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/proposals/${APPROVE_ID}/approve")"
check "不存在的提案（应 404）" "404" "$(curl -s -o /dev/null -w '%{http_code}' "${API}/proposals/00000000-0000-7000-8000-000000000000")"

echo "== 8. 没有章节时触发分析（应 400）"
EMPTY_PROJECT=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"空气原著","type":"ORIGINAL"}' | getid)
EMPTY_WORK=$(curl -sf -X POST "${API}/projects/${EMPTY_PROJECT}/original" -H 'Content-Type: application/json' -d '{"title":"无章节"}' | getid)
check "无章节应 400" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/original/${EMPTY_WORK}/analysis" -H 'Content-Type: application/json' -d '{"stage":"character_extract"}')"
psqlq "delete from original_works where id='${EMPTY_WORK}'" >/dev/null 2>&1
psqlq "delete from projects where id='${EMPTY_PROJECT}'" >/dev/null 2>&1

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项"
[ "${FAIL}" -eq 0 ]
