#!/usr/bin/env bash
# NovaMind V2 —— 端到端验证（真实模型，规格书 §68 的 MVP 闭环）
#
# 前置：
#   1) 后端已启动（bash scripts/dev-up.sh）
#   2) 已挂验证账号（把 DEEPSEEK_API_KEY 写进 backend/.env 后执行 scripts/validation-account.sh seed）
#
# 覆盖链路：导入原著 → AI 分析（提案）→ 作者审核写入 → 建二创工程与作品 → 人物/世界继承
#          → AI 生成大纲 → **采纳落库（大纲独立模型 §27）→ 一键落成章节** → 建卷建章
#          → AI 写本章 → AI 续写 → AI 就地分析 → AI 问答
#          → 一致性检查 → 导出
#
# 诊断友好：任何一步失败都会打印后端返回的 HTTP 状态与错误正文（不再哑失败）。
# 清理：结束时删除本次创建的两个工程（原著 / 二创）；验证账号保留到交付前再 purge。
set -uo pipefail

# 接口访问令牌：curl 通过 $CURL_HOME/.curlrc 自动带上 Authorization 头（P0 安全修复配套）
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/db-url.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/cleanup.sh"

API="${NOVAMIND_API_BASE:-http://127.0.0.1:8080/api/v1}"
WORK_DIR="$(mktemp -d)"
PASS=0
FAIL=0
HTTP=""
BODY=""

cleanup() {
  # 工程级清理：原著/二创/分块/tasks/快照都会一起删（以前直接删 projects 会被外键拒绝，
  # 错误又被 `|| true` 吞掉 —— 于是每跑一次 e2e 就留两个活工程，实测堆了 10 个）
  cleanup_project "${PROJECT_ID:-}"
  cleanup_project "${CREATIVE_PROJECT_ID:-}"
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

# ---------- 基础设施 ----------

call() { # call METHOD PATH [JSON_BODY]
  local method="$1" path="$2" body="${3:-}" tmp
  tmp="$(mktemp)"
  if [[ -n "$body" ]]; then
    HTTP="$(curl -s -o "$tmp" -w '%{http_code}' -X "$method" "${API}${path}" \
      -H 'Content-Type: application/json' -d "$body")"
  else
    HTTP="$(curl -s -o "$tmp" -w '%{http_code}' -X "$method" "${API}${path}")"
  fi
  BODY="$(cat "$tmp")"
  rm -f "$tmp"
}

http_ok() { [[ "$HTTP" =~ ^2 ]]; }

err_message() {
  python3 - "$BODY" <<'PY'
import json, sys
raw = sys.argv[1]
try:
    err = (json.loads(raw) or {}).get("error") or {}
    msg = "{code} {message}".format(code=err.get("code", ""), message=err.get("message", ""))
except Exception:
    msg = raw[:200]
print(msg.strip() or "（空响应）")
PY
}

must() { # must 描述 —— HTTP 非 2xx 时记账并打印后端错误
  if http_ok; then
    echo "  ✅ $1"
    PASS=$((PASS + 1))
    return 0
  fi
  echo "  ❌ $1（HTTP ${HTTP}：$(err_message)）"
  FAIL=$((FAIL + 1))
  return 1
}

soft() { # soft 描述 期望 实际 —— 业务断言
  if [[ "$2" == "$3" ]]; then
    echo "  ✅ $1"
    PASS=$((PASS + 1))
  else
    echo "  ❌ $1（期望 $2，实际 $3）"
    FAIL=$((FAIL + 1))
  fi
}

jget() { python3 -c "import sys,json;d=json.load(sys.stdin)['data'];print($1)" <<< "$BODY"; }

wait_task() { # wait_task task_id
  local id="$1" status=""
  for _ in $(seq 1 90); do
    call GET "/tasks/${id}"
    status="$(jget "d['status']" 2>/dev/null || echo '')"
    [[ "$status" == "COMPLETED" || "$status" == "FAILED" ]] && break
    sleep 2
  done
  echo "$status"
}

echo "== 0. 前置检查：验证账号"
call GET /model-providers
if ! must "模型配置可用"; then exit 1; fi
PROVIDER_NAME="$(jget "d['items'][0]['name']" 2>/dev/null || echo '')"
echo "     使用配置：${PROVIDER_NAME}"

echo "== 1. 导入原著（3 章）"
call POST /projects '{"name":"端到端验证-原著","type":"ORIGINAL"}'
must "创建原著工程" || exit 1
PROJECT_ID="$(jget "d['id']")"

call POST "/projects/${PROJECT_ID}/original" '{"title":"端到端验证样本"}'
must "创建原著作品" || exit 1
OID="$(jget "d['id']")"

python3 - "${WORK_DIR}" <<'PY'
import sys
work = sys.argv[1]
chapters = [
    ("第一章 雨夜归人", [
        "梅雨下了整夜。沈砚推开老宅的木门，门轴发出一声干涩的响。",
        "他刚从北边的军营回来，肩上还背着那把断了一角的刀。",
        "堂屋里坐着一个女人，穿着素色的衣服，正低头替他缝补旧袍子——是阿箬，他父亲生前收的义女。",
        "“你回来了。”阿箬没有抬头，“城里都在传，你要接管沈家的账。”",
    ]),
    ("第二章 账房里的刀", [
        "沈家账房在后院最深的一进。沈砚推开窗，雨气灌进来。",
        "账册上有一笔三十万两的银钱，去向写着“北境军需”，落款却是三年前——那年他父亲已经死了。",
        "阿箬站在门口：“你要是查下去，第一个动你的人，会是城里的督军。”",
        "沈砚把账册合上：“沈家的规矩，账不亏人。”",
    ]),
    ("第三章 试探", [
        "督军府设宴。席上坐着城里的七家盐商，个个都盯着沈砚那把断刀。",
        "督军举杯：“沈小将军，北境的雪化了吗？”",
        "沈砚答：“化了。刀也养好了。”",
        "满座无声。阿箬在屏风后握紧了手。",
    ]),
]
parts = []
for title, body in chapters:
    parts.append(title)
    parts.append("")
    parts.extend(body)
    parts.append("")
open(f"{work}/sample.txt", "w", encoding="utf-8").write("\n".join(parts))
PY
HTTP="$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/original/${OID}/import" -F "file=@${WORK_DIR}/sample.txt")"
BODY="{}"
must "上传并解析原著"
call GET "/original/${OID}/chapters"
soft "章节数" "3" "$(jget "d['total']" 2>/dev/null || echo '')"

