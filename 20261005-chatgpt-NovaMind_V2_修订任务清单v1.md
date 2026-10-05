# NovaMind V2 修订任务清单

> 基于 2026-10-05 对 `yanaoyi/novamind-v2` 当前 GitHub `main` 版本的代码审核整理。
>
> 本文件不是重新设计 NovaMind，而是针对当前版本的实际缺口制定下一阶段修订任务。
>
> **核心原则：先修核心能力，再扩展 Agent；先小版本、可验证、可回滚，再继续开发。**

---

## 1. 当前审核结论

当前版本已经具备较完整的 MVP：

- 原著模型
- 二创模型
- 人物/世界/时间线
- 大纲
- 卷/节/章/场景
- AI 生成/续写/扩写/改写
- 一致性检查
- 版本管理
- AI Model Gateway
- 任务系统
- 鉴权
- SSRF 防护
- AI 接口限流
- 前端竞态与数据丢失修复
- 大纲落成
- 真实模型 E2E 验证

因此**不建议推倒重来，也不建议进行大规模架构重构**。

当前最重要的产品缺口不是普通 UI 功能，而是：

1. Retrieval
2. Context Engine
3. ContextSnapshot
4. Memory

这四项直接决定 NovaMind 能否从“AI 写作 MVP”升级成“长篇 AI 二创写作系统”。

---

# 2. 修订优先级

## P0：必须完成

### P0-1 Retrieval 检索系统

建立原著和二创内容的可检索基础设施。

#### 要求

至少支持：

- 文档/章节 Chunk
- Chunk 元数据
- Embedding
- pgvector
- 关键词检索
- 向量检索
- Hybrid Retrieval
- Top-K
- 基础 Ranking

检索对象至少包括：

- 原著章节
- 原著人物相关内容
- 原著世界规则
- 原著事件
- 原著时间线
- 二创章节
- 二创人物事实
- 二创世界事实
- 二创剧情事件

#### 不允许

不能直接把整本原著全文塞入 Prompt。

必须：

```text
Query
→ Retrieval
→ Ranking
→ Context
→ LLM
```

#### 建议数据模型

```text
documents
chunks
embeddings
retrieval_metadata
```

Chunk 至少保存：

```text
id
project_id
source_type
source_id
chapter_id
content
content_hash
token_count
metadata
embedding
created_at
updated_at
```

#### 验收标准

- 能对原著进行 Chunk
- 能生成 Embedding
- 能执行向量检索
- 能执行关键词检索
- 能执行 Hybrid Retrieval
- 返回结果带 source_id/chapter_id
- 能追溯每一条检索结果来自哪里
- Retrieval 单元测试通过
- 至少建立 20 个固定检索测试样例

---

# 3. P0-2 Context Engine

Context Engine 是 NovaMind 的核心 AI 上下文组装层。

## 目标

将：

```text
当前作品
+
当前卷
+
当前节
+
当前章
+
当前场景
+
当前大纲
+
人物状态
+
世界规则
+
时间线
+
最近剧情
+
Retrieval
+
作者指令
```

组装成模型真正需要的 Context。

---

## 建议结构

```text
ContextRequest
      ↓
Context Engine
      ↓
┌───────────────────────┐
│ Project Context       │
│ Creative Context      │
│ Character Context     │
│ World Context         │
│ Timeline Context      │
│ Plot Context          │
│ Recent Chapter        │
│ Retrieved Evidence    │
│ Author Instruction    │
└───────────┬───────────┘
            ↓
       ContextSnapshot
            ↓
            LLM
```

---

## 要求

Context Engine 不应该直接依赖具体 AI 模型。

应该：

```text
Context Engine
      ↓
ContextSnapshot
      ↓
Model Gateway
```

这样以后切换 DeepSeek / OpenAI / Claude / 其他模型时，上下文逻辑不会重复实现。

---

## 验收标准

