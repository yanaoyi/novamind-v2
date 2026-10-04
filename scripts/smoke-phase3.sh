#!/usr/bin/env bash
# Phase 3 冒烟：Model Gateway（模型配置 CRUD + 密钥加密 + 真实调用测试）+ Prompt 清单。
#
# 关键点：本脚本会起一个"假的 OpenAI 兼容上游"，让网关真的去调一次 HTTP，
# 从而验证协议拼装、鉴权头、响应解析与错误处理 —— 而不是只测 CRUD。
#
# 用法：bash scripts/smoke-phase3.sh   （需要后端在 127.0.0.1:8080 运行）
set -uo pipefail

# 接口访问令牌：curl 通过 $CURL_HOME/.curlrc 自动带上 Authorization 头（P0 安全修复配套）
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/db-url.sh"

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
FAKE_PORT=19555
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
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

# ---------- 假上游：OpenAI 兼容 ----------
cat > "${WORK_DIR}/fake_openai.py" <<'PY'
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get('Content-Length') or 0)
        body = self.rfile.read(length) if length else b'{}'
        try:
            payload = json.loads(body or b'{}')
        except Exception:
            payload = {}
        # 记录收到的关键信息，便于断言
        record = {
            "path": self.path,
            "authorization": self.headers.get("Authorization", ""),
            "model": payload.get("model", ""),
            "has_messages": bool(payload.get("messages")),
            "json_mode": bool(payload.get("response_format")),
        }
        with open(sys.argv[2], "a", encoding="utf-8") as fh:
            fh.write(json.dumps(record, ensure_ascii=False) + "\n")

        reply = "OK"
        resp = {
            "model": payload.get("model", "fake-model"),
            "choices": [{"message": {"content": reply}, "finish_reason": "stop"}],
            "usage": {"prompt_tokens": 3, "completion_tokens": 1, "total_tokens": 4},
        }
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

UPSTREAM_LOG="${WORK_DIR}/upstream.jsonl"
touch "${UPSTREAM_LOG}"
python3 "${WORK_DIR}/fake_openai.py" "${FAKE_PORT}" "${UPSTREAM_LOG}" &
FAKE_PID=$!

