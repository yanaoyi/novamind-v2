# NovaMind V2 开发规格说明书

> 文档用途：本文件作为 Codex 开发 NovaMind 的主规格文件。\
> 开发目标：构建一个面向 AI
> 写作、尤其是"基于原著进行二次创作/同人小说创作"的专业 AI 写作系统。\
> 开发原则：先完成可运行 V1，再逐步增强
> Agent、模型编排和高级能力。不要在 V2 阶段过度平台化。

------------------------------------------------------------------------

# 1. 产品定义

## 1.1 产品名称

NovaMind

## 1.2 产品定位

NovaMind 是一个以 AI 写作为核心的专业创作系统，重点解决：

1.  导入原著；
2.  AI 分析原著结构；
3.  提取原著人物、人物性格、人物关系；
4.  提取原著世界观、规则、势力、地点；
5.  提取原著剧情和时间线；
6.  将原著内容结构化为可编辑的"原著世界模型"；
7.  作者选择继承、修改、融合原著元素；
8.  建立新的二创世界、人物、剧情和时间线；
9.  AI 根据二创设定生成大纲、章节和场景；
10. 持续检查人物、世界观、剧情、时间线的一致性。

NovaMind 不是一个通用 AI Agent 平台。

内部技术架构允许模块化和复用，但产品界面和核心业务必须围绕"AI
小说创作"展开。

------------------------------------------------------------------------

# 2. 核心产品理念

NovaMind 的核心不是简单的"AI 续写"。

核心工作流：

``` text
原著导入
   ↓
原著分析
   ↓
原著结构化
   ↓
原著世界模型
   ↓
作者选择继承/修改/融合
   ↓
建立二创世界模型
   ↓
确定分叉点
   ↓
生成二创大纲
   ↓
AI 写作
   ↓
一致性检查
   ↓
版本管理
```

必须严格区分：

``` text
Original Model
原著模型

Creative Model
二创模型
```

不能让 AI 把原著事实和作者新设定混在一起。

------------------------------------------------------------------------

# 3. V2 产品边界

## 3.1 V2 必须实现

### 原著

-   创建原著项目
-   上传 TXT / DOCX / PDF
-   文本解析
-   自动章节识别
-   原著结构分析
-   人物提取
-   人物性格分析
-   人物关系分析
-   世界观提取
-   地点提取
-   势力提取
-   世界规则提取
-   剧情事件提取
-   原著时间线提取
-   原著知识库

### 二创

-   从原著创建二创作品
-   选择原著元素继承
-   修改人物
-   人物融合
-   修改世界观
-   修改世界规则
-   继承原著时间线
-   创建分叉点
-   修改/新增事件
-   创建二创时间线
-   原著与二创映射关系

### 创作

-   故事设定
-   人物设定
-   世界观设定
-   剧情设定
-   大纲
-   卷
-   章节
-   场景
-   编辑器
-   AI 续写
-   AI 扩写
-   AI 改写
-   AI 生成大纲
-   AI 生成人物
-   AI 生成剧情
-   AI 场景生成

### 一致性

-   人物一致性
-   世界观一致性
-   时间线一致性
-   剧情一致性
-   原著继承一致性

### 工程

-   项目管理
-   文件管理
-   版本管理
-   AI 任务管理
-   模型配置
-   导出 Markdown / TXT / DOCX

------------------------------------------------------------------------

# 4. V2 暂不实现

以下能力暂不作为 V1 必须项：

-   多用户复杂协作
-   商业化支付
-   社交社区
-   在线发布平台
-   推荐算法
-   多租户 SaaS
-   复杂插件市场
-   自动训练模型
-   自研大模型
-   复杂工作流编排器
-   多 Agent 自主长期运行
-   自动发布到第三方平台
-   复杂知识图谱可视化
-   移动端 App

架构必须允许未来增加，但不要因为这些功能影响 V1。

------------------------------------------------------------------------

# 5. 总体架构

``` text
NovaMind
│
├── Frontend
│   ├── Dashboard
│   ├── Original Analysis
│   ├── Creative Workspace
│   ├── Character
│   ├── World
│   ├── Timeline
│   ├── Plot
│   ├── Outline
│   ├── Chapter
│   ├── Editor
│   ├── AI Assistant
│   └── Task Center
│
├── Writing Core
│   ├── Original Analysis
│   ├── Character Engine
│   ├── World Engine
│   ├── Plot Engine
│   ├── Timeline Engine
│   ├── Outline Engine
│   ├── Chapter Engine
│   ├── Scene Engine
│   ├── Writing Engine
│   └── Consistency Engine
│
├── AI Core
│   ├── Agent Runtime
│   ├── Context Engine
│   ├── Memory Engine
│   ├── Model Gateway
│   ├── Prompt Engine
│   └── Task Engine
│
├── Data
│   ├── PostgreSQL
│   ├── Vector Database
│   ├── File Storage
│   └── Cache
│
└── Infrastructure
    ├── API
    ├── Worker
    ├── Queue
    ├── Logging
    └── Configuration
```