- AI 写作调用统一经过 Context Engine
- AI 续写经过 Context Engine
- AI 改写经过 Context Engine
- AI 扩写经过 Context Engine
- AI 一致性检查可以调用 Context Engine
- Context 具有明确来源
- Context 可以记录 token 预算
- Context 可以进行裁剪
- Context 不允许无限增长

---

# 4. P0-3 ContextSnapshot

每次重要 AI 操作都必须保存生成前的上下文快照。

## 必须覆盖

- AI generate
- AI continue
- AI rewrite
- AI expand
- AI analyze
- consistency check

---

## 建议结构

```json
{
  "project_id": "...",
  "creative_work_id": "...",
  "chapter_id": "...",
  "scene_id": "...",

  "model_provider": "...",
  "model": "...",
  "prompt_version": "...",

  "outline_context": {},
  "character_context": {},
  "world_context": {},
  "timeline_context": {},
  "plot_context": {},

  "retrieved_sources": [],

  "author_instruction": "...",

  "token_budget": {},
  "created_at": "..."
}
```

---

## 核心要求

生成结果必须能够回答：

> “AI 当时为什么这么写？”

因此至少可以追溯：

```text
模型
Prompt
作者指令
人物状态
世界规则
时间线
大纲
检索结果
生成时间
```

---

## 验收标准

任意一章 AI 生成完成后，可以通过数据库或 API 找到对应 ContextSnapshot。

Snapshot 必须不可被普通 AI 写作流程静默修改。

---

# 5. P0-4 Memory

Memory 不要简单实现成一个 `memories` 表。

建议至少分为三层。

## 5.1 Canonical Memory

原著事实。

例如：

```text
原著人物
原著关系
原著世界规则
原著地点
原著势力
原著事件
```

特点：

> 原著事实默认不可被二创 AI 直接覆盖。

---

## 5.2 Creative Memory

作者确认后的二创事实。

例如：

```text
人物修改
人物关系变化
世界规则变化
剧情变化
时间线变化
作者新增设定
```

特点：

> 作者确认后成为二创世界的权威事实。

---

## 5.3 Episodic Memory

写作过程中实际发生的剧情事实。

例如：

```text
第 88 章：
人物 A 受伤
人物 B 离开城市
某物品被使用
某伏笔已经揭示
人物 A 得知某信息
```

特点：

> 由已完成章节产生，并服务后续章节。

---

# 6. P0-5 写作后 Memory 自动更新

这是 Memory 的真正闭环。

流程：

```text
AI 写作
   ↓
章节完成
   ↓
事实抽取
   ↓
人物状态变化
   ↓
世界状态变化
   ↓
事件
   ↓
时间线
   ↓
Episodic Memory
   ↓
Consistency Check
```

不要让 Memory 只存在于数据库里而不参与下一次写作。

---

# 7. P1：JSON Schema 真正落地

当前已经修复：

```text
JSON Prompt
≠
文本 Prompt
```

这是正确的。

但还需要继续增加：

```text
LLM
 ↓
JSON
 ↓
JSON Schema Validation
 ↓
失败
 ↓
Repair
 ↓
Retry
 ↓
通过
 ↓
Service
```

## 要求

至少为以下结构建立 Schema：

- Character
- World
- Timeline
- Event
- Outline
- Consistency Report
- Memory Fact

---

## 验收标准

非法 JSON：

> 自动修复或失败重试。

字段类型错误：

> 不能直接写入数据库。

缺少必填字段：

> 必须被 Schema 拦截。

---

# 8. P1：Prompt Injection 防护继续完善

当前已经使用用户内容分隔符，并明确告诉模型用户内容是素材而不是指令。

这属于正确的第一层防护，但不能认为已经彻底解决。

建议明确分层：

```text
SYSTEM INSTRUCTION
        ↓
AUTHOR INSTRUCTION
        ↓
STRUCTURED CONTEXT
        ↓
RETRIEVED EVIDENCE
        ↓
ORIGINAL / CREATIVE CONTENT
```

