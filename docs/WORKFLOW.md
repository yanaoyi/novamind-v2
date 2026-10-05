# 开发工作流：一次「小版本修订」

> 约定时间：2026-10-05（BOSS 定）。每个改动都走同一条流水线，**不留半成品提交，也不跳过验证**。

```
提出问题
   ↓
Codex 修改代码
   ↓
自动测试（后端 go build/vet/test + 前端 tsc/vitest，必要时冒烟）
   ↓
测试通过 ── 不通过 → 停在原地改，绝不提交
   ↓
git commit（写清"改了什么 + 怎么验证"）
   ↓
git push 到 GitHub main
   ↓
打小版本号 tag（v0.1.N）
```

## 一条命令跑完后半段

```bash
bash scripts/ship.sh -m "修：AI 写本章偶发空正文"            # 标准检查
bash scripts/ship.sh -m "修：xxx" --with-smoke                # 额外跑端到端冒烟
bash scripts/ship.sh -m "修：xxx" --fast                      # 只跑编译与单测
bash scripts/ship.sh -m "修：xxx" --dry-run                   # 只跑检查，不提交
```

脚本会：检查（后端正则、`go build`/`vet`/`test`、前端 `tsc`/`vitest`、可选冒烟）→ 全部通过才 `git add -A && commit` → `push` → 打附注 tag `v0.1.N`（N 自增）→ 推送 tag。

**任何一步失败都会立即中止**：不提交、不推送、不打 tag —— 半成品不进历史。

## 版本号约定

| 形态 | 含义 |
|---|---|
| `v0.1.N` | 一次小版本修订（一个需求/缺陷闭环 + 验证通过） |
| 附注 tag | tag 消息就是这次修订的提交说明，`git show v0.1.3` 能直接看到改了什么 |
| 首个版本 | `v0.1.0` = 2026-10-05 的交付前清理节点 |

查看历史修订：

```bash
git tag -l 'v0.1.*' --sort=v:refname
git show --stat v0.1.2
```

## 每次提交必须带的三样信息

1. **改了什么**（用户视角的现象 + 根因，不写"优化代码"这种空话）；
2. **怎么验证的**（跑了哪些测试/冒烟，数字写出来）；
3. **有没有遗留**（暂缓项要写明原因，不装作已修）。

提交说明的详版写进 `docs/CHANGELOG.md`，状态变化写进 `docs/CODEX_STATE.md`；
如果是响应外部审查（muse/grok），逐条结论写进 `docs/审查响应-*.md`。

## 交付前的两步（只在发版时做）

1. `bash scripts/validation-account.sh purge`：清掉验证用模型账号（库 + `.env`），输出留证；
2. 按干净状态复跑一遍全量冒烟 + 真实模型验收，把数字记进 `CODEX_STATE.md`。