------------------------------------------------------------------------

# 6. 推荐技术栈

V2推荐：

## Backend

-   Go
-   Gin 或 Fiber
-   GORM
-   PostgreSQL
-   Redis
-   Asynq 或等价任务队列

## Frontend

-   React
-   TypeScript
-   Vite
-   Ant Design 或 Shadcn/ui
-   Zustand
-   Tiptap

## AI

通过统一 Model Gateway 接入：

-   OpenAI
-   DeepSeek
-   智谱
-   Claude
-   其他 OpenAI Compatible API

不要把任何一个模型厂商写死在业务代码中。

## 文件

V2：

-   本地文件系统

生产环境可扩展：

-   OSS / S3 / MinIO

## 向量检索

V2 可使用：

-   pgvector

不要为了 V2 引入过多基础设施。

------------------------------------------------------------------------

# 7. 核心领域模型

## 7.1 Project

``` text
Project
- id
- name
- description
- type
- status
- created_at
- updated_at
```

type：

``` text
ORIGINAL
CREATIVE
```

更推荐实际结构：

``` text
Project
 ├── OriginalWork
 └── CreativeWork
```

------------------------------------------------------------------------

# 8. 原著模型

## 8.1 OriginalWork

``` text
OriginalWork
- id
- project_id
- title
- author
- description
- source_type
- source_file_id
- status
```

------------------------------------------------------------------------

# 9. 原著章节

``` text
OriginalChapter
- id
- original_work_id
- chapter_no
- title
- content
- summary
- start_position
- end_position
```

------------------------------------------------------------------------

# 10. 原著人物

``` text
OriginalCharacter
- id
- original_work_id
- name
- aliases
- role
- gender
- age
- appearance
- personality
- motivation
- values
- fears
- desires
- behavior_patterns
- speech_style
- abilities
- relationships
- first_appearance
- last_appearance
```

------------------------------------------------------------------------

# 11. Character DNA

人物 DNA 是 NovaMind 的核心数据结构之一。

``` text
CharacterDNA
- core_personality
- values
- motivations
- fears
- desires
- decision_style
- conflict_response
- emotional_response
- relationship_pattern
- behavior_pattern
- speech_style
```

每项应允许结构化评分或权重。

例如：

``` json
{
  "personality": 90,
  "values": 80,
  "motivation": 70,
  "behavior": 60,
  "speech_style": 30,
  "background": 10,
  "ability": 0
}
```

注意：

Character DNA 是抽象人物特征，不是复制原著正文。

------------------------------------------------------------------------

# 12. 原著人物关系

``` text
CharacterRelationship
- id
- source_character_id
- target_character_id
- relation_type
- strength
- description
```

relation_type：

``` text
family
friend
lover
enemy
mentor
student
colleague
rival
organization
other
```

------------------------------------------------------------------------

# 13. 原著世界观

``` text
OriginalWorld
- id
- original_work_id
- name
- description
```

## WorldRule

``` text
WorldRule
- id
- world_id
- category
- name
- description
- importance
```

## Location

``` text
Location
- id
- world_id
- name
- type
- description
- parent_location_id
```

## Faction

``` text
Faction
- id
- world_id
- name
- type
- description
- goals
- relationships
```

------------------------------------------------------------------------

# 14. 原著事件

``` text
OriginalEvent
- id
- original_work_id
- title
- description
- chapter_id
- time_order
- participants
- location
- consequences
- importance
```

------------------------------------------------------------------------

# 15. 原著时间线

``` text
OriginalTimeline
- id
- original_work_id
- name
```

``` text
TimelineEvent
- id
- timeline_id
- event_id
- sequence
- time_label
- duration
```

------------------------------------------------------------------------

# 16. 原著剧情结构

``` text
PlotArc
- id
- work_id
- type
- title
- summary
- start_event
- end_event
```

type：

``` text
main
subplot
character_arc
relationship_arc
world_arc
```

------------------------------------------------------------------------

# 17. 二创作品

``` text
CreativeWork
- id
- original_work_id
- title
- description
- status
- divergence_point_id
```

------------------------------------------------------------------------

# 18. 二创人物

