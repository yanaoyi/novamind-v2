# SPEC.md — NovaMind V2 开发规格说明书

> 项目：NovaMind V2（Go + PostgreSQL + Redis + React 全新技术线）
> 本文件是 **NovaMind V2 的唯一开发规格基准**。代码注释、迁移脚本、冒烟脚本中出现的「SPEC.md §N」全部指向本文件。
> 本文件由 V2 自己维护：V2 **不再引用、不再依赖** v1（`novamind-pro/`）的开发规格说明书；v1 仅作为历史归档与交互参考存在。
> 与 `ARCHITECTURE.md` / `PRODUCT_SPEC.md` 冲突时，以本文件为准；改规格必须先改本文件，再改代码。

---

# 第一部分 · 产品定义

## §1 产品定义与定位

NovaMind 是一个以 **AI 长篇小说创作** 为核心的专业创作系统。第一目标不是「AI 续写」，而是：

> 让作者导入一部原著 → 把原著结构化成可编辑的世界模型 → 基于该模型进行**受控的**二次创作 → 由 AI 长期辅助写作与一致性维护。

核心链路：

```
原著导入 → 原著分析 → 原著结构化 → 原著世界模型
   → 作者选择继承/修改/融合 → 建立二创世界模型
   → 确定分叉点 → 生成二创大纲 → AI 写作
   → 一致性检查 → 版本管理 → 导出
```

NovaMind **不是**通用 AI Agent 平台。内部模块可复用，但产品界面与核心业务必须围绕「AI 小说创作」。

## §2 产品原则与红线

### §2.1 作者拥有最终控制权

AI 只做 **分析 / 建议 / 生成 / 检查**；作者做 **确认 / 修改 / 删除 / 决定**。

### §2.2 原著与二创物理分离

| 模型 | 含义 | 可变性 |
|---|---|---|
| **Original Model** | 原著事实（canon / reference） | 只读基础；改动只能由 AI 提取 → 作者审核 → 作者确认 → 写入 |
| **Creative Model** | 作者掌控的二创设定 | 作者可自由增删改 |

两者在数据层是**不同的表、不同的写路径**。任何时候不得混淆，AI 不得直接改写原著表。

### §2.3 不让 AI 自己决定世界

AI 只能提出建议、推断、候选方案；关键设定必须由作者确认后才落库。

### §2.4 一致性优先于文采

长期竞争力 = 理解长篇 + 长期记忆 + 人物/世界/时间线/剧情一致 + 作者控制。

### §2.5 数据权限边界（红线）

1. Agent 只能走 `Agent → Tool → Service → Repository → DB`，**不许碰数据库**。
2. AI 分析产出**只写入提案表**（`analysis_proposals`），作者在界面上 approve 之后才写入原著正式表。
3. AI 输出必须过 JSON Schema；**校验失败的输出不许入库**。
4. 一致性检查**只报告问题，不自动改文**。

## §3 范围边界

### §3.1 必须实现

* **原著侧**：创建原著工程、上传 TXT/DOCX/PDF、文本解析、自动章节识别、AI 结构分析、人物提取、人物性格与关系分析、世界观/地点/势力/规则提取、事件提取、时间线提取。
* **二创侧**：从原著创建二创作品、选择继承元素、修改与融合人物、修改世界观与规则、继承与改写时间线、创建分叉点、原著↔二创映射。
* **创作侧**：设定、大纲、卷、章节、场景、编辑器、AI 续写/扩写/改写、AI 生成大纲/章节。
* **一致性**：人物、世界观、时间线、剧情、原著继承一致性。
* **工程**：项目管理、文件管理、版本管理、AI 任务管理、模型配置、导出 Markdown/TXT/DOCX。

### §3.2 明确不做

多用户复杂协作、商业化支付、社交社区、在线发布平台、推荐算法、多租户 SaaS、插件市场、自动训练模型、自研大模型、复杂工作流编排器、多 Agent 自主长期运行、自动发布到第三方平台、复杂知识图谱可视化、移动端 App。

架构必须**允许**未来扩展，但不为这些影响当前版本。

---

# 第二部分 · 领域模型

约定：所有实体 ID 为 UUID 字符串；所有表有 `created_at` / `updated_at` / `deleted_at`（软删除）。

## §4 Project（工程）

### §4.1 Project 模型

