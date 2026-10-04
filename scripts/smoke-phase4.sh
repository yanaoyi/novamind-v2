#!/usr/bin/env bash
# Phase 4 冒烟：二创作品 → 人物继承（DNA 权重派生）→ 人物融合（维度来源追溯）→ 映射。
#
# 用法：bash scripts/smoke-phase4.sh   （需要后端在 127.0.0.1:8080 运行）
set -uo pipefail

# 接口访问令牌：curl 通过 $CURL_HOME/.curlrc 自动带上 Authorization 头（P0 安全修复配套）
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
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

PID=""
OID=""
CPID=""
OID2=""
PID2=""
cleanup() {
  if [ -n "${OID}" ]; then
    psqlq "delete from original_creative_mappings where creative_work_id in (select id from creative_works where original_work_id='${OID}')" >/dev/null 2>&1
    psqlq "delete from creative_works where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from character_relationships where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_characters where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_chapters where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_timelines where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_works where id='${OID}'" >/dev/null 2>&1
  fi
  [ -n "${OID2}" ] && psqlq "delete from original_characters where original_work_id='${OID2}'" >/dev/null 2>&1
  [ -n "${OID2}" ] && psqlq "delete from original_chapters where original_work_id='${OID2}'" >/dev/null 2>&1
  [ -n "${OID2}" ] && psqlq "delete from original_works where id='${OID2}'" >/dev/null 2>&1
  for id in "${PID}" "${CPID}" "${PID2}"; do
    [ -n "${id}" ] && psqlq "delete from files where project_id='${id}'" >/dev/null 2>&1
    [ -n "${id}" ] && psqlq "delete from projects where id='${id}'" >/dev/null 2>&1
  done
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

echo "== 1. 准备原著与人物（带 DNA）"
PID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"二创冒烟-原著","type":"ORIGINAL"}' | getid)
OID=$(curl -sf -X POST "${API}/projects/${PID}/original" -H 'Content-Type: application/json' -d '{"title":"暗涌"}' | getid)
printf '第一章 初遇\n\n林默站在月台上。\n陈述笑了一下。\n' > "${WORK_DIR}/s.txt"
curl -sf -o /dev/null -X POST "${API}/original/${OID}/import" -F "file=@${WORK_DIR}/s.txt"

LIN=$(curl -sf -X POST "${API}/original/${OID}/characters" -H 'Content-Type: application/json' -d '{
  "name":"林默","role":"主角","importance":5,
  "dna":{"personality":{"text":"克制、锋利","weight":90},"values":{"text":"重承诺","weight":80},
         "speech_style":{"text":"短句","weight":40},"ability":{"text":"会修车","weight":50}}}' | getid)
CHEN=$(curl -sf -X POST "${API}/original/${OID}/characters" -H 'Content-Type: application/json' -d '{
  "name":"陈述","role":"男主","importance":4,
  "dna":{"personality":{"text":"圆滑","weight":60},"speech_style":{"text":"爱开玩笑","weight":80}}}' | getid)
check "原著人物已建" "2" "$(curl -sf "${API}/original/${OID}/characters" | field "d['total']")"

echo "== 2. 创建二创作品"
CPID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"二创冒烟-二创","type":"CREATIVE"}' | getid)
CREATIVE=$(curl -sf -X POST "${API}/original/${OID}/create-creative" -H 'Content-Type: application/json' \
  -d "{\"project_id\":\"${CPID}\",\"title\":\"暗涌·另一条路\",\"description\":\"如果那天林默没有离开\"}")
CID=$(echo "$CREATIVE" | getid)
check "二创作品创建" "36" "${#CID}"
check "初始状态" "DRAFT" "$(echo "$CREATIVE" | field "d['status']")"

echo "== 3. 人物继承（按权重派生 DNA）"
INHERITED=$(curl -sf -X POST "${API}/creative/${CID}/characters/inherit" -H 'Content-Type: application/json' -d "{
  \"source_character_id\":\"${LIN}\",\"importance\":5,
  \"weights\":{\"personality\":80,\"values\":80,\"speech_style\":30,\"ability\":0}}")
CC_LIN=$(echo "$INHERITED" | getid)
check "继承来源类型" "ORIGINAL_INHERITED" "$(echo "$INHERITED" | field "d['source_type']")"
check "人格 90×80%=72" "72" "$(echo "$INHERITED" | field "d['dna']['personality']['weight']")"
check "价值观 80×80%=64" "64" "$(echo "$INHERITED" | field "d['dna']['values']['weight']")"
check "语言风格 40×30%=12" "12" "$(echo "$INHERITED" | field "d['dna']['speech_style']['weight']")"
check "能力权重 0 → 不继承" "0" "$(echo "$INHERITED" | field "d['dna'].get('ability',{}).get('weight',0)")"
check "描述随权重保留" "克制、锋利" "$(echo "$INHERITED" | field "d['dna']['personality']['text']")"

CC_CHEN=$(curl -sf -X POST "${API}/creative/${CID}/characters/inherit" -H 'Content-Type: application/json' -d "{
  \"source_character_id\":\"${CHEN}\",\"importance\":4,
  \"weights\":{\"personality\":100,\"speech_style\":100}}" | getid)
check "第二个人物继承" "36" "${#CC_CHEN}"

echo "== 4. 继承权重可回读"
DETAIL=$(curl -sf "${API}/creative-characters/${CC_LIN}")
check "带出继承规则" "1" "$(echo "$DETAIL" | field "len(d.get('rules',[]))")"
check "规则里的人格权重" "80" "$(echo "$DETAIL" | field "d['rules'][0]['personality']")"
check "规则里的能力权重" "0" "$(echo "$DETAIL" | field "d['rules'][0]['ability']")"