``` text
CreativeCharacter
- id
- creative_work_id
- name
- description
- source_type
- source_character_id
- dna
- modifications
```

source_type：

``` text
ORIGINAL_INHERITED
MODIFIED
FUSED
NEW
```

------------------------------------------------------------------------

# 19. 人物继承规则

``` text
InheritanceRule
- id
- creative_character_id
- source_character_id
- personality_weight
- value_weight
- motivation_weight
- behavior_weight
- speech_weight
- background_weight
- ability_weight
- relationship_weight
```

例如：

``` text
原著人物 A
    ↓
新人物 B

性格 90%
价值观 80%
行为 60%
语言风格 30%
背景 10%
能力 0%
```

------------------------------------------------------------------------

# 20. 人物融合

允许：

``` text
Character A
      +
Character B
      ↓
New Character C
```

系统生成融合说明：

``` text
人格来源
价值观来源
行为来源
能力来源
语言来源
冲突方式
```

作者可以修改。

------------------------------------------------------------------------

# 21. 二创世界

``` text
CreativeWorld
- id
- creative_work_id
- source_world_id
- inheritance_mode
- description
```

inheritance_mode：

``` text
FULL
PARTIAL
MODIFIED
NEW
```

世界元素必须允许：

``` text
继承
修改
删除
新增
```

------------------------------------------------------------------------

# 22. 二创世界规则

``` text
CreativeWorldRule
- id
- creative_world_id
- source_rule_id
- status
- name
- description
```

status：

``` text
INHERITED
MODIFIED
REMOVED
NEW
```

------------------------------------------------------------------------

# 23. 原著与二创映射

这是 NovaMind 的关键模型。

``` text
OriginalCreativeMapping
- id
- creative_work_id
- original_type
- original_id
- creative_type
- creative_id
- mapping_type
- description
```

mapping_type：

``` text
INHERITED
MODIFIED
REPLACED
FUSED
REMOVED
NEW
```

示例：

``` text
原著人物 A
   ↓ MODIFIED
二创人物 A'

原著事件 E
   ↓ REPLACED
二创事件 E'

原著世界 W
   ↓ INHERITED
二创世界 W'
```

------------------------------------------------------------------------

# 24. 分叉点

二创作品必须有明确的 Divergence Point。

``` text
DivergencePoint
- id
- creative_work_id
- original_chapter_id
- original_event_id
- time_label
- description
```

例如：

``` text
原著第 100 章
某关键事件发生前

↓ 分叉

二创作品开始新的剧情
```

------------------------------------------------------------------------

# 25. 二创时间线

``` text
CreativeTimeline
- id
- creative_work_id
- name
```

``` text
CreativeTimelineEvent
- id
- timeline_id
- source_original_event_id
- event_id
- sequence
- time_label
- status
```

status：

``` text
INHERITED
MODIFIED
NEW
REMOVED
```

必须支持：

``` text
原著时间线
       │
       ├── 分叉点
       │
       └── 二创时间线
```

------------------------------------------------------------------------

# 26. 二创剧情

``` text
CreativePlot
- id
- creative_work_id
- title
- summary
- type
- status
```

支持：

``` text
主线
支线
人物线
感情线
世界线
```

------------------------------------------------------------------------

# 27. 大纲

``` text
Outline
- id
- creative_work_id
- title
- summary
- version
```

``` text
OutlineNode
- id
- outline_id
- parent_id
- level
- title
- summary
- purpose
- characters
- location
- conflict
- outcome
```

结构：

``` text
作品
 ├── 卷
 │    ├── 节
 │    │    ├── 章
 │    │    │    └── 场景
```

------------------------------------------------------------------------

# 28. Chapter

``` text
Chapter
- id
- creative_work_id
- volume_id
- chapter_no
- title
- summary
- content
- status
- word_count
```

status：

``` text
DRAFT
REVIEW
FINAL
```

------------------------------------------------------------------------

# 29. Scene

``` text
Scene
- id
- chapter_id
- sequence
- title
- location
- characters
- purpose
- conflict
- emotional_goal
- content
```

------------------------------------------------------------------------

# 30. 素材

``` text
Material
- id
- creative_work_id
- type
- title
- content
- tags
```

type：

``` text
idea
reference
research
dialogue
scene
description
character
world
```

------------------------------------------------------------------------

# 31. AI Context Engine

AI 每次生成前必须动态组装上下文。

优先级：

``` text
1. 用户当前指令
2. 当前章节/场景
3. 二创人物设定
4. 二创世界设定
5. 二创时间线
6. 二创剧情
7. 原著继承设定
8. 原著人物 DNA
9. 原著世界模型
10. 原著剧情背景
```