PROVIDER_ID=""
cleanup() {
  [ -n "${PROVIDER_ID}" ] && curl -s -o /dev/null -X DELETE "${API}/model-providers/${PROVIDER_ID}" 2>/dev/null
  kill "${FAKE_PID}" 2>/dev/null || true
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

sleep 1
if ! curl -s -o /dev/null --max-time 3 "http://127.0.0.1:${FAKE_PORT}/"; then
  # 假上游只实现 POST，GET 返回 501 也算起来了
  true
fi

echo "== 1. Prompt 模板清单"
PROMPTS=$(curl -sf "${API}/prompts")
check "模板数量 ≥ 7" "True" "$(echo "$PROMPTS" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["items"]) >= 7)')"
check "含 character_extract" "True" "$(echo "$PROMPTS" | python3 -c 'import sys,json;print(any(i["name"]=="character_extract" for i in json.load(sys.stdin)["data"]["items"]))')"
check "含版本号" "True" "$(echo "$PROMPTS" | python3 -c 'import sys,json;print(all(i["version"] for i in json.load(sys.stdin)["data"]["items"]))')"

echo "== 2. 新增模型配置（密钥加密）"
# 自清理：同名残留（上一次跑挂了没删干净）会让"创建"直接 409，
# 表现为脚本莫名失败。这里先把本项目冒烟用的配置清掉，保证可重复运行。
LEFTOVER=$(curl -sf "${API}/model-providers" | python3 -c '
import sys, json
items = json.load(sys.stdin)["data"]["items"]
print("\n".join(i["id"] for i in items if i["name"].startswith("冒烟-")))')
for id in ${LEFTOVER:-}; do
  curl -sf -X DELETE "${API}/model-providers/${id}" >/dev/null 2>&1 || true
done

CREATED=$(curl -sf -X POST "${API}/model-providers" -H 'Content-Type: application/json' -d "{
  \"name\":\"冒烟-假上游\",\"provider\":\"OPENAI_COMPATIBLE\",
  \"api_base\":\"http://127.0.0.1:${FAKE_PORT}\",\"api_key\":\"sk-smoke-secret-1234\",
  \"model_name\":\"fake-model\",\"purpose\":\"chat\",\"temperature\":0.2,\"max_tokens\":64,
  \"timeout_sec\":15,\"enabled\":true,\"is_default\":false,\"notes\":\"冒烟用\"}")
PROVIDER_ID=$(echo "$CREATED" | getid)
check "创建成功" "36" "${#PROVIDER_ID}"
check "has_api_key=true" "True" "$(echo "$CREATED" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["has_api_key"])')"
check "响应不含密钥字段" "False" "$(echo "$CREATED" | python3 -c 'import sys,json;print("api_key" in json.load(sys.stdin)["data"])')"
check "响应不含密钥明文" "False" "$(echo "$CREATED" | python3 -c 'import sys,json;print("sk-smoke-secret-1234" in json.dumps(json.load(sys.stdin)))')"

echo "== 3. 库里存的是密文（不是明文）"
CIPHER=$(psql "$PSQL_URL" -tAc \
  "select api_key_cipher from model_providers where id='${PROVIDER_ID}'" 2>/dev/null)
check "密文非空" "true" "$([ -n "${CIPHER}" ] && echo true || echo false)"
check "密文不等于明文" "true" "$([ "${CIPHER}" != "sk-smoke-secret-1234" ] && echo true || echo false)"
check "密文长度合理" "true" "$([ "${#CIPHER}" -gt 40 ] && echo true || echo false)"

echo "== 4. 连通性测试（真实调用假上游）"
TESTED=$(curl -sf -X POST "${API}/model-providers/${PROVIDER_ID}/test")
check "测试成功" "True" "$(echo "$TESTED" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["ok"])')"
check "收到回复" "OK" "$(echo "$TESTED" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["reply"])')"
check "上游收到的鉴权头" "Bearer sk-smoke-secret-1234" "$(python3 -c "
import json
lines=[json.loads(l) for l in open('${UPSTREAM_LOG}',encoding='utf-8')]
print(lines[0]['authorization'] if lines else '')")"
check "上游收到的模型名" "fake-model" "$(python3 -c "
import json
lines=[json.loads(l) for l in open('${UPSTREAM_LOG}',encoding='utf-8')]
print(lines[0]['model'] if lines else '')")"

echo "== 5. 更新时留空密钥应保持不变"
UPDATED=$(curl -sf -X PUT "${API}/model-providers/${PROVIDER_ID}" -H 'Content-Type: application/json' -d "{
  \"name\":\"冒烟-假上游（改名）\",\"provider\":\"OPENAI_COMPATIBLE\",
  \"api_base\":\"http://127.0.0.1:${FAKE_PORT}\",\"api_key\":\"\",
  \"model_name\":\"fake-model\",\"purpose\":\"chat\",\"temperature\":0.5,\"max_tokens\":128,
  \"timeout_sec\":15,\"enabled\":true,\"is_default\":false}")
check "改名生效" "冒烟-假上游（改名）" "$(echo "$UPDATED" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["name"])')"
check "密钥仍在" "True" "$(echo "$UPDATED" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["has_api_key"])')"
check "改名后仍可调用" "True" "$(curl -sf -X POST "${API}/model-providers/${PROVIDER_ID}/test" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["ok"])')"

echo "== 6. 列表与默认设置"
check "列表包含该项" "True" "$(curl -sf "${API}/model-providers" | python3 -c "import sys,json;print(any(i['id']=='${PROVIDER_ID}' for i in json.load(sys.stdin)['data']['items']))")"
check "创建的是非默认配置（不抢占已有默认）" "False" "$(echo "$CREATED" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["is_default"])')"

echo "== 7. 错误场景"
check "重复名称（应 409）" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/model-providers" -H 'Content-Type: application/json' -d "{\"name\":\"冒烟-假上游（改名）\",\"provider\":\"OPENAI_COMPATIBLE\",\"api_base\":\"http://127.0.0.1:${FAKE_PORT}\",\"model_name\":\"m\"}")"
check "非法提供商类型（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/model-providers" -H 'Content-Type: application/json' -d '{"name":"坏配置","provider":"BOGUS","api_base":"http://x","model_name":"m"}')"
check "温度越界（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/model-providers" -H 'Content-Type: application/json' -d "{\"name\":\"越界温度\",\"provider\":\"OPENAI_COMPATIBLE\",\"api_base\":\"http://127.0.0.1:${FAKE_PORT}\",\"model_name\":\"m\",\"temperature\":5}")"
check "不存在的配置（应 404）" "404" "$(curl -s -o /dev/null -w '%{http_code}' "${API}/model-providers/00000000-0000-7000-8000-000000000000")"

echo "== 8. 删除"
check "删除（应 200）" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "${API}/model-providers/${PROVIDER_ID}")"
PROVIDER_ID=""
check "删除后列表不含该项" "True" "$(curl -sf "${API}/model-providers" | python3 -c 'import sys,json;print(all("冒烟" not in i["name"] for i in json.load(sys.stdin)["data"]["items"]))')"

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项"
[ "${FAIL}" -eq 0 ]