| 字段 | 说明 |
|---|---|
| `id` | UUID |
| `name` | 工程名，非空，≤ 200 字符 |
| `description` | 描述，可空 |
| `type` | `ORIGINAL` / `CREATIVE` |
| `status` | `DRAFT` / `ACTIVE` / `ARCHIVED` |
| `created_at` / `updated_at` / `deleted_at` | 时间戳与软删除 |

实际结构：`Project → OriginalWork / CreativeWork`。一个工程承载一部作品。

### §4.2 领域校验

* `name` 去空格后不能为空，长度 ≤ 200；
* `type` / `status` 必须是枚举内取值，非法值在 domain 层就报错；
* 软删除后不出现在列表，但数据保留可恢复。

### §4.3 文件管理

上传文件统一登记在 `files` 表：`id / project_id / original_name / stored_path / size_bytes / sha256 / mime_type / created_at`。

* 同一文件重复上传按 SHA256 去重；
* 存储路径必须做路径安全校验，禁止越出 `STORAGE_DIR`；
* 文件是解析的输入，解析结果落到 `original_chapters`，原文文件本身不参与业务查询。

## §5 原著

### §5.1 OriginalWork

| 字段 | 说明 |
|---|---|
| `id` / `project_id` | 主键与所属工程 |
| `title` | 书名 |
| `author` | 作者，可空 |
| `source_type` | `MANUAL`（手动录入）/ `AI`（AI 提取） |
| `source_file_id` | 关联上传文件，可空 |
| `summary` | 原著简介，可空 |
| `word_count` | 字数统计 |

一个 Project 至多一部 OriginalWork。

### §5.2 OriginalChapter

| 字段 | 说明 |
|---|---|
| `id` / `original_work_id` | 主键与所属原著 |
| `chapter_no` | 章节序号，从 1 开始，同一原著内唯一 |
| `title` | 章节标题 |
| `content` | 章节正文 |
| `start_offset` / `end_offset` | 在全文中的字节偏移，支持回溯原文 |
| `word_count` | 本章字数 |

章节切分由 `internal/parser` 完成：识别中文网文常见标题形式（第 N 章 / 第 N 节 / 卷标等），并记录偏移。

### §5.3 导入与解析

支持格式：**TXT / DOCX / PDF**。流程：编码探测 → 正文抽取 → 章节切分 → 章节入库（事务化替换，幂等）。

* TXT：探测 UTF-8 / GBK / GB18030 / Big5 等编码，避免乱码；
* DOCX：解析 `word/document.xml` 抽取段落，保留换行结构；
* PDF：自研解析器（对象流展开、滤镜与预测器、页树遍历、内容流算子抽文本、Form XObject 递归、ToUnicode CMap + 编码名兜底 + 内嵌 TrueType cmap 反查、标准安全处理器解密）。
* **扫描件（无文本层）必须明确提示需要 OCR**，不许把空字符串当成功；
* AES-256（R5/R6）加密的 PDF 明确提示「请用阅读器另存一份再导入」。

解析质量守门：`scripts/check-pdf-extract.sh` 以 PyMuPDF 为基准跑真实样本，按字符召回率分桶，并单列「基准自身即乱码」的样本。

## §6 人物

### §6.1 OriginalCharacter

| 字段 | 说明 |
|---|---|
| `id` / `original_work_id` | 主键与所属原著 |
| `name` | 姓名，非空，≤ 120 字符 |
| `aliases` | 别名列表 |
| `role` | 角色定位（主角/配角/反派…） |
| `importance` | 重要度 1–5 |
| `source` | `MANUAL` / `AI` |
| `profile` | 结构化档案（外貌、动机、恐惧、行为模式、语言风格、能力、背景…，JSONB） |
| `dna` | 人物 DNA（见 §6.2） |

约束：同一原著内 `name` 唯一（重名返回 `CHARACTER_DUPLICATE`）。

### §6.2 CharacterDNA（人物 DNA）

DNA 是「抽象特征 + 权重」的结构，**不是正文复制**，是继承与融合的核心数据结构。固定 **11 个维度**，每个维度为 `{ text, weight }`，`weight ∈ [0,100]`，0 表示不继承：

`personality`（性格）、`values`（价值观）、`motivation`（动机）、`behavior`（行为模式）、`speech_style`（语言风格）、`background`（背景）、`ability`（能力）、`decision_style`（决策风格）、`conflict_response`（冲突反应）、`emotional_response`（情绪反应）、`relationship_pattern`（关系模式）。

约束：任一维度权重越界即拒绝（`DNA_WEIGHT_OUT_OF_RANGE`）；11 个维度一个都不能漏。

### §6.3 CharacterRelationship