echo "== 5. 人物融合（每个维度可追溯来源）"
FUSED=$(curl -sf -X POST "${API}/creative/${CID}/characters/fuse" -H 'Content-Type: application/json' -d "{
  \"name\":\"林述\",\"description\":\"林默与陈述的融合体\",\"importance\":5,
  \"sources\":[{\"character_id\":\"${CC_LIN}\",\"weight\":60},{\"character_id\":\"${CC_CHEN}\",\"weight\":100}]}")
FUSED_ID=$(echo "$FUSED" | getid)
check "融合来源类型" "FUSED" "$(echo "$FUSED" | field "d['source_type']")"
# 人格：林默 72×60%=43 vs 陈述 60×100%=60 → 取陈述
check "人格取自更强的来源" "圆滑" "$(echo "$FUSED" | field "d['dna']['personality']['text']")"
check "人格权重 60" "60" "$(echo "$FUSED" | field "d['dna']['personality']['weight']")"
check "融合来源数" "2" "$(echo "$FUSED" | field "len(d['fusion_sources'])")"
check "融合说明含维度来源" "陈述" "$(echo "$FUSED" | field "[x['from_name'] for x in d['fusion_detail'] if x['dimension']=='personality'][0]")"
check "融合说明条数 ≥ 3" "True" "$(echo "$FUSED" | field "len(d['fusion_detail']) >= 3")"

echo "== 6. 映射关系（原著↔二创，可追溯）"
MAPPINGS=$(curl -sf "${API}/creative/${CID}/mappings")
check "映射条数（2 继承 + 2 融合）" "4" "$(echo "$MAPPINGS" | field "d['total']")"
check "含 INHERITED 映射" "True" "$(echo "$MAPPINGS" | field "any(m['mapping_type']=='INHERITED' for m in d['items'])")"
check "含 FUSED 映射" "True" "$(echo "$MAPPINGS" | field "any(m['mapping_type']=='FUSED' for m in d['items'])")"
check "映射指向原著人物" "True" "$(echo "$MAPPINGS" | field "all(m['original_type']=='character' for m in d['items'])")"

echo "== 7. 二创人物列表与原创人物"
curl -sf -o /dev/null -X POST "${API}/creative/${CID}/characters/new" -H 'Content-Type: application/json' \
  -d '{"name":"苏晚","description":"原创角色","importance":3,"dna":{"personality":{"text":"沉静","weight":70}}}'
# 林默(继承) + 陈述(继承) + 林述(融合) + 苏晚(原创) = 4
check "二创人物总数" "4" "$(curl -sf "${API}/creative/${CID}/characters" | field "d['total']")"
check "原创人物无来源" "NEW" "$(curl -sf "${API}/creative/${CID}/characters" | field "[c['source_type'] for c in d['items'] if c['name']=='苏晚'][0]")"

echo "== 8. 错误场景"
# 跨原著继承：另建一部原著与人物
PID2=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"二创冒烟-外部","type":"ORIGINAL"}' | getid)
OID2=$(curl -sf -X POST "${API}/projects/${PID2}/original" -H 'Content-Type: application/json' -d '{"title":"另一部"}' | getid)
OUTSIDER=$(curl -sf -X POST "${API}/original/${OID2}/characters" -H 'Content-Type: application/json' -d '{"name":"外人"}' | getid)
check "跨原著继承（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/creative/${CID}/characters/inherit" -H 'Content-Type: application/json' -d "{\"source_character_id\":\"${OUTSIDER}\",\"weights\":{\"personality\":100}}")"
check "权重全 0（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/creative/${CID}/characters/inherit" -H 'Content-Type: application/json' -d "{\"source_character_id\":\"${LIN}\",\"weights\":{\"personality\":0}}")"
check "单来源融合（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/creative/${CID}/characters/fuse" -H 'Content-Type: application/json' -d "{\"name\":\"单人\",\"sources\":[{\"character_id\":\"${CC_LIN}\",\"weight\":100}]}")"
check "同工程重复建二创（应 409）" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/original/${OID}/create-creative" -H 'Content-Type: application/json' -d "{\"project_id\":\"${CPID}\",\"title\":\"重复\"}")"
check "给原著工程建二创（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/original/${OID}/create-creative" -H 'Content-Type: application/json' -d "{\"project_id\":\"${PID}\",\"title\":\"不该成功\"}")"

echo "== 9. 锁定后拒绝被重新继承覆盖"
curl -sf -o /dev/null -X PUT "${API}/creative-characters/${CC_LIN}" -H 'Content-Type: application/json' -d '{"is_locked":true}'
check "锁定后重新继承（应 409）" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/creative/${CID}/characters/inherit" -H 'Content-Type: application/json' -d "{\"source_character_id\":\"${LIN}\",\"weights\":{\"personality\":50}}")"
curl -sf -o /dev/null -X PUT "${API}/creative-characters/${CC_LIN}" -H 'Content-Type: application/json' -d '{"is_locked":false}'
check "解锁后可重新继承" "201" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/creative/${CID}/characters/inherit" -H 'Content-Type: application/json' -d "{\"source_character_id\":\"${LIN}\",\"name\":\"林默\",\"weights\":{\"personality\":50}}")"
check "重新继承后权重变为 90×50%=45" "45" "$(curl -sf "${API}/creative-characters/${CC_LIN}" | field "d['dna']['personality']['weight']")"

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项"
[ "${FAIL}" -eq 0 ]
