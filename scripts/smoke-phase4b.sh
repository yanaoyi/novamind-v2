#!/usr/bin/env bash
# Phase 4 冒烟（第二片）：二创世界继承 → 规则逐条改/删/新增 → 分叉点 → 二创时间线自动构建。
#
# 用法：bash scripts/smoke-phase4b.sh   （需要后端在 127.0.0.1:8080 运行）
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

PID=""; OID=""; CPID=""; CID=""; PID2=""; OID2=""
cleanup() {
  if [ -n "${CID}" ]; then
    psqlq "delete from original_creative_mappings where creative_work_id='${CID}'" >/dev/null 2>&1
    psqlq "delete from creative_works where id='${CID}'" >/dev/null 2>&1
  fi
  if [ -n "${OID}" ]; then
    psqlq "delete from divergence_points where original_event_id in (select id from original_events where original_work_id='${OID}')" >/dev/null 2>&1
    psqlq "delete from timeline_events where timeline_id in (select id from original_timelines where original_work_id='${OID}')" >/dev/null 2>&1
    psqlq "delete from original_timelines where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_events where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from world_rules where world_id in (select id from original_worlds where original_work_id='${OID}')" >/dev/null 2>&1
    psqlq "delete from original_worlds where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_chapters where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_works where id='${OID}'" >/dev/null 2>&1
  fi
  [ -n "${OID2}" ] && psqlq "delete from original_events where original_work_id='${OID2}'" >/dev/null 2>&1
  [ -n "${OID2}" ] && psqlq "delete from original_timelines where original_work_id='${OID2}'" >/dev/null 2>&1
  [ -n "${OID2}" ] && psqlq "delete from original_works where id='${OID2}'" >/dev/null 2>&1
  for id in "${PID}" "${CPID}" "${PID2}"; do
    [ -n "${id}" ] && psqlq "delete from files where project_id='${id}'" >/dev/null 2>&1
    [ -n "${id}" ] && psqlq "delete from projects where id='${id}'" >/dev/null 2>&1
  done
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

echo "== 1. 准备原著（世界 + 规则 + 事件 + 时间线）"
PID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"二创B-原著","type":"ORIGINAL"}' | getid)
OID=$(curl -sf -X POST "${API}/projects/${PID}/original" -H 'Content-Type: application/json' -d '{"title":"暗涌"}' | getid)
printf '第一章 初遇\n\n林默站在月台上。\n第二章 裂痕\n\n三年后一切都变了。\n' > "${WORK_DIR}/s.txt"
curl -sf -o /dev/null -X POST "${API}/original/${OID}/import" -F "file=@${WORK_DIR}/s.txt"
curl -sf -o /dev/null -X PUT "${API}/original/${OID}/world" -H 'Content-Type: application/json' -d '{"name":"江城","description":"被大雨困住的南方城市"}'
R1=$(curl -sf -X POST "${API}/original/${OID}/rules" -H 'Content-Type: application/json' -d '{"category":"社会规则","name":"老城区的人情规则","description":"先讲人情再讲道理","importance":4}' | getid)
R2=$(curl -sf -X POST "${API}/original/${OID}/rules" -H 'Content-Type: application/json' -d '{"category":"现实基调","name":"没有超自然力量","description":"一切都有现实解释","importance":5}' | getid)
E1=$(curl -sf -X POST "${API}/original/${OID}/events" -H 'Content-Type: application/json' -d '{"title":"母亲的意外","chapter_no":1,"time_order":10,"importance":5}' | getid)
E2=$(curl -sf -X POST "${API}/original/${OID}/events" -H 'Content-Type: application/json' -d '{"title":"拆迁通知贴出","chapter_no":2,"time_order":20,"importance":4}' | getid)
E3=$(curl -sf -X POST "${API}/original/${OID}/events" -H 'Content-Type: application/json' -d '{"title":"真相揭开","chapter_no":2,"time_order":30,"importance":5}' | getid)
curl -sf -o /dev/null -X PUT "${API}/original/${OID}/timeline" -H 'Content-Type: application/json' \
  -d "{\"items\":[{\"event_id\":\"${E1}\",\"time_label\":\"三年前\"},{\"event_id\":\"${E2}\",\"time_label\":\"现在·六月初\"},{\"event_id\":\"${E3}\",\"time_label\":\"月底\"}]}"