注意：

不能简单把整个原著全文塞进 Prompt。

必须：

``` text
结构化数据
+
向量检索
+
当前章节上下文
+
作者指令
```

------------------------------------------------------------------------

# 32. Context Snapshot

每次 AI 生成前生成上下文快照：

``` text
ContextSnapshot
- id
- work_id
- chapter_id
- user_instruction
- characters
- world_rules
- timeline_events
- plot_context
- retrieved_original_context
- retrieved_creative_context
- model
- prompt_version
```

用于：

-   调试
-   重现
-   版本比较
-   AI 错误分析

------------------------------------------------------------------------

# 33. Memory Engine

Memory 分三层：

## Long-term

``` text
人物
世界
时间线
剧情
作者偏好
```

## Project

``` text
本作品设定
```

## Short-term

``` text
当前章节
当前场景
最近几轮 AI 对话
```

禁止将所有内容混成一个 Memory。

------------------------------------------------------------------------

# 34. AI Agent

V1 不要设计成几十个 Agent。

建议先实现：

## 1. Original Analyzer Agent

负责：

``` text
章节分析
人物提取
人物关系
世界观
剧情
时间线
叙事结构
```

## 2. Character Agent

负责：

``` text
人物创建
人物分析
人物 DNA
人物融合
人物一致性
```

## 3. World Agent

负责：

``` text
世界创建
世界继承
世界修改
世界规则
世界一致性
```

## 4. Plot Agent

负责：

``` text
剧情
冲突
事件
剧情线
```

## 5. Outline Agent

负责：

``` text
生成大纲
修改大纲
检查大纲
```

## 6. Writing Agent

负责：

``` text
续写
扩写
改写
场景写作
章节写作
```

## 7. Review Agent

负责：

``` text
人物一致性
世界一致性
时间线一致性
剧情一致性
语言问题
```

------------------------------------------------------------------------

# 35. Agent Runtime

Agent 不直接操作数据库。

统一：

``` text
Agent
 ↓
Tool
 ↓
Service
 ↓
Database
```

工具示例：

``` text
get_character
get_world
get_timeline
get_plot
get_chapter
search_original
search_creative
create_character
update_character
create_outline
create_chapter
check_consistency
```

------------------------------------------------------------------------

# 36. Model Gateway

统一模型接口：

``` text
ModelProvider
- id
- name
- provider
- api_base
- api_key
- model_name
- enabled
```

统一调用：

``` text
Generate()
Chat()
Embedding()
```

业务层不得直接调用 OpenAI / DeepSeek / Claude SDK。

------------------------------------------------------------------------

# 37. Prompt Engine

Prompt 必须模板化。

例如：

``` text
OriginalCharacterAnalysis
CharacterGeneration
WorldAnalysis
OutlineGeneration
ChapterGeneration
ConsistencyCheck
```

Prompt 必须版本化：

``` text
prompt_name
prompt_version
template
```

------------------------------------------------------------------------

# 38. AI Writing 功能

编辑器中支持：

``` text
续写
扩写
改写
缩写
润色
增强冲突
增强情绪
增加动作
增加对白
调整节奏
改变叙事视角
```

选中文本后可以直接调用。

------------------------------------------------------------------------

# 39. 一致性检查

## Character Consistency

检查：

``` text
性格
价值观
行为
语言
关系
能力
```

## World Consistency

检查：

``` text
世界规则
地点
势力
能力体系
历史
```

## Timeline Consistency

检查：

``` text
时间顺序
人物年龄
事件先后
角色是否同时出现
```

## Plot Consistency

检查：

``` text
前后因果
伏笔
人物动机
剧情冲突
```

输出：

``` json
{
  "severity": "high",
  "type": "timeline",
  "description": "...",
  "evidence": "...",
  "suggestion": "..."
}
```

------------------------------------------------------------------------

# 40. 原著分析流程

``` text
上传原著
 ↓
文本解析
 ↓
章节识别
 ↓
分章节
 ↓
章节摘要
 ↓
人物提取
 ↓
人物关系
 ↓
人物 DNA
 ↓
世界观
 ↓
地点
 ↓
势力
 ↓
事件
 ↓
时间线
 ↓
剧情结构
 ↓
写入 Original Model
 ↓
作者审核
 ↓
确认
```

重要：

AI 提取结果必须允许作者修改。

AI 不能自动把推断内容当成绝对事实。

------------------------------------------------------------------------

# 41. 二创创建流程

