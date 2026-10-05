#!/usr/bin/env bash
# 一次「小版本修订」的完整流程（BOSS 2026-10-05 定的节奏）：
#
#   提出问题 → 改代码 → 自动测试 → 测试通过 → git commit → push → 形成一个"小版本修订"
#
# 本脚本负责后半段：跑检查 → 提交 → 推送 → 打版本号（tag）。任何一步失败就停下，
# 不产生半成品提交，也不打 tag。
#
# 用法：
#   bash scripts/ship.sh -m "修：xxx"                  # 跑标准检查（后端 + 前端）
#   bash scripts/ship.sh -m "修：xxx" --with-smoke     # 额外跑端到端冒烟（需要服务在跑）
#   bash scripts/ship.sh -m "修：xxx" --fast           # 只跑编译与单测，跳过前端全量
#   bash scripts/ship.sh -m "修：xxx" --dry-run        # 只跑检查，不提交不打 tag
#
# 版本号约定：v0.1.N（N 自增），语义是"一次小版本修订"；tag 是附注 tag，附上提交说明。
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export PATH="${HOME}/.local/go/bin:${PATH}"

MESSAGE=""
WITH_SMOKE=0
FAST=0
DRY_RUN=0
PATHS=""
while [ $# -gt 0 ]; do
  case "$1" in
    -m|--message) MESSAGE="${2:-}"; shift 2 ;;
    --paths) PATHS="${2:-}"; shift 2 ;;
    --with-smoke) WITH_SMOKE=1; shift ;;
    --fast) FAST=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) sed -n '2,20p' "$0"; exit 0 ;;
    *) echo "未知参数：$1" >&2; exit 2 ;;
  esac
done

if [ -z "${MESSAGE}" ]; then
  echo "错误：必须给提交说明，例如 bash scripts/ship.sh -m \"修：写本章偶发空正文\"" >&2
  exit 2
fi

PASS=0
FAIL=0
step() { echo; echo "==> $1"; }
ok()   { echo "    ✓ $1"; PASS=$((PASS + 1)); }
bad()  { echo "    ✗ $1"; FAIL=$((FAIL + 1)); }

step "1/6 后端：格式 / 编译 / vet / 单测"
( cd "${REPO_ROOT}/backend" && gofmt -l . > /tmp/nm-gofmt.txt 2>&1 )
if [ -s /tmp/nm-gofmt.txt ]; then
  bad "gofmt 有未格式化文件：$(tr '\n' ' ' < /tmp/nm-gofmt.txt)"
else
  ok "gofmt 干净"
fi
if ( cd "${REPO_ROOT}/backend" && go build ./... && go vet ./... ); then
  ok "go build + vet 通过"
else
  bad "go build / vet 失败"
fi
if ( cd "${REPO_ROOT}/backend" && go test ./... -count=1 > /tmp/nm-gotest.txt 2>&1 ); then
  ok "go test 全绿（$(grep -c '^ok' /tmp/nm-gotest.txt) 个包）"
else
  bad "go test 失败（详见 /tmp/nm-gotest.txt）"
fi

step "2/6 前端：类型检查 / 单测"
if ( cd "${REPO_ROOT}/frontend" && npx tsc -b ); then
  ok "tsc 通过"
else
  bad "tsc 失败"
fi
if [ "${FAST}" -eq 0 ]; then
  if ( cd "${REPO_ROOT}/frontend" && npx vitest run > /tmp/nm-vitest.txt 2>&1 ); then
    ok "vitest 全绿（$(grep -oE 'Tests +[0-9]+ passed' /tmp/nm-vitest.txt | head -1)）"
  else
    bad "vitest 失败（详见 /tmp/nm-vitest.txt）"
  fi
else
  echo "    -（--fast：跳过前端全量测试）"
fi

step "3/6 端到端冒烟"
if [ "${WITH_SMOKE}" -eq 1 ]; then
  if ( cd "${REPO_ROOT}" && bash scripts/smoke-phase8-outline.sh > /tmp/nm-smoke.txt 2>&1 ); then
    ok "大纲冒烟通过（$(grep -oE '通过 [0-9]+ 项' /tmp/nm-smoke.txt | tail -1)）"
  else
    bad "冒烟失败（详见 /tmp/nm-smoke.txt）"
  fi
else
  echo "    -（未加 --with-smoke：跳过；改了后端核心逻辑时建议加上）"
fi

if [ "${FAIL}" -gt 0 ]; then
  echo
  echo "==> 检查未通过（${FAIL} 项失败），已中止：不提交、不推送、不打 tag。"
  exit 1
fi
echo
echo "==> 检查全部通过（${PASS} 项）"

step "4/6 提交"
cd "${REPO_ROOT}"
CHANGES="$(git status --porcelain | wc -l | tr -d ' ')"
if [ "${DRY_RUN}" -eq 1 ]; then
  echo "    -（--dry-run：跳过提交/推送/tag）"
  echo "    待提交改动：${CHANGES} 个文件"
  exit 0
fi
if [ "${CHANGES}" -eq 0 ]; then
  echo "    -（没有未提交改动，跳过提交）"
else
  if [ -n "${PATHS}" ]; then
    # 只暂存点名的路径：一个提交一个主题（Git 规则第 2 条）
    # shellcheck disable=SC2086
    git add ${PATHS//,/ }
    ok "只暂存指定路径：${PATHS}"
  else
    git add -A
    echo "    提示：本次提交涉及 ${CHANGES} 个文件。若它们不属于同一个问题，"
    echo "          请改用 --paths <路径,路径> 分开提交（仓库规则：一个 commit 只解决一个主要问题）。"
  fi
  git commit -q -m "${MESSAGE}"
  ok "已提交：$(git log --oneline -1)"
fi

step "5/6 推送"
if git push origin HEAD > /tmp/nm-push.txt 2>&1; then
  ok "已推送到 $(git rev-parse --abbrev-ref HEAD)"
else
  bad "推送失败（详见 /tmp/nm-push.txt）"; exit 1
fi

step "6/6 打小版本号"
LAST="$(git tag --list 'v0.1.*' --sort=-v:refname | head -1)"
if [ -z "${LAST}" ]; then
  NEXT="v0.1.0"
else
  NEXT="v0.1.$(( ${LAST##*.} + 1 ))"
fi
git tag -a "${NEXT}" -m "${MESSAGE}"
git push origin "${NEXT}" > /tmp/nm-tagpush.txt 2>&1
ok "小版本修订：${NEXT}（上一版：${LAST:-无}）"

echo
echo "==> 完成：${NEXT} 已发布到 GitHub"
