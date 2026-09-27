import { ApiError, request } from './client'
import type {
  CreateOriginalInput,
  Original,
  OriginalChapterDetail,
  OriginalChapterList,
  OriginalImportResult,
} from './types'

/** 原著接口（对应后端 /api/v1/original 与 /projects/{id}/original） */
export const originalApi = {
  /** 按工程取原著；该工程还没有原著时返回 null（而不是抛错） */
  async getByProject(projectId: string): Promise<Original | null> {
    try {
      return await request<Original>(`/projects/${projectId}/original`)
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) return null
      throw err
    }
  },

  create(projectId: string, input: CreateOriginalInput): Promise<Original> {
    return request<Original>(`/projects/${projectId}/original`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  get(id: string): Promise<Original> {
    return request<Original>(`/original/${id}`)
  },

  /** 上传原文并导入（重复导入会整体替换章节） */
  importFile(id: string, file: File): Promise<OriginalImportResult> {
    const form = new FormData()
    form.append('file', file)
    return request<OriginalImportResult>(`/original/${id}/import`, {
      method: 'POST',
      body: form,
    })
  },

  listChapters(id: string, page = 1, pageSize = 50): Promise<OriginalChapterList> {
    return request<OriginalChapterList>(
      `/original/${id}/chapters?page=${page}&page_size=${pageSize}`,
    )
  },

  getChapter(id: string, no: number): Promise<OriginalChapterDetail> {
    return request<OriginalChapterDetail>(`/original/${id}/chapters/${no}`)
  },
}