---

## 要求

检索到的原著文本、用户上传内容、小说正文都必须被当作：

> 数据 / 证据

而不是：

> 指令。

建立 Prompt Injection 测试集，至少包含：

- “忽略之前所有指令”
- 伪系统提示
- 伪开发者提示
- 恶意 JSON
- 恶意 Markdown
- HTML 注入
- 长文本注入
- 原著正文中嵌入指令

---

# 9. P1：PostgreSQL 版本统一

当前架构文档 / Docker 使用 PostgreSQL 16，而开发状态记录存在 PostgreSQL 15。

必须统一。

推荐：

```text
PostgreSQL 16
pgvector
```

统一：

- 本地开发
- 测试
- Docker
- CI
- 生产

同时更新：

- ARCHITECTURE.md
- SPEC.md
- CODEX_STATE.md
- README
- Docker 配置
- 开发环境说明

---

# 10. P1：任务系统文档与实现统一

当前实际实现已经采用 PostgreSQL queue / `FOR UPDATE SKIP LOCKED` 一类方案，而旧架构文档仍存在 Redis + Asynq 的描述。

如果当前实现继续采用 PostgreSQL queue：

> 不要为了迁就旧文档重新改代码。

直接把架构文档改成真实实现。

---

## 推荐

```text
PostgreSQL
    ↓
jobs
    ↓
FOR UPDATE SKIP LOCKED
    ↓
Worker
```

如果未来任务量真正超过 PostgreSQL queue 的合理范围，再考虑迁移到 Redis/Asynq。

---

# 11. P1：Docker 真正部署验证

当前本地 E2E 通过并不等于 Docker 部署一定通过。

必须增加一次完整部署验证：

```text
docker compose up
      ↓
PostgreSQL
      ↓
Redis（如果实际需要）
      ↓
Backend
      ↓
Frontend
      ↓
Migration
      ↓
真实模型
      ↓
E2E
```

至少验证：

- 登录
- 创建项目
- 导入原著
- 原著分析
- 创建二创
- 创建大纲
- 大纲落成
- AI 生成章节
- AI 续写
- AI 改写
- 一致性检查
- 版本恢复
- 导出

---

# 12. P1：更新项目状态管理

目前存在：

```text
CODEX_STATE
“主体全部完成”
```

与：

```text
SPEC
Retrieval ❌
ContextSnapshot ❌
Memory ❌
Agent ❌
```

之间的语义不一致。

不要再使用：

> “主体全部完成”

这种容易误导后续 AI 的描述。

---

## 建议改成

```text
V2.0 MVP：
完成

应用层：
完成度 85~90%

AI Context Engine：
未完成

Retrieval：
未完成

Memory：
未完成

ContextSnapshot：
未完成

Agent：
未开始

生产就绪：
待验证
```

---

# 13. P2：二创剧情模块

当前规格仍存在二创剧情的缺口。

建议实现：

```text
Creative Plot
Creative Plot Arc
Creative Event
Creative Timeline
```

并与：

```text
Creative Character
Creative World
Outline
Chapter
Memory
```

建立关系。

---

# 14. P2：素材模块

增加统一素材层：

```text
素材
 ├─ 原著引用
 ├─ 作者资料
 ├─ 世界观资料
 ├─ 人物资料
 ├─ 参考文本
 ├─ 灵感
 └─ 外部资料
```

素材必须可以被 Retrieval 使用。

不要再建立一套与 Retrieval 无关的“素材数据库”。

---

# 15. P2：知识库

知识库不要独立成另一套 AI 系统。

应该统一进入：

```text
Knowledge
   ↓
Chunk
   ↓
Embedding
   ↓
Retrieval
   ↓
Context Engine
```

这样：

> 原著、二创、作者资料、知识库

最终都可以通过统一 Retrieval 进入 AI 上下文。

