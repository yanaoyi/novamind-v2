#!/usr/bin/env bash
# Phase 8 大纲冒烟：大纲树（卷 → 节 → 章）→ 单节点增改删 → 整树替换 → 版本快照与恢复
#                  → 落成写作系统的卷与章节 → 版本比较。
#
# 用法：bash scripts/smoke-phase8-outline.sh   （需要后端在 127.0.0.1:8080 运行）
set -uo pipefail

# 接口访问令牌：curl 通过 $CURL_HOME/.curlrc 自动带上 Authorization 头（P0 安全修复配套）
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/db-url.sh"

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
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

PID=""; OID=""; CPID=""; CID=""; OUTLINE=""
cleanup() {
  if [ -n "${OUTLINE}" ]; then
    psqlq "delete from entity_versions where entity_type='creative_outline_tree' and entity_id='${OUTLINE}'" >/dev/null 2>&1
    psqlq "delete from outline_nodes where outline_id='${OUTLINE}'" >/dev/null 2>&1
    psqlq "delete from outlines where id='${OUTLINE}'" >/dev/null 2>&1
  fi
  if [ -n "${CID}" ]; then
    psqlq "delete from outlines where creative_work_id='${CID}'" >/dev/null 2>&1
    psqlq "delete from entity_versions where creative_work_id='${CID}'" >/dev/null 2>&1
    psqlq "delete from chapter_versions where chapter_id in (select id from creative_chapters where creative_work_id='${CID}')" >/dev/null 2>&1
    psqlq "delete from creative_chapters where creative_work_id='${CID}'" >/dev/null 2>&1
    psqlq "delete from creative_volumes where creative_work_id='${CID}'" >/dev/null 2>&1
    psqlq "delete from creative_works where id='${CID}'" >/dev/null 2>&1
  fi
  if [ -n "${OID}" ]; then
    psqlq "delete from original_chapters where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_works where id='${OID}'" >/dev/null 2>&1
  fi
  for id in "${PID}" "${CPID}"; do
    [ -n "${id}" ] && psqlq "delete from files where project_id='${id}'" >/dev/null 2>&1
    [ -n "${id}" ] && psqlq "delete from projects where id='${id}'" >/dev/null 2>&1
  done
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

echo "== 1. 准备（原著 + 二创作品）"
PID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"大纲冒烟-原著","type":"ORIGINAL"}' | getid)
OID=$(curl -sf -X POST "${API}/projects/${PID}/original" -H 'Content-Type: application/json' -d '{"title":"暗涌"}' | getid)
printf '第一章 初遇\n\n林默站在月台上。\n' > "${WORK_DIR}/s.txt"
curl -sf -o /dev/null -X POST "${API}/original/${OID}/import" -F "file=@${WORK_DIR}/s.txt"
CPID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"大纲冒烟-二创","type":"CREATIVE"}' | getid)
CID=$(curl -sf -X POST "${API}/original/${OID}/create-creative" -H 'Content-Type: application/json' -d "{\"project_id\":\"${CPID}\",\"title\":\"暗涌·另一条路\"}" | getid)
check "二创作品就绪" "36" "${#CID}"

echo "== 2. 建大纲（卷 → 节 → 章 一次性写入）"
TREE='{"title":"二创大纲 v1","summary":"卷节章三层结构","source":"MANUAL","nodes":[
  {"title":"第一卷 · 梅雨","summary":"老城区的雨季","children":[
    {"title":"第一节 · 归乡","children":[
      {"title":"旧信","summary":"林默发现母亲留下的信","purpose":"引出母亲意外","characters":["林默"],"location":"老屋","conflict":"是否回江城","outcome":"决定留下"},
      {"title":"对峙","purpose":"与城建集团正面冲突","conflict":"拆迁补偿","outcome":"结下梁子"}
    ]},
    {"title":"第二节 · 暗流"}
  ]},
  {"title":"第二卷 · 决堤"}
]}'
RESP=$(curl -sf -X POST "${API}/creative/${CID}/outlines" -H 'Content-Type: application/json' -d "$TREE")
OUTLINE=$(echo "$RESP" | field "d['outline']['id']")
check "大纲创建" "36" "${#OUTLINE}"
check "节点数 6" "6" "$(echo "$RESP" | field "d['outline']['node_count']")"
check "根节点 2 个（两卷）" "2" "$(echo "$RESP" | field "len(d['nodes'])")"
check "卷层级=1" "1" "$(echo "$RESP" | field "d['nodes'][0]['level']")"
check "卷名正确" "第一卷 · 梅雨" "$(echo "$RESP" | field "d['nodes'][0]['title']")"
check "节层级=2" "2" "$(echo "$RESP" | field "d['nodes'][0]['children'][0]['level']")"
check "章层级=3" "3" "$(echo "$RESP" | field "d['nodes'][0]['children'][0]['children'][0]['level']")"
check "章的三要素保留" "True" "$(echo "$RESP" | field "d['nodes'][0]['children'][0]['children'][0]['purpose']=='引出母亲意外'")"
check "章的人物保留" "True" "$(echo "$RESP" | field "d['nodes'][0]['children'][0]['children'][0]['characters']==['林默']")"
check "来源标记 MANUAL" "MANUAL" "$(echo "$RESP" | field "d['outline']['source']")"