``` text
选择原著
 ↓
创建二创作品
 ↓
选择继承元素
 ↓
选择人物
 ↓
人物修改/融合
 ↓
选择世界继承方式
 ↓
修改世界规则
 ↓
继承原著时间线
 ↓
选择分叉点
 ↓
创建新剧情
 ↓
生成大纲
 ↓
作者确认
 ↓
开始写作
```

------------------------------------------------------------------------

# 42. 原著继承界面

用户应该看到：

``` text
原著元素

人物
☑ 主角A
☑ 主角B
☐ 配角C

世界
☑ 世界规则
☑ 地理
☑ 势力
☐ 历史

剧情
☑ 前100章事件
☐ 后续事件

时间线
☑ 分叉点之前
☐ 分叉点之后
```

------------------------------------------------------------------------

# 43. 人物继承界面

例如：

``` text
选择原著人物：A

人物 DNA

人格       █████████░ 90%
价值观     ████████░░ 80%
动机       ███████░░░ 70%
行为       ██████░░░░ 60%
语言风格   ███░░░░░░░ 30%
背景       █░░░░░░░░░ 10%
能力       ░░░░░░░░░░ 0%
```

作者可以修改。

------------------------------------------------------------------------

# 44. 世界继承界面

每个元素显示：

``` text
元素
状态
来源
修改
```

状态：

``` text
继承
修改
删除
新增
```

------------------------------------------------------------------------

# 45. 时间线界面

推荐使用横向时间轴：

``` text
原著

T1 ─── T2 ─── T3 ─── T4 ─── T5
                  │
                分叉点
                  │
                  ├── C1
                  ├── C2
                  ├── C3
                  └── C4
```

------------------------------------------------------------------------

# 46. 前端页面结构

``` text
/
├── dashboard
│
├── projects
│
├── original
│   ├── overview
│   ├── chapters
│   ├── characters
│   ├── relationships
│   ├── world
│   ├── locations
│   ├── factions
│   ├── plot
│   ├── timeline
│   └── knowledge
│
├── creative
│   ├── overview
│   ├── settings
│   ├── characters
│   ├── world
│   ├── plot
│   ├── timeline
│   ├── outline
│   ├── chapters
│   ├── materials
│   └── mappings
│
├── editor
│
├── ai
│
├── consistency
│
├── tasks
│
└── settings
```

------------------------------------------------------------------------

# 47. 主界面

推荐：

``` text
┌─────────────────────────────────────────────────────────────┐
│ NovaMind     《作品名称》                   AI  保存  设置 │
├──────────────┬──────────────────────────────┬───────────────┤
│ 作品导航      │                              │ AI助手         │
│              │                              │               │
│ 📚 原著       │       当前章节               │ ✦ 续写         │
│ 📖 故事       │                              │ ✦ 扩写         │
│ 📋 大纲       │       编辑器                 │ ✦ 改写         │
│ 👤 人物       │                              │ ✦ 生成         │
│ 🌍 世界观     │                              │ ✦ 分析         │
│ 🕐 时间线     │                              │ ✦ 检查         │
│ 🎬 场景       │                              │               │
│ 📚 素材       │                              │               │
└──────────────┴──────────────────────────────┴───────────────┘
```

------------------------------------------------------------------------

# 48. 编辑器

推荐 Tiptap。

必须支持：

-   Markdown
-   富文本
-   标题
-   段落
-   引用
-   加粗
-   斜体
-   AI 操作
-   自动保存
-   版本恢复

AI 操作：

``` text
选中文本
 ↓
AI
 ↓
续写 / 改写 / 扩写 / 润色
```

------------------------------------------------------------------------

# 49. API 设计

API 前缀：

``` text
/api/v1
```

## Projects

``` http
GET    /projects
POST   /projects
GET    /projects/:id
PUT    /projects/:id
DELETE /projects/:id
```

## Original

``` http
POST /projects/:id/original
POST /original/:id/import
GET  /original/:id/chapters
GET  /original/:id/characters
GET  /original/:id/world
GET  /original/:id/timeline
POST /original/:id/analyze
```

## Creative

``` http
POST /original/:id/create-creative
GET  /creative/:id
PUT  /creative/:id
```

## Characters

``` http
GET  /creative/:id/characters
POST /creative/:id/characters
PUT  /characters/:id
DELETE /characters/:id
POST /characters/:id/fuse
```

## World

``` http
GET  /creative/:id/world
PUT  /creative/:id/world
POST /creative/:id/world/inherit
```

## Timeline

``` http
GET  /creative/:id/timeline
POST /creative/:id/timeline/events
PUT  /timeline/events/:id
POST /creative/:id/divergence
```

## Outline

