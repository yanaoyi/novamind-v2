# PRODUCT_SPEC.md — 产品需求与功能定义

> 项目：NovaMind V2（全新实现）
> 规格来源：`../../novamind-pro/NovaMind_V1_开发规格说明书.md`（2571 行，逐条执行）
> 本文件是产品层面的唯一裁定文件。与 ARCHITECTURE.md / CODEX_STATE.md 冲突时，以本文件为准。

---

## 1. 产品定位

NovaMind 是一个以 **AI 长篇小说创作** 为核心的专业创作系统，第一目标不是"AI 续写"，而是：

> 让作者导入一部原著 → 把原著结构化成可编辑的世界模型 → 基于该模型进行**受控的**二次创作 → 由 AI 长期辅助写作与一致性维护。

NovaMind **不是**通用 AI Agent 平台。内部可以模块化复用，但产品界面与核心业务必须围绕"AI 小说创作"。

---

## 2. 核心工作流（产品主线）

```
原著导入 → 原著分析 → 原著结构化 → 原著世界模型
   → 作者选择继承/修改/融合 → 建立二创世界模型
   → 确定分叉点 → 生成二创大纲 → AI 写作
   → 一致性检查 → 版本管理 → 导出
```

必须严格区分两个模型，任何时候不得混淆：

| 模型 | 含义 | 可变性 |
|---|---|---|
| **Original Model** | 原著事实（canon / reference） | 只读基础；改动只能由 AI 提取 → 作者审核 → 作者确认 → 写入 |
| **Creative Model** | 作者掌控的二创设定 | 作者可自由增删改 |

---

## 3. 四条不可动摇的产品原则

1. **作者拥有最终控制权**：AI 只做 分析 / 建议 / 生成 / 检查；作者做 确认 / 修改 / 删除 / 决定。
2. **原著与二创必须分离**：`Original = Reference / Canon Model`，`Creative = Author Controlled Model`。
3. **不让 AI 自己决定世界**：AI 只能提出建议、推断、候选方案；关键设定必须由作者确认。
4. **一致性优先于文采**：长期竞争力 = 理解长篇 + 长期记忆 + 人物/世界/时间线/剧情一致 + 作者控制。

---

## 4. V1（本项目交付范围）必须实现

### 4.1 原著侧
创建原著项目、上传 TXT/DOCX/PDF、文本解析、自动章节识别、原著结构分析、人物提取、人物性格分析、人物关系分析、世界观提取、地点提取、势力提取、世界规则提取、剧情事件提取、原著时间线提取、原著知识库。

### 4.2 二创侧
从原著创建二创作品、选择原著元素继承、修改人物、人物融合、修改世界观、修改世界规则、继承原著时间线、创建分叉点、修改/新增事件、创建二创时间线、原著与二创映射关系。

### 4.3 创作侧
故事设定、人物设定、世界观设定、剧情设定、大纲、卷、章节、场景、编辑器、AI 续写/扩写/改写、AI 生成大纲/人物/剧情、AI 场景生成。

### 4.4 一致性
人物一致性、世界观一致性、时间线一致性、剧情一致性、原著继承一致性。

### 4.5 工程
项目管理、文件管理、版本管理、AI 任务管理、模型配置、导出 Markdown/TXT/DOCX。

---

## 5. V1 明确不做（写进代码里也别做）

多用户复杂协作、商业化支付、社交社区、在线发布平台、推荐算法、多租户 SaaS、复杂插件市场、自动训练模型、自研大模型、复杂工作流编排器、多 Agent 自主长期运行、自动发布到第三方平台、复杂知识图谱可视化、移动端 App。

架构必须**允许**未来增加，但不因为这些影响 V1。

---

## 6. 核心领域概念（产品语义，字段细节见 ARCHITECTURE.md）

