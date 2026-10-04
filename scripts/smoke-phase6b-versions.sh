#!/usr/bin/env bash
# 版本历史冒烟：人物 / 世界观 / 大纲的「改 → 看历史 → 回滚」闭环。
#
# 关键断言：回滚后数据真的回到旧值；恢复动作本身也会留一版；内容没变时不产生噪声版本。
#
# 用法：bash scripts/smoke-phase6b-versions.sh   （需要后端在 127.0.0.1:8080 运行）
set -uo pipefail

# 接口访问令牌：curl 通过 $CURL_HOME/.curlrc 自动带上 Authorization 头（P0 安全修复配套）
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
PSQL_URL="postgresql://novamind:novamind@127.0.0.1:5432/novamind"

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

PID=""; OID=""; CPID=""; CID=""; CHID=""
cleanup() {
  [ -n "${CID}" ] && psqlq "delete from entity_versions where creative_work_id='${CID}'" >/dev/null 2>&1
  [ -n "${CID}" ] && psqlq "delete from chapter_versions where chapter_id in (select id from creative_chapters where creative_work_id='${CID}')" >/dev/null 2>&1
  [ -n "${CID}" ] && psqlq "delete from creative_chapters where creative_work_id='${CID}'" >/dev/null 2>&1
  [ -n "${CID}" ] && psqlq "delete from creative_volumes where creative_work_id='${CID}'" >/dev/null 2>&1
  [ -n "${CID}" ] && psqlq "delete from creative_world_rules where creative_world_id in (select id from creative_worlds where creative_work_id='${CID}')" >/dev/null 2>&1
  [ -n "${CID}" ] && psqlq "delete from creative_worlds where creative_work_id='${CID}'" >/dev/null 2>&1
  [ -n "${CID}" ] && psqlq "delete from creative_characters where creative_work_id='${CID}'" >/dev/null 2>&1
  [ -n "${CID}" ] && psqlq "delete from creative_works where id='${CID}'" >/dev/null 2>&1
  if [ -n "${OID}" ]; then
    psqlq "delete from original_characters where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_works where id='${OID}'" >/dev/null 2>&1
  fi
  for id in "${PID}" "${CPID}"; do
    [ -n "${id}" ] && psqlq "delete from projects where id='${id}'" >/dev/null 2>&1
  done
}
trap cleanup EXIT

echo "== 1. 准备（原著 + 二创 + 人物 + 世界 + 章节）"
PID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"版本冒烟-原著","type":"ORIGINAL"}' | getid)
OID=$(curl -sf -X POST "${API}/projects/${PID}/original" -H 'Content-Type: application/json' -d '{"title":"潮汐"}' | getid)
printf '第一章 起风\n\n海面很平。\n' > /tmp/ver-src.txt
curl -sf -o /dev/null -X POST "${API}/original/${OID}/import" -F "file=@/tmp/ver-src.txt"
SRC_CHAR=$(curl -sf -X POST "${API}/original/${OID}/characters" -H 'Content-Type: application/json' -d '{
  "name":"林默","role":"主角","importance":5,"description":"外冷内热",
  "dna":{"personality":{"text":"克制","weight":80}}}' | getid)
curl -sf -o /dev/null -X PUT "${API}/original/${OID}/world" -H 'Content-Type: application/json' -d '{"name":"江城"}'
curl -sf -o /dev/null -X POST "${API}/original/${OID}/rules" -H 'Content-Type: application/json' -d '{"category":"社会规则","name":"老城区人情","description":"先讲人情","importance":4}'
CPID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"版本冒烟-二创","type":"CREATIVE"}' | getid)
CID=$(curl -sf -X POST "${API}/original/${OID}/create-creative" -H 'Content-Type: application/json' -d "{\"project_id\":\"${CPID}\",\"title\":\"潮汐·另一面\"}" | getid)

