import type { Envelope } from './types'
import { getToken, notifyTokenRequired } from './token'

/** 开发环境走 vite 代理（/api → 后端:8080），生产可用 VITE_API_BASE 覆盖 */
export const BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api/v1'

/** 后端统一错误体对应的前端异常 */
export class ApiError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly status: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

/**
 * 统一请求入口：解析后端统一响应包 {data, error, trace_id}。
 * 出错时抛出 ApiError（带后端错误码），调用方按 code 处理。
 */
export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  // FormData 必须由浏览器自己设置 Content-Type（含 multipart boundary），
  // 手动写死 application/json 会让后端解析不出文件。
  const isFormData = typeof FormData !== 'undefined' && init?.body instanceof FormData

  const token = getToken()
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      ...(isFormData ? {} : { 'Content-Type': 'application/json' }),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init?.headers ?? {}),
    },
  })

  let body: Envelope<T> | null = null
  try {
    body = (await res.json()) as Envelope<T>
  } catch {
    body = null
  }

  if (!res.ok) {
    const code = body?.error?.code ?? 'HTTP_ERROR'
    if (res.status === 401 || code === 'UNAUTHORIZED') {
      // 让界面弹出"填访问令牌"的入口，而不是只留一句报错
      notifyTokenRequired()
      throw new ApiError('UNAUTHORIZED', body?.error?.message ?? '需要访问令牌', res.status)
    }
    throw new ApiError(
      code,
      body?.error?.message ?? `请求失败（HTTP ${res.status}）`,
      res.status,
    )
  }
  return body?.data as T
}
