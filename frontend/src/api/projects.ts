import { request } from './client'
import type {
  CreateProjectInput,
  Project,
  ProjectList,
  ProjectListQuery,
  UpdateProjectInput,
} from './types'

function toQueryString(query: ProjectListQuery): string {
  const params = new URLSearchParams()
  if (query.page) params.set('page', String(query.page))
  if (query.page_size) params.set('page_size', String(query.page_size))
  if (query.type) params.set('type', query.type)
  if (query.status) params.set('status', query.status)
  if (query.keyword) params.set('keyword', query.keyword)
  return params.toString()
}

/** 工程管理接口（对应后端 /api/v1/projects） */
export const projectApi = {
  list(query: ProjectListQuery = {}): Promise<ProjectList> {
    const qs = toQueryString(query)
    return request<ProjectList>(`/projects${qs ? `?${qs}` : ''}`)
  },

  get(id: string): Promise<Project> {
    return request<Project>(`/projects/${id}`)
  },

  create(input: CreateProjectInput): Promise<Project> {
    return request<Project>('/projects', {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  update(id: string, input: UpdateProjectInput): Promise<Project> {
    return request<Project>(`/projects/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    })
  },

  remove(id: string): Promise<{ deleted: boolean }> {
    return request<{ deleted: boolean }>(`/projects/${id}`, { method: 'DELETE' })
  },
}