有向边：`from_character_id → to_character_id`。

`relation_type ∈ {family, friend, lover, enemy, mentor, student, colleague, rival, organization, other}`；
`strength ∈ [0,100]`；附 `description` 与证据。

约束：不能自指；同一对人物 + 同一关系类型不可重复；**两端人物必须属于同一部原著**。

## §7 世界观

### §7.1 OriginalWorld

`id / original_work_id / name / description / background / era / technology_level / power_system / geography / culture / extra(JSONB)`。一部原著一个世界。

### §7.2 WorldRule

世界规则：`id / world_id / category / title / content / priority / exceptions`。规则是「这个世界怎么运转」的硬约束，写作与一致性检查都要读它。

### §7.3 Location

地点支持**父子层级**（`parent_id`），字段含 `name / type / description / geography / significance`。

约束：父子关系必须构成树，**禁止环**；父地点必须属于同一个世界。

### §7.4 Faction

势力：`id / world_id / name / type / description / goal / resources / relationships`。

## §8 事件与剧情结构

### §8.1 OriginalEvent

`id / original_work_id / title / description / event_type / chapter_no / participants / location / cause / result / importance`。

事件是时间线的原子单位，必须能回溯到原著章节。

### §8.2 原著时间线

时间线是事件的**有序序列**：`original_timeline_events(event_id, order_index, time_label, note)`。

排序由作者决定（支持上下移动后整体重排）；`time_label` 是叙事时间标签（如「三年前·梅雨季」），与 `order_index` 并存。

### §8.3 PlotArc（剧情弧）

`arc_type ∈ {main, subplot, character_arc, relationship_arc, world_arc}`，字段含 `name / description / start_chapter / end_chapter / involved_characters / arc_summary`。

## §9 二创

### §9.1 CreativeWork

二创作品必须挂在某部 OriginalWork 上：

| 字段 | 说明 |
|---|---|
| `id` / `project_id` / `original_work_id` | 归属关系，缺一不可 |
| `title` | 作品名 |
| `status` | `DRAFT` / `WRITING` / `FINISHED` |
| `premise` | 二创前提 |
| `summary` | 简介 |

先有原著，才有二创；不得凭空创建无原著的二创作品。

### §9.2 CreativeCharacter

二创人物：`id / creative_work_id / name / source_type / profile / dna`。

`source_type ∈ {ORIGINAL_INHERITED, MODIFIED, FUSED, NEW}`。

### §9.3 继承规则（InheritanceRule）

逐维度继承：`creative_character_id + origin_character_id + dimension + weight(0–100)`。

派生算法：对每个维度，从原著人物取该维度的 `text`，按继承权重生成二创人物的维度值。示例：性格 90%、语言风格 30%、能力 0% → 性格几乎照搬、语言风格弱化保留、能力完全独立。

**0 权重必须被正确保存**（历史缺陷：GORM 零值被忽略导致 0 权重静默丢失，已修复 —— 更新时用显式列更新）。

### §9.4 人物融合

把多个来源人物（原著或二创）融合为一个新人物，**每个 DNA 维度都要记录来源**（`FusionSource`：来自哪个人物、原权重多少、融合后取值多少）。

融合结果必须可解释：作者能看出每个维度是从谁那里来的。

### §9.5 原著↔二创映射

`OriginalCreativeMapping`：`creative_work_id + original_entity_type + original_entity_id + creative_entity_type + creative_entity_id + mapping_type + note`。

`mapping_type ∈ {INHERITED, MODIFIED, REPLACED, FUSED, REMOVED, NEW}`。

映射是显式记录，**不允许「靠名字猜」**：每个二创元素从哪来，数据里必须查得到。

### §9.6 二创创建流程

1. 选原著 → 建 CreativeWork（记 `original_work_id`）；
2. 人物继承：`ORIGINAL_INHERITED`（可带 DNA 权重）或手动新增 `NEW`；
3. 世界继承：见 §10.1；
4. 设定分叉点：见 §10.3；
5. 构建二创时间线：见 §10.4；
6. 写大纲与章节：见 §12。

## §10 二创世界与时间线

### §10.1 CreativeWorld

继承模式 `inherit_mode ∈ {FULL, PARTIAL, MODIFIED, NEW}`：

| 模式 | 语义 |
|---|---|
| `FULL` | 完全继承原著世界 |
| `PARTIAL` | 部分继承（按规则逐条决定） |
| `MODIFIED` | 以原著为基础做修改 |
| `NEW` | 脱离原著另起世界 |