---

# 16. 暂缓：Agent

当前不建议立即开发 7 个 Agent。

暂缓：

```text
Character Agent
World Agent
Plot Agent
Timeline Agent
Writing Agent
Consistency Agent
Research Agent
```

原因：

当前：

```text
Retrieval
Memory
Context Engine
ContextSnapshot
```

尚未完成。

如果现在开发 Agent，很容易形成：

```text
Agent
 ↓
直接调用 Service
```

而不是：

```text
Agent
 ↓
Tool
 ↓
Context Engine
 ↓
Retrieval / Memory
 ↓
Service
```

---

# 17. 建议的下一版本规划

## NovaMind V2.1 — Context Engine

### V2.1-01

Chunk

### V2.1-02

Embedding

### V2.1-03

pgvector

### V2.1-04

Hybrid Retrieval

### V2.1-05

Context Assembly

### V2.1-06

ContextSnapshot

### V2.1-07

Canonical Memory

### V2.1-08

Creative Memory

### V2.1-09

Episodic Memory

### V2.1-10

写作后自动更新 Memory

### V2.1-11

Context 回归测试

---

# 18. Git 修订要求

本轮不要一次性提交所有修改。

推荐：

```text
v2.0.0
  ↓
冻结当前稳定 MVP
  ↓
v2.1.0-dev
```

然后每完成一个明确功能就提交。

例如：

```text
feat(retrieval): add document chunking
feat(retrieval): add pgvector embedding
feat(retrieval): add hybrid search
feat(context): add context assembly
feat(context): add context snapshot
feat(memory): add canonical memory
feat(memory): add creative memory
feat(memory): add episodic memory
test(context): add regression suite
```

---

# 19. Codex 开发安全边界

后续交给 Codex 时必须遵守：

## 禁止

- 不得推倒现有数据模型
- 不得重写 Model Gateway
- 不得删除现有版本系统
- 不得删除原著/二创隔离
- 不得修改已经通过的安全机制
- 不得为了新功能修改无关模块
- 不得一次修改大量无关文件
- 不得把 Agent 作为 Retrieval/Memory 的替代品

## 必须

每完成一个功能：

```text
代码
 ↓
单元测试
 ↓
API测试
 ↓
E2E
 ↓
Git commit
```

并在 `CODEX_STATE.md` 中记录：

```text
修改内容
测试结果
未完成事项
风险
下一步
```

---

# 20. 最终验收标准

NovaMind V2.1 不应以：

> “页面能打开”

作为完成标准。

必须满足：

```text
原著
 ↓
Chunk
 ↓
Embedding
 ↓
Retrieval
 ↓
Context Engine
 ↓
ContextSnapshot
 ↓
AI生成
 ↓
事实抽取
 ↓
Memory
 ↓
下一章
 ↓
再次 Retrieval
 ↓
再次 Context
```

形成完整闭环。

最终测试：

### 长篇连续写作测试

至少模拟：

```text
100+ 章节
```

验证：

1. 早期人物设定能否被检索
2. 早期伏笔能否被找到
3. 人物关系是否保持
4. 世界规则是否保持
5. 时间线是否保持
6. 已发生事件是否不会丢失
7. 作者修改是否覆盖原始继承
8. 原著事实和二创事实是否混淆
9. Memory 是否随着章节增长
10. Context Token 是否始终可控

---

# 21. 本轮最终结论

当前 NovaMind：

> **不需要推倒重来。**

当前 MVP：

> **已经达到可以继续迭代的稳定阶段。**

下一阶段不应该继续无序增加 UI 或 Agent。

真正应该完成：

```text
Retrieval
     ↓
Context Engine
     ↓
ContextSnapshot
     ↓
Memory
     ↓
Writing
     ↓
Memory Update
```

这条链完成以后，NovaMind 才真正具备：

> **“越写越懂这部作品”的长期 AI 写作能力。**

