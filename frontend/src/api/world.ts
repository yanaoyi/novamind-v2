import { request } from './client'
import type {
  Faction,
  FactionInput,
  ListOf,
  World,
  WorldLocation,
  WorldLocationInput,
  WorldRule,
  WorldRuleInput,
} from './types'

/** 原著世界观接口（世界 / 规则 / 地点 / 势力） */
export const worldApi = {
  /** 世界观概览；尚未创建世界时返回 null */
  async get(workId: string): Promise<World | null> {
    try {
      return await request<World>(`/original/${workId}/world`)
    } catch (err) {
      // 404 = 还没建世界，不是错误
      if (typeof err === 'object' && err !== null && 'status' in err && (err as { status: number }).status === 404) {
        return null
      }
      throw err
    }
  },

  save(workId: string, input: { name?: string; description?: string }): Promise<World> {
    return request<World>(`/original/${workId}/world`, {
      method: 'PUT',
      body: JSON.stringify(input),
    })
  },

  listRules(workId: string) {
    return request<ListOf<WorldRule>>(`/original/${workId}/rules`)
  },
  createRule(workId: string, input: WorldRuleInput): Promise<WorldRule> {
    return request<WorldRule>(`/original/${workId}/rules`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  updateRule(id: string, input: WorldRuleInput): Promise<WorldRule> {
    return request<WorldRule>(`/world-rules/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    })
  },
  removeRule(id: string): Promise<{ deleted: boolean }> {
    return request<{ deleted: boolean }>(`/world-rules/${id}`, { method: 'DELETE' })
  },

  listLocations(workId: string) {
    return request<ListOf<WorldLocation>>(`/original/${workId}/locations`)
  },
  createLocation(workId: string, input: WorldLocationInput): Promise<WorldLocation> {
    return request<WorldLocation>(`/original/${workId}/locations`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  updateLocation(id: string, input: WorldLocationInput): Promise<WorldLocation> {
    return request<WorldLocation>(`/locations/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    })
  },
  removeLocation(id: string): Promise<{ deleted: boolean }> {
    return request<{ deleted: boolean }>(`/locations/${id}`, { method: 'DELETE' })
  },

  listFactions(workId: string) {
    return request<ListOf<Faction>>(`/original/${workId}/factions`)
  },
  createFaction(workId: string, input: FactionInput): Promise<Faction> {
    return request<Faction>(`/original/${workId}/factions`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  updateFaction(id: string, input: FactionInput): Promise<Faction> {
    return request<Faction>(`/factions/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    })
  },
  removeFaction(id: string): Promise<{ deleted: boolean }> {
    return request<{ deleted: boolean }>(`/factions/${id}`, { method: 'DELETE' })
  },
}
