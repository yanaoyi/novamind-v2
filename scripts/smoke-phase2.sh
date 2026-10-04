#!/usr/bin/env bash
# Phase 2 原著导入链路冒烟测试（需要后端在 127.0.0.1:8080 运行、数据库可用）。
#
# 用法：bash scripts/smoke-phase2.sh
#
# 覆盖：创建工程 → 创建原著 → 导入 GBK 中文原文 → 详情/章节目录/章节正文
#       → 重复导入幂等 → 五类错误码 → 清理（按外键顺序）
set -uo pipefail

# 接口访问令牌：curl 通过 $CURL_HOME/.curlrc 自动带上 Authorization 头（P0 安全修复配套）
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/db-url.sh"

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
DB="${DATABASE_URL:-${PSQL_URL}}"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="$(mktemp -d)"

PASS=0
FAIL=0
check() { # check <描述> <期望> <实际>
  if [ "$2" = "$3" ]; then
    echo "  ✓ $1 ($3)"
    PASS=$((PASS + 1))
  else
    echo "  ✗ $1：期望 $2，实际 $3"
    FAIL=$((FAIL + 1))
  fi
}
getid() { python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["id"])'; }
psqlq() { psql "$PSQL_URL" -tAc "$1"; }

# ---------- 准备样本 ----------
python3 - "$WORK_DIR" <<'PY'
import sys
work = sys.argv[1]
parts = ["《暗涌》", "", "作者：测试", ""]
for i, t in enumerate(["第一章 初遇", "第二章 裂痕", "第三章 旧信", "第四章 归途"], 1):
    parts.append(t)
    parts.append("")
    parts.extend(f"这是第{i}章的正文第{j}句，林默在月台上等待。" for j in range(1, 21))
    parts.append("")
text = "\n".join(parts)
open(f"{work}/gbk.txt", "w", encoding="gb18030").write(text)
open(f"{work}/utf8.txt", "w", encoding="utf-8").write(text)
PY

echo "== 准备样本完成：$WORK_DIR"