### §10.2 CreativeWorldRule

二创规则状态 `status ∈ {INHERITED, MODIFIED, REMOVED, NEW}`：

* `INHERITED`：原样继承原著规则；
* `MODIFIED`：继承但内容被作者改过；
* `REMOVED`：原著有，二创世界不成立；
* `NEW`：二创新增。

删除二创规则用软删除，状态变更走 `PUT`。

### §10.3 DivergencePoint（分叉点）

```json
{ "chapter_no": 12, "event_id": "...", "description": "二创从这里离开原著", "story_time": "..." }
```

分叉点是二创时间线的切分依据：**分叉点之前继承原著事件，之后是二创新内容**。二创作品必须明确「从哪里离开原著」。

### §10.4 二创时间线

`CreativeTimelineEvent`：`status ∈ {INHERITED, MODIFIED, NEW, REMOVED}` + `order_index`。

构建规则（`POST /creative/{id}/timeline/build`）：

1. 分叉点之前的事件按原著时间线顺序继承（`INHERITED`）；
2. 分叉点之后只保留二创新增/改写的事件；
3. 重建**不覆盖**作者已有的二创新内容（幂等，可重复执行）。

## §11 二创剧情（规划中）

产品定义上二创剧情线分主线/支线/人物线/感情线/世界线。

**当前实现状态**：尚未落独立的二创剧情模型；现有能力分散在「二创时间线」（§10.4）与「分叉点」（§10.3）。在实现之前，前端对应入口不得伪装成可用功能。

## §12 写作系统

层级：`CreativeWork → CreativeVolume（卷）→ CreativeChapter（章节）→ CreativeScene（场景）`。

### §12.1 大纲与卷

大纲的第一层是卷（`CreativeVolume`：`title / order_index / summary`）；章节承载大纲三要素（`summary` / `conflict` / `emotional_goal`）。

### §12.2 Chapter

| 字段 | 说明 |
|---|---|
| `id` / `creative_work_id` / `volume_id` | 归属 |
| `title` / `order_index` | 标题与排序 |
| `summary` / `conflict` / `emotional_goal` | 大纲三要素 |
| `content` | 正文（Markdown 文本） |
| `status` | `DRAFT` / `REVIEW` / `FINAL` |
| `word_count` | 字数 |

**列表接口默认不返回正文**（`GET /creative/{id}/chapters` 去掉 `content`），需要正文用 `?full=true` 或取详情 —— 长篇正文随列表返回会拖垮前端。

### §12.3 Scene

场景：`id / chapter_id / title / order_index / purpose / description / participants / location`。

### §12.4 编辑器与存储格式

* **正文一律以 Markdown 文本存储**（数据库里就是一个 `text` 字段）。富文本编辑器（Tiptap / ProseMirror）只负责所见即所得的呈现与编辑，进出都经过 `frontend/src/editor/markdown.ts` 的 Markdown↔HTML 转换。
* 理由：Markdown 能被 AI 直接读、能 diff、能导出、能进版本快照。
* 转换器只覆盖写作子集：标题 / 加粗 / 斜体 / 删除线 / 引用 / 有序无序列表 / 分割线 / 代码。**子集外的写法按纯文本处理，绝不丢字**；HTML 特殊字符必须转义。
* 编辑器支持「富文本 / Markdown 源码」双模式；外部改动（切章、AI 改写、恢复版本）通过 `lastEmitted` 比较后同步进编辑器，避免回写环路。
* 自动保存：debounce 1–2 秒；切章或离开页面前强制 flush。

### §12.5 AI 写作操作

* **写本章**：`writing_chapter` 异步任务，组装上下文（人物 DNA + 世界规则 + 前几章摘要 + 本章大纲）后生成正文，产出落章节内容并留版本。
* **编辑器内同步操作**（`POST /ai/rewrite`）：续写 / 扩写 / 改写 / 缩写 / 润色 / 增强冲突 / 增加对白 / 调整节奏。操作对象是选中文本，结果返回给作者确认后替换，**不静默覆盖**。

## §13 版本管理

覆盖 **Chapter / Character / World / Outline** 四类，支持查看 / 预览 / 恢复。

* 章节版本用 `chapter_versions`（正文体积大、读写频繁，单独存表）。
* 结构化实体（人物 / 世界观 / 大纲）用**通用快照表** `entity_versions`：`entity_type + entity_id + version_no + payload(JSONB)`，唯一约束防重号。

语义（必须坚持，界面上也这么写）：

