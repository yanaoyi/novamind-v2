#!/usr/bin/env bash
# Phase 5/6/7 冒烟：章节与版本 → AI 写作（写本章 / 编辑器改写）→ 一致性检查 → 导出。
#
# 假模型会按收到的 prompt 内容分别返回"正文 / 改写文本 / 一致性问题"，从而验证三条链路。
#
# 用法：bash scripts/smoke-phase5.sh   （需要后端在 127.0.0.1:8080 运行）
set -uo pipefail

# 接口访问令牌：curl 通过 $CURL_HOME/.curlrc 自动带上 Authorization 头（P0 安全修复配套）
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
FAKE_PORT=19557
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

cat > "${WORK_DIR}/fake_model.py" <<'PY'
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get('Content-Length') or 0)
        raw = self.rfile.read(length) if length else b'{}'
        try:
            payload = json.loads(raw or b'{}')
        except Exception:
            payload = {}
        text = " ".join(m.get("content", "") for m in payload.get("messages", []))

        if "一致性审查员" in text or "一致性" in text:
            content = json.dumps({"issues": [
                {"severity": "high", "type": "character", "description": "林默的说话方式与设定不符",
                 "evidence": "原文片段：……", "suggestion": "改成更克制的短句"},
                {"severity": "low", "type": "world", "description": "出现了一处未定义的机构名",
                 "evidence": "原文片段：……", "suggestion": "改为已设定的机构"}
            ]}, ensure_ascii=False)
        elif "撰写一章正文" in text or "本章目标" in text:
            content = "雨停了。\n\n林默把信折好，放进外套内袋。她没有回头。"
        else:
            content = "【AI 处理结果】雨停了，林默把信折好放进外套内袋，没有回头。"

        resp = {"model": "fake-writing-model",
                "choices": [{"message": {"content": content}, "finish_reason": "stop"}],
                "usage": {"prompt_tokens": 50, "completion_tokens": 80, "total_tokens": 130}}
        data = json.dumps(resp).encode()
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

PID=""; OID=""; CPID=""; CID=""; PROVIDER_ID=""
cleanup() {
  if [ -n "${CID}" ]; then
    psqlq "delete from consistency_issues where creative_work_id='${CID}'" >/dev/null 2>&1
    psqlq "delete from chapter_versions where chapter_id in (select id from creative_chapters where creative_work_id='${CID}')" >/dev/null 2>&1
    psqlq "delete from creative_scenes where chapter_id in (select id from creative_chapters where creative_work_id='${CID}')" >/dev/null 2>&1
    psqlq "delete from creative_chapters where creative_work_id='${CID}'" >/dev/null 2>&1
    psqlq "delete from creative_volumes where creative_work_id='${CID}'" >/dev/null 2>&1
    psqlq "delete from original_creative_mappings where creative_work_id='${CID}'" >/dev/null 2>&1
    psqlq "delete from creative_works where id='${CID}'" >/dev/null 2>&1
  fi
  [ -n "${PROVIDER_ID}" ] && psqlq "delete from model_providers where id='${PROVIDER_ID}'" >/dev/null 2>&1
  if [ -n "${OID}" ]; then
    psqlq "delete from original_characters where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from world_rules where world_id in (select id from original_worlds where original_work_id='${OID}')" >/dev/null 2>&1
    psqlq "delete from original_worlds where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_chapters where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_works where id='${OID}'" >/dev/null 2>&1
  fi
  for id in "${PID}" "${CPID}"; do
    [ -n "${id}" ] && psqlq "delete from files where project_id='${id}'" >/dev/null 2>&1
    [ -n "${id}" ] && psqlq "delete from projects where id='${id}'" >/dev/null 2>&1
  done
  kill "${FAKE_PID}" 2>/dev/null || true
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT
sleep 1

echo "== 1. 准备（原著 + 二创作品 + 继承人物与世界 + 模型配置）"
PROVIDER_ID=$(curl -sf -X POST "${API}/model-providers" -H 'Content-Type: application/json' -d "{
  \"name\":\"冒烟-写作模型\",\"provider\":\"OPENAI_COMPATIBLE\",\"api_base\":\"http://127.0.0.1:${FAKE_PORT}\",
  \"api_key\":\"sk-fake\",\"model_name\":\"fake-writing-model\",\"purpose\":\"chat\",
  \"temperature\":0.7,\"max_tokens\":2048,\"timeout_sec\":15,\"enabled\":true,\"is_default\":true}" | getid)
PID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"写作冒烟-原著","type":"ORIGINAL"}' | getid)
OID=$(curl -sf -X POST "${API}/projects/${PID}/original" -H 'Content-Type: application/json' -d '{"title":"暗涌"}' | getid)
printf '第一章 初遇\n\n林默站在月台上。\n' > "${WORK_DIR}/s.txt"
curl -sf -o /dev/null -X POST "${API}/original/${OID}/import" -F "file=@${WORK_DIR}/s.txt"
LIN=$(curl -sf -X POST "${API}/original/${OID}/characters" -H 'Content-Type: application/json' -d '{
  "name":"林默","role":"主角","importance":5,"description":"外冷内热",
  "dna":{"personality":{"text":"克制、锋利","weight":90},"motivation":{"text":"查清真相","weight":80}}}' | getid)