# ---------- 清理口袋（保证异常退出也能清干净） ----------
PID=""
CPID=""
OID=""
cleanup() {
  [ -n "$OID" ] && psqlq "delete from timeline_events where timeline_id in (select id from original_timelines where original_work_id='$OID')" >/dev/null 2>&1
  [ -n "$OID" ] && psqlq "delete from original_timelines where original_work_id='$OID'" >/dev/null 2>&1
  [ -n "$OID" ] && psqlq "delete from plot_arcs where original_work_id='$OID'" >/dev/null 2>&1
  [ -n "$OID" ] && psqlq "delete from original_events where original_work_id='$OID'" >/dev/null 2>&1
  [ -n "$OID" ] && psqlq "delete from factions where world_id in (select id from original_worlds where original_work_id='$OID')" >/dev/null 2>&1
  [ -n "$OID" ] && psqlq "delete from locations where world_id in (select id from original_worlds where original_work_id='$OID')" >/dev/null 2>&1
  [ -n "$OID" ] && psqlq "delete from world_rules where world_id in (select id from original_worlds where original_work_id='$OID')" >/dev/null 2>&1
  [ -n "$OID" ] && psqlq "delete from original_worlds where original_work_id='$OID'" >/dev/null 2>&1
  [ -n "$OID" ] && psqlq "delete from character_relationships where original_work_id='$OID'" >/dev/null 2>&1
  [ -n "$OID" ] && psqlq "delete from original_characters where original_work_id='$OID'" >/dev/null 2>&1
  [ -n "$OID" ] && psqlq "delete from original_chapters where original_work_id='$OID'" >/dev/null 2>&1
  [ -n "$OID" ] && psqlq "delete from original_works where id='$OID'" >/dev/null 2>&1
  [ -n "$PID" ] && psqlq "delete from files where project_id='$PID'" >/dev/null 2>&1
  [ -n "$CPID" ] && psqlq "delete from files where project_id='$CPID'" >/dev/null 2>&1
  for id in "$PID" "$CPID"; do
    [ -n "$id" ] && psqlq "delete from projects where id='$id'" >/dev/null 2>&1
  done
  [ -n "$PID" ] && rm -rf "${PROJECT_ROOT}/backend/data/uploads/${PID}"
  [ -n "$CPID" ] && rm -rf "${PROJECT_ROOT}/backend/data/uploads/${CPID}"
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

# ---------- 1. 建工程与原著 ----------
echo "== 1. 创建 ORIGINAL 工程与原著"
PID=$(curl -sf -X POST "$API/projects" -H 'Content-Type: application/json' \
  -d '{"name":"原著导入冒烟","type":"ORIGINAL"}' | getid)
check "创建工程" "36" "${#PID}"

OID=$(curl -sf -X POST "$API/projects/$PID/original" -H 'Content-Type: application/json' \
  -d '{"title":"暗涌","author":"测试作者","description":"GBK 编码样本"}' | getid)
check "创建原著" "36" "${#OID}"

# ---------- 2. 导入 GBK 原文 ----------
echo "== 2. 导入 GBK 编码原文"
IMPORT=$(curl -sf -X POST "$API/original/$OID/import" -F "file=@${WORK_DIR}/gbk.txt")
check "识别编码" "GB18030" "$(echo "$IMPORT" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["encoding"])')"
check "章节数" "5" "$(echo "$IMPORT" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["chapter_count"])')"

echo "== 3. 原著详情"
check "状态" "PARSED" "$(curl -sf "$API/original/$OID" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["status"])')"
check "来源类型" "TXT" "$(curl -sf "$API/original/$OID" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["source_type"])')"

echo "== 4. 章节目录与正文"
check "目录 total" "5" "$(curl -sf "$API/original/$OID/chapters?page=1&page_size=2" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["total"])')"
check "第 2 章标题" "第一章 初遇" "$(curl -sf "$API/original/$OID/chapters/2" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["title"])')"

echo "== 5. 重复导入（幂等）"
REIMPORT=$(curl -sf -X POST "$API/original/$OID/import" -F "file=@${WORK_DIR}/utf8.txt")
check "重导后章节数" "5" "$(echo "$REIMPORT" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["chapter_count"])')"
check "库中未删除章节数" "5" "$(psqlq "select count(*) from original_chapters where original_work_id='$OID' and deleted_at is null")"

echo "== 6. 错误场景"
check "PDF 导入" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/import" -F "file=@/etc/hostname;filename=book.pdf")"
check "重复创建原著" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/projects/$PID/original" -H 'Content-Type: application/json' -d '{"title":"再建一个"}')"
CPID=$(curl -sf -X POST "$API/projects" -H 'Content-Type: application/json' -d '{"name":"二创工程冒烟","type":"CREATIVE"}' | getid)
check "给 CREATIVE 工程建原著" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/projects/$CPID/original" -H 'Content-Type: application/json' -d '{"title":"不该成功"}')"
check "不存在的原著" "404" "$(curl -s -o /dev/null -w '%{http_code}' "$API/original/00000000-0000-7000-8000-000000000000")"
check "不存在的章节" "404" "$(curl -s -o /dev/null -w '%{http_code}' "$API/original/$OID/chapters/99")"

# ---------- 7. 人物与人物 DNA ----------
echo "== 7. 人物与人物 DNA"
CHAR_A=$(curl -sf -X POST "$API/original/$OID/characters" -H 'Content-Type: application/json' -d '{
  "name":"林默","aliases":[" 小默 ","小默"],"role":"主角","gender":"女","age":"24",
  "personality":"外冷内热","importance":5,
  "dna":{"personality":{"text":"克制而敏锐","weight":90},"values":{"text":"守信","weight":80}}
}')
CID_A=$(echo "$CHAR_A" | getid)
check "创建人物 A" "36" "${#CID_A}"
check "别名去重（去掉空白与重复）" "['小默']" "$(echo "$CHAR_A" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["aliases"])')"
check "DNA 权重保留" "90" "$(echo "$CHAR_A" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["dna"]["personality"]["weight"])')"

CID_B=$(curl -sf -X POST "$API/original/$OID/characters" -H 'Content-Type: application/json' \
  -d '{"name":"陈述","role":"配角","importance":3}' | getid)
check "创建人物 B" "36" "${#CID_B}"

check "同名人物（应 409）" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/characters" -H 'Content-Type: application/json' -d '{"name":"林默"}')"
check "DNA 权重 120（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/characters" -H 'Content-Type: application/json' -d '{"name":"越界者","dna":{"personality":{"text":"x","weight":120}}}')"
check "人物列表 total" "2" "$(curl -sf "$API/original/$OID/characters" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["total"])')"
check "关键字搜索" "1" "$(curl -sf "$API/original/$OID/characters?keyword=林" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["total"])')"

