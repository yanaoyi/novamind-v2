import { create } from 'zustand'

import { ApiError } from '../api/client'
import { projectApi } from '../api/projects'
import type {
  CreateProjectInput,
  Project,
  ProjectListQuery,
  ProjectType,
  UpdateProjectInput,
} from '../api/types'

interface ProjectState {
  items: Project[]
  total: number
  page: number
  pageSize: number
  keyword: string
  type?: ProjectType
  loading: boolean
  error: string | null

  load: (override?: Partial<ProjectListQuery>) => Promise<void>
  create: (input: CreateProjectInput) => Promise<void>
  update: (id: string, input: UpdateProjectInput) => Promise<void>
  remove: (id: string) => Promise<void>
}

function messageOf(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return '未知错误'
}

export const useProjectStore = create<ProjectState>((set, get) => ({
  items: [],
  total: 0,
  page: 1,
  pageSize: 20,
  keyword: '',
  type: undefined,
  loading: false,
  error: null,

  async load(override) {
    const { page, pageSize, keyword, type } = get()
    const query: ProjectListQuery = {
      page: override?.page ?? page,
      page_size: override?.page_size ?? pageSize,
      keyword: override?.keyword ?? keyword,
      type: override?.type ?? type,
    }
    set({ loading: true, error: null })
    try {
      const data = await projectApi.list(query)
      set({
        items: data.items,
        total: data.total,
        page: data.page,
        pageSize: data.page_size,
        keyword: query.keyword ?? '',
        type: query.type,
        loading: false,
      })
    } catch (err) {
      set({ loading: false, error: messageOf(err) })
      throw err
    }
  },

  async create(input) {
    await projectApi.create(input)
    await get().load({ page: 1 })
  },

  async update(id, input) {
    await projectApi.update(id, input)
    await get().load()
  },

  async remove(id) {
    await projectApi.remove(id)
    await get().load()
  },
}))