``` http
GET  /creative/:id/outline
POST /creative/:id/outline/generate
PUT  /outline/:id
```

## Chapter

``` http
GET  /creative/:id/chapters
POST /creative/:id/chapters
PUT  /chapters/:id
POST /chapters/:id/generate
```

## AI

``` http
POST /ai/chat
POST /ai/generate
POST /ai/continue
POST /ai/rewrite
POST /ai/expand
POST /ai/analyze
```

## Consistency

``` http
POST /creative/:id/consistency/check
GET  /creative/:id/consistency/issues
```

------------------------------------------------------------------------

# 50. 文件结构

推荐：

``` text
novamind/
├── backend/
│   ├── cmd/
│   ├── internal/
│   │   ├── api/
│   │   ├── domain/
│   │   ├── service/
│   │   ├── repository/
│   │   ├── agent/
│   │   ├── ai/
│   │   ├── context/
│   │   ├── memory/
│   │   ├── task/
│   │   └── consistency/
│   └── migrations/
│
├── frontend/
│   ├── src/
│   │   ├── pages/
│   │   ├── components/
│   │   ├── features/
│   │   ├── stores/
│   │   ├── api/
│   │   └── editor/
│
├── prompts/
│   ├── original/
│   ├── character/
│   ├── world/
│   ├── plot/
│   ├── outline/
│   ├── writing/
│   └── review/
│
├── docs/
├── scripts/
├── docker/
├── docker-compose.yml
└── README.md
```

------------------------------------------------------------------------

# 51. 数据库设计原则

必须：

-   所有表有 UUID 主键；
-   created_at；
-   updated_at；
-   soft delete；
-   外键约束；
-   JSONB 用于 AI 提取的半结构化数据；
-   核心实体不要全部塞进 JSON；
-   原著和二创数据严格区分。

------------------------------------------------------------------------

# 52. 数据权限边界

必须保证：

``` text
Original Model
     ↓
只读基础
     ↓
Creative Model
     ↓
允许修改
```

AI 不能直接修改 Original Model。

AI 对 Original Model 的修改只能通过：

``` text
AI 提取结果
 ↓
作者审核
 ↓
作者确认
 ↓
写入
```

------------------------------------------------------------------------

# 53. AI 任务系统

所有长任务异步执行：

``` text
Task
- id
- project_id
- type
- status
- progress
- input
- output
- error
- created_at
- started_at
- finished_at
```

状态：

``` text
PENDING
RUNNING
PAUSED
COMPLETED
FAILED
CANCELLED
```

例如：

``` text
原著分析任务 37%

章节解析       ✓
人物提取       ✓
世界观分析     运行中
时间线         等待
```

------------------------------------------------------------------------

# 54. 原著分析必须支持分阶段任务

不能一次性让 LLM 分析整本书。

流程：

``` text
文件解析
 ↓
章节切分
 ↓
章节摘要
 ↓
人物提取
 ↓
人物关系
 ↓
世界元素
 ↓
事件
 ↓
时间线
 ↓
全局剧情
```

每阶段可以重试。

------------------------------------------------------------------------

# 55. 检索系统

使用 RAG。

建议：

``` text
原著章节
 ↓
Chunk
 ↓
Embedding
 ↓
pgvector
```

检索时结合：

``` text
语义搜索
+
章节范围
+
人物
+
时间
+
场景
```

不要单纯依赖向量相似度。

------------------------------------------------------------------------

# 56. 写作时的 Context Assembly

例如写第 23 章：

``` text
当前章节目标
+
当前场景
+
当前人物
+
人物 DNA
+
人物关系
+
世界规则
+
当前位置
+
当前时间
+
最近剧情
+
相关原著继承信息
+
二创剧情
+
作者指令
```

然后：

``` text
Context
 ↓
Writing Agent
 ↓
Draft
 ↓
Consistency Agent
 ↓
Final Draft
```

------------------------------------------------------------------------

# 57. AI 输出必须结构化

大纲、人物、世界、事件等不能只输出自然语言。

例如人物：

``` json
{
  "name": "",
  "personality": [],
  "motivation": [],
  "values": [],
  "fears": [],
  "relationships": []
}
```

AI 返回 JSON。

后端验证 JSON Schema。

失败自动重试。

------------------------------------------------------------------------

# 58. AI 错误处理

必须实现：

``` text
JSON解析失败
 ↓
自动修复
 ↓
重新请求
 ↓
仍失败
 ↓
任务失败
```

不能把错误 AI 输出直接写入数据库。

------------------------------------------------------------------------

# 59. 版本管理

需要支持：

``` text
Chapter Version
Character Version
World Version
Outline Version
```