UPDATED=$(curl -sf -X PUT "$API/characters/$CID_A" -H 'Content-Type: application/json' -d '{
  "name":"林默","role":"主角","importance":5,
  "dna":{"personality":{"text":"克制而敏锐","weight":70}}
}')
check "更新 DNA 权重" "70" "$(echo "$UPDATED" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["dna"]["personality"]["weight"])')"

# ---------- 8. 人物关系 ----------
echo "== 8. 人物关系"
REL=$(curl -sf -X POST "$API/original/$OID/relationships" -H 'Content-Type: application/json' \
  -d "{\"source_character_id\":\"$CID_A\",\"target_character_id\":\"$CID_B\",\"relation_type\":\"friend\",\"strength\":80,\"description\":\"自幼相识\"}")
RID=$(echo "$REL" | getid)
check "创建关系" "36" "${#RID}"
check "关系强度" "80" "$(echo "$REL" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["strength"])')"
check "重复关系（应 409）" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/relationships" -H 'Content-Type: application/json' -d "{\"source_character_id\":\"$CID_A\",\"target_character_id\":\"$CID_B\",\"relation_type\":\"friend\"}")"
check "自己连自己（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/relationships" -H 'Content-Type: application/json' -d "{\"source_character_id\":\"$CID_A\",\"target_character_id\":\"$CID_A\",\"relation_type\":\"friend\"}")"
PROJ2=$(curl -sf -X POST "$API/projects" -H 'Content-Type: application/json' -d '{"name":"跨原著冒烟","type":"ORIGINAL"}' | getid)
OID2=$(curl -sf -X POST "$API/projects/$PROJ2/original" -H 'Content-Type: application/json' -d '{"title":"另一部"}' | getid)
CID_C=$(curl -sf -X POST "$API/original/$OID2/characters" -H 'Content-Type: application/json' -d '{"name":"异书人物"}' | getid)
check "跨原著建立关系" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/relationships" -H 'Content-Type: application/json' -d "{\"source_character_id\":\"$CID_A\",\"target_character_id\":\"$CID_C\",\"relation_type\":\"friend\"}")"
check "关系列表 total" "1" "$(curl -sf "$API/original/$OID/relationships" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["total"])')"
check "更新关系强度" "55" "$(curl -sf -X PUT "$API/relationships/$RID" -H 'Content-Type: application/json' -d '{"strength":55}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["strength"])')"
check "删除人物（应 200）" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "$API/characters/$CID_B")"
check "人物删除后其关系也消失" "0" "$(curl -sf "$API/original/$OID/relationships" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["total"])')"

# ---------- 9. 世界观：世界 / 规则 / 地点 / 势力 ----------
echo "== 9. 世界观"
WORLD=$(curl -sf -X PUT "$API/original/$OID/world" -H 'Content-Type: application/json' \
  -d '{"name":"九州","description":"架空大陆，灵力是唯一超自然力量"}')