| 概念 | 一句话定义 |
|---|---|
| Project | 工程容器，type ∈ {ORIGINAL, CREATIVE}；实际结构为 Project → OriginalWork / CreativeWork |
| OriginalWork | 一部原著作品 |
| OriginalChapter | 原著章节（含起止位置，供回溯原文） |
| OriginalCharacter | 原著人物事实（别名、性格、动机、恐惧、行为模式、语言风格、能力…） |
| **CharacterDNA** | 人物抽象特征 + 权重，是继承与融合的核心数据结构（不是复制正文） |
| CharacterRelationship | 人物关系（family/friend/lover/enemy/mentor/student/colleague/rival/organization/other） |
| OriginalWorld / WorldRule / Location / Faction | 原著世界观、规则、地点（可嵌套）、势力 |
| OriginalEvent / TimelineEvent / PlotArc | 原著事件、时间线顺序、剧情弧（main/subplot/character_arc/relationship_arc/world_arc） |
| CreativeWork | 二创作品，必须挂在一个 OriginalWork 上并带 divergence_point |
| CreativeCharacter | 二创人物，source_type ∈ {ORIGINAL_INHERITED, MODIFIED, FUSED, NEW} |
| InheritanceRule | 人物逐维度继承权重（性格/价值观/动机/行为/语言/背景/能力/关系） |
| CreativeWorld / CreativeWorldRule | 二创世界与规则，status ∈ {INHERITED, MODIFIED, REMOVED, NEW} |
| OriginalCreativeMapping | 原著↔二创映射，mapping_type ∈ {INHERITED, MODIFIED, REPLACED, FUSED, REMOVED, NEW} |
| **DivergencePoint** | 分叉点：二创作品必须明确从哪里离开原著 |
| CreativeTimeline / CreativeTimelineEvent | 二创时间线（status ∈ {INHERITED, MODIFIED, NEW, REMOVED}） |
| CreativePlot | 二创剧情线（主线/支线/人物线/感情线/世界线） |
| Outline / OutlineNode | 大纲（作品 → 卷 → 节 → 章 → 场景） |
| Chapter / Scene | 章节（DRAFT/REVIEW/FINAL）与场景 |
| Material | 素材（idea/reference/research/dialogue/scene/description/character/world） |
| ContextSnapshot | 每次 AI 生成前的上下文快照（可调试、可重现、可比较） |
| Task | 异步任务（PENDING/RUNNING/PAUSED/COMPLETED/FAILED/CANCELLED） |

---

## 7. 关键用户流程

**流程 A｜创建原著**：创建项目 → 上传原著 → 解析 → 章节识别 → AI 分析 → 作者审核 → 原著模型完成。

**流程 B｜创建二创**：原著 → 创建二创 → 选择继承 → 人物 DNA → 世界继承 → 时间线 → 分叉点 → 新剧情。

**流程 C｜AI 写作**：选择章节 → 选择场景 → AI 读取 Context → 生成 → 一致性检查 → 作者修改 → 保存版本。

**原著分析必须分阶段**（禁止一次性把整本书丢给 LLM）：
文件解析 → 章节切分 → 章节摘要 → 人物提取 → 人物关系 → 世界元素 → 事件 → 时间线 → 全局剧情。每阶段可重试。

---

## 8. 页面结构（前端路由契约）

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

主界面为三栏：左（作品导航：原著/故事/大纲/人物/世界观/时间线/场景/素材）+ 中（当前章节编辑器）+ 右（AI 助手：续写/扩写/改写/生成/分析/检查）。

编辑器必须支持：Markdown、富文本、标题/段落/引用/加粗/斜体、AI 操作、自动保存（debounce 1–2 秒）、版本恢复。

---

## 9. MVP 验收标准（闭环未完成 = V1 未完成）

用户能连续完成：

上传一本原著 → 系统识别章节 → 分析人物 → 分析世界 → 分析时间线 → **作者修改分析结果** → 创建二创作品 → 继承原著人物 → **修改人物 DNA** → 继承世界 → 修改世界规则 → 选择分叉点 → 生成二创大纲 → 生成章节 → AI 一致性检查 → 作者修改 → 导出小说。

这 12 个核心能力必须做通：原著、人物、人物 DNA、世界、时间线、分叉点、二创、大纲、章节、编辑器、AI 写作、一致性。

---

## 10. 导出与版本

- 导出格式：TXT / Markdown / DOCX；结构：作品 → 卷 → 章。
- 版本管理覆盖：Chapter / Character / World / Outline；支持查看 / 恢复 / 比较。