例如：

``` text
第23章

v1
v2
v3
v4 当前版本
```

支持：

``` text
查看
恢复
比较
```

------------------------------------------------------------------------

# 60. 自动保存

编辑器：

``` text
用户输入
 ↓
Debounce 1~2秒
 ↓
保存草稿
```

必须避免频繁请求。

------------------------------------------------------------------------

# 61. 导出

V1：

``` text
TXT
Markdown
DOCX
```

导出结构：

``` text
作品
 ├── 第一卷
 │    ├── 第1章
 │    ├── 第2章
 │    └── ...
 └── 第二卷
```

------------------------------------------------------------------------

# 62. 关键用户流程

## 流程 A：创建原著

``` text
创建项目
 ↓
上传原著
 ↓
解析
 ↓
章节识别
 ↓
AI分析
 ↓
作者审核
 ↓
原著模型完成
```

## 流程 B：创建二创

``` text
原著
 ↓
创建二创
 ↓
选择继承
 ↓
人物 DNA
 ↓
世界继承
 ↓
时间线
 ↓
分叉点
 ↓
新剧情
```

## 流程 C：AI写作

``` text
选择章节
 ↓
选择场景
 ↓
AI读取 Context
 ↓
生成
 ↓
一致性检查
 ↓
作者修改
 ↓
保存版本
```

------------------------------------------------------------------------

# 63. V1 开发顺序

严格按照以下顺序：

## Phase 1：基础框架

-   Go Backend
-   React Frontend
-   PostgreSQL
-   Redis
-   Docker
-   基础 API
-   项目管理

## Phase 2：原著系统

-   文件上传
-   文本解析
-   章节识别
-   OriginalWork
-   OriginalChapter
-   OriginalCharacter
-   World
-   Timeline
-   Plot

## Phase 3：AI 原著分析

-   Model Gateway
-   Prompt Engine
-   Original Analyzer
-   异步任务
-   JSON Schema
-   AI 分析结果审核

## Phase 4：二创系统

-   CreativeWork
-   CreativeCharacter
-   Character DNA
-   Character Fusion
-   CreativeWorld
-   InheritanceRule
-   Mapping
-   DivergencePoint
-   CreativeTimeline

## Phase 5：写作

-   Outline
-   Chapter
-   Scene
-   Tiptap Editor
-   AI Writing
-   AI Rewrite
-   AI Expand

## Phase 6：一致性

-   Character Check
-   World Check
-   Timeline Check
-   Plot Check

## Phase 7：版本与导出

-   Version
-   Restore
-   Compare
-   TXT
-   Markdown
-   DOCX

------------------------------------------------------------------------

# 64. Codex 开发要求

Codex 必须遵循：

1.  不要一次生成整个项目。
2.  每个 Phase 完成后运行测试。
3.  数据库 Migration 必须可重复执行。
4.  API 必须有 Swagger/OpenAPI。
5.  前后端类型尽量保持一致。
6.  AI 调用统一经过 Model Gateway。
7.  Agent 不允许直接操作数据库。
8.  原著数据和二创数据严格隔离。
9.  AI 提取结果必须经过 Schema 验证。
10. 所有长任务必须异步化。
11. 所有重要操作必须记录日志。
12. 所有 AI Prompt 必须版本化。
13. 不允许硬编码 API Key。
14. `.env` 不提交 Git。
15. 每完成一个阶段更新 `docs/STATUS.md`。

------------------------------------------------------------------------

# 65. Codex 首次启动要求

Codex 第一次接手项目时：

1.  检查当前代码；
2.  检查 Git 状态；
3.  检查 README；
4.  检查数据库；
5.  检查 Docker；
6.  检查已有 API；
7.  检查已有前端页面；
8.  不要覆盖已有代码；
9.  生成：

``` text
docs/CODEX_STATE.md
```

内容：

``` text
项目当前状态
已经完成
正在进行
未完成
已知问题
下一步任务
数据库状态
API状态
前端状态
AI模型状态
```

以后每次 Codex 重启都先读取：

``` text
docs/CODEX_STATE.md
docs/ARCHITECTURE.md
docs/PRODUCT_SPEC.md
```

------------------------------------------------------------------------

# 66. Codex 开发纪律

每次修改前：

``` text
1. 阅读相关模块
2. 判断现有代码是否可以复用
3. 先设计
4. 再修改
5. 测试
6. 更新状态
```

禁止：

``` text
为了实现一个功能直接重构整个项目
```

除非明确发现架构性错误。

------------------------------------------------------------------------

# 67. 测试要求

Backend：

-   Unit Test
-   Service Test
-   API Test

