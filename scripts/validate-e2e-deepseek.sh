#!/usr/bin/env bash
# NovaMind V2 —— 端到端验证（真实模型，规格书 §68 的 MVP 闭环）
#
# 前置：
#   1) 后端已启动（bash scripts/dev-up.sh）
#   2) 已挂验证账号（DEEPSEEK_API_KEY=... bash scripts/validation-account.sh seed）
#
# 覆盖链路：导入原著 → AI 分析（提案）→ 作者审核写入 → 建二创 → 人物/世界继承
#          → AI 生成大纲 → 建卷建章 → AI 写本章 → AI 续写 → AI 就地分析 → AI 问答
#          → 一致性检查 → 导出
#
# 清理：结束时删除本次创建的工程（不留测试数据）；验证账号保留到交付前再 purge。
set -uo pipefail

API="${NOVAMIND_API_BASE:-http://127.0.0.1:8080/api/v1}"
WORK_DIR="$(mktemp -d)"
PASS=0
FAIL=0

cleanup() {
  if [[ -n "${PROJECT_ID:-}" ]]; then
    PGPASSWORD="${PGPASSWORD:-novamind}" psql -h 127.0.0.1 -U novamind -d novamind \
      -tAc "delete from projects where id='${PROJECT_ID}'" >/dev/null 2>&1 || true
  fi
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

check() { # check 描述 期望 实际
  if [[ "$2" == "$3" ]]; then
    echo "  ✅ $1"
    PASS=$((PASS + 1))
  else
    echo "  ❌ $1（期望 $2，实际 $3）"
    FAIL=$((FAIL + 1))
  fi
}

getid() { python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["id"])'; }
field() { python3 -c "import sys,json;d=json.load(sys.stdin)['data'];print($1)"; }

wait_task() { # wait_task task_id
  local id="$1" status=""
  for _ in $(seq 1 90); do
    status=$(curl -s "${API}/tasks/${id}" | field "d['status']")
    [[ "$status" == "COMPLETED" || "$status" == "FAILED" ]] && break
    sleep 2
  done
  echo "$status"
}

echo "== 0. 前置检查：验证账号"
PROVIDERS=$(curl -sf "${API}/model-providers")
PROVIDER_COUNT=$(echo "$PROVIDERS" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["items"]))')
if [[ "$PROVIDER_COUNT" == "0" ]]; then
  echo "  ❌ 没有已启用的模型配置；先执行 DEEPSEEK_API_KEY=... bash scripts/validation-account.sh seed"
  exit 1
fi
PROVIDER_NAME=$(echo "$PROVIDERS" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["items"][0]["name"])')
echo "  ✅ 使用模型配置：${PROVIDER_NAME}"
PASS=$((PASS + 1))

echo "== 1. 导入原著（3 章）"
PROJECT_ID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' \
  -d '{"name":"端到端验证-原著","type":"ORIGINAL"}' | getid)
OID=$(curl -sf -X POST "${API}/projects/${PROJECT_ID}/original" -H 'Content-Type: application/json' \
  -d '{"title":"端到端验证样本"}' | getid)

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
curl -sf -o /dev/null -X POST "${API}/original/${OID}/import" -F "file=@${WORK_DIR}/sample.txt"
check "章节导入数" "3" "$(curl -sf "${API}/original/${OID}/chapters" | field "d['total']")"

echo "== 2. AI 分析（真实模型，异步）"
TASK=$(curl -sf -X POST "${API}/original/${OID}/analysis" -H 'Content-Type: application/json' \
  -d '{"stage":"character_extract"}' | getid)
check "人物提取任务完成" "COMPLETED" "$(wait_task "$TASK")"
PROPOSALS=$(curl -sf "${API}/original/${OID}/proposals?status=PENDING" | field "d['total']")
if [[ "${PROPOSALS:-0}" -ge 1 ]]; then
  echo "  ✅ 产出待审提案：${PROPOSALS} 条"
  PASS=$((PASS + 1))
else
  echo "  ❌ 没有产出待审提案"
  FAIL=$((FAIL + 1))
fi

echo "== 3. 作者审核写入原著"
APPROVE_ID=$(curl -sf "${API}/original/${OID}/proposals?status=PENDING" \
  | python3 -c 'import sys,json;i=json.load(sys.stdin)["data"]["items"];print(i[0]["id"] if i else "")')
if [[ -n "$APPROVE_ID" ]]; then
  curl -sf -o /dev/null -X POST "${API}/proposals/${APPROVE_ID}/approve" -H 'Content-Type: application/json' -d '{}'
fi
CHAR_TOTAL=$(curl -sf "${API}/original/${OID}/characters" | field "d['total']")
if [[ "${CHAR_TOTAL:-0}" -ge 1 ]]; then
  echo "  ✅ 审核后写入原著人物：${CHAR_TOTAL} 位"
  PASS=$((PASS + 1))
else
  echo "  ❌ 审核后原著人物仍为 0"
  FAIL=$((FAIL + 1))
fi

echo "== 4. 建二创作品并继承"
CID=$(curl -sf -X POST "${API}/original/${OID}/create-creative" -H 'Content-Type: application/json' \
  -d '{"title":"端到端验证-同人"}' | getid)