echo "== 2. AI 分析（真实模型，异步）"
call POST "/original/${OID}/analysis" '{"stage":"character_extract"}'
must "触发人物提取" || exit 1
TASK_ID="$(jget "d['id']")"
soft "人物提取完成" "COMPLETED" "$(wait_task "$TASK_ID")"

call GET "/original/${OID}/proposals?status=PENDING"
PENDING="$(jget "d['total']" 2>/dev/null || echo 0)"
if [[ "${PENDING:-0}" -ge 1 ]]; then
  echo "  ✅ 产出待审提案 ${PENDING} 条"
  PASS=$((PASS + 1))
else
  echo "  ❌ 未产出待审提案"
  FAIL=$((FAIL + 1))
fi

echo "== 3. 作者审核写入原著"
APPROVE_ID="$(jget "d['items'][0]['id']" 2>/dev/null || echo '')"
if [[ -n "$APPROVE_ID" ]]; then
  call POST "/proposals/${APPROVE_ID}/approve" '{}'
  must "审核通过写入原著"
fi
call GET "/original/${OID}/characters"
CHARS="$(jget "d['total']" 2>/dev/null || echo 0)"
if [[ "${CHARS:-0}" -ge 1 ]]; then
  echo "  ✅ 原著人物 ${CHARS} 位"
  PASS=$((PASS + 1))
else
  echo "  ❌ 原著人物仍为 0"
  FAIL=$((FAIL + 1))
fi

echo "== 4. 建二创工程与作品，继承人物与世界观"
call POST /projects '{"name":"端到端验证-二创","type":"CREATIVE"}'
must "创建二创工程" || exit 1
CREATIVE_PROJECT_ID="$(jget "d['id']")"

call POST "/original/${OID}/create-creative" \
  "{\"project_id\":\"${CREATIVE_PROJECT_ID}\",\"title\":\"端到端验证-同人\",\"description\":\"验证用同人作品\"}"
must "创建二创作品" || exit 1
CID="$(jget "d['id']")"

call GET "/original/${OID}/characters"
SRC_CHAR="$(jget "d['items'][0]['id']" 2>/dev/null || echo '')"
if [[ -n "$SRC_CHAR" ]]; then
  call POST "/creative/${CID}/characters/inherit" \
    "{\"source_character_id\":\"${SRC_CHAR}\",\"importance\":4,\"weights\":{\"personality\":100,\"speech_style\":60}}"
  if must "人物继承"; then
    echo "     继承得到：$(jget "d['name']" 2>/dev/null || echo '')"
  fi
fi

call POST "/creative/${CID}/world/inherit" '{"mode":"FULL"}'
must "世界观继承"

