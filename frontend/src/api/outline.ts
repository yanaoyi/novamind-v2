import { request } from './client'
import type {
  EntityVersion,
  ListOf,
  Outline,
  OutlineDetail,
  OutlineMaterializeResult,
  OutlineNode,
  OutlineNodeInput,
} from './types'

export interface CreateOutlineInput {
  title: string
  summary?: string
  version?: number
  source?: 'MANUAL' | 'AI'
  /** 为空则创建空大纲；非空则一次性写入整棵树 */
  nodes?: OutlineNodeInput[]
}

export interface OutlineNodePatch {
  title?: string
  summary?: string
  purpose?: string
  characters?: string[]
  location?: string
  conflict?: string
  outcome?: string
  sequence?: number
}

/** 大纲（规格书 §27）：卷 → 节 → 章的独立模型 */
export const outlineApi = {
  list(workId: string) {
    return request<ListOf<Outline>>(`/creative/${workId}/outlines`)
  },
  create(workId: string, input: CreateOutlineInput) {
    return request<OutlineDetail>(`/creative/${workId}/outlines`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  get(outlineId: string) {
    return request<OutlineDetail>(`/outlines/${outlineId}`)
  },
  update(outlineId: string, patch: { title?: string; summary?: string; version?: number }) {
    return request<OutlineDetail>(`/outlines/${outlineId}`, {
      method: 'PUT',
      body: JSON.stringify(patch),
    })
  },
  remove(outlineId: string) {
    return request<{ deleted: boolean }>(`/outlines/${outlineId}`, { method: 'DELETE' })
  },
  /** 整树替换（作者在 AI 候选上改完后整体提交走这里） */
  replaceTree(outlineId: string, nodes: OutlineNodeInput[]) {
    return request<OutlineDetail>(`/outlines/${outlineId}/tree`, {
      method: 'PUT',
      body: JSON.stringify({ nodes }),
    })
  },
  /** 新增节点：不传 parent_id 就是「卷」，层级由父节点推导 */
  createNode(outlineId: string, input: OutlineNodePatch & { parent_id?: string | null }) {
    return request<OutlineNode>(`/outlines/${outlineId}/nodes`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  updateNode(nodeId: string, patch: OutlineNodePatch) {
    return request<OutlineNode>(`/outline-nodes/${nodeId}`, {
      method: 'PUT',
      body: JSON.stringify(patch),
    })
  },
  /** 删除节点（连同子树） */
  removeNode(nodeId: string) {
    return request<{ deleted: boolean; deleted_nodes: number }>(`/outline-nodes/${nodeId}`, {
      method: 'DELETE',
    })
  },
  /** 把「章」节点落成写作系统的卷与章节（卷按标题复用，章节只追加） */
  materialize(outlineId: string) {
    return request<OutlineMaterializeResult>(`/outlines/${outlineId}/materialize`, { method: 'POST' })
  },
  versions(outlineId: string) {
    return request<ListOf<EntityVersion>>(`/outlines/${outlineId}/versions`)
  },
}

/** AI 生成（规格书 §38 §49）：只返回候选，不落库；作者确认后走 outlineApi 写入 */
export const aiApi = {
  generate(kind: 'outline' | 'character' | 'plot' | 'scene', input: { workId: string; instruction?: string; chapterId?: string }) {
    return request<Record<string, unknown>>('/ai/generate', {
      method: 'POST',
      body: JSON.stringify({
        kind,
        work_id: input.workId,
        chapter_id: input.chapterId,
        instruction: input.instruction ?? '',
      }),
    })
  },
}
