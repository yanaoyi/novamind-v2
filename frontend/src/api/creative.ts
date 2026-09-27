import { request } from './client'
import type {
  CharacterDNA,
  CreativeCharacter,
  CreativeMapping,
  CreativeTimelineEvent,
  CreativeWork,
  CreativeWorld,
  DivergencePoint,
  InheritanceRule,
  ListOf,
  WorldInheritanceMode,
} from './types'

/** 从原著创建二创作品 */
export function createCreativeWork(
  originalWorkId: string,
  input: { project_id: string; title: string; description?: string },
): Promise<CreativeWork> {
  return request<CreativeWork>(`/original/${originalWorkId}/create-creative`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export const creativeApi = {
  get(id: string): Promise<CreativeWork> {
    return request<CreativeWork>(`/creative/${id}`)
  },
  update(id: string, input: { title?: string; description?: string; status?: string }): Promise<CreativeWork> {
    return request<CreativeWork>(`/creative/${id}`, { method: 'PUT', body: JSON.stringify(input) })
  },
  listByOriginal(originalWorkId: string) {
    return request<ListOf<CreativeWork>>(`/original/${originalWorkId}/creative-works`)
  },

  listCharacters(workId: string) {
    return request<ListOf<CreativeCharacter>>(`/creative/${workId}/characters`)
  },
  getCharacter(id: string): Promise<CreativeCharacter> {
    return request<CreativeCharacter>(`/creative-characters/${id}`)
  },
  inheritCharacter(
    workId: string,
    input: { source_character_id: string; name?: string; description?: string; importance?: number; weights: Record<string, number> },
  ): Promise<CreativeCharacter> {
    return request<CreativeCharacter>(`/creative/${workId}/characters/inherit`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  createNewCharacter(
    workId: string,
    input: { name: string; description?: string; importance?: number; dna?: CharacterDNA },
  ): Promise<CreativeCharacter> {
    return request<CreativeCharacter>(`/creative/${workId}/characters/new`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  fuseCharacters(
    workId: string,
    input: { name: string; description?: string; importance?: number; sources: Array<{ character_id: string; weight: number }> },
  ): Promise<CreativeCharacter> {
    return request<CreativeCharacter>(`/creative/${workId}/characters/fuse`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  updateCharacter(
    id: string,
    input: { name?: string; description?: string; dna?: CharacterDNA; importance?: number; is_locked?: boolean },
  ): Promise<CreativeCharacter> {
    return request<CreativeCharacter>(`/creative-characters/${id}`, { method: 'PUT', body: JSON.stringify(input) })
  },
  deleteCharacter(id: string) {
    return request<{ deleted: boolean }>(`/creative-characters/${id}`, { method: 'DELETE' })
  },

  getWorld(workId: string): Promise<CreativeWorld> {
    return request<CreativeWorld>(`/creative/${workId}/world`)
  },
  inheritWorld(workId: string, mode: WorldInheritanceMode): Promise<CreativeWorld> {
    return request<CreativeWorld>(`/creative/${workId}/world/inherit`, {
      method: 'POST',
      body: JSON.stringify({ mode }),
    })
  },
  updateWorld(workId: string, input: { name?: string; description?: string; inheritance_mode?: WorldInheritanceMode }) {
    return request<CreativeWorld>(`/creative/${workId}/world`, { method: 'PUT', body: JSON.stringify(input) })
  },
  createWorldRule(
    workId: string,
    input: { name: string; category?: string; description?: string; importance?: number; source_rule_id?: string; status?: string },
  ) {
    return request<{ id: string }>(`/creative/${workId}/world/rules`, { method: 'POST', body: JSON.stringify(input) })
  },
  updateWorldRule(
    id: string,
    input: { name?: string; category?: string; description?: string; importance?: number; status?: string },
  ) {
    return request<{ id: string }>(`/creative-world-rules/${id}`, { method: 'PUT', body: JSON.stringify(input) })
  },
  deleteWorldRule(id: string) {
    return request<{ deleted: boolean }>(`/creative-world-rules/${id}`, { method: 'DELETE' })
  },

  getDivergence(workId: string): Promise<DivergencePoint | null> {
    return request<DivergencePoint | null>(`/creative/${workId}/divergence`)
  },
  setDivergence(
    workId: string,
    input: { original_event_id?: string | null; original_chapter_id?: string | null; time_label?: string; description?: string },
  ): Promise<DivergencePoint> {
    return request<DivergencePoint>(`/creative/${workId}/divergence`, { method: 'PUT', body: JSON.stringify(input) })
  },

  getTimeline(workId: string) {
    return request<ListOf<CreativeTimelineEvent>>(`/creative/${workId}/timeline`)
  },
  buildTimeline(workId: string) {
    return request<ListOf<CreativeTimelineEvent>>(`/creative/${workId}/timeline/build`, { method: 'POST' })
  },
  saveTimeline(
    workId: string,
    items: Array<{ source_original_event_id?: string | null; status?: string; time_label?: string; title: string; description?: string }>,
  ) {
    return request<ListOf<CreativeTimelineEvent>>(`/creative/${workId}/timeline`, {
      method: 'PUT',
      body: JSON.stringify({ items }),
    })
  },

  listMappings(workId: string) {
    return request<ListOf<CreativeMapping>>(`/creative/${workId}/mappings`)
  },
}

export type { InheritanceRule }