check "原著世界已建" "江城" "$(curl -sf "${API}/original/${OID}/world" | field "d['name']")"
check "原著规则 2 条" "2" "$(curl -sf "${API}/original/${OID}/rules" | field "d['total']")"
check "原著时间线 3 段" "3" "$(curl -sf "${API}/original/${OID}/timeline" | field "len(d['entries'])")"

echo "== 2. 创建二创作品"
CPID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"二创B-二创","type":"CREATIVE"}' | getid)
CID=$(curl -sf -X POST "${API}/original/${OID}/create-creative" -H 'Content-Type: application/json' \
  -d "{\"project_id\":\"${CPID}\",\"title\":\"暗涌·另一条路\"}" | getid)
check "二创作品创建" "36" "${#CID}"
check "未继承时世界为空壳" "None" "$(curl -sf "${API}/creative/${CID}/world" | field "d['id']")"

echo "== 3. 继承原著世界（PARTIAL：规则整套带过来）"
WORLD=$(curl -sf -X POST "${API}/creative/${CID}/world/inherit" -H 'Content-Type: application/json' -d '{"mode":"PARTIAL"}')
check "继承模式" "PARTIAL" "$(echo "$WORLD" | field "d['inheritance_mode']")"
check "世界名沿用原著" "江城" "$(echo "$WORLD" | field "d['name']")"
check "规则继承 2 条" "2" "$(echo "$WORLD" | field "d['inherited_count']")"
check "规则状态为 INHERITED" "INHERITED" "$(echo "$WORLD" | field "[r['status'] for r in d['rules']][0]")"
# 明确按名字取，避免依赖排序（之前踩过：改的和删的是同一条）
RID_MODIFY=$(echo "$WORLD" | field "[r['id'] for r in d['rules'] if r['name']=='老城区的人情规则'][0]")
RID_REMOVE=$(echo "$WORLD" | field "[r['id'] for r in d['rules'] if r['name']=='没有超自然力量'][0]")

echo "== 4. 逐条改 / 新增 / 删"
check "改动继承规则 → MODIFIED" "MODIFIED" "$(curl -sf -X PUT "${API}/creative-world-rules/${RID_MODIFY}" -H 'Content-Type: application/json' -d '{"name":"老城区的人情规则（二创版）","category":"社会规则","description":"改写：人情也有代价","importance":4}' | field "d['status']")"
NEW_RULE=$(curl -sf -X POST "${API}/creative/${CID}/world/rules" -H 'Content-Type: application/json' \
  -d '{"category":"二创新增","name":"青石巷的雨","description":"这场雨下了整整一个月","importance":3}')
check "新增规则 → NEW" "NEW" "$(echo "$NEW_RULE" | field "d['status']")"
check "第二条继承规则删除 → REMOVED" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "${API}/creative-world-rules/${RID_REMOVE}")"
AFTER=$(curl -sf "${API}/creative/${CID}/world")
check "统计：改 1 / 删 1 / 新 1" "1,1,1" "$(echo "$AFTER" | field "str(d['modified_count'])+','+str(d['removed_count'])+','+str(d['new_count'])")"
check "被删的规则仍在列表（可追溯）" "True" "$(echo "$AFTER" | field "any(r['status']=='REMOVED' for r in d['rules'])")"

echo "== 5. 设置分叉点（指向「拆迁通知贴出」）"
check "分叉点前置校验" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X PUT "${API}/creative/${CID}/divergence" -H 'Content-Type: application/json' -d '{"time_label":"没有指向"}')"
POINT=$(curl -sf -X PUT "${API}/creative/${CID}/divergence" -H 'Content-Type: application/json' \
  -d "{\"original_event_id\":\"${E2}\",\"time_label\":\"现在·六月初\",\"description\":\"从拆迁通知贴出之后改写\"}")