check "保存世界设定" "九州" "$(echo "$WORLD" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["name"])')"
check "初始规则数" "0" "$(echo "$WORLD" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["rule_count"])')"

RULE=$(curl -sf -X POST "$API/original/$OID/rules" -H 'Content-Type: application/json' \
  -d '{"category":"力量体系","name":"灵力不可凭空产生","description":"必须从灵脉汲取","importance":5}')
RID_W=$(echo "$RULE" | getid)
check "新增规则" "36" "${#RID_W}"
check "重复规则（应 409）" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/rules" -H 'Content-Type: application/json' -d '{"name":"灵力不可凭空产生"}')"
check "规则重要度 9（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/rules" -H 'Content-Type: application/json' -d '{"name":"越界规则","importance":9}')"

PARENT=$(curl -sf -X POST "$API/original/$OID/locations" -H 'Content-Type: application/json' \
  -d '{"name":"青云城","type":"城市"}' | getid)
check "新增顶层地点" "36" "${#PARENT}"
CHILD=$(curl -sf -X POST "$API/original/$OID/locations" -H 'Content-Type: application/json' \
  -d "{\"name\":\"青云城南门\",\"type\":\"城门\",\"parent_location_id\":\"$PARENT\"}" | getid)
check "新增子地点" "36" "${#CHILD}"
check "自己当自己的上级（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$API/locations/$PARENT" -H 'Content-Type: application/json' -d "{\"name\":\"青云城\",\"parent_location_id\":\"$PARENT\"}")"
check "层级成环（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$API/locations/$PARENT" -H 'Content-Type: application/json' -d "{\"name\":\"青云城\",\"parent_location_id\":\"$CHILD\"}")"

# 跨世界引用：用另一部原著的的地点当上级
OTHER_LOC=$(curl -sf -X POST "$API/original/$OID2/locations" -H 'Content-Type: application/json' -d '{"name":"异界城池"}' | getid)
check "跨世界上级（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$API/locations/$PARENT" -H 'Content-Type: application/json' -d "{\"name\":\"青云城\",\"parent_location_id\":\"$OTHER_LOC\"}")"

FACTION=$(curl -sf -X POST "$API/original/$OID/factions" -H 'Content-Type: application/json' \
  -d '{"name":"天枢阁","type":"宗门","goals":"垄断灵矿","relationships":"与玄冥教敌对"}' | getid)
check "新增势力" "36" "${#FACTION}"
check "重复势力（应 409）" "409" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/factions" -H 'Content-Type: application/json' -d '{"name":"天枢阁"}')"

SUMMARY=$(curl -sf "$API/original/$OID/world")
check "世界统计-规则" "1" "$(echo "$SUMMARY" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["rule_count"])')"
check "世界统计-地点" "2" "$(echo "$SUMMARY" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["location_count"])')"
check "世界统计-势力" "1" "$(echo "$SUMMARY" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["faction_count"])')"
check "更新规则重要度" "4" "$(curl -sf -X PUT "$API/world-rules/$RID_W" -H 'Content-Type: application/json' -d '{"name":"灵力不可凭空产生","category":"力量体系","importance":4}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["importance"])')"
check "删除势力（应 200）" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "$API/factions/$FACTION")"
check "删除后势力数" "0" "$(curl -sf "$API/original/$OID/world" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["faction_count"])')"

# ---------- 10. 事件 / 时间线 / 剧情弧 ----------
echo "== 10. 事件 / 时间线 / 剧情弧"
EVENT=$(curl -sf -X POST "$API/original/$OID/events" -H 'Content-Type: application/json' -d "{
  \"title\":\"母亲的意外\",\"description\":\"糖厂夜里的那场火\",
  \"chapter_no\":3,\"time_order\":10,\"participants\":[\"$CID_A\"],
  \"location_text\":\"老糖厂\",\"consequences\":\"林默决定回江城\",\"importance\":5}")
