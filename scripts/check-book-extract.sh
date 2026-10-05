#!/usr/bin/env bash
# EPUB / MOBI 解析对照校验（2026-10-05 新增格式时建立）。
#
# 做法：在书库里找**同一本书的两种格式**（实测有 904 本同名 .mobi + .epub），
# 分别用同一份解析器抽取正文，再看 mobi 结果覆盖了 epub 结果的多少（char 级召回）。
# 两种格式互不依赖，所以能互相当基准 —— 与 check-pdf-extract.sh 用 PyMuPDF 当基准同理。
#
# 用法：
#   bash scripts/check-book-extract.sh [对照本数] [随机种子]
#   BOOK_DIR=/some/dir bash scripts/check-book-extract.sh 20 7
#
# 输出：逐本的召回率与字数、解析失败清单、汇总（同时报告纯 EPUB 的解析成功率）。
set -uo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BOOK_DIR="${BOOK_DIR:-/lzcapp/document/懒猫读书}"
LIMIT="${1:-12}"
SEED="${2:-20261005}"
EPUB_SCAN="${EPUB_SCAN:-60}"
WORK_DIR="$(mktemp -d)"
export PATH="${HOME}/.local/go/bin:${PATH}"

cleanup() { rm -rf "${WORK_DIR}"; }
trap cleanup EXIT

echo "== 构建 booktext 工具"
( cd "${PROJECT_ROOT}/backend" && GOFLAGS=-mod=mod GOPROXY=off go build -o "${WORK_DIR}/booktext" ./cmd/booktext ) || exit 1

echo "== 找同名双格式书（目录 ${BOOK_DIR}，随机取 ${LIMIT} 本，种子 ${SEED}）"
python3 - "${BOOK_DIR}" "${LIMIT}" "${SEED}" "${WORK_DIR}/booktext" "${EPUB_SCAN}" <<'PY'
import os, random, re, subprocess, sys

root, limit, seed, tool, epub_scan = (
    sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), sys.argv[4], int(sys.argv[5]),
)

mobi, epub = {}, {}
for dirpath, _, files in os.walk(root):
    for fn in files:
        stem, ext = os.path.splitext(fn)
        low = ext.lower()
        if low == ".mobi":
            mobi.setdefault(stem, os.path.join(dirpath, fn))
        elif low == ".epub":
            epub.setdefault(stem, os.path.join(dirpath, fn))

pairs = sorted(set(mobi) & set(epub))
print(f"   书库：mobi {len(mobi)} 本 / epub {len(epub)} 本 / 同名双格式 {len(pairs)} 本")
random.seed(seed)
sample = random.sample(pairs, min(limit, len(pairs)))


def extract(path):
    r = subprocess.run([tool, path], capture_output=True)
    if r.returncode != 0:
        return None, r.stderr.decode("utf-8", "replace").strip()
    return r.stdout.decode("utf-8", "replace"), ""


def norm(s):
    return re.sub(r"\s+", "", s)


def recall(base, probe, n=20, step=40):
    """base 的 n-gram 有多少出现在 probe 里（抽样，避免大书吃内存）。"""
    if len(base) < n:
        return 1.0 if base and base in probe else 0.0
    grams = {base[i:i + n] for i in range(0, len(base) - n, step)}
    if not grams:
        return 0.0
    hit = sum(1 for g in grams if g in probe)
    return hit / len(grams)


print("== 逐本对照（epub 结果当基准，看 mobi 覆盖多少）")
rows, fails = [], []
for stem in sample:
    epub_text, eerr = extract(epub[stem])
    mobi_text, merr = extract(mobi[stem])
    if eerr or merr:
        fails.append((stem, eerr or merr))
        continue
    e, m = norm(epub_text), norm(mobi_text)
    if not e:
        fails.append((stem, "epub 抽取为空"))
        continue
    if not m:
        fails.append((stem, "mobi 抽取为空"))
        continue
    r = recall(e, m)
    rows.append((stem, r, len(e), len(m)))
    flag = "✓" if r >= 0.95 else ("~" if r >= 0.80 else "✗")
    print(f"   {flag} {stem[:34]:<34} 召回 {r:5.1%}   epub {len(e):>7} 字 / mobi {len(m):>7} 字")

if rows:
    avg = sum(r for _, r, _, _ in rows) / len(rows)
    good = sum(1 for _, r, _, _ in rows if r >= 0.95)
    print(f"== 对照汇总：{len(rows)} 本，平均召回 {avg:.1%}，召回 ≥95% 的 {good}/{len(rows)}")

if fails:
    print(f"== 需要看的失败/异常（{len(fails)} 本）")
    for stem, err in fails[:10]:
        print(f"   · {stem[:34]:<34} {err[:110]}")

print(f"== 纯 EPUB 抽样解析（{epub_scan} 本，验覆盖率不是只有对照那几本）")
random.seed(seed + 1)
epub_sample = random.sample(sorted(epub), min(epub_scan, len(epub)))
ok = 0
epub_fails = []
for stem in epub_sample:
    text, err = extract(epub[stem])
    if err or not text.strip():
        epub_fails.append((stem, err or "抽出为空"))
        continue
    ok += 1
print(f"   成功 {ok}/{len(epub_sample)}")
for stem, err in epub_fails[:6]:
    print(f"   · {stem[:34]:<34} {err[:110]}")

sys.exit(0 if not fails and ok == len(epub_sample) else 1)
PY
