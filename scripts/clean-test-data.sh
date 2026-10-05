#!/usr/bin/env bash
# 清除开发/验收过程中产生的业务数据（交付前清理用）。
#
# 用法：bash scripts/clean-test-data.sh            # 清业务数据
#       bash scripts/clean-test-data.sh --dry-run  # 只看会清哪些表
#
# 重要：**配置类表必须保留**，否则会像我 2026-10-05 那次一样踩坑 ——
# 当时用"遍历 public 下所有表 TRUNCATE"的清法，把 users 表（默认账号）也清了，
# 结果检索/索引任务立刻拿不到默认 owner（chunks.owner_user_id 依赖它）。
# 下面用白名单排除：schema_migrations（迁移记录）、users（账号配置）。
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATABASE_URL="${DATABASE_URL:-$(grep -E '^DATABASE_URL=' "${REPO_ROOT}/backend/.env" | head -1 | cut -d= -f2-)}"
[[ -n "${DATABASE_URL}" ]] || { echo "错误：拿不到 DATABASE_URL" >&2; exit 1; }

KEEP="('schema_migrations','users')"

echo "== 将被清空的表 =="
psql "$DATABASE_URL" -tAc "select tablename from pg_tables where schemaname='public' and tablename not in ${KEEP} order by tablename" | sed 's/^/  /'

if [[ "${1:-}" == "--dry-run" ]]; then
  echo "（--dry-run：未执行任何删除）"
  exit 0
fi

echo
echo "== 执行清理 =="
psql "$DATABASE_URL" -q -v ON_ERROR_STOP=1 <<SQL
DO \$\$
DECLARE t text;
BEGIN
  FOR t IN SELECT tablename FROM pg_tables
           WHERE schemaname='public' AND tablename NOT IN ${KEEP}
  LOOP
    EXECUTE format('TRUNCATE TABLE public.%I RESTART IDENTITY CASCADE', t);
  END LOOP;
END \$\$;
SQL

rm -rf "${REPO_ROOT}/backend/data/uploads/"* 2>/dev/null || true

echo "== 清理后核对 =="
psql "$DATABASE_URL" -tAc "select '业务表总行数=' || coalesce(sum((xpath('/row/c/text()', query_to_xml(format('select count(*) as c from public.%I', tablename), false, true, '')))[1]::text::int),0) from pg_tables where schemaname='public' and tablename not in ${KEEP}" | sed 's/^/  /'
psql "$DATABASE_URL" -tAc "select 'users=' || count(*) from users" | sed 's/^/  /'
psql "$DATABASE_URL" -tAc "select '迁移版本=' || version || ' dirty=' || dirty from schema_migrations" | sed 's/^/  /'
echo
echo "提示：模型配置（验证账号）不在本脚本清理范围 —— 交付前请执行 scripts/validation-account.sh purge。"
