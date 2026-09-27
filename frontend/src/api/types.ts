// 与后端 OpenAPI（backend/internal/api/openapi.yaml）保持一致。
// 后端契约变更时，同步改这里；后端有防漂移测试，前端类型以此文件为准。

/** 统一响应包：所有接口都返回这个结构 */
export interface Envelope<T> {
  data: T | null
  error: ApiErrorBody | null
  trace_id: string
}

export interface ApiErrorBody {
  code: string
  message: string
  details?: Record<string, unknown> | null
}

/** ORIGINAL=原著工程，CREATIVE=二创工程；创建后不可变更 */
export type ProjectType = 'ORIGINAL' | 'CREATIVE'

export type ProjectStatus = 'DRAFT' | 'ACTIVE' | 'ARCHIVED'

export interface Project {
  id: string
  name: string
  description: string
  type: ProjectType
  status: ProjectStatus
  created_at: string
  updated_at: string
}

export interface ProjectList {
  items: Project[]
  total: number
  page: number
  page_size: number
}

export interface ProjectListQuery {
  page?: number
  page_size?: number
  type?: ProjectType
  status?: ProjectStatus
  keyword?: string
}

export interface CreateProjectInput {
  name: string
  description?: string
  type: ProjectType
}

/** 缺省字段表示不修改；type 不可更新 */
export interface UpdateProjectInput {
  name?: string
  description?: string
  status?: ProjectStatus
}

// ---------- 原著（对应后端 /api/v1/original） ----------

export type SourceType = 'MANUAL' | 'TXT' | 'DOCX' | 'PDF'
export type OriginalStatus = 'DRAFT' | 'PARSED' | 'ANALYZED'

export interface Original {
  id: string
  project_id: string
  title: string
  author: string
  description: string
  source_type: SourceType
  status: OriginalStatus
  char_count: number
  chapter_count: number
  created_at: string
  updated_at: string
}

export interface CreateOriginalInput {
  title: string
  author?: string
  description?: string
}

export interface OriginalChapterBrief {
  chapter_no: number
  title: string
  char_count: number
  summary: string
}

export interface OriginalChapterDetail extends OriginalChapterBrief {
  content: string
  start_position: number
  end_position: number
}

export interface OriginalChapterList {
  items: OriginalChapterBrief[]
  total: number
  page: number
  page_size: number
}

export interface OriginalImportResult {
  file_id: string
  file_name: string
  size_bytes: number
  encoding: string
  char_count: number
  chapter_count: number
  chapters: OriginalChapterBrief[]
}

// ---------- 人物 / 人物 DNA / 人物关系（对应后端 /characters 与 /relationships） ----------

export interface DNADimension {
  text: string
  /** 继承权重 0-100 */
  weight: number
}

/** DNA 的 11 个维度（与后端 domain.CharacterDNA 一致） */
export const DNA_KEYS = [
  'personality',
  'values',
  'motivation',
  'behavior',
  'speech_style',
  'background',
  'ability',
  'decision_style',
  'conflict_response',
  'emotional_response',
  'relationship_pattern',
] as const

export type DNAKey = (typeof DNA_KEYS)[number]

export const DNA_LABELS: Record<DNAKey, string> = {
  personality: '人格',
  values: '价值观',
  motivation: '动机',
  behavior: '行为',
  speech_style: '语言风格',
  background: '背景',
  ability: '能力',
  decision_style: '决策方式',
  conflict_response: '冲突反应',
  emotional_response: '情绪反应',
  relationship_pattern: '关系模式',
}

export type CharacterDNA = Record<DNAKey, DNADimension>

export function emptyDNA(): CharacterDNA {
  return DNA_KEYS.reduce((acc, key) => {
    acc[key] = { text: '', weight: 0 }
    return acc
  }, {} as CharacterDNA)
}

export type CharacterSource = 'MANUAL' | 'AI'

export interface Character {
  id: string
  original_work_id: string
  name: string
  aliases: string[]
  role: string
  gender: string
  age: string
  appearance: string
  personality: string
  motivation: string
  values: string
  fears: string
  desires: string
  behavior_patterns: string
  speech_style: string
  abilities: string
  first_appearance: string
  last_appearance: string
  dna: CharacterDNA
  importance: number
  source: CharacterSource
  notes: string
  created_at: string
  updated_at: string
}

