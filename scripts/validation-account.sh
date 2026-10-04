#!/usr/bin/env bash
# NovaMind V2 —— 验证账号管理（规格书 §36 §69）
#
# 纪律（BOSS 2026-10-04 定）：
#   * 开发/验证阶段可以临时挂一个 DeepSeek 账号，用来跑通端到端 AI 链路；
#   * **系统交付前必须执行 `purge`**，把验证账号从库里删干净；
#   * 系统不内置任何模型账号，使用者自备 API Key（在「模型设置」页自己配）。
#
# 用法：
#   scripts/validation-account.sh status              # 看当前挂了哪些模型配置
#   DEEPSEEK_API_KEY=sk-xxx scripts/validation-account.sh seed
#   （也可以把 DEEPSEEK_API_KEY=sk-xxx 写进 backend/.env —— 该文件已在 .gitignore 里，
#     这样密钥不会出现在命令行、shell 历史或聊天记录里；purge 会把它一并清掉）
#   scripts/validation-account.sh purge               # 交付前必做：清库 + 清 backend/.env 里的验证密钥
#   scripts/validation-account.sh check               # 扫仓库，确认没有硬编码密钥
#
# 说明：密钥只从环境变量读取，绝不写进文件、绝不进命令行参数（避免进 shell 历史与进程列表）。
set -euo pipefail

# 接口访问令牌：curl 通过 $CURL_HOME/.curlrc 自动带上 Authorization 头（P0 安全修复配套）
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/db-url.sh"

API_BASE="${NOVAMIND_API_BASE:-http://127.0.0.1:8080/api/v1}"
VALIDATION_NAME="${NOVAMIND_VALIDATION_NAME:-验证用-DeepSeek}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

die() { echo "错误：$*" >&2; exit 1; }

# DATABASE_URL 用于"硬清理"：API 的删除是软删除，密文仍留在表里，
# 交付前必须走 SQL 把行彻底删掉（见 cmd_purge）。
DATABASE_URL="${DATABASE_URL:-}"
if [[ -z "$DATABASE_URL" && -f "${REPO_ROOT}/backend/.env" ]]; then
  DATABASE_URL="$(grep -E '^DATABASE_URL=' "${REPO_ROOT}/backend/.env" | head -1 | cut -d= -f2-)"
fi

sql() {
  [[ -n "$DATABASE_URL" ]] || die "拿不到 DATABASE_URL（可在 backend/.env 里配，或用环境变量传入）"
  psql "$DATABASE_URL" -tAc "$1"
}

api() {
  local method="$1" path="$2" body="${3:-}"
  if [[ -n "$body" ]]; then
    curl -sS -X "$method" "${API_BASE}${path}" -H 'Content-Type: application/json' -d "$body"
  else
    curl -sS -X "$method" "${API_BASE}${path}"
  fi
}

list_ids() {
  api GET /model-providers | python3 -c '
import json,sys
payload = json.load(sys.stdin)
items = (payload.get("data") or {}).get("items") or []
for it in items:
    print(it["id"])
'
}

cmd_status() {
  api GET /model-providers | python3 -c '
import json,sys
payload = json.load(sys.stdin)
items = (payload.get("data") or {}).get("items") or []
if not items:
    print("接口可见的模型配置：0 条")
else:
    print(f"接口可见的模型配置：{len(items)} 条")
    for it in items:
        print("  - {name} | {provider} | {model} | {base} | enabled={enabled} | has_key={key}".format(
            name=it.get("name"), provider=it.get("provider"), model=it.get("model_name"),
            base=it.get("api_base"), enabled=it.get("enabled"), key=it.get("has_api_key")))
'
  # 软删除的行不在接口里，但仍然占着密钥密文 —— 这里必须一起报出来
  if [[ -n "$DATABASE_URL" ]]; then
    echo "（含软删除）表内总行数：$(sql 'select count(*) from model_providers;')，仍存密钥密文：$(sql "select count(*) from model_providers where coalesce(api_key_cipher,'') <> '';")"
  fi
}

