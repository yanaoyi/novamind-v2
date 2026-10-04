#!/usr/bin/env bash
# 从 backend/.env 解析数据库连接串，供各脚本复用。
#
# 为什么要有这个文件：以前冒烟脚本把 `postgresql://novamind:novamind@...` 写死在脚本里，
# 一旦轮换数据库口令，脚本就会静默连不上；口令也不该散落在十几处。
#
# 导出：PSQL_URL（含口令的完整连接串）、PGPASSWORD（供直接调 psql 的脚本用）

_NM_REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ -z "${PSQL_URL:-}" ]]; then
  _nm_db=""
  if [[ -f "${_NM_REPO_ROOT}/backend/.env" ]]; then
    _nm_db="$(grep -E '^DATABASE_URL=' "${_NM_REPO_ROOT}/backend/.env" | head -1 | cut -d= -f2-)"
  fi
  _nm_db="${_nm_db:-${DATABASE_URL:-}}"
  # psql 更认 postgresql:// 前缀
  PSQL_URL="${_nm_db/postgres:\/\//postgresql://}"
fi
export PSQL_URL

if [[ -z "${PGPASSWORD:-}" && -n "${PSQL_URL:-}" ]]; then
  _nm_creds="${PSQL_URL#*://}"
  _nm_creds="${_nm_creds%%@*}"
  if [[ "${_nm_creds}" == *:* ]]; then
    PGPASSWORD="${_nm_creds#*:}"
    export PGPASSWORD
  fi
fi

unset _nm_db _nm_creds
