#!/usr/bin/env bash
# Phase 9 §9.6 总验收：固定测试剧本（"越写越懂"的端到端证明）。
#
# 剧本（与任务书 §9.6 一一对应）：
#   1. 造原著（3 章）→ 导入 → 索引自动构建
#   2. 新开二创 → 继承世界 → 加一条世界规则「夜禁」
#   3. 作者写 3 章：
#        第 1 章 埋伏笔：左肩中箭、伤口未愈；从祖宅第三块砖下取出青铜钥匙
#        第 2 章 回收伏笔：用青铜钥匙打开夹墙，拿到军需账册
#        第 3 章 预设矛盾：子时后出城而守军不拦（违反夜禁）+ 左肩中箭却单臂举三百斤石锁
#      → 每章应自动产生 facts / summary（§9.3）
#   4. AI 写第 4 章（目标里点名"青铜钥匙"）→ 断言 snapshot 里检索到了第 1 章的伏笔（§9.1+§9.2）
#   5. 对第 3 章做一致性检查 → 断言能指出这处预设矛盾（§9.4）
#
# 用法：bash scripts/acceptance-phase9.sh   （需要后端在 127.0.0.1:8080 + 已挂验证账号）
#
# 与 smoke-phase9.sh 的区别：smoke 验"每条链路通不通"，本脚本验"整条链合起来能干活"——
# 尤其是"第 4 章真的用上了第 1 章的伏笔"和"预设矛盾真的被指出来"这两件事。
set -uo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/db-url.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/cleanup.sh"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-test.sh"

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
WORK_DIR="$(mktemp -d)"
PASS=0
FAIL=0

