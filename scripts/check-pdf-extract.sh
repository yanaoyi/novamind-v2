#!/usr/bin/env bash
# PDF 解析质量对照校验：用 PyMuPDF 的抽取结果当基准，量化我们自己的解析器。
#
# 用法：
#   bash scripts/check-pdf-extract.sh [样本数] [随机种子]
#   PDF_DIR=/some/dir bash scripts/check-pdf-extract.sh 60 7
#
# 输出：逐文件字符召回率 + 汇总（基准本身没文本的扫描件单独归类，不算失败）。
set -uo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PDF_DIR="${PDF_DIR:-/lzcapp/document}"
LIMIT="${1:-40}"
SEED="${2:-20260928}"
WORK_DIR="$(mktemp -d)"
export PATH="${HOME}/.local/go/bin:${PATH}"

cleanup() { rm -rf "${WORK_DIR}"; }
trap cleanup EXIT

echo "== 构建 pdftext 工具"
( cd "${PROJECT_ROOT}/backend" && export GOFLAGS=-mod=mod GOPROXY=off && go build -o "${WORK_DIR}/pdftext" ./cmd/pdftext ) || exit 1

echo "== 抽样（目录 ${PDF_DIR}，样本 ${LIMIT}，种子 ${SEED}）"
mapfile -t FILES < <(python3 - "${PDF_DIR}" "${LIMIT}" "${SEED}" <<'PY'
import glob, os, random, sys
root, limit, seed = sys.argv[1], int(sys.argv[2]), int(sys.argv[3])
files = [p for p in glob.glob(os.path.join(root, "**", "*.pdf"), recursive=True) if os.path.getsize(p) > 4096]
files += [p for p in glob.glob(os.path.join(root, "**", "*.PDF"), recursive=True) if os.path.getsize(p) > 4096]
files = sorted(set(files))
random.seed(seed)
for p in random.sample(files, min(limit, len(files))):
    print(p)
PY
)
echo "   实取 ${#FILES[@]} 个文件"
[ "${#FILES[@]}" -eq 0 ] && { echo "没有可用样本" >&2; exit 1; }

mkdir -p "${WORK_DIR}/truth" "${WORK_DIR}/ours"

echo "== 基准抽取（PyMuPDF）"
python3 - "${WORK_DIR}" "${FILES[@]}" <<'PY'
import os, sys
import fitz
work = sys.argv[1]
paths = sys.argv[2:]
for i, p in enumerate(paths):
    try:
        doc = fitz.open(p)
        text = "".join(doc.get_page_text(n) for n in range(doc.page_count))
        doc.close()
    except Exception:
        text = ""
    with open(os.path.join(work, "truth", f"{i}.txt"), "w", encoding="utf-8") as fh:
        fh.write(text)
print(f"   完成 {len(paths)} 个")
PY

echo "== 我们的解析器（Go）"
HARD_FAIL=0
for i in "${!FILES[@]}"; do
  if ! "${WORK_DIR}/pdftext" "${FILES[$i]}" > "${WORK_DIR}/ours/${i}.txt" 2>"${WORK_DIR}/ours/${i}.err"; then
    HARD_FAIL=$((HARD_FAIL + 1))
    : > "${WORK_DIR}/ours/${i}.txt"
  else
    : > "${WORK_DIR}/ours/${i}.err"
  fi
done
echo "   返回错误的文件数：${HARD_FAIL}"

echo
echo "== 逐文件对照（recall = 基准里的字符有多少被我们抽到）"
python3 - "${WORK_DIR}" "${FILES[@]}" <<'PY'
import collections, os, sys
work = sys.argv[1]
paths = sys.argv[2:]

def chars(text):
    return collections.Counter(c for c in text if not c.isspace())

def text_likeness(s):
    """估算一段文本"像正常文字"的比例：CJK/全角/ASCII 记好，控制符与其他乱码记差。"""
    runes = [c for c in s if not c.isspace()]
    if not runes:
        return 0.0
    good = 0
    for r in runes:
        o = ord(r)
        if 0x4E00 <= o <= 0x9FFF or 0x3400 <= o <= 0x4DBF:   # CJK
            good += 1
        elif 0x3000 <= o <= 0x303F or 0xFF00 <= o <= 0xFFEF:  # CJK 标点 / 全角
            good += 1
        elif 0x20 <= o < 0x7F:                                 # ASCII 可打印
            good += 1
    return good / len(runes)

rows = []
agg_base = collections.Counter()
agg_ours = collections.Counter()
agg_inter = collections.Counter()
buckets = collections.Counter()

for i, p in enumerate(paths):
    truth_path = os.path.join(work, "truth", f"{i}.txt")
    ours_path = os.path.join(work, "ours", f"{i}.txt")
    t = open(truth_path, encoding="utf-8", errors="ignore").read() if os.path.exists(truth_path) else ""
    o = open(ours_path, encoding="utf-8", errors="ignore").read() if os.path.exists(ours_path) else ""
    err = open(os.path.join(work, "ours", f"{i}.err"), encoding="utf-8", errors="ignore").read().strip()
    tc, oc = chars(t), chars(o)
    inter = tc & oc
    recall = sum(inter.values()) / max(1, sum(tc.values()))
    precision = sum(inter.values()) / max(1, sum(oc.values()))
    if not t.strip():
        bucket = "基准无文本（扫描件/图片型）"
    elif text_likeness(t) < 0.8:
        # 基准自己就是乱码（ToUnicode 是空壳、方正私有编码等），两边没有可比性
        bucket = "基准本身是乱码（无可比性，需 OCR）"
    elif recall >= 0.98:
        bucket = "≥0.98 完全一致"
    elif recall >= 0.9:
        bucket = "0.90-0.98 基本一致"
    elif recall >= 0.5:
        bucket = "0.50-0.90 部分缺失"
    else:
        bucket = "<0.50 几乎没抽到"
    buckets[bucket] += 1
    agg_base += tc
    agg_ours += oc
    agg_inter += inter
    if t.strip() and text_likeness(t) >= 0.8:
        rows.append((recall, precision, len(t), len(o), os.path.basename(p)[:46], err[:60]))

rows.sort()
for recall, precision, tl, ol, name, err in rows[:12]:
    print(f"   {recall:5.3f} / P {precision:5.3f}  基准 {tl:7d} 字  我们 {ol:7d} 字  {name}  {err}")
if len(rows) > 12:
    print(f"   …… 其余 {len(rows) - 12} 个文件的召回率均不低于 {rows[11][0]:.3f}")

print()
print("== 汇总")
for k in sorted(buckets):
    print(f"   {k}：{buckets[k]} 个")
if agg_base:
    total = sum(agg_inter.values())
    print(f"   整体字符召回率：{total / sum(agg_base.values()):.4f}")
    print(f"   整体字符准确率：{total / max(1, sum(agg_ours.values())):.4f}")
PY