curl -sf -o /dev/null -X PUT "${API}/original/${OID}/world" -H 'Content-Type: application/json' -d '{"name":"江城"}'
curl -sf -o /dev/null -X POST "${API}/original/${OID}/rules" -H 'Content-Type: application/json' -d '{"category":"社会规则","name":"老城区的人情规则","description":"先讲人情","importance":4}'
CPID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"写作冒烟-二创","type":"CREATIVE"}' | getid)
CID=$(curl -sf -X POST "${API}/original/${OID}/create-creative" -H 'Content-Type: application/json' -d "{\"project_id\":\"${CPID}\",\"title\":\"暗涌·另一条路\"}" | getid)
curl -sf -o /dev/null -X POST "${API}/creative/${CID}/characters/inherit" -H 'Content-Type: application/json' -d "{\"source_character_id\":\"${LIN}\",\"weights\":{\"personality\":90,\"motivation\":80}}"
curl -sf -o /dev/null -X POST "${API}/creative/${CID}/world/inherit" -H 'Content-Type: application/json' -d '{"mode":"PARTIAL"}'
check "二创人物已继承" "1" "$(curl -sf "${API}/creative/${CID}/characters" | field "d['total']")"
check "二创世界已继承" "1" "$(curl -sf "${API}/creative/${CID}/world" | field "d['inherited_count']")"

echo "== 2. 卷与章节"
VOL=$(curl -sf -X POST "${API}/creative/${CID}/volumes" -H 'Content-Type: application/json' -d '{"title":"第一卷 · 梅雨","summary":"老城区的雨季","sequence":1}' | getid)
check "卷创建" "36" "${#VOL}"
CH1=$(curl -sf -X POST "${API}/creative/${CID}/chapters" -H 'Content-Type: application/json' -d "{
  \"volume_id\":\"${VOL}\",\"chapter_no\":1,\"title\":\"旧信\",\"summary\":\"林默发现母亲留下的信\",
  \"purpose\":\"引出母亲意外\",\"conflict\":\"是否回江城\",\"outcome\":\"决定留下\",
  \"content\":\"雨下了三天。林默在抽屉最里面摸到一封信。\"}" | getid)
check "章节创建" "36" "${#CH1}"
check "字数已统计" "True" "$(curl -sf "${API}/chapters/${CH1}" | field "d['word_count'] > 0")"
curl -sf -o /dev/null -X POST "${API}/creative/${CID}/chapters" -H 'Content-Type: application/json' -d "{\"chapter_no\":2,\"title\":\"对峙\",\"summary\":\"与城建集团正面冲突\"}"
check "章节列表 2 章" "2" "$(curl -sf "${API}/creative/${CID}/chapters" | field "d['total']")"

echo "== 3. 版本管理"
curl -sf -o /dev/null -X PUT "${API}/chapters/${CH1}" -H 'Content-Type: application/json' -d '{"content":"雨下了三天。林默在抽屉最里面摸到一封信。信封上是母亲的字。"}'
VERSIONS=$(curl -sf "${API}/chapters/${CH1}/versions")
check "正文修改后生成新版本" "2" "$(echo "$VERSIONS" | field "d['total']")"
check "版本 v1 正文是旧内容" "True" "$(curl -sf "${API}/chapters/${CH1}/versions/1" | field "'母亲的字' not in d['content']")"
curl -sf -o /dev/null -X POST "${API}/chapters/${CH1}/versions/1/restore"
check "恢复 v1 后正文回到旧内容" "True" "$(curl -sf "${API}/chapters/${CH1}" | field "'母亲的字' not in d['content']")"
# v1 创建 → v2 编辑 → v3 恢复前自动备份 → v4 恢复结果
check "恢复动作也留了版本" "4" "$(curl -sf "${API}/chapters/${CH1}/versions" | field "d['total']")"

