import { create } from 'zustand'
import { persist } from 'zustand/middleware'

import { ApiError } from '../api/client'
import { originalApi } from '../api/original'
import type {
  CreateOriginalInput,
  Original,
  OriginalChapterBrief,
  OriginalChapterDetail,
  OriginalImportResult,
} from '../api/types'

interface OriginalState {
  /** 当前选中的原著 ID（持久化，刷新后仍在同一部原著里） */
  workId: string | null
  work: Original | null
  loading: boolean
  error: string | null

  chapters: OriginalChapterBrief[]
  chaptersTotal: number
  chapter: OriginalChapterDetail | null

  setWorkId: (id: string | null) => void
  loadWork: (workId?: string) => Promise<Original | null>
  /** 按工程找原著；找不到返回 null（由调用方决定要不要创建） */
  selectByProject: (projectId: string) => Promise<Original | null>
  createForProject: (projectId: string, input: CreateOriginalInput) => Promise<Original>
  importFile: (file: File) => Promise<OriginalImportResult>
  loadChapters: (page?: number, pageSize?: number) => Promise<void>
  loadChapter: (no: number) => Promise<void>
  reset: () => void
}

function messageOf(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return '未知错误'
}

// 请求序号：章节/章节列表的响应可能乱序返回（快速翻页时慢响应后到），
// 只采纳"最后一次请求"的结果，避免 URL 与内容对不上（审查 P1-12 / P2）。
let chapterRequestId = 0
let chaptersRequestId = 0
let workRequestId = 0

export const useOriginalStore = create<OriginalState>()(
  persist(
    (set, get) => ({
      workId: null,
      work: null,
      loading: false,
      error: null,
      chapters: [],
      chaptersTotal: 0,
      chapter: null,

      setWorkId(id) {
        set({ workId: id, work: null, chapters: [], chapter: null, error: null })
      },

      async loadWork(workId) {
        const id = workId ?? get().workId
        if (!id) return null
        const requestId = ++workRequestId
        set({ loading: true, error: null })
        try {
          const work = await originalApi.get(id)
          if (requestId !== workRequestId) return work // 过期响应：不写入
          set({ work, workId: id, loading: false })
          return work
        } catch (err) {
          if (requestId !== workRequestId) throw err
          set({ loading: false, error: messageOf(err), work: null })
          throw err
        }
      },

      async selectByProject(projectId) {
        const work = await originalApi.getByProject(projectId)
        if (work) {
          set({ workId: work.id, work, chapters: [], chapter: null, error: null })
        } else {
          set({ workId: null, work: null, chapters: [], chapter: null, error: null })
        }
        return work
      },

      async createForProject(projectId, input) {
        const work = await originalApi.create(projectId, input)
        set({ workId: work.id, work, chapters: [], chapter: null, error: null })
        return work
      },

      async importFile(file) {
        const id = get().workId
        if (!id) throw new Error('尚未选择原著')
        const result = await originalApi.importFile(id, file)
        // 导入会改变章节数与字数，刷新一次详情
        const work = await originalApi.get(id)
        set({ work, chapters: [], chapter: null })
        return result
      },

      async loadChapters(page = 1, pageSize = 50) {
        const id = get().workId
        if (!id) return
        const requestId = ++chaptersRequestId
        set({ loading: true, error: null })
        try {
          const data = await originalApi.listChapters(id, page, pageSize)
          if (requestId !== chaptersRequestId) return
          set({ chapters: data.items, chaptersTotal: data.total, loading: false })
        } catch (err) {
          if (requestId !== chaptersRequestId) return
          set({ loading: false, error: messageOf(err) })
          throw err
        }
      },

      async loadChapter(no) {
        const id = get().workId
        if (!id) return
        const requestId = ++chapterRequestId
        set({ loading: true, error: null })
        try {
          const chapter = await originalApi.getChapter(id, no)
          if (requestId !== chapterRequestId) return // 已被更新的请求取代
          set({ chapter, loading: false })
        } catch (err) {
          if (requestId !== chapterRequestId) return
          set({ loading: false, error: messageOf(err), chapter: null })
          throw err
        }
      },

      reset() {
        set({ workId: null, work: null, chapters: [], chaptersTotal: 0, chapter: null, error: null })
      },
    }),
    {
      name: 'novamind.original',
      // 只持久化选中项，不持久化数据本身（避免脏数据）
      partialize: (state) => ({ workId: state.workId }),
    },
  ),
)
