import { request } from './client'
import type {
  Character,
  CharacterInput,
  CharacterList,
  Relationship,
  RelationshipInput,
} from './types'

/** 原著人物与人物关系接口 */
export const characterApi = {
  list(workId: string, params: { page?: number; pageSize?: number; keyword?: string } = {}) {
    const query = new URLSearchParams()
    if (params.page) query.set('page', String(params.page))
    if (params.pageSize) query.set('page_size', String(params.pageSize))
    if (params.keyword) query.set('keyword', params.keyword)
    const qs = query.toString()
    return request<CharacterList>(`/original/${workId}/characters${qs ? `?${qs}` : ''}`)
  },

  create(workId: string, input: CharacterInput): Promise<Character> {
    return request<Character>(`/original/${workId}/characters`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  update(id: string, input: CharacterInput): Promise<Character> {
    return request<Character>(`/characters/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    })
  },

  remove(id: string): Promise<{ deleted: boolean }> {
    return request<{ deleted: boolean }>(`/characters/${id}`, { method: 'DELETE' })
  },

  listRelationships(workId: string) {
    return request<{ items: Relationship[]; total: number }>(`/original/${workId}/relationships`)
  },

  createRelationship(workId: string, input: RelationshipInput): Promise<Relationship> {
    return request<Relationship>(`/original/${workId}/relationships`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  updateRelationship(
    id: string,
    input: Partial<Pick<RelationshipInput, 'relation_type' | 'strength' | 'description'>>,
  ): Promise<Relationship> {
    return request<Relationship>(`/relationships/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    })
  },

  removeRelationship(id: string): Promise<{ deleted: boolean }> {
    return request<{ deleted: boolean }>(`/relationships/${id}`, { method: 'DELETE' })
  },
}