echo "== 4. 编辑器 AI 改写（同步）"
REWRITTEN=$(curl -sf -X POST "${API}/ai/rewrite" -H 'Content-Type: application/json' -d "{\"chapter_id\":\"${CH1}\",\"text\":\"雨下了三天。\",\"action\":\"扩写\",\"instruction\":\"增加环境描写\"}")
check "返回处理后的文本" "True" "$(echo "$REWRITTEN" | field "'AI 处理结果' in d['text']")"
check "空文本被拒绝" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/ai/rewrite" -H 'Content-Type: application/json' -d "{\"chapter_id\":\"${CH1}\",\"text\":\"\",\"action\":\"改写\"}")"
check "非法操作类型被拒" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/ai/rewrite" -H 'Content-Type: application/json' -d "{\"chapter_id\":\"${CH1}\",\"text\":\"x\",\"action\":\"翻译成英文\"}")"

echo "== 5. AI 写本章（异步）"
HTTP=$(curl -s -o "${WORK_DIR}/task.json" -w '%{http_code}' -X POST "${API}/chapters/${CH1}/generate" -H 'Content-Type: application/json' -d '{"target_words":800,"instruction":"保持克制"}')
check "返回 202" "202" "$HTTP"
TASK_ID=$(python3 -c 'import json;print(json.load(open("'"${WORK_DIR}"'/task.json"))["data"]["id"])')
for _ in $(seq 1 40); do
  STATUS=$(curl -sf "${API}/tasks/${TASK_ID}" | field "d['status']")
  [ "$STATUS" = "COMPLETED" ] || [ "$STATUS" = "FAILED" ] && break
  sleep 1
done
check "写作任务完成" "COMPLETED" "$STATUS"
check "正文被写入" "True" "$(curl -sf "${API}/chapters/${CH1}" | field "'林默把信折好' in d['content']")"
check "生成为新版本" "True" "$(curl -sf "${API}/chapters/${CH1}/versions" | field "d['total'] >= 4")"

echo "== 6. 一致性检查（异步）"
HTTP=$(curl -s -o "${WORK_DIR}/task2.json" -w '%{http_code}' -X POST "${API}/creative/${CID}/consistency/check" -H 'Content-Type: application/json' -d '{}')
check "返回 202" "202" "$HTTP"
TASK2=$(python3 -c 'import json;print(json.load(open("'"${WORK_DIR}"'/task2.json"))["data"]["id"])')
for _ in $(seq 1 40); do
  STATUS=$(curl -sf "${API}/tasks/${TASK2}" | field "d['status']")
  [ "$STATUS" = "COMPLETED" ] || [ "$STATUS" = "FAILED" ] && break
  sleep 1
done
check "检查任务完成" "COMPLETED" "$STATUS"
ISSUES=$(curl -sf "${API}/creative/${CID}/consistency/issues?status=OPEN")
check "发现 2 个问题" "2" "$(echo "$ISSUES" | field "d['total']")"
check "含 high 严重度" "True" "$(echo "$ISSUES" | field "any(i['severity']=='high' for i in d['items'])")"
check "含人物类问题" "True" "$(echo "$ISSUES" | field "any(i['type']=='character' for i in d['items'])")"
ISSUE_ID=$(echo "$ISSUES" | field "d['items'][0]['id']")
check "标记为解决" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X PUT "${API}/consistency-issues/${ISSUE_ID}" -H 'Content-Type: application/json' -d '{"status":"RESOLVED"}')"
check "剩余待处理 1 个" "1" "$(curl -sf "${API}/creative/${CID}/consistency/issues?status=OPEN" | field "d['total']")"

echo "== 7. 导出"
TXT=$(curl -sf "${API}/creative/${CID}/export?format=txt")
check "TXT 含作品名" "True" "$(echo "$TXT" | grep -q "暗涌·另一条路" && echo True || echo False)"
check "TXT 含章节标题" "True" "$(echo "$TXT" | grep -q "第 1 章 旧信" && echo True || echo False)"
MD=$(curl -sf "${API}/creative/${CID}/export?format=md")
check "Markdown 含标题层级" "True" "$(echo "$MD" | grep -q "^# 暗涌·另一条路" && echo True || echo False)"
curl -sf -o "${WORK_DIR}/out.docx" "${API}/creative/${CID}/export?format=docx"
check "DOCX 是合法 zip（PK 头）" "PK" "$(head -c 2 "${WORK_DIR}/out.docx")"
check "DOCX 大小合理" "True" "$([ "$(stat -c%s "${WORK_DIR}/out.docx")" -gt 1000 ] && echo True || echo False)"
check "不支持的格式（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' "${API}/creative/${CID}/export?format=pdf")"

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项"
[ "${FAIL}" -eq 0 ]