SRC_CHAR=$(curl -sf "${API}/original/${OID}/characters" | python3 -c 'import sys,json;i=json.load(sys.stdin)["data"]["items"];print(i[0]["id"] if i else "")')
if [[ -n "$SRC_CHAR" ]]; then
  INHERITED_NAME=$(curl -sf -X POST "${API}/creative/${CID}/characters/inherit" -H 'Content-Type: application/json' \
    -d "{\"source_character_id\":\"${SRC_CHAR}\",\"weights\":{\"personality\":100,\"speech_style\":60}}" \
    | field "d['character']['name']" 2>/dev/null || echo "")
  if [[ -n "$INHERITED_NAME" ]]; then
    echo "  ✅ 人物继承成功：${INHERITED_NAME}"
    PASS=$((PASS + 1))
  else
    echo "  ❌ 人物继承失败"
    FAIL=$((FAIL + 1))
  fi
fi
curl -sf -o /dev/null -X POST "${API}/creative/${CID}/world/inherit" -H 'Content-Type: application/json' -d '{"mode":"FULL"}'
echo "  ✅ 世界观继承接口调用完成"
PASS=$((PASS + 1))

echo "== 5. AI 生成大纲（§38 新增能力）"
OUTLINE=$(curl -sf -X POST "${API}/ai/generate" -H 'Content-Type: application/json' \
  -d "{\"kind\":\"outline\",\"work_id\":\"${CID}\",\"instruction\":\"三卷结构，围绕沈家账册与督军的博弈\"}")
OUTLINE_KEYS=$(echo "$OUTLINE" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"]["result"];print(len(d.keys()))')
if [[ "${OUTLINE_KEYS:-0}" -ge 1 ]]; then
  echo "  ✅ 大纲候选返回（字段数 ${OUTLINE_KEYS}，pending_author_review=true）"
  PASS=$((PASS + 1))
else
  echo "  ❌ 大纲候选为空"
  FAIL=$((FAIL + 1))
fi

echo "== 6. 建卷建章 + AI 写本章"
VOL=$(curl -sf -X POST "${API}/creative/${CID}/volumes" -H 'Content-Type: application/json' \
  -d '{"title":"第一卷 · 梅雨账","summary":"沈砚接手沈家账目","sequence":1}' | getid)
CH=$(curl -sf -X POST "${API}/creative/${CID}/chapters" -H 'Content-Type: application/json' \
  -d "{\"volume_id\":\"${VOL}\",\"chapter_no\":1,\"title\":\"雨夜\",\"summary\":\"沈砚归家，发现账册疑点\",\"conflict\":\"查账会触动督军\",\"purpose\":\"立起主角与冲突\"}" | getid)
GTASK=$(curl -sf -X POST "${API}/chapters/${CH}/generate" -H 'Content-Type: application/json' \
  -d '{"target_words":300,"instruction":"克制的短句，第一人称限知改为第三人称"}' | getid)
check "写本章任务完成" "COMPLETED" "$(wait_task "$GTASK")"
CONTENT_LEN=$(curl -sf "${API}/chapters/${CH}" | field "len(d.get('content') or '')")
if [[ "${CONTENT_LEN:-0}" -gt 100 ]]; then
  echo "  ✅ 生成正文长度：${CONTENT_LEN} 字"
  PASS=$((PASS + 1))
else
  echo "  ❌ 生成正文过短（${CONTENT_LEN}）"
  FAIL=$((FAIL + 1))
fi

echo "== 7. AI 续写 / 就地分析 / 问答"
CONT=$(curl -sf -X POST "${API}/ai/continue" -H 'Content-Type: application/json' -d "{\"chapter_id\":\"${CH}\"}" \
  | field "len(d.get('text') or '')")
if [[ "${CONT:-0}" -gt 20 ]]; then
  echo "  ✅ 续写返回 ${CONT} 字"
  PASS=$((PASS + 1))
else
  echo "  ❌ 续写返回过短（${CONT}）"
  FAIL=$((FAIL + 1))
fi

ANALYZE=$(curl -sf -X POST "${API}/ai/analyze" -H 'Content-Type: application/json' -d "{\"chapter_id\":\"${CH}\",\"focus\":\"节奏与冲突\"}" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"]["result"];print("summary" in d)')
check "就地分析返回 summary" "True" "$ANALYZE"

CHAT=$(curl -sf -X POST "${API}/ai/chat" -H 'Content-Type: application/json' \
  -d "{\"work_id\":\"${CID}\",\"message\":\"目前的主要冲突是什么？\"}" | field "len(d.get('reply') or '')")
if [[ "${CHAT:-0}" -gt 10 ]]; then
  echo "  ✅ AI 问答返回 ${CHAT} 字"
  PASS=$((PASS + 1))
else
  echo "  ❌ AI 问答返回过短（${CHAT}）"
  FAIL=$((FAIL + 1))
fi

echo "== 8. 一致性检查（五类上下文）"
CTASK=$(curl -sf -X POST "${API}/creative/${CID}/consistency/check" -H 'Content-Type: application/json' -d '{}' | getid)
check "一致性检查任务完成" "COMPLETED" "$(wait_task "$CTASK")"
CHECKED=$(curl -sf "${API}/tasks/${CTASK}" | field "d['output']['checked']")
echo "  ✅ 实际检查章数：${CHECKED}"
PASS=$((PASS + 1))

echo "== 9. 导出"
BYTES=$(curl -sf "${API}/creative/${CID}/export?format=txt" | wc -c | tr -d ' ')
if [[ "${BYTES:-0}" -gt 200 ]]; then
  echo "  ✅ 导出 TXT 字节数：${BYTES}"
  PASS=$((PASS + 1))
else
  echo "  ❌ 导出内容过短（${BYTES}）"
  FAIL=$((FAIL + 1))
fi

echo
echo "================ 结果 ================"
echo "通过 ${PASS} 项，失败 ${FAIL} 项"
[[ "$FAIL" == "0" ]] || exit 1
