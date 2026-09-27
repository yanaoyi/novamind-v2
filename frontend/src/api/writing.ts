import { BASE, request } from './client'
import type {
  ChapterVersion,
  ConsistencyIssue,
  CreativeChapter,
  CreativeScene,
  CreativeVolume,
  IssueStatus,
  ListOf,
  RewriteAction,
  Task,
} from './types'

export interface ChapterInput {
  volume_id?: string | null
  chapter_no: number
  title: string
  summary?: string
  purpose?: string
  conflict?: string
  outcome?: string
  content?: string
}

export interface ChapterPatch {
  title?: string
  summary?: string
  content?: string
  status?: string
  volume_id?: string | null
  purpose?: string
  conflict?: string
  outcome?: string
}

/** 写作系统：大纲 / 章节 / 版本 / 场景 / AI 写作 / 一致性 / 导出 */
export const writingApi = {
  // --- 卷（大纲第一层） ---
  listVolumes(workId: string) {
    return request<ListOf<CreativeVolume>>(`/creative/${workId}/volumes`)
  },
  createVolume(workId: string, input: { title: string; summary?: string; sequence?: number }) {
    return request<CreativeVolume>(`/creative/${workId}/volumes`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  // --- 章节 ---
  listChapters(workId: string, full = false) {
    return request<ListOf<CreativeChapter>>(`/creative/${workId}/chapters?full=${full ? 'true' : 'false'}`)
  },
  createChapter(workId: string, input: ChapterInput) {
    return request<CreativeChapter>(`/creative/${workId}/chapters`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  getChapter(chapterId: string) {
    return request<CreativeChapter>(`/chapters/${chapterId}`)
  },
  updateChapter(chapterId: string, patch: ChapterPatch) {
    return request<{ chapter: CreativeChapter; version_created: boolean }>(`/chapters/${chapterId}`, {
      method: 'PUT',
      body: JSON.stringify(patch),
    })
  },
  removeChapter(chapterId: string) {
    return request<{ deleted: boolean }>(`/chapters/${chapterId}`, { method: 'DELETE' })
  },
  /** 让 AI 写本章（异步任务，返回 task 用于轮询） */
  generateChapter(chapterId: string, input?: { target_words?: number; instruction?: string }) {
    return request<Task>(`/chapters/${chapterId}/generate`, {
      method: 'POST',
      body: JSON.stringify(input ?? {}),
    })
  },

  // --- 版本 ---
  listVersions(chapterId: string) {
    return request<ListOf<ChapterVersion>>(`/chapters/${chapterId}/versions`)
  },
  getVersion(chapterId: string, no: number) {
    return request<ChapterVersion>(`/chapters/${chapterId}/versions/${no}`)
  },
  restoreVersion(chapterId: string, no: number) {
    return request<CreativeChapter>(`/chapters/${chapterId}/versions/${no}/restore`, { method: 'POST' })
  },

  // --- 场景 ---
  listScenes(chapterId: string) {
    return request<ListOf<CreativeScene>>(`/chapters/${chapterId}/scenes`)
  },
  createScene(chapterId: string, input: Partial<CreativeScene>) {
    return request<CreativeScene>(`/chapters/${chapterId}/scenes`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  // --- 编辑器内的 AI 操作（同步返回新文本） ---
  rewrite(chapterId: string, text: string, action: RewriteAction, instruction?: string) {
    return request<{ text: string }>('/ai/rewrite', {
      method: 'POST',
      body: JSON.stringify({ chapter_id: chapterId, text, action, instruction }),
    })
  },

  // --- 一致性 ---
  checkConsistency(workId: string, chapterIds?: string[]) {
    return request<Task>(`/creative/${workId}/consistency/check`, {
      method: 'POST',
      body: JSON.stringify({ chapter_ids: chapterIds ?? [] }),
    })
  },
  listIssues(workId: string, status?: IssueStatus | '') {
    const query = status ? `?status=${status}` : ''
    return request<ListOf<ConsistencyIssue>>(`/creative/${workId}/consistency/issues${query}`)
  },
  updateIssue(issueId: string, status: IssueStatus) {
    return request<{ updated: boolean }>(`/consistency-issues/${issueId}`, {
      method: 'PUT',
      body: JSON.stringify({ status }),
    })
  },
}

/** 导出地址（供 <a download> 或 fetch 使用） */
export function exportUrl(workId: string, format: 'txt' | 'md' | 'docx'): string {
  return `${BASE}/creative/${workId}/export?format=${format}`
}

/** 触发浏览器下载（导出走二进制流，不能用统一 JSON 解析） */
export async function downloadExport(workId: string, format: 'txt' | 'md' | 'docx'): Promise<void> {
  const res = await fetch(exportUrl(workId, format))
  if (!res.ok) {
    throw new Error(`导出失败（HTTP ${res.status}）`)
  }
  const blob = await res.blob()
  const disposition = res.headers.get('Content-Disposition') ?? ''
  const matched = /filename\*=UTF-8''([^;]+)/.exec(disposition)
  const filename = matched ? decodeURIComponent(matched[1]) : `novamind.${format}`
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}