1. 快照 = 某一刻该实体的完整状态，每次真实改动后存一份；
2. 与最新一版 payload 相同则**不建版本**（去重，避免噪声版本）；
3. 恢复 = 把快照里记录的字段写回去，**不回滚删除** —— 快照之后新建的人物/规则/章节不会因此消失；
4. 恢复前若当前内容与最新版本不同，**先自动留一版**（任何一次恢复都不会让内容凭空消失）；
5. 正文变化才留章节版本：标题、大纲、状态、归属卷的修改不产生版本噪声。

## §14 一致性检查

四类检查：Character / World / Timeline / Plot，外加原著继承一致性。

统一输出结构：

```json
{ "severity": "high", "type": "timeline", "description": "...", "evidence": "...", "suggestion": "..." }
```

实现：`consistency_check` 异步任务 → 逐章送审 `prompts/review/consistency_check.v1.md` → 模型输出容错提取 → 校验 `severity / type / description` 后写 `consistency_issues` 表 → 作者在 `/consistency` 页逐条「已解决 / 忽略 / 重新打开」。

**AI 只报告问题，不自动改文。**

## §15 素材（规划中）

产品定义上素材类型为 `idea / reference / research / dialogue / scene / description / character / world`。

**当前实现状态**：尚未实现素材模型与接口。前端对应入口不得伪装成可用功能。

---

# 第三部分 · AI 层

## §16 Model Gateway

统一模型接入层，屏蔽厂商差异。

```
ModelProvider { id, name, provider, api_base, api_key, model_name, enabled, purpose }
统一接口：Generate() / Chat() / Embedding()
```

* `provider ∈ {OPENAI_COMPATIBLE, ANTHROPIC}`，覆盖 OpenAI / DeepSeek / 智谱 / Kimi / one-api / vLLM / Claude；
* `purpose ∈ {chat, embedding, both}`；
* 业务层只认 ProviderType 枚举，**不认任何厂商 SDK**；
* 必须有连通性测试接口，配置错误要能当面暴露；
* API Key 加密存储，接口**永不返回**密钥（见 §33）。

## §17 Prompt Engine

* Prompt 模板文件化 + 版本化：`prompts/<域>/<name>.<version>.md`，编译进二进制；
* 代码里只引用 `prompt_name + prompt_version`，不内联长 Prompt 字符串；
* 已规划的模板域：`original / character / world / plot / outline / writing / review`；
* 提供模板清单接口（名称 + 全部可用版本），便于前端展示与排查。

## §18 Context Engine

### §18.1 上下文优先级（固定）

```
1 用户当前指令 → 2 当前章节/场景 → 3 二创人物 → 4 二创世界 → 5 二创时间线
→ 6 二创剧情 → 7 原著继承设定 → 8 原著人物 DNA → 9 原著世界模型 → 10 原著剧情背景
```

### §18.2 写作时的上下文组装

写章节时至少组装：二创人物 DNA、二创世界规则、分叉点之后的时间线、前几章摘要、本章大纲三要素、作者指令。上下文按预算裁剪（`§18.1` 越靠前越优先保留）。

### §18.3 禁止把原著全文塞进 Prompt

上下文 = 结构化数据 + 向量检索片段 + 当前章节上下文 + 作者指令。长篇原文只能通过检索片段进入上下文。

### §18.4 ContextSnapshot

每次 AI 生成前落一条快照：模型、`prompt_version`、检索到的原著/二创片段、最终上下文。用于调试、重现与比较。

## §19 Memory 三层

| 层 | 内容 |
|---|---|
| Long-term | 人物、世界、时间线、剧情、作者偏好 |
| Project | 本作品设定 |
| Short-term | 当前章节、当前场景、最近几轮对话 |

三层**禁止混成一坨**。

## §20 Agent 与工具边界

### §20.1 Agent 清单

7 个 Agent：Original Analyzer / Character / World / Plot / Outline / Writing / Review。

### §20.2 Runtime 约束

```
Agent → Tool → Service → Repository → DB
```

* **Agent 不允许直接操作数据库**，Tool 是 Agent 触达能力的唯一边界；
* 第一版工具集：`get_character / get_world / get_timeline / get_plot / get_chapter / search_original / search_creative / create_character / update_character / create_outline / create_chapter / check_consistency`；
* 涉及写原著的工具一律先产提案（§22.2）。

## §21 结构化输出

### §21.1 JSON Schema 强制

AI 返回必须是 JSON，后端用 Schema 校验：字段、类型、枚举、必填项。