PID=""; OID=""; CPID=""; CID=""
cleanup() {
  cleanup_project "${PID:-}"
  cleanup_project "${CPID:-}"
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

echo "== 1. 造原著并导入（第 1 章埋伏笔：左肩中箭 + 青铜钥匙）"
python3 - "$WORK_DIR/book.txt" <<'PY'
import sys
chapters = [
    ("第1章 雨夜归人", [
        "梅雨下了整夜。沈砚推开老宅的木门，左肩的箭伤被雨水泡得发白——那是三日前在渡口中的箭，至今未愈。",
        "他从祖宅第三块砖下取出青铜钥匙，钥匙上刻着一只断尾的鹤。",
    ]),
    ("第2章 夹墙", [
        "沈砚用青铜钥匙打开了夹墙。墙里是三十年前的军需账册，纸页发黄。",
        "账房先生站在门口，没有说话。",
    ]),
    ("第3章 夜行", [
        "夜色很深。老城的规矩是子时之后不得出城，违者按通敌论处。",
        "沈砚照旧推开城门，守军垂着眼睛，仿佛没有看见他。",
        "他左肩中箭不过三日，此刻却单臂举起三百斤的石锁，伤口半点也没碍着他。",
    ]),
]
with open(sys.argv[1], "w", encoding="utf-8") as f:
    for title, body in chapters:
        f.write(title + "\n\n")
        f.write("\n\n".join(body) + "\n\n")
PY

mkv PID "创建原著工程" /projects '{"name":"Phase9总验收-原著","type":"ORIGINAL"}' "d['id']"
mkv OID "创建原著作品" "/projects/${PID}/original" '{"title":"Phase9总验收原著"}' "d['id']"
if ! curl -s -o "${WORK_DIR}/import.json" -w '%{http_code}' -X POST "${API}/original/${OID}/import" \
  -F "file=@${WORK_DIR}/book.txt" | grep -q '^2'; then
  echo "  ✗ 原著导入失败：$(head -c 300 "${WORK_DIR}/import.json")" >&2
  exit 1
fi
CH=$(curl -s "${API}/original/${OID}/chapters?page=1&page_size=1" | field "d['total']")
check "原著切分 3 章" "3" "${CH}"
check "原著索引由导入自动触发并完成" "COMPLETED" "$(wait_index work_id "${OID}")"

echo "== 2. 新开二创作品 + 继承世界 + 一条世界规则（夜禁）"
mkv CPID "创建二创工程" /projects '{"name":"Phase9总验收-二创","type":"CREATIVE"}' "d['id']"
mkv CID "创建二创作品" "/original/${OID}/create-creative" \
  "{\"project_id\":\"${CPID}\",\"title\":\"Phase9总验收同人\",\"description\":\"总验收剧本\"}" "d['id']"
post_json "/creative/${CID}/world/inherit" '{"mode":"FULL"}' >/dev/null || {
  echo "== 中断（继承二创世界失败，已清理）" >&2
  exit 1
}
mkv RULE "创建世界规则（夜禁）" "/creative/${CID}/world/rules" \
  '{"category":"社会","name":"夜禁","description":"子时之后不得出城，违者按通敌论处","importance":5}' "d['id']"
mkv VOL "创建卷" "/creative/${CID}/volumes" '{"title":"第一卷","summary":"军需账册","sequence":1}' "d['id']"

echo "== 3. 作者写 3 章（第 3 章埋预设矛盾），记忆应逐章自动产生"
mkv CH1 "写第 1 章" "/creative/${CID}/chapters" \
  "{\"volume_id\":\"${VOL}\",\"chapter_no\":1,\"title\":\"雨夜归人\",\"summary\":\"沈砚回到老宅\",\"content\":\"梅雨下了整夜。沈砚推开老宅的木门，左肩的箭伤被雨水泡得发白——那是三日前在渡口中的箭，至今未愈。他从祖宅第三块砖下取出青铜钥匙，钥匙上刻着一只断尾的鹤。\",\"purpose\":\"交代伤情与青铜钥匙\",\"conflict\":\"旧伤未愈\",\"outcome\":\"钥匙到手\"}" \
  "d['id']"
mkv CH2 "写第 2 章" "/creative/${CID}/chapters" \
  "{\"volume_id\":\"${VOL}\",\"chapter_no\":2,\"title\":\"夹墙\",\"summary\":\"用钥匙打开夹墙\",\"content\":\"沈砚用青铜钥匙打开了夹墙。墙里是三十年前的军需账册，纸页发黄。账房先生站在门口，没有说话。\",\"purpose\":\"回收钥匙这条伏笔\",\"conflict\":\"账册来路不明\",\"outcome\":\"拿到军需账册\"}" \
  "d['id']"
mkv CH3 "写第 3 章（预设矛盾）" "/creative/${CID}/chapters" \
  "{\"volume_id\":\"${VOL}\",\"chapter_no\":3,\"title\":\"夜行\",\"summary\":\"沈砚夜里出城\",\"content\":\"夜色很深。老城的规矩是子时之后不得出城，违者按通敌论处。沈砚照旧推开城门，守军垂着眼睛，仿佛没有看见他。他左肩中箭不过三日，此刻却单臂举起三百斤的石锁，伤口半点也没碍着他。\",\"purpose\":\"把冲突推到台面上\",\"conflict\":\"夜禁与旧伤\",\"outcome\":\"沈砚离城\"}" \
  "d['id']"

SUMS=$(wait_sql "select count(*) from chapter_summaries where chapter_id in ('${CH1}','${CH2}','${CH3}')" "3" 240)
check "3 章摘要都自动产生（§9.3 回写）" "3" "${SUMS}"
FACTS=$(psqlq "select count(*) from memory_facts where creative_work_id='${CID}'")
check "记忆事实已入库（≥3 条）" "True" "$([ "${FACTS:-0}" -ge 3 ] && echo True || echo False)"
INJURY=$(psqlq "select count(*) from memory_facts where creative_work_id='${CID}' and fact like '%箭%'")
check "第 1 章的伤情进了记忆" "True" "$([ "${INJURY:-0}" -ge 1 ] && echo True || echo False)"
FCHUNKS=$(psqlq "select count(*) from chunks where work_id='${CID}' and ref_kind in ('chapter','memory_fact','chapter_summary','world_rule')")
check "章节/记忆/规则都已进检索索引" "True" "$([ "${FCHUNKS:-0}" -ge 6 ] && echo True || echo False)"

echo "== 4. AI 写第 4 章 → 上下文里应带回第 1 章的伏笔（青铜钥匙）"
mkv CH4 "创建第 4 章" "/creative/${CID}/chapters" \
  "{\"volume_id\":\"${VOL}\",\"chapter_no\":4,\"title\":\"暗格\",\"summary\":\"他要用青铜钥匙打开第三块砖下的暗格\",\"purpose\":\"让沈砚用青铜钥匙打开祖宅第三块砖下的暗格，取出一封旧信；此时他的左肩箭伤仍未痊愈\",\"conflict\":\"督军的人已经盯上祖宅\",\"outcome\":\"旧信到手，伤情拖重\"}" \
  "d['id']"
mkv TASK "触发写本章" "/chapters/${CH4}/generate" \
  '{"target_words":300,"instruction":"克制的短句；呼应第 1 章的青铜钥匙与左肩箭伤"}' "d['id']"
check "写第 4 章任务完成" "COMPLETED" "$(wait_task "${TASK}" 300)"

SNAPS_ID=$(curl -s "${API}/chapters/${CH4}/snapshots" | field "d['items'][0]['id'] if d['items'] else ''")
check "第 4 章落了上下文快照" "True" "$([[ -n "${SNAPS_ID}" ]] && echo True || echo False)"
DETAIL=$(curl -s "${API}/snapshots/${SNAPS_ID}")
RECALL=$(echo "${DETAIL}" | field "d['snapshot'].get('retrieved_creative','')")
check "第 4 章上下文带回了第 1 章的青铜钥匙" "True" "$([[ "${RECALL}" == *青铜钥匙* ]] && echo True || echo False)"
FROM_CH1=$(echo "${DETAIL}" | field "any(s.get('ref_id')=='${CH1}' for s in d['snapshot'].get('retrieved_sources',[]))")
check "检索来源能指到第 1 章（可追溯）" "True" "${FROM_CH1}"
MODELV=$(echo "${DETAIL}" | field "d['snapshot'].get('model','')+' / '+d['snapshot'].get('prompt_version','')")
echo "     快照记：${MODELV}"

echo "== 5. 一致性检查（第 3 章）→ 应指出预设矛盾"
post_json "/creative/${CID}/consistency/check" "{\"chapter_ids\":[\"${CH3}\"]}" >/dev/null || {
  echo "== 中断（一致性检查接口失败，已清理）" >&2
  exit 1
}
ISSUES=$(wait_sql_ge "select count(*) from consistency_issues where creative_work_id='${CID}' and deleted_at is null" 1 240)
check "一致性检查指出了问题（预设矛盾）" "True" "$([ "${ISSUES:-0}" -ge 1 ] && echo True || echo False)"
echo "     系统指出的问题："
psqlq "select '       · ['||severity||'/'||type||'] '||left(description, 60) from consistency_issues where creative_work_id='${CID}' and deleted_at is null order by created_at limit 5"

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项"
[ "${FAIL}" -eq 0 ]