echo "== 3. 入参校验（这些必须是 400，不能是 500）"
check "空标题被拒" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/creative/${CID}/outlines" -H 'Content-Type: application/json' -d '{"title":"  "}')"
check "四层嵌套被拒" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/creative/${CID}/outlines" -H 'Content-Type: application/json' -d '{"title":"太深","nodes":[{"title":"卷","children":[{"title":"节","children":[{"title":"章","children":[{"title":"第四层"}]}]}]}]}')"
check "不存在的大纲返回 404" "404" "$(curl -s -o /dev/null -w '%{http_code}' "${API}/outlines/00000000-0000-7000-8000-000000000000")"

echo "== 4. 单节点增改删"
VOL1=$(curl -sf "${API}/outlines/${OUTLINE}" | field "d['nodes'][0]['id']")
SEC2=$(curl -sf "${API}/outlines/${OUTLINE}" | field "d['nodes'][0]['children'][1]['id']")
NEWNODE=$(curl -sf -X POST "${API}/outlines/${OUTLINE}/nodes" -H 'Content-Type: application/json' -d "{\"parent_id\":\"${SEC2}\",\"title\":\"第三幕\",\"purpose\":\"补一场夜谈\"}")
NEWID=$(echo "$NEWNODE" | field "d['id']")
check "父节点下新增节点层级=3" "3" "$(echo "$NEWNODE" | field "d['level']")"
check "节点总数变 7" "7" "$(curl -sf "${API}/outlines/${OUTLINE}" | field "d['outline']['node_count']")"
check "改标题成功" "第三幕 · 夜谈" "$(curl -sf -X PUT "${API}/outline-nodes/${NEWID}" -H 'Content-Type: application/json' -d '{"title":"第三幕 · 夜谈"}' | field "d['title']")"
CH1=$(curl -sf "${API}/outlines/${OUTLINE}" | field "d['nodes'][0]['children'][0]['children'][0]['id']")
check "章下不能加子节点（400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/outlines/${OUTLINE}/nodes" -H 'Content-Type: application/json' -d "{\"parent_id\":\"${CH1}\",\"title\":\"违规\"}")"
check "删除节点及其子树" "True" "$(curl -sf -X DELETE "${API}/outline-nodes/${NEWID}" | field "d['deleted_nodes'] >= 1")"
check "节点总数回到 6" "6" "$(curl -sf "${API}/outlines/${OUTLINE}" | field "d['outline']['node_count']")"
# 跨大纲挂节点必须被拒（父节点存在，但不属于这份大纲）——这是"树不能串线"的护栏
OTHER=$(curl -sf -X POST "${API}/creative/${CID}/outlines" -H 'Content-Type: application/json' -d '{"title":"另一份大纲","nodes":[{"title":"别的卷"}]}' | field "d['outline']['id']")
OTHERNODE=$(curl -sf "${API}/outlines/${OTHER}" | field "d['nodes'][0]['id']")
check "跨树父节点被拒（400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/outlines/${OUTLINE}/nodes" -H 'Content-Type: application/json' -d "{\"parent_id\":\"${OTHERNODE}\",\"title\":\"串线的节点\"}")"
check "不存在的父节点返回 404" "404" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/outlines/${OUTLINE}/nodes" -H 'Content-Type: application/json' -d '{"parent_id":"00000000-0000-7000-8000-000000000001","title":"孤儿"}')"
check "两份大纲互不干扰" "1" "$(curl -sf "${API}/outlines/${OTHER}" | field "d['outline']['node_count']")"