cmd_seed() {
  local key="${DEEPSEEK_API_KEY:-${NOVAMIND_VALIDATION_KEY:-}}"
  if [[ -z "$key" && -f "${REPO_ROOT}/backend/.env" ]]; then
    key="$(grep -E '^DEEPSEEK_API_KEY=' "${REPO_ROOT}/backend/.env" | head -1 | cut -d= -f2-)"
  fi
  [[ -n "$key" ]] || die "没有拿到密钥。两种方式任选：① 环境变量 DEEPSEEK_API_KEY=sk-xxx；② 写进 backend/.env（该文件不入库）"

  local model="${DEEPSEEK_MODEL:-deepseek-chat}"
  local base="${DEEPSEEK_API_BASE:-https://api.deepseek.com/v1}"
  echo "使用的模型：${model} @ ${base}"
  local body
  body="$(python3 - "$VALIDATION_NAME" "$base" "$model" "$key" <<'PY'
import json,sys
name, base, model, key = sys.argv[1:5]
print(json.dumps({
    "name": name,
    "provider": "OPENAI_COMPATIBLE",
    "api_base": base,
    "model_name": model,
    "api_key": key,
    "purpose": "chat",
    "temperature": 0.7,
    "max_tokens": 4096,
    "timeout_sec": 120,
    "enabled": True,
    "is_default": True,
    "notes": "验证账号（BOSS 授权，仅验证期使用）；交付前必须执行 scripts/validation-account.sh purge 删除",
}, ensure_ascii=False))
PY
)"
  api POST /model-providers "$body" | python3 -c '
import json,sys
p = json.load(sys.stdin)
if p.get("error"):
    print("创建失败：", p["error"]); raise SystemExit(1)
d = p["data"]
name = d["name"]; model = d["model_name"]; has_key = d["has_api_key"]
print("已创建验证模型配置：{}（{}，has_api_key={}）".format(name, model, has_key))
print("提醒：交付前必须运行 scripts/validation-account.sh purge")
'
}

cmd_purge() {
  local ids
  ids="$(list_ids)"
  local count=0 id
  if [[ -n "$ids" ]]; then
    while read -r id; do
      [[ -n "$id" ]] || continue
      api DELETE "/model-providers/${id}" >/dev/null
      count=$((count + 1))
    done <<< "$ids"
    echo "已通过接口删除 ${count} 条启用中的配置（同时清空其密钥密文）。"
  fi

  # 硬清理：软删除的历史行（以及任何残留密文）一并抹掉
  sql "update model_providers set api_key_cipher = '' where coalesce(api_key_cipher,'') <> '';" >/dev/null
  sql "delete from model_providers;" >/dev/null
  echo "已执行硬清理：物理删除 model_providers 全部行。"

  local rows residue
  rows="$(sql 'select count(*) from model_providers;')"
  residue="$(sql "select count(*) from model_providers where coalesce(api_key_cipher,'') <> '';")"
  [[ "$rows" == "0" ]] || die "清理后仍剩 ${rows} 行"
  [[ "$residue" == "0" ]] || die "清理后仍残留 ${residue} 条密钥密文"
  echo "校验通过：模型配置 0 行、密钥密文 0 条 —— 系统不再持有任何第三方账号。"
  echo "使用者请在「模型设置」页配置自己的 API Key。"

  # 本地 .env 里的验证密钥也要清掉，否则"删了库里的、留着文件里的"等于没删
  local env_file="${REPO_ROOT}/backend/.env"
  if [[ -f "$env_file" ]] && grep -qE '^DEEPSEEK_API_KEY=' "$env_file"; then
    python3 - "$env_file" <<'PY'
import re, sys
path = sys.argv[1]
text = open(path, encoding="utf-8").read()
new = re.sub(r'^DEEPSEEK_API_KEY=.*\n?', '', text, flags=re.M)
open(path, "w", encoding="utf-8").write(new)
print("已从 backend/.env 移除 DEEPSEEK_API_KEY（该文件不入库）")
PY
  fi
}

cmd_check() {
  # 只看被 git 跟踪的文件：本地 .env 里放着验证密钥是允许的（它被 .gitignore 排除），
  # 真正要防的是"密钥被误提交进版本库"。
  echo "扫描 git 跟踪文件里的疑似硬编码密钥..."
  cd "$REPO_ROOT" || die "进入仓库目录失败"

  local hits
  hits="$(git ls-files -z \
    | grep -zvE '(_test\.go$|smoke-.*\.sh$)' \
    | xargs -0 -r grep -nE 'sk-[A-Za-z0-9]{24,}' 2>/dev/null || true)"
  if [[ -n "$hits" ]]; then
    echo "$hits"
    die "发现疑似硬编码密钥，必须移除后才能交付"
  fi
  echo "未发现硬编码密钥（本地 .env 不在跟踪范围内；测试与冒烟脚本的假 Key 已排除）。"
}

case "${1:-}" in
  status) cmd_status ;;
  seed)   cmd_seed ;;
  purge)  cmd_purge ;;
  check)  cmd_check ;;
  *) sed -n '2,20p' "${BASH_SOURCE[0]}"; exit 1 ;;
esac