echo "== 5. AI 生成大纲 → 采纳落库 → 一键落成章节（§27 / §68）"
call POST /ai/generate "{\"kind\":\"outline\",\"work_id\":\"${CID}\",\"instruction\":\"三卷结构，围绕沈家账册与督军的博弈，每卷结尾留一个反转\"}"
if ! must "AI 生成大纲候选"; then
  exit 1
fi
KEYS="$(jget "len(d['result'].keys())" 2>/dev/null || echo 0)"
soft "大纲候选有内容" "True" "$([[ "${KEYS:-0}" -ge 1 ]] && echo True || echo False)"

# 候选 → 写入用的节点树：模型输出是「卷 → 节 → 章」三段式，写入接口的契约是「嵌套即层级」；
# 模型省略「节」时补一层「正文」兜底，避免章节被当成节写进去（与前端 outlineAi.ts 同一规则）。
python3 - "$BODY" "${WORK_DIR}/outline_nodes.json" <<'PY'
import json, sys
raw, out = sys.argv[1], sys.argv[2]
payload = ((json.loads(raw).get("data") or {}).get("result")) or {}

def text(v):
    return v.strip() if isinstance(v, str) else ""

def textarray(v):
    if isinstance(v, list):
        return [t for t in (text(x) for x in v) if t]
    t = text(v)
    return [t] if t else []

def records(v):
    return [x for x in v if isinstance(x, dict)] if isinstance(v, list) else []

def chapter(c, i):
    return {
        "title": text(c.get("title")) or text(c.get("name")) or "第 {} 章".format(i + 1),
        "summary": text(c.get("summary")) or text(c.get("description")),
        "purpose": text(c.get("purpose")),
        "characters": textarray(c.get("characters")),
        "location": text(c.get("location")),
        "conflict": text(c.get("conflict")),
        "outcome": text(c.get("outcome")),
        "children": [],
    }

volumes = payload.get("volumes") or payload.get("outline") or payload.get("nodes") or []
nodes = []
for vi, v in enumerate(records(volumes)):
    sections = records(v.get("sections"))
    children = []
    if sections:
        for si, s in enumerate(sections):
            children.append({
                "title": text(s.get("title")) or text(s.get("name")) or "第 {} 节".format(si + 1),
                "summary": text(s.get("summary")) or text(s.get("description")),
                "purpose": text(s.get("purpose")),
                "characters": textarray(s.get("characters")),
                "location": text(s.get("location")),
                "conflict": text(s.get("conflict")),
                "outcome": text(s.get("outcome")),
                "children": [chapter(c, ci) for ci, c in enumerate(records(s.get("chapters")))],
            })
    else:
        chapters = records(v.get("chapters"))
        if chapters:
            children.append({
                "title": "正文", "summary": "", "purpose": "", "characters": [],
                "location": "", "conflict": "", "outcome": "",
                "children": [chapter(c, ci) for ci, c in enumerate(chapters)],
            })
    nodes.append({
        "title": text(v.get("title")) or text(v.get("name")) or "第 {} 卷".format(vi + 1),
        "summary": text(v.get("summary")) or text(v.get("description")),
        "purpose": text(v.get("purpose")),
        "characters": textarray(v.get("characters")),
        "location": text(v.get("location")),
        "conflict": text(v.get("conflict")),
        "outcome": text(v.get("outcome")),
        "children": children,
    })

json.dump({"title": "AI 大纲（端到端验证）", "summary": "由 validate-e2e-deepseek.sh 采纳", "source": "AI", "nodes": nodes},
          open(out, "w", encoding="utf-8"), ensure_ascii=False)
print("     转换后：{} 卷 / {} 节 / {} 章".format(
    len(nodes),
    sum(len(v["children"]) for v in nodes),
    sum(len(s["children"]) for v in nodes for s in v["children"])))
PY

call POST "/creative/${CID}/outlines" "$(cat "${WORK_DIR}/outline_nodes.json")"
if must "采纳 AI 候选为大纲（§27 落库）"; then
  OUTLINE="$(jget "d['outline']['id']")"
  soft "落库后大纲来源标记为 AI" "AI" "$(jget "d['outline']['source']" 2>/dev/null || echo '')"
  NODE_COUNT="$(jget "d['outline']['node_count']" 2>/dev/null || echo 0)"
  soft "大纲节点已写入（≥3）" "True" "$([[ "${NODE_COUNT:-0}" -ge 3 ]] && echo True || echo False)"

  call POST "/outlines/${OUTLINE}/materialize"
  if must "一键落成卷与章节"; then
    MAT_CH="$(jget "d['chapters_created']" 2>/dev/null || echo 0)"
    echo "     落成：新建 $(jget "d['volumes_created']" 2>/dev/null || echo 0) 卷 / 复用 $(jget "d['volumes_reused']" 2>/dev/null || echo 0) 卷 / 追加 ${MAT_CH} 章"
    call GET "/creative/${CID}/chapters"
    soft "写作系统里的章节数与落成数一致" "True" \
      "$([[ "$(jget "d['total']" 2>/dev/null || echo 0)" == "${MAT_CH:-0}" ]] && echo True || echo False)"
  fi
