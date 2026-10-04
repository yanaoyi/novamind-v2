#!/usr/bin/env bash
# 初始化 NovaMind 本地开发用的 PostgreSQL 账号与数据库。
#
# 幂等：角色/库已存在时会跳过创建、但会重置密码，因此可以反复执行。
#
# 用法（需要 root）：
#   sudo bash scripts/setup-local-db.sh
#   sudo bash scripts/setup-local-db.sh <用户名> <密码> <库名>
#
# 密码来源（按优先级）：
#   1) 第二个位置参数
#   2) 环境变量 NOVAMIND_DB_PASSWORD
#   3) backend/.env 里 DATABASE_URL 中已有的口令
# 刻意不再提供弱口令默认值（安全审查 P1-15 同源问题）：拿不到口令就拒绝执行，
# 免得"随手跑一下"把开发库口令重置成一个谁都知道的字符串。
#
# 注意：本脚本会**重置**该角色口令（幂等设计的代价）。执行后请同步 backend/.env。

set -euo pipefail

DB_USER="${1:-novamind}"
DB_NAME="${3:-novamind}"

DB_PASS="${2:-${NOVAMIND_DB_PASSWORD:-}}"
if [ -z "${DB_PASS}" ]; then
  ENV_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/backend/.env"
  if [ -f "${ENV_FILE}" ]; then
    DB_PASS="$(grep -E '^DATABASE_URL=' "${ENV_FILE}" | head -1 | cut -d= -f2- |
      sed -E 's#^[a-z]+://[^:]+:([^@]+)@.*#\1#')"
  fi
fi
if [ -z "${DB_PASS}" ]; then
  echo "错误：没有拿到数据库口令。" >&2
  echo "  用法：sudo bash $0 [用户名] [密码] [库名]" >&2
  echo "  或先设置 NOVAMIND_DB_PASSWORD，或让 backend/.env 里的 DATABASE_URL 带口令。" >&2
  exit 1
fi

if [ "$(id -u)" -ne 0 ]; then
  echo "错误：需要 root 权限，请用 sudo 执行：" >&2
  echo "  sudo bash $0" >&2
  exit 1
fi

run_psql() {
  sudo -u postgres psql -v ON_ERROR_STOP=1 "$@"
}

echo "==> 1/4 确保角色 ${DB_USER} 存在"
if run_psql -tAc "SELECT 1 FROM pg_roles WHERE rolname = '${DB_USER}'" | grep -q 1; then
  echo "    角色已存在，跳过创建"
else
  run_psql -c "CREATE ROLE ${DB_USER} LOGIN"
  echo "    已创建角色"
fi

echo "==> 2/4 设置角色密码"
run_psql -c "ALTER ROLE ${DB_USER} WITH LOGIN PASSWORD '${DB_PASS}'"

echo "==> 3/4 确保数据库 ${DB_NAME} 存在且属主正确"
if run_psql -tAc "SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}'" | grep -q 1; then
  echo "    数据库已存在"
else
  run_psql -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER}"
  echo "    已创建数据库"
fi
run_psql -c "ALTER DATABASE ${DB_NAME} OWNER TO ${DB_USER}"

echo "==> 4/4 验证业务账号登录"
PGPASSWORD="${DB_PASS}" psql -h 127.0.0.1 -p 5432 -U "${DB_USER}" -d "${DB_NAME}" \
  -tAc "SELECT 'OK: ' || current_user || ' @ ' || current_database()"

echo
echo "完成。后端 .env 使用："
echo "  DATABASE_URL=postgres://${DB_USER}:${DB_PASS}@127.0.0.1:5432/${DB_NAME}?sslmode=disable"