echo "== 2. 人物版本：改 DNA → 回滚"
CHAR=$(curl -sf -X POST "${API}/creative/${CID}/characters/inherit" -H 'Content-Type: application/json' -d "{\"source_character_id\":\"${SRC_CHAR}\",\"description\":\"继承来的描述\",\"weights\":{\"personality\":80}}" | getid)
curl -sf "${API}/creative/${CID}/characters" >/dev/null
check "继承后已有 v1" "1" "$(curl -sf "${API}/creative-characters/${CHAR}/versions" | field "d['total']")"
curl -sf -o /dev/null -X PUT "${API}/creative-characters/${CHAR}" -H 'Content-Type: application/json' -d '{"name":"林默（改）","description":"彻底改写过的描述"}'
check "v1 里是继承来的描述" "继承来的描述" "$(curl -sf "${API}/creative-characters/${CHAR}/versions/1" | field "d['payload']['description']")"
check "改完有 v2" "2" "$(curl -sf "${API}/creative-characters/${CHAR}/versions" | field "d['total']")"
check "v1 里是旧名字" "林默" "$(curl -sf "${API}/creative-characters/${CHAR}/versions/1" | field "d['payload']['name']")"
check "v2 里是新名字" "林默（改）" "$(curl -sf "${API}/creative-characters/${CHAR}/versions/2" | field "d['payload']['name']")"
check "恢复成功" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/creative-characters/${CHAR}/versions/1/restore")"
check "名字回到旧值" "林默" "$(curl -sf "${API}/creative-characters/${CHAR}" | field "d['name']")"
check "描述回到旧值" "继承来的描述" "$(curl -sf "${API}/creative-characters/${CHAR}" | field "d['description']")"
# 恢复前的"自动备份"只在内容与最新版不同时才新建；改动时已经存过 v2，所以这里得到 3 版（v1/v2/恢复结果），
# 这正是"不产生噪声版本"的预期行为。
check "恢复后共 3 版（自动去重）" "3" "$(curl -sf "${API}/creative-characters/${CHAR}/versions" | field "d['total']")"
check "内容没变时不产生噪声版本" "False" "$(curl -sf -X POST "${API}/creative-characters/${CHAR}/versions" -H 'Content-Type: application/json' -d '{}' | field "d['created']")"

echo "== 3. 世界观版本：改名字 → 回滚"
curl -sf -o /dev/null -X POST "${API}/creative/${CID}/world/inherit" -H 'Content-Type: application/json' -d '{"mode":"FULL"}'
check "继承后有 v1" "1" "$(curl -sf "${API}/creative/${CID}/world/versions" | field "d['total']")"
curl -sf -o /dev/null -X PUT "${API}/creative/${CID}/world" -H 'Content-Type: application/json' -d '{"name":"被改坏的世界名"}'
check "改完有 v2" "2" "$(curl -sf "${API}/creative/${CID}/world/versions" | field "d['total']")"
check "v1 记录旧世界名" "江城" "$(curl -sf "${API}/creative/${CID}/world/versions/1" | field "d['payload']['world']['name']")"
check "v1 记录了规则" "1" "$(curl -sf "${API}/creative/${CID}/world/versions/1" | field "len(d['payload']['rules'])")"
curl -sf -o /dev/null -X POST "${API}/creative/${CID}/world/versions/1/restore"
check "世界名已回滚" "江城" "$(curl -sf "${API}/creative/${CID}/world" | field "d['name']")"

echo "== 4. 大纲版本：改章节大纲 → 回滚"
VOL=$(curl -sf -X POST "${API}/creative/${CID}/volumes" -H 'Content-Type: application/json' -d '{"title":"第一卷","sequence":1}' | getid)
CH=$(curl -sf -X POST "${API}/creative/${CID}/chapters" -H 'Content-Type: application/json' -d "{\"volume_id\":\"${VOL}\",\"chapter_no\":1,\"title\":\"起风\",\"purpose\":\"原目的\"}" | getid)
check "建卷建章后有版本" "True" "$(curl -sf "${API}/creative/${CID}/outline/versions" | field "d['total'] >= 1")"
curl -sf -o /dev/null -X PUT "${API}/chapters/${CH}" -H 'Content-Type: application/json' -d '{"purpose":"被改坏的目的"}'
check "改完目的已生效" "被改坏的目的" "$(curl -sf "${API}/chapters/${CH}" | field "d['purpose']")"
LATEST=$(curl -sf "${API}/creative/${CID}/outline/versions" | field "d['items'][0]['version_no']")
OLD=""
for no in $(seq 1 "${LATEST}"); do
  P=$(curl -sf "${API}/creative/${CID}/outline/versions/${no}" | field "
next((c['purpose'] for c in d['payload']['chapters'] if c['id']=='${CH}'), '')")
  [ "$P" = "原目的" ] && OLD="$no" && break
done
check "找到了记录旧目的的版本" "True" "$([ -n "${OLD}" ] && echo True || echo False)"
check "恢复成功" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/creative/${CID}/outline/versions/${OLD}/restore")"
check "目的已回滚" "原目的" "$(curl -sf "${API}/chapters/${CH}" | field "d['purpose']")"

echo "== 5. 参数校验"
check "非法版本号（0）应 400" "400" "$(curl -s -o /dev/null -w '%{http_code}' "${API}/creative-characters/${CHAR}/versions/0")"
check "不存在的版本应 404" "404" "$(curl -s -o /dev/null -w '%{http_code}' "${API}/creative-characters/${CHAR}/versions/99")"
check "不存在的实体的版本列表为空" "0" "$(curl -sf "${API}/creative-characters/11111111-1111-1111-1111-111111111111/versions" | field "d['total']")"

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项"
[ "${FAIL}" -eq 0 ]