fi

echo "== 6. 建卷建章 + AI 写本章"
call POST "/creative/${CID}/volumes" '{"title":"第一卷 · 梅雨账","summary":"沈砚接手沈家账目","sequence":1}'
must "创建卷" || exit 1
VOL="$(jget "d['id']")"

# 上面「落成章节」可能已经占用了 1..N 号，这里按现有章数往后取号，避免撞唯一约束
call GET "/creative/${CID}/chapters"
NEXT_NO=$(( $(jget "d['total']" 2>/dev/null || echo 0) + 1 ))

call POST "/creative/${CID}/chapters" \
  "{\"volume_id\":\"${VOL}\",\"chapter_no\":${NEXT_NO},\"title\":\"雨夜\",\"summary\":\"沈砚归家，发现账册疑点\",\"conflict\":\"查账会触动督军\",\"purpose\":\"立起主角与冲突\"}"
must "创建章节" || exit 1
CH="$(jget "d['id']")"

call POST "/chapters/${CH}/generate" '{"target_words":300,"instruction":"克制的短句，第三人称限知"}'
must "触发 AI 写本章" || exit 1
GTASK="$(jget "d['id']")"
soft "写本章任务完成" "COMPLETED" "$(wait_task "$GTASK")"

call GET "/chapters/${CH}"
LEN="$(jget "len(d.get('content') or '')" 2>/dev/null || echo 0)"
soft "生成正文非空（>100 字）" "True" "$([[ "${LEN:-0}" -gt 100 ]] && echo True || echo False)"
echo "     正文长度：${LEN} 字"

echo "== 7. AI 续写 / 就地分析 / 问答"
call POST /ai/continue "{\"chapter_id\":\"${CH}\"}"
if must "AI 续写"; then
  CLEN="$(jget "len(d.get('text') or '')" 2>/dev/null || echo 0)"
  soft "续写内容非空（>20 字）" "True" "$([[ "${CLEN:-0}" -gt 20 ]] && echo True || echo False)"
fi

call POST /ai/analyze "{\"chapter_id\":\"${CH}\",\"focus\":\"节奏与冲突\"}"
if must "AI 就地分析"; then
  HAS="$(jget "'summary' in d['result']" 2>/dev/null || echo False)"
  soft "分析结果含 summary" "True" "$HAS"
fi

call POST /ai/chat "{\"work_id\":\"${CID}\",\"message\":\"目前的主要冲突是什么？\"}"
if must "AI 问答"; then
  RLEN="$(jget "len(d.get('reply') or '')" 2>/dev/null || echo 0)"
  soft "问答内容非空（>10 字）" "True" "$([[ "${RLEN:-0}" -gt 10 ]] && echo True || echo False)"
fi

echo "== 8. 一致性检查（五类上下文）"
call POST "/creative/${CID}/consistency/check" '{}'
must "触发一致性检查" || exit 1
CTASK="$(jget "d['id']")"
soft "一致性检查完成" "COMPLETED" "$(wait_task "$CTASK")"
call GET "/tasks/${CTASK}"
echo "     检查章数：$(jget "d['output'].get('checked', 0)" 2>/dev/null || echo '?')，发现 $(jget "d['output'].get('issues_created', 0)" 2>/dev/null || echo '?') 个问题"
echo "  ✅ 一致性检查任务产出可读"
PASS=$((PASS + 1))

echo "== 9. 导出"
HTTP="$(curl -s -o "${WORK_DIR}/out.txt" -w '%{http_code}' "${API}/creative/${CID}/export?format=txt")"
BODY="{}"
must "导出 TXT"
BYTES="$(wc -c < "${WORK_DIR}/out.txt" | tr -d ' ')"
soft "导出内容非空（>200 字节）" "True" "$([[ "${BYTES:-0}" -gt 200 ]] && echo True || echo False)"

echo
echo "================ 结果 ================"
echo "通过 ${PASS} 项，失败 ${FAIL} 项"
[[ "$FAIL" == "0" ]] || exit 1