### §21.2 容错与重试

校验失败 → 自动修复重试 → 仍失败则任务失败并记录原始输出。**绝不把未通过校验的 AI 输出写入数据库。**

## §22 AI 分析流水线

### §22.1 必须分阶段

禁止一次性把整本书丢给 LLM。原著分析切成阶段任务，每阶段可单独重试：

```
文件解析 → 章节切分 → 章节摘要 → 人物提取 → 人物关系 → 世界元素 → 事件 → 时间线 → 全局剧情
```

当前实现的 4 个分析阶段：`chapter_summary` / `character_extract` / `world_extract` / `plot_extract`。

### §22.2 提案与审核（红线）

分析阶段**只写提案表** `analysis_proposals`（`entity_type / stage / payload / status / evidence`），不碰原著正式表。

```json
{ "status": "PENDING", "entity_type": "character", "stage": "character_extract", "payload": { ... } }
```

作者在界面上 `approve` 后，才在同一个事务里写入原著正式表；`reject` 则丢弃。这是 §2.2 与 §2.5 的落地形式。

### §22.3 作者可修改

`payload` 允许作者在通过前修改；接口接受修改后的内容（`payloadOverride`），落库的是**作者确认的版本**，不是模型的原始输出。

---

# 第四部分 · 工程

## §23 技术栈

| 层 | 选型 |
|---|---|
| Backend | Go + Gin + GORM（module `github.com/yanaoyi/novamindv2/backend`） |
| 数据库 | PostgreSQL（JSONB 存 AI 半结构化结果；规划启用 pgvector） |
| 缓存/队列 | Redis（异步任务） |
| Frontend | React 18 + TypeScript + Vite（SPA） |
| UI | Ant Design 5 |
| 状态 | Zustand |
| 编辑器 | Tiptap 3 |
| API 文档 | OpenAPI 3 + 内嵌 Swagger UI |
| 迁移 | golang-migrate（SQL 文件，必须可重复执行） |
| 文件存储 | 本地文件系统，抽象成 Storage 接口，生产可换 OSS/S3/MinIO |

本地开发环境实测：Go 1.26.8（装在 `~/.local/go`，系统自带 1.19.8 不用）、Node v22.18.0 / npm 10.9.3、PostgreSQL 15.19、Redis 7.0.15、**本机无 Docker**（`docker-compose.yml` 只作部署/CI 产物，本地流程不得依赖 Docker）。

## §24 目录结构

```
novamindv2/
├── backend/
│   ├── cmd/server/            # main
│   ├── internal/
│   │   ├── api/               # HTTP handler、路由、中间件、DTO、OpenAPI
│   │   ├── config/            # 配置加载（env）
│   │   ├── domain/            # 实体、枚举、领域规则（无外部依赖）
│   │   ├── infra/             # PostgreSQL / Redis 连接与健康检查
│   │   ├── parser/            # 文本/DOCX/PDF 解析、编码探测、章节切分
│   │   ├── repository/        # 数据访问（GORM）
│   │   ├── service/           # 业务编排
│   │   ├── storage/           # 文件存储抽象
│   │   ├── ai/                # Model Gateway / Prompt Engine
│   │   └── task/              # 任务系统与 handler
│   ├── migrations/            # 可重复执行的 SQL 迁移
│   └── testdata/
├── frontend/src/              # pages / components / stores / api / editor
├── prompts/                   # 版本化 Prompt 模板
├── docs/                      # SPEC / PRODUCT_SPEC / ARCHITECTURE / CODEX_STATE / CHANGELOG
├── scripts/                   # 开发与冒烟脚本
├── docker/
├── docker-compose.yml
└── README.md
```

分层与依赖方向（硬约束）：

```
HTTP → api → service → repository → DB
```

* `domain` 不依赖任何外层（不 import gin/gorm）；
* `api` 只做参数校验 + 调用 service + 组装响应，不写业务逻辑；
* `repository` 只管数据存取，不写业务判断。

## §25 数据与迁移

1. 所有表 **UUID 主键**；
2. 必须有 `created_at` / `updated_at`；
3. **软删除**（`deleted_at`）；
4. 外键约束齐全；
5. AI 提取的半结构化数据用 **JSONB**，核心实体不许整块塞进 JSON；
6. 原著与二创数据严格区分。

迁移文件：`backend/migrations/*.up.sql | *.down.sql`，用 golang-migrate 执行。**必须能从零重建库**，重复执行不报错；schema 变更一律写迁移文件，不允许只靠 `AutoMigrate` 交付。

