import { request } from './client'
import type {
  EntityVersion,
  EntityVersionType,
  ListOf,
  VersionCompare,
  VersionCompareEntityType,
} from './types'

/** 版本历史（规格书 §59）：人物 / 世界观 / 大纲 */
export const versionApi = {
  /** 列表（不带 payload，避免一次拉回几十份快照） */
  list(entityType: EntityVersionType, entityId: string) {
    return request<ListOf<EntityVersion>>(pathFor(entityType, entityId, 'versions'))
  },
  /** 详情（含快照内容） */
  get(entityType: EntityVersionType, entityId: string, no: number) {
    return request<EntityVersion>(pathFor(entityType, entityId, `versions/${no}`))
  },
  /** 恢复 */
  restore(entityType: EntityVersionType, entityId: string, no: number) {
    return request<{ restored: boolean; version_no: number }>(
      pathFor(entityType, entityId, `versions/${no}/restore`),
      { method: 'POST' },
    )
  },
  /** 手动存档（内容与最新一版相同则不会新建） */
  snapshot(entityType: EntityVersionType, entityId: string, note?: string) {
    return request<{ created?: boolean; reason?: string; version_no?: number }>(
      pathFor(entityType, entityId, 'versions'),
      { method: 'POST', body: JSON.stringify({ note: note ?? '' }) },
    )
  },
  /** 版本比较（规格书 §59）；版本号传 0 表示「当前状态」 */
  compare(params: {
    entityType: VersionCompareEntityType
    entityId: string
    from: number
    to: number
  }) {
    const query = new URLSearchParams({
      entity_type: params.entityType,
      entity_id: params.entityId,
      from: String(params.from),
      to: String(params.to),
    })
    return request<VersionCompare>(`/versions/compare?${query.toString()}`)
  },
}

function pathFor(entityType: EntityVersionType, entityId: string, suffix: string): string {
  switch (entityType) {
    case 'creative_character':
      return `/creative-characters/${entityId}/${suffix}`
    case 'creative_world':
      return `/creative/${entityId}/world/${suffix}`
    case 'creative_outline':
      return `/creative/${entityId}/outline/${suffix}`
    case 'creative_outline_tree':
      return `/outlines/${entityId}/${suffix}`
  }
}