EID=$(echo "$EVENT" | getid)
check "新增事件" "36" "${#EID}"
check "事件重要度" "5" "$(echo "$EVENT" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["importance"])')"
check "事件参与者" "1" "$(echo "$EVENT" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["participants"]))')"
check "事件章节号" "3" "$(echo "$EVENT" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["chapter_no"])')"
check "跨原著参与者（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/events" -H 'Content-Type: application/json' -d "{\"title\":\"越界事件\",\"participants\":[\"$CID_C\"]}")"
check "空标题（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/events" -H 'Content-Type: application/json' -d '{"title":"   "}')"
check "事件列表 total" "1" "$(curl -sf "$API/original/$OID/events" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["total"])')"

check "初始时间线条目" "0" "$(curl -sf "$API/original/$OID/timeline" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["entries"]))')"
ORDERED=$(curl -sf -X PUT "$API/original/$OID/timeline" -H 'Content-Type: application/json' \
  -d "{\"items\":[{\"event_id\":\"$EID\",\"time_label\":\"三年前·梅雨季\",\"duration\":\"两天\"}]}")
check "加入时间线" "1" "$(echo "$ORDERED" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["entries"]))')"
check "时间标签" "三年前·梅雨季" "$(echo "$ORDERED" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["entries"][0]["time_label"])')"
check "条目序号从 1 开始" "1" "$(echo "$ORDERED" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["entries"][0]["sequence"])')"
check "重复设置（幂等）" "1" "$(curl -sf -X PUT "$API/original/$OID/timeline" -H 'Content-Type: application/json' -d "{\"items\":[{\"event_id\":\"$EID\"}]}" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["entries"]))')"
# 先在另一部原著里造一个事件，再尝试把它排进本原著的时间线
EID2=$(curl -sf -X POST "$API/original/$OID2/events" -H 'Content-Type: application/json' -d '{"title":"异界事件"}' | getid)
check "把别原著的事件排进时间线（应 404）" "404" "$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$API/original/$OID/timeline" -H 'Content-Type: application/json' -d "{\"items\":[{\"event_id\":\"$EID2\"}]}")"

ARC=$(curl -sf -X POST "$API/original/$OID/plot-arcs" -H 'Content-Type: application/json' \
  -d "{\"type\":\"main\",\"title\":\"老城改造之争\",\"summary\":\"从母亲意外到真相揭开\",\"start_event_id\":\"$EID\"}")
AID=$(echo "$ARC" | getid)
check "新增剧情弧" "36" "${#AID}"
check "剧情弧类型" "main" "$(echo "$ARC" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["type"])')"
check "非法剧情弧类型（应 400）" "400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/original/$OID/plot-arcs" -H 'Content-Type: application/json' -d '{"title":"坏线","type":"bogus"}')"
check "剧情弧列表 total" "1" "$(curl -sf "$API/original/$OID/plot-arcs" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["total"])')"
check "更新剧情弧标题" "老城改造之争（修订）" "$(curl -sf -X PUT "$API/plot-arcs/$AID" -H 'Content-Type: application/json' -d '{"title":"老城改造之争（修订）","type":"main"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["title"])')"
check "删除剧情弧（应 200）" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "$API/plot-arcs/$AID")"

check "删除事件（应 200）" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "$API/events/$EID")"
check "事件删除后时间线条目清空" "0" "$(curl -sf "$API/original/$OID/timeline" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["entries"]))')"

# 清理第二部原著
psqlq "delete from character_relationships where original_work_id='$OID2'" >/dev/null 2>&1
psqlq "delete from original_characters where original_work_id='$OID2'" >/dev/null 2>&1
psqlq "delete from original_works where id='$OID2'" >/dev/null 2>&1
psqlq "delete from projects where id='$PROJ2'" >/dev/null 2>&1

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项（清理由 trap 自动完成）"
[ "$FAIL" -eq 0 ]
