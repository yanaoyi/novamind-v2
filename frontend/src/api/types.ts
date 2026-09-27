// 与后端 OpenAPI（backend/internal/api/openapi.yaml）保持一致。
// 后端契约变更时，同步改这里；后端有防漂移测试，前端类型以此文件为准。

/** 统一响应包：所有接口都返回这个结构 */
export interface Envelope<T> {
  data: T | null
  error: ApiErrorBody | null
  trace_id: string
}

export interface ApiErrorBody {
  code: string
  message: string
  details?: Record<string, unknown> | null
}

/** ORIGINAL=原著工程，CREATIVE=二创工程；创建后不可变更 */
export type ProjectType = 'ORIGINAL' | 'CREATIVE'

export type ProjectStatus = 'DRAFT' | 'ACTIVE' | 'ARCHIVED'

export interface Project {
  id: string
  name: string
  description: string
  type: ProjectType
  status: ProjectStatus
  created_at: string
  updated_at: string
}

export interface ProjectList {
  items: Project[]
  total: number
  page: number
  page_size: number
}

export interface ProjectListQuery {
  page?: number
  page_size?: number
  type?: ProjectType
  status?: ProjectStatus
  keyword?: string
}

export interface CreateProjectInput {
  name: string
  description?: string
  type: ProjectType
}

/** 缺省字段表示不修改；type 不可更新 */
export interface UpdateProjectInput {
  name?: string
  description?: string
  status?: ProjectStatus
}
