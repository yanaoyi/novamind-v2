#!/usr/bin/env bash
# 工程级清理：按外键安全顺序删掉一个工程及其全部从属数据。
#
# 为什么必须有这个文件（2026-10-05 实测出来的真缺陷）：
#   各验收脚本原先的清理都是 `delete from projects where id=...`，而
#   original_works / creative_works / files / tasks 对 projects 的外键都是 NO ACTION ——
#   删除会被外键直接拒绝；脚本又用 `>/dev/null 2>&1 || true` 吞掉错误，
#   于是"脚本自带清理、跑完不留数据"是一句空话：每跑一次端到端/冒烟就多留两个**活**工程，
#   库里实测堆了 15 个（端到端验证 ×10、Phase9 上下文验收 ×4、检索验收 ×1）。
#   更糟的是它们会出现在界面上，像一个"文章"列表里的垃圾。
#
# 外键约束决定删除顺序（有两个坑是实测撞出来的）：
#   1) 二创作品 reference 原著作品（creative_works.original_work_id，NO ACTION）：
#      e2e 的二创工程是拿**另一个工程的**原著开出来的，所以清原著工程前必须先清掉
#      引用它的二创作品 —— 源原著都没了，那个二创本身也活不下去。
#   2) analysis_proposals reference tasks（NO ACTION）：删任务前先删提案，
#      否则报 "analysis_proposals_task_id_fkey"。
# 因此顺序是：先算出三个目标集合（原著 / 二创 / 任务），再按
#   chunks → analysis_proposals → tasks → creative_works → original_works → files → projects 删。
# chunks 没有外键（跨表设计），必须显式删，否则会留下指向已删作品的孤儿。
#
# 用法：source scripts/lib/cleanup.sh; cleanup_project "<project_id>"
# 依赖：PSQL_URL（由 scripts/lib/db-url.sh 导出）

cleanup_project() {
  local pid="${1:-}" out
  [ -n "${pid}" ] || return 0
  if ! out="$(
    psql "${PSQL_URL}" -q -v ON_ERROR_STOP=1 -v pid="${pid}" -f - 2>&1 <<'SQL'
create temp table nm_orig as
  select id from original_works where project_id = :'pid';
create temp table nm_crea as
  select id from creative_works
   where project_id = :'pid'
      or original_work_id in (select id from nm_orig);
create temp table nm_task as
  select id from tasks
   where project_id = :'pid'
      or work_id in (select id from nm_orig)
      or creative_work_id in (select id from nm_crea);

delete from chunks
 where work_id in (select id from nm_orig)
    or work_id in (select id from nm_crea);
delete from analysis_proposals
 where work_id in (select id from nm_orig)   -- 该表里原著外键叫 work_id（见迁移 0008）
    or task_id in (select id from nm_task);
delete from tasks          where id in (select id from nm_task);
delete from creative_works where id in (select id from nm_crea);
delete from original_works where id in (select id from nm_orig);
delete from files          where project_id = :'pid';
delete from projects       where id         = :'pid';

drop table nm_orig, nm_crea, nm_task;
SQL
  )"; then
    # 不静默：清理失败必须说出来（以前正是静默吞错，才让残留堆到 15 个）
    echo "[cleanup] 工程 ${pid} 清理失败：${out}" >&2
    return 1
  fi
}