check "分叉点事件" "${E2}" "$(echo "$POINT" | field "d['original_event_id']")"
check "分叉点可回读" "${E2}" "$(curl -sf "${API}/creative/${CID}/divergence" | field "d['original_event_id']")"
# 跨原著事件应被拒
PID2=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"二创B-外部","type":"ORIGINAL"}' | getid)
OID2=$(curl -sf -X POST "${API}/projects/${PID2}/original" -H 'Content-Type: application/json' -d '{"title":"另一部"}' | getid)
OUTSIDE_EVENT=$(curl -sf -X POST "${API}/original/${OID2}/events" -H 'Content-Type: application/json' -d '{"title":"外部事件"}' | getid)
check "跨原著分叉点（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X PUT "${API}/creative/${CID}/divergence" -H 'Content-Type: application/json' -d "{\"original_event_id\":\"${OUTSIDE_EVENT}\"}")"

echo "== 6. 自动构建二创时间线（只继承分叉点及之前）"
BUILT=$(curl -sf -X POST "${API}/creative/${CID}/timeline/build")
check "继承条目数（分叉点及之前 = 2）" "2" "$(echo "$BUILT" | field "d['total']")"
check "第一条继承状态" "INHERITED" "$(echo "$BUILT" | field "d['items'][0]['status']")"
check "第一条是原著事件" "母亲的意外" "$(echo "$BUILT" | field "d['items'][0]['title']")"
check "分叉点之后的事件未被继承" "False" "$(echo "$BUILT" | field "any(i['title']=='真相揭开' for i in d['items'])")"

echo "== 7. 追加二创新事件后重新构建（作者内容不被覆盖）"
MANUAL=$(curl -sf -X PUT "${API}/creative/${CID}/timeline" -H 'Content-Type: application/json' -d "{\"items\":[
  {\"source_original_event_id\":\"${E1}\",\"status\":\"INHERITED\",\"time_label\":\"三年前\",\"title\":\"母亲的意外\"},
  {\"source_original_event_id\":\"${E2}\",\"status\":\"INHERITED\",\"time_label\":\"现在·六月初\",\"title\":\"拆迁通知贴出\"},
  {\"status\":\"NEW\",\"time_label\":\"一周后\",\"title\":\"林默找到旧信\",\"description\":\"二创新增事件\"}]}")
check "手工时间线 3 条" "3" "$(echo "$MANUAL" | field "d['total']")"
REBUILT=$(curl -sf -X POST "${API}/creative/${CID}/timeline/build")
check "重建后总数 3（继承 2 + 二创 1）" "3" "$(echo "$REBUILT" | field "d['total']")"
check "二创新事件被保留在末尾" "林默找到旧信" "$(echo "$REBUILT" | field "d['items'][-1]['title']")"
check "末尾条目状态为 NEW" "NEW" "$(echo "$REBUILT" | field "d['items'][-1]['status']")"

echo "== 8. 映射可追溯"
MAPPINGS=$(curl -sf "${API}/creative/${CID}/mappings")
check "含世界映射" "True" "$(echo "$MAPPINGS" | field "any(m['original_type']=='world' and m['mapping_type']=='INHERITED' for m in d['items'])")"
check "含事件继承映射" "True" "$(echo "$MAPPINGS" | field "any(m['original_type']=='event' for m in d['items'])")"
check "含规则删除映射" "True" "$(echo "$MAPPINGS" | field "any(m['original_type']=='world_rule' and m['mapping_type']=='REMOVED' for m in d['items'])")"

echo "== 9. 错误场景"
check "非法继承模式（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/creative/${CID}/world/inherit" -H 'Content-Type: application/json' -d '{"mode":"BOGUS"}')"
check "重复规则名（应 409）" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/creative/${CID}/world/rules" -H 'Content-Type: application/json' -d '{"name":"青石巷的雨"}')"
check "不存在的二创作品（应 404）" "404" "$(curl -s -o /dev/null -w '%{http_code}' "${API}/creative/00000000-0000-7000-8000-000000000000/world")"

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项"
[ "${FAIL}" -eq 0 ]