---

## 11. 交付纪律（本项目的"完成"定义）

每完成一个 Phase：编译 → 测试 → 启动 → 验证核心流程 → 更新 `docs/CODEX_STATE.md` → 更新 `docs/CHANGELOG.md`。

第一阶段不追求功能数量，只追求把骨架与项目管理闭环做扎实。

---

## 12. 交付现状对账（2026-09-27 收尾）

对照 §9「MVP 验收标准」的 12 个核心能力，当前状态如下（每条都有端到端冒烟或前端测试证据，见 `CHANGELOG.md`）：

| # | 核心能力 | 状态 | 界面入口 |
|---|---|---|---|
| 1 | 原著 | ✅ | 原著 · 总览 / 章节 |
| 2 | 人物 | ✅ | 原著 · 人物 |
| 3 | 人物 DNA | ✅ | 原著 · 人物（DNA 编辑器） |
| 4 | 世界 | ✅ | 原著 · 世界观 / 地点 / 势力 |
| 5 | 时间线 | ✅ | 原著 · 时间线 |
| 6 | 分叉点 | ✅ | 二创 · 剧情 / 时间线 |
| 7 | 二创 | ✅ | 二创 · 总览 / 设定 / 人物 / 世界观 |
| 8 | 大纲 | ✅ | 二创 · 大纲（卷 + 章节三要素） |
| 9 | 章节 | ✅ | 二创 · 章节 |
| 10 | 编辑器 | ✅ | 编辑器（自动保存 / 版本 / AI 操作） |
| 11 | AI 写作 | ✅ | 编辑器内「让 AI 写本章」+ AI 改写/扩写/缩写/润色/增强冲突/增强情绪 |
| 12 | 一致性 | ✅ | 一致性检查 |

外加 §10 的 **导出与版本**：导出 TXT / Markdown / DOCX（结构为「作品 → 卷 → 章」）✅；版本管理覆盖 **Chapter / Character / World / Outline 四类**（章节用 `chapter_versions`，其余三类用通用快照表 `entity_versions`）✅。

**2026-09-28 补齐的三项**（上一轮收尾时如实列为缺口的）：

1. **PDF 解析** ✅ —— §4.1 的 TXT / DOCX / PDF 三种上传格式现在全部支持。自研解析器：ToUnicode CMap、编码名兜底（GBK/Big5/KSC/Unicode）、内嵌 TrueType cmap 反查、注释外观流、空密码加密（RC4/AESV2）解密；扫描件明确提示需要 OCR。真实样本对照 PyMuPDF：整体字符召回 **0.9569**、准确 **0.9595**（120 个样本，67 个可用文件 ≥0.98；36 个是素材本身没有文本层的扫描件，7 个是基准自身乱码的文件）。
2. **富文本编辑器** ✅ —— §8 要求的富文本编辑已实现（Tiptap：标题/加粗/斜体/删除线/引用/列表/分割线 + Markdown 源码双模式）。**正文仍以 Markdown 存储**，因此 AI 上下文、版本快照、导出格式都不受影响。
3. **人物 / 世界观 / 大纲的版本历史** ✅ —— §10 要求的四类版本管理全部到位：可查看历史、预览快照、一键恢复。

### 12.1 仍然存在的限制（写清楚，别让人误以为有）

1. **扫描件与私有编码 PDF 只能靠 OCR**：纯图片 PDF（本机样本里约 30%）没有文本层；方正书版私有编码、ToUnicode 空壳的 PDF 连 MuPDF 抽出来都是乱码。这两类目前都只给出明确提示，不做 OCR。
2. **版本恢复不回滚删除**：快照是「某一刻长什么样」，恢复只回填快照里记录的字段；快照之后新建的人物/规则/章节不会消失（这是刻意的，避免作者丢东西）。
3. **AES-256（R5/R6）加密的 PDF 不支持**：会明确提示「请用阅读器另存一份再导入」，不做静默失败。
4. **编辑器转换器只覆盖写作子集**：表格、脚注、图片等 Markdown 扩展语法不解析，按纯文本处理（不丢字，但不会渲染成富文本）。