export interface CharacterInput {
  name: string
  aliases?: string[]
  role?: string
  gender?: string
  age?: string
  appearance?: string
  personality?: string
  motivation?: string
  values?: string
  fears?: string
  desires?: string
  behavior_patterns?: string
  speech_style?: string
  abilities?: string
  first_appearance?: string
  last_appearance?: string
  dna?: CharacterDNA
  importance?: number
  notes?: string
}

export interface CharacterList {
  items: Character[]
  total: number
  page: number
  page_size: number
}

export type RelationType =
  | 'family'
  | 'friend'
  | 'lover'
  | 'enemy'
  | 'mentor'
  | 'student'
  | 'colleague'
  | 'rival'
  | 'organization'
  | 'other'

export const RELATION_LABELS: Record<RelationType, string> = {
  family: '亲属',
  friend: '朋友',
  lover: '恋人',
  enemy: '敌对',
  mentor: '师长',
  student: '学生',
  colleague: '同僚',
  rival: '竞争',
  organization: '组织',
  other: '其他',
}

export interface Relationship {
  id: string
  original_work_id: string
  source_character_id: string
  target_character_id: string
  relation_type: RelationType
  strength: number
  description: string
  source: CharacterSource
  created_at: string
  updated_at: string
}

export interface RelationshipInput {
  source_character_id: string
  target_character_id: string
  relation_type: RelationType
  strength?: number
  description?: string
}

// ---------- 世界观（对应后端 /world、/rules、/locations、/factions） ----------

export interface World {
  id: string
  original_work_id: string
  name: string
  description: string
  rule_count: number
  location_count: number
  faction_count: number
  created_at: string
  updated_at: string
}

export interface WorldRule {
  id: string
  world_id: string
  category: string
  name: string
  description: string
  importance: number
  created_at: string
  updated_at: string
}

export interface WorldRuleInput {
  category?: string
  name: string
  description?: string
  importance?: number
}

export interface WorldLocation {
  id: string
  world_id: string
  name: string
  type: string
  description: string
  parent_location_id: string | null
  created_at: string
  updated_at: string
}

export interface WorldLocationInput {
  name: string
  type?: string
  description?: string
  parent_location_id?: string | null
}

export interface Faction {
  id: string
  world_id: string
  name: string
  type: string
  description: string
  goals: string
  relationships: string
  created_at: string
  updated_at: string
}

export interface FactionInput {
  name: string
  type?: string
  description?: string
  goals?: string
  relationships?: string
}

/** 后端列表接口的统一形状（世界观下的规则/地点/势力用它） */
export interface ListOf<T> {
  items: T[]
  total: number
}

// ---------- 事件 / 时间线 / 剧情弧 ----------

export interface OriginalEvent {
  id: string
  original_work_id: string
  title: string
  description: string
  chapter_no: number | null
  time_order: number
  participants: string[]
  location_id: string | null
  location_text: string
  consequences: string
  importance: number
  source: CharacterSource
  created_at: string
  updated_at: string
}

export interface EventInput {
  title: string
  description?: string
  chapter_no?: number | null
  time_order?: number
  participants?: string[]
  location_id?: string | null
  location_text?: string
  consequences?: string
  importance?: number
}

export interface EventList {
  items: OriginalEvent[]
  total: number
  page: number
  page_size: number
}

export interface TimelineItem {
  event_id: string
  time_label?: string
  duration?: string
}

export interface TimelineEntry {
  sequence: number
  time_label: string
  duration: string
  event: OriginalEvent
}

export interface Timeline {
  id: string
  name: string
  description: string
  entries: TimelineEntry[]
}

export type PlotArcType = 'main' | 'subplot' | 'character_arc' | 'relationship_arc' | 'world_arc'

export const PLOT_ARC_LABELS: Record<PlotArcType, string> = {
  main: '主线',
  subplot: '支线',
  character_arc: '人物线',
  relationship_arc: '感情线',
  world_arc: '世界线',
}

export interface PlotArc {
  id: string
  original_work_id: string
  type: PlotArcType
  title: string
  summary: string
  start_event_id: string | null
  end_event_id: string | null
  created_at: string
  updated_at: string
}

export interface PlotArcInput {
  type?: PlotArcType
  title: string
  summary?: string
  start_event_id?: string | null
  end_event_id?: string | null
}

// ---------- 模型接入（对应后端 /model-providers、/prompts） ----------