echo "== 5. 整树替换 + 版本历史（§59）"
HTTP=$(curl -s -o "${WORK_DIR}/replace.json" -w '%{http_code}' -X PUT "${API}/outlines/${OUTLINE}/tree" -H 'Content-Type: application/json' -d '{"nodes":[{"title":"第一幕","children":[{"title":"开场"}]}]}')
check "整树替换 200" "200" "$HTTP"
check "替换后节点数 2" "2" "$(python3 -c 'import json;print(json.load(open("'"${WORK_DIR}"'/replace.json"))["data"]["outline"]["node_count"])')"
VERSIONS=$(curl -sf "${API}/outlines/${OUTLINE}/versions")
VCOUNT_BEFORE=$(echo "$VERSIONS" | field "d['total']")
check "每次改动都留版本（≥5 版，实际 ${VCOUNT_BEFORE}）" "True" "$([ "${VCOUNT_BEFORE}" -ge 5 ] && echo True || echo False)"
check "v1 快照含 6 个节点" "6" "$(curl -sf "${API}/outlines/${OUTLINE}/versions/1" | field "len(d['payload']['nodes'])")"
check "v1 快照标题正确" "二创大纲 v1" "$(curl -sf "${API}/outlines/${OUTLINE}/versions/1" | field "d['payload']['title']")"
curl -sf -o /dev/null -X POST "${API}/outlines/${OUTLINE}/versions/1/restore"
check "恢复 v1 后节点数回到 6" "6" "$(curl -sf "${API}/outlines/${OUTLINE}" | field "d['outline']['node_count']")"
check "恢复 v1 后章名回来" "旧信" "$(curl -sf "${API}/outlines/${OUTLINE}" | field "[c for v in d['nodes'] for s in v['children'] for c in s['children']][0]['title']")"
VCOUNT_AFTER=$(curl -sf "${API}/outlines/${OUTLINE}/versions" | field "d['total']")
check "恢复动作也留了版本（${VCOUNT_BEFORE} → ${VCOUNT_AFTER}）" "True" "$([ "${VCOUNT_AFTER}" -gt "${VCOUNT_BEFORE}" ] && echo True || echo False)"
check "重复快照不建版本" "False" "$(curl -sf -X POST "${API}/outlines/${OUTLINE}/versions" -H 'Content-Type: application/json' -d '{"note":"重复"}' | field "d['created']")"
check "版本比较有差异" "True" "$(curl -sf "${API}/versions/compare?entity_type=creative_outline_tree&entity_id=${OUTLINE}&from=1&to=${VCOUNT_AFTER}" | field "d['summary']['changed'] + d['summary']['added'] + d['summary']['removed'] > 0")"

echo "== 6. 落成写作系统的卷与章节（§68 的下一步）"
MAT=$(curl -sf -X POST "${API}/outlines/${OUTLINE}/materialize")
check "建了 2 个卷" "2" "$(echo "$MAT" | field "d['volumes_created']")"
check "建了 2 章" "2" "$(echo "$MAT" | field "d['chapters_created']")"
CHAP=$(curl -sf "${API}/creative/${CID}/chapters")
check "写作系统里真的有 2 章" "2" "$(echo "$CHAP" | field "d['total']")"
check "章标题来自大纲" "旧信" "$(echo "$CHAP" | field "d['items'][0]['title']")"
check "章大纲信息已带上（purpose）" "True" "$(echo "$CHAP" | field "d['items'][0]['purpose']=='引出母亲意外'")"
check "节信息以摘要前缀保留" "True" "$(echo "$CHAP" | field "d['items'][0]['summary'].startswith('【第一节 · 归乡】')")"
check "章挂到了卷上" "True" "$(echo "$CHAP" | field "d['items'][0]['volume_id'] is not None")"
MAT2=$(curl -sf -X POST "${API}/outlines/${OUTLINE}/materialize")
check "再次落成复用卷" "2" "$(echo "$MAT2" | field "d['volumes_reused']")"
check "再次落成不重复建章" "0" "$(echo "$MAT2" | field "d['chapters_created']")"
check "已落成的章被跳过" "2" "$(echo "$MAT2" | field "d['chapters_skipped']")"
check "章节总数不变（防重生效）" "2" "$(curl -sf "${API}/creative/${CID}/chapters" | field "d['total']")"

echo "== 7. 删除大纲"
check "删除返回 200" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "${API}/outlines/${OUTLINE}")"
check "删除后查询 404" "404" "$(curl -s -o /dev/null -w '%{http_code}' "${API}/outlines/${OUTLINE}")"
check "删除大纲不动章节（章还在）" "2" "$(curl -sf "${API}/creative/${CID}/chapters" | field "d['total']")"

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项"
[ "${FAIL}" -eq 0 ]
