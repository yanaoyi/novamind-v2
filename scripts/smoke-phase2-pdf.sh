#!/usr/bin/env bash
# PDF 导入冒烟：正常中文 PDF / 扫描件（无文本层）/ 空密码加密 PDF 三条路径。
#
# 用 fpdf2 现场生成 PDF（第三方实现，不是我们自己的代码产物），
# 这样"能不能读别人的 PDF"才算真验证过。
#
# 用法：bash scripts/smoke-phase2-pdf.sh   （需要后端在 127.0.0.1:8080 运行）
set -uo pipefail

API="${API_BASE:-http://127.0.0.1:8080/api/v1}"
PSQL_URL="postgresql://novamind:novamind@127.0.0.1:5432/novamind"
WORK_DIR="$(mktemp -d)"
FONT="${PDF_CJK_FONT:-/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc}"

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
psqlq() { psql "$PSQL_URL" -tAc "$1"; }

PID=""; OID=""
cleanup() {
  if [ -n "${OID}" ]; then
    psqlq "delete from original_chapters where original_work_id='${OID}'" >/dev/null 2>&1
    psqlq "delete from original_works where id='${OID}'" >/dev/null 2>&1
  fi
  [ -n "${PID}" ] && psqlq "delete from files where project_id='${PID}'" >/dev/null 2>&1
  [ -n "${PID}" ] && psqlq "delete from projects where id='${PID}'" >/dev/null 2>&1
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

if [ ! -f "${FONT}" ]; then
  echo "缺少中文字体 ${FONT}，跳过 PDF 冒烟" >&2
  exit 0
fi

echo "== 0. 生成测试用 PDF（fpdf2）"
python3 - "${WORK_DIR}" "${FONT}" <<'PY'
import sys
from fpdf import FPDF
import fitz

work, font = sys.argv[1], sys.argv[2]

def new_pdf(font_path=None):
    pdf = FPDF()
    if font_path:
        pdf.add_font("cjk", "", font_path)
        pdf.set_font("cjk", size=14)
    return pdf

# 1) 三章中文小说
pdf = new_pdf(font)
for title, body in [
    ("第一章 初遇", "林默站在月台上，雨停了。她把信折好，放进外套内袋。"),
    ("第二章 旧城", "老城区的巷子很窄，墙上爬满了青苔。她在这里长大。"),
    ("第三章 对峙", "“你早就知道了。”陈述没有回头，声音很轻。"),
]:
    pdf.add_page()
    pdf.cell(0, 10, title, new_x="LMARGIN", new_y="NEXT")
    pdf.multi_cell(0, 8, body)
pdf.output(f"{work}/novel.pdf")

# 2) 只有图片的"扫描件"
pix = fitz.Pixmap(fitz.csRGB, fitz.IRect(0, 0, 400, 500))
pix.clear_with(230)
pix.save(f"{work}/scan.png")
scan = FPDF()
scan.add_page()
scan.image(f"{work}/scan.png", x=10, y=10, w=180)
scan.output(f"{work}/scanned.pdf")

# 3) 空用户密码 + 所有者密码（阅读器能直接打开，字节流是加密的）
enc = new_pdf(font)
enc.add_page()
enc.cell(0, 10, "第一章 密信", new_x="LMARGIN", new_y="NEXT")
enc.multi_cell(0, 8, "这封信是用所有者密码保护的，用户密码为空。")
enc.set_encryption(owner_password="boss-only", user_password="")
enc.output(f"{work}/encrypted.pdf")
print("   生成完毕")
PY
ls -la "${WORK_DIR}" | tail -5

echo "== 1. 中文 PDF 导入（章节识别 + 字数）"
PID=$(curl -sf -X POST "${API}/projects" -H 'Content-Type: application/json' -d '{"name":"PDF 冒烟","type":"ORIGINAL"}' | getid)
OID=$(curl -sf -X POST "${API}/projects/${PID}/original" -H 'Content-Type: application/json' -d '{"title":"潮汐"}' | getid)
check "导入返回 200" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/original/${OID}/import" -F "file=@${WORK_DIR}/novel.pdf")"
check "识别出 3 章" "3" "$(curl -sf "${API}/original/${OID}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["chapter_count"])')"
check "来源标记为 PDF" "PDF" "$(curl -sf "${API}/original/${OID}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["source_type"])')"
check "章节标题正确" "第一章 初遇" "$(curl -sf "${API}/original/${OID}/chapters" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["items"][0]["title"])')"
check "正文抽到中文（无乱码）" "True" "$(curl -sf "${API}/original/${OID}/chapters/1" | python3 -c '
import sys,json
c=json.load(sys.stdin)["data"]["content"]
print("True" if "林默站在月台上" in c else "False")')"

echo "== 2. 扫描件（没有文本层）要给出可操作提示"
CODE=$(curl -s -o "${WORK_DIR}/err.json" -w '%{http_code}' -X POST "${API}/original/${OID}/import" -F "file=@${WORK_DIR}/scanned.pdf")
check "返回 400" "400" "${CODE}"
check "提示是扫描件/需要 OCR" "True" "$(python3 -c '
import json
msg = json.load(open("'"${WORK_DIR}"'/err.json"))["error"]["message"]
print("True" if ("扫描件" in msg or "文本层" in msg) else "False")')"

echo "== 3. 空密码加密 PDF 要能正常读（不能被误判成打不开）"
check "导入返回 200" "200" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "${API}/original/${OID}/import" -F "file=@${WORK_DIR}/encrypted.pdf")"
check "加密 PDF 的正文也抽出来了" "True" "$(curl -sf "${API}/original/${OID}/chapters/1" | python3 -c '
import sys,json
c=json.load(sys.stdin)["data"]["content"]
print("True" if "所有者密码保护" in c else "False")')"

echo "== 4. 导入后章节数被替换而不是累加"
check "重新导入后仍是 1 章" "1" "$(curl -sf "${API}/original/${OID}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["chapter_count"])')"

echo
echo "== 结果：通过 ${PASS} 项，失败 ${FAIL} 项"
[ "${FAIL}" -eq 0 ]