删除顺序必须满足外键依赖：`original_chapters → original_works → files → projects`。

命名：表名复数蛇形（`creative_characters`），外键 `<单数表名>_id`，索引 `idx_<表>_<列>`。

## §26 API 规范

### §26.1 前缀与风格

前缀固定 `/api/v1`，REST 风格资源路径。

### §26.2 统一响应包

```json
{ "data": {...}, "error": null, "trace_id": "..." }
{ "data": null, "error": { "code": "PROJECT_NOT_FOUND", "message": "...", "details": {} }, "trace_id": "..." }
```

### §26.3 分页与追踪

分页 `?page=1&page_size=20`，响应带 `total`；所有请求带 `X-Request-Id`（无则生成），贯穿日志与响应。

### §26.4 长任务与二进制响应

* 长任务不阻塞 HTTP：创建任务返回 `202 + task_id`，进度走 `GET /api/v1/tasks/:id`；
* 二进制响应（导出）不走统一响应包，直接 `Content-Type` + `Content-Disposition` 返回流。

### §26.5 DTO 与 OpenAPI（硬性）

* **接口禁止直接返回领域结构体**：handler 一律转 `xxxResponse` DTO。领域结构体没有 json tag，直接返回会把 `VersionNo` / `EmotionalGoal` 这类 Go 字段名漏给前端。
* OpenAPI 文档必须与代码同步，且有一条防漂移测试守住「文档中的端点 = 路由注册的端点」。

## §27 任务系统

所有长任务异步执行，可查看进度、可重试、可取消。

```
Task { id, project_id, type, status, progress, input, output, error, created_at, started_at, finished_at }
status ∈ {PENDING, RUNNING, PAUSED, COMPLETED, FAILED, CANCELLED}
```

实现要点：PostgreSQL 队列 + `FOR UPDATE SKIP LOCKED` 原子领取 + worker 池 + 进度节流上报 + panic 兜底 + 自动重试。

任务类型：`original_reparse`、4 个分析任务（§22.1）、`writing_chapter`（§12.5）、`consistency_check`（§14）。

## §28 检索系统

`原著章节 → Chunk → Embedding → 向量库`；检索时组合：语义搜索 + 章节范围 + 人物 + 时间 + 场景。

**不要只靠向量相似度。**

## §29 文件与存储

本地文件系统实现，抽象为 Storage 接口（`Save / Open / Delete`），路径带 SHA256 与路径安全校验。生产环境可替换为 OSS/S3/MinIO，业务层不感知。

## §30 导出

格式：**TXT / Markdown / DOCX**；结构：作品 → 卷 → 章。

* DOCX 用自建最小 OOXML 生成，不引第三方依赖；
* 非法格式返回 400，不做静默降级；
* 走 `GET /creative/{id}/export?format=`，二进制流响应。

## §31 前端页面契约

前端路由与菜单必须与下列结构一致（`PRODUCT_SPEC §8` 同源，本表为准）：

```
/
├── dashboard
├── projects
├── original
│   ├── overview / chapters / characters / relationships
│   ├── world / locations / factions
│   └── plot / timeline / knowledge
├── creative
│   ├── overview / settings / characters / world / plot
│   ├── timeline / outline / chapters / materials / mappings
├── editor
├── ai
├── consistency
├── tasks
└── settings
```

主界面为三栏：左（作品导航）+ 中（当前章节编辑器）+ 右（AI 助手）。

**硬性要求**：路由可以先建，但**不允许用占位页假装功能已实现** —— 未实现的模块必须在页面上如实说明「未实现 / 规划中」，并在 `§36` 的对账表里登记。

## §32 测试策略

| 层 | 要求 |
|---|---|
| Backend Unit | domain / service / parser / storage / ai 的纯逻辑测试 |
| Backend API | httptest 端到端，覆盖错误码与边界 |
| Frontend | 组件测试（Vitest + Testing Library），请求打桩 |
| 冒烟 | `scripts/smoke-*.sh` 覆盖面到面主链路，用 `trap` 自清理，不在库里留数据 |
| AI | JSON Schema 测试、Prompt 回归测试、上下文组装测试 |

核心必测案例：人物提取、人物 DNA、人物融合、世界继承、时间线继承、分叉点、二创事件、章节生成、一致性检查。

测试**不得污染开发库**：用真实 PG 测试库并在结束后清理。

## §33 安全与配置