export type ProviderType = 'OPENAI_COMPATIBLE' | 'ANTHROPIC'
export type ProviderPurpose = 'chat' | 'embedding' | 'both'

export const PROVIDER_LABELS: Record<ProviderType, string> = {
  OPENAI_COMPATIBLE: 'OpenAI 兼容（DeepSeek / 智谱 / Kimi / vLLM…）',
  ANTHROPIC: 'Anthropic（Claude）',
}

export interface ModelProvider {
  id: string
  name: string
  provider: ProviderType
  api_base: string
  model_name: string
  purpose: ProviderPurpose
  temperature: number
  max_tokens: number
  timeout_sec: number
  enabled: boolean
  is_default: boolean
  notes: string
  /** 后端只暴露这个布尔值，永不返回密钥本身 */
  has_api_key: boolean
  created_at: string
  updated_at: string
}

export interface ModelProviderInput {
  name: string
  provider: ProviderType
  api_base: string
  /** 留空表示保持原密钥不变 */
  api_key?: string
  model_name: string
  purpose?: ProviderPurpose
  temperature?: number
  max_tokens?: number
  timeout_sec?: number
  enabled?: boolean
  is_default?: boolean
  notes?: string
}

export interface ModelProviderTestResult {
  ok: boolean
  model: string
  reply: string
  latency_ms: number
  total_tokens: number
  error_message?: string
}

export interface PromptMeta {
  name: string
  version: string
  path: string
}

// ---------- 异步任务（对应后端 /tasks） ----------

export type TaskStatus = 'PENDING' | 'RUNNING' | 'PAUSED' | 'COMPLETED' | 'FAILED' | 'CANCELLED'

export const TASK_STATUS_LABEL: Record<TaskStatus, string> = {
  PENDING: '排队中',
  RUNNING: '执行中',
  PAUSED: '已暂停',
  COMPLETED: '已完成',
  FAILED: '失败',
  CANCELLED: '已取消',
}

export const TASK_STATUS_COLOR: Record<TaskStatus, string> = {
  PENDING: 'default',
  RUNNING: 'processing',
  PAUSED: 'warning',
  COMPLETED: 'success',
  FAILED: 'error',
  CANCELLED: 'default',
}

export interface Task {
  id: string
  project_id: string | null
  work_id: string | null
  type: string
  status: TaskStatus
  progress: number
  progress_message: string
  input: Record<string, unknown>
  output: Record<string, unknown>
  error: string
  attempts: number
  max_attempts: number
  created_at: string
  started_at: string | null
  finished_at: string | null
  updated_at: string
}

export interface TaskList {
  items: Task[]
  total: number
  page: number
  page_size: number
}

// ---------- AI 分析提案（对应后端 /proposals 与 /analysis） ----------

export type AnalysisStage = 'chapter_summary' | 'character_extract' | 'world_extract' | 'plot_extract'

export const STAGE_LABELS: Record<AnalysisStage, string> = {
  chapter_summary: '章节摘要',
  character_extract: '人物提取',
  world_extract: '世界观提取',
  plot_extract: '剧情与事件提取',
}

export type ProposalEntity =
  | 'chapter_summary'
  | 'character'
  | 'world'
  | 'world_rule'
  | 'location'
  | 'faction'
  | 'event'
  | 'plot_arc'

export const ENTITY_LABELS: Record<ProposalEntity, string> = {
  chapter_summary: '章节摘要',
  character: '人物',
  world: '世界设定',
  world_rule: '世界规则',
  location: '地点',
  faction: '势力',
  event: '事件',
  plot_arc: '剧情弧',
}

export type ProposalStatus = 'PENDING' | 'APPROVED' | 'REJECTED'

export const PROPOSAL_STATUS_LABEL: Record<ProposalStatus, string> = {
  PENDING: '待审核',
  APPROVED: '已通过',
  REJECTED: '已驳回',
}

export interface Proposal {
  id: string
  work_id: string
  task_id: string | null
  stage: AnalysisStage
  entity_type: ProposalEntity
  title: string
  payload: Record<string, unknown>
  evidence: string
  confidence: number
  status: ProposalStatus
  review_note: string
  reviewed_at: string | null
  applied_id: string | null
  created_at: string
  updated_at: string
}

export interface ProposalList {
  items: Proposal[]
  total: number
  page: number
  page_size: number
}

export interface ProposalSummary {
  pending: number
  approved: number
  rejected: number
}