Frontend：

-   Component Test
-   API Mock Test

AI：

-   JSON Schema Test
-   Prompt Regression Test
-   Context Assembly Test

核心测试案例：

``` text
人物提取
人物 DNA
人物融合
世界继承
时间线继承
分叉点
二创事件
章节生成
一致性检查
```

------------------------------------------------------------------------

# 68. MVP 验收标准

用户能够完成：

``` text
上传一本原著
 ↓
系统识别章节
 ↓
系统分析人物
 ↓
系统分析世界
 ↓
系统分析时间线
 ↓
作者修改分析结果
 ↓
创建二创作品
 ↓
继承原著人物
 ↓
修改人物 DNA
 ↓
继承世界
 ↓
修改世界规则
 ↓
选择分叉点
 ↓
生成二创大纲
 ↓
生成章节
 ↓
AI 检查一致性
 ↓
作者修改
 ↓
导出小说
```

如果这个闭环没有完成，不要把 V1 认为已经完成。

------------------------------------------------------------------------

# 69. 重要产品原则

## 原则一：作者拥有最终控制权

AI：

``` text
分析
建议
生成
检查
```

作者：

``` text
确认
修改
删除
决定
```

------------------------------------------------------------------------

## 原则二：原著和二创必须分离

``` text
Original = Reference / Canon Model

Creative = Author Controlled Model
```

------------------------------------------------------------------------

## 原则三：不要让 AI 自己决定世界

AI 可以提出：

``` text
建议
推断
候选方案
```

但关键设定必须允许作者确认。

------------------------------------------------------------------------

## 原则四：一致性比单纯文采更重要

NovaMind 的长期核心竞争力不是：

``` text
写一段漂亮文字
```

而是：

``` text
理解长篇作品
+
长期记忆
+
人物一致
+
世界一致
+
时间线一致
+
剧情一致
+
作者控制
```

------------------------------------------------------------------------

# 70. 后续可扩展能力

V1 稳定后再考虑：

``` text
多 Agent 协作
长篇小说自动规划
自动伏笔管理
自动人物成长曲线
自动关系图
自动剧情图
自动世界地图
知识图谱
语音创作
图片角色设定
AI 封面
多模型自动路由
模型能力评测
插件系统
多人协作
云端同步
商业化 SaaS
```

这些不属于 V1。

------------------------------------------------------------------------

# 71. 最终产品闭环

NovaMind 最终形成：

``` text
              ┌───────────────┐
              │     原著       │
              └───────┬───────┘
                      ↓
              ┌───────────────┐
              │   原著分析     │
              └───────┬───────┘
                      ↓
        ┌──────────────────────────┐
        │     Original Model       │
        │ 人物 / 世界 / 剧情 / 时间线 │
        └────────────┬─────────────┘
                     ↓
              ┌───────────────┐
              │   二创设计     │
              └───────┬───────┘
                      ↓
          ┌──────────────────────┐
          │   Creative Model     │
          │ 人物 / 世界 / 剧情 / 时间线│
          └──────────┬───────────┘
                     ↓
               分叉点 Divergence
                     ↓
               大纲 Outline
                     ↓
               场景 Scene
                     ↓
               章节 Chapter
                     ↓
               AI Writing
                     ↓
             Consistency Check
                     ↓
                 作者修改
                     ↓
                  版本管理
                     ↓
                   导出
```

------------------------------------------------------------------------

# 72. 给 Codex 的最终执行指令

你现在负责开发 NovaMind。

不要把 NovaMind 开发成通用 AI Agent 平台。

NovaMind 的第一产品目标是：

> "让作者能够导入一部原著，把原著结构化成可编辑的世界模型，然后基于这个模型进行受控的二次创作，并由
> AI 长期辅助完成小说创作和一致性维护。"

开发必须优先保证以下核心闭环：

``` text
Original
→ Analyze
→ Structure
→ Author Review
→ Creative
→ Inherit
→ Modify
→ Divergence
→ Outline
→ Chapter
→ AI Writing
→ Consistency
→ Version
→ Export
```

第一阶段不要追求功能数量。

优先把：

``` text
原著
人物
人物 DNA
世界
时间线
分叉点
二创
大纲
章节
编辑器
AI 写作
一致性
```

这 12 个核心能力做通。

每完成一个 Phase：

1.  编译；
2.  测试；
3.  启动；
4.  验证核心流程；
5.  更新 `docs/CODEX_STATE.md`；
6.  更新 `docs/CHANGELOG.md`。

最终目标不是生成一个演示页面，而是生成一个真正可以开始创作小说的
NovaMind V1。
