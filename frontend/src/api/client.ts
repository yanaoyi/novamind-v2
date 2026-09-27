import type { Envelope } from './types'

/** 开发环境走 vite 代理（/api → 后端:8080），生产可用 VITE_API_BASE 覆盖 */
const BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api/v1'

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
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
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
    throw new ApiError(
      body?.error?.code ?? 'HTTP_ERROR',
      body?.error?.message ?? `请求失败（HTTP ${res.status}）`,
      res.status,
    )
  }
  return body?.data as T
}
