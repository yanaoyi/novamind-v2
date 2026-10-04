#!/usr/bin/env bash
# 让脚本里的所有 curl 自动带上访问令牌（P0 安全修复的配套）。
#
# 原理：curl 会从 $CURL_HOME/.curlrc 读取默认参数。这里把
#   header = "Authorization: Bearer <token>"
# 写进一个临时 .curlrc，于是各脚本里成百处 curl 调用一行都不用改，也不会把令牌写进命令行。
#
# 令牌来源优先级：环境变量 ADMIN_TOKEN → backend/.env 里的 ADMIN_TOKEN。
# 没拿到令牌时不中断（开发环境后端允许匿名），只在 stderr 提示一句。
#
# 用法：在被 source 的脚本靠前位置加一行
#   source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/api-auth.sh"

_NM_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_NM_TOKEN="${ADMIN_TOKEN:-}"
if [[ -z "$_NM_TOKEN" && -f "${_NM_ROOT}/backend/.env" ]]; then
  _NM_TOKEN="$(grep -E '^ADMIN_TOKEN=' "${_NM_ROOT}/backend/.env" | head -1 | cut -d= -f2-)"
fi

if [[ -n "$_NM_TOKEN" ]]; then
  # 放在仓库的运行时目录（已在 .gitignore 里，和 backend/.env 同级保管），
  # 不用 mktemp：脚本各自设了 EXIT trap，用临时目录反而容易被后设的 trap 顶掉而泄漏。
  _NM_CURL_HOME="${_NM_ROOT}/.run/curlrc"
  mkdir -p "${_NM_CURL_HOME}"
  chmod 700 "${_NM_CURL_HOME}"
  printf 'header = "Authorization: Bearer %s"\n' "$_NM_TOKEN" > "${_NM_CURL_HOME}/.curlrc"
  chmod 600 "${_NM_CURL_HOME}/.curlrc"
  export CURL_HOME="${_NM_CURL_HOME}"
else
  echo "提示：未找到 ADMIN_TOKEN（环境变量或 backend/.env），脚本将以匿名方式访问接口。" >&2
fi

unset _NM_TOKEN