* API Key、`NOVAMIND_SECRET` 等密钥：**只存库（加密）或环境变量**，严禁硬编码、严禁提交 Git；
* 密钥加密使用 AES-256-GCM，主密钥来自 `NOVAMIND_SECRET`；
* 接口响应与日志**永不出现**密钥明文；
* `.env` 不提交；配置全部走环境变量 + `.env`；
* 缺关键配置要 fail fast，不许静默降级成「假健康」。

## §34 交付纪律

每个 Phase 收尾必须：

```
编译 → 测试 → 启动 → 验证核心流程 → 更新 docs/CODEX_STATE.md → 更新 docs/CHANGELOG.md
```

其他纪律：

* 不许「为实现一个功能顺手重构全项目」；改动前先读模块 → 判断能否复用 → 先设计 → 再改 → 再测 → 再更新状态；
* 错误必须包装（`fmt.Errorf("...: %w", err)`），统一在 api 层翻译成错误码，不吞错；
* 日志结构化，含 `trace_id / project_id / task_id`。

---

# 第五部分 · 计划与验收

## §35 Phase 划分与验收标准

| Phase | 内容 | 状态 |
|---|---|---|
| Phase 1 | Go 后端骨架 + React 前端骨架 + PostgreSQL + Redis + 基础 API + 项目管理 | ✅ |
| Phase 2 | 原著导入与解析、章节、人物/DNA/关系、世界观、事件/时间线/剧情 | ✅ |
| Phase 3 | Model Gateway + Prompt Engine + 任务系统 + 分阶段分析（提案→审核） | ✅ |
| Phase 4 | 二创：作品、人物继承与融合、世界继承、分叉点、二创时间线、映射 | ✅ |
| Phase 5 | 写作系统：大纲/卷/章节/场景、编辑器、AI 写作、版本 | ✅ |
| Phase 6 | 一致性检查与问题处理 | ✅ |
| Phase 7 | 导出（TXT/MD/DOCX）与版本历史 | ✅ |
| Phase 8 | 补齐规划中模块：二创设定、二创剧情、素材、知识库、AI 助手页 | 待做 |

**MVP 验收标准（12 项核心能力全通才算完成）**：

上传原著 → 识别章节 → 分析人物 → 分析世界 → 分析时间线 → **作者修改分析结果** → 创建二创 → 继承人物 → **修改人物 DNA** → 继承世界 → 改世界规则 → 选分叉点 → 生成大纲 → 生成章节 → AI 一致性检查 → 作者修改 → 导出。

对应 12 个能力：原著 / 人物 / 人物 DNA / 世界 / 时间线 / 分叉点 / 二创 / 大纲 / 章节 / 编辑器 / AI 写作 / 一致性。

## §36 当前对账与未完成项

### §36.1 已交付（12 项核心能力）

全部 ✅，证据见 `CHANGELOG.md` 与 `scripts/smoke-*.sh`。

### §36.2 尚未实现（必须如实标注，不得假装可用）

| 模块 | 状态 | 说明 |
|---|---|---|
| 二创 · 设定（`/creative/settings`） | 未实现 | 二创作品基本信息已有接口，页面待做 |
| 二创 · 剧情（`/creative/plot`） | 未实现 | 能力暂分散在时间线与分叉点（§11） |
| 二创 · 素材（`/creative/materials`） | 未实现 | 素材模型未建（§15） |
| 原著 · 人物关系（`/original/relationships`） | 未实现独立页 | 关系数据与接口已有，入口在「原著 · 人物」页内 |
| 原著 · 知识库（`/original/knowledge`） | 未实现 | 检索系统（§28）尚未落地 pgvector |
| AI 助手（`/ai`） | 未实现独立页 | AI 能力现都在编辑器与任务中心内 |

### §36.3 已知限制

1. 扫描件与私有编码 PDF 只能靠 OCR（明确提示，不做 OCR）；
2. 版本恢复不回滚删除（刻意设计，见 §13）；
3. AES-256（R5/R6）加密 PDF 不支持，提示另存后导入；
4. 编辑器 Markdown 转换器只覆盖写作子集，子集外按纯文本处理；
5. 本机无 Docker，本地验收不依赖容器。

---

## 变更记录

| 日期 | 变更 |
|---|---|
| 2026-10-03 | 建立本文件。NovaMind V2 自此拥有独立开发规格，**不再引用 v1 的《NovaMind V1 开发规格说明书》**；全仓 158 处「规格书 §N」引用统一改为「SPEC.md §N」。 |
