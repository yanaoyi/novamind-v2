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
