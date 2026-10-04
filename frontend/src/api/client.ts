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
 * 统一的错误消息提取（审查 P2）：散落的 `(err as Error).message` 在抛出字符串时
 * 会显示空白，各页面统一用它取提示文案。
 */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  if (typeof err === 'string' && err) return err
  return '未知错误'
}

/**
 * 路径参数编码（审查 P2：以前所有 api 文件都直接拼 `${id}`，没有编码）。
 *
 * 放在请求层统一处理，而不是改上百处拼接：
 *   - 只对**路径段**编码，查询串原样保留；
 *   - 先 decode 再 encode，已经是编码形态的段不会被二次编码成 %2520；
 *   - UUID / 数字这类本来就安全的段结果不变。
 */
function encodePath(path: string): string {
  const [rawPath, query] = path.split(/\?(.*)/s)
  const encoded = rawPath
    .split('/')
    .map((segment) => {
      if (!segment) return segment
      try {
        return encodeURIComponent(decodeURIComponent(segment))
      } catch {
        return encodeURIComponent(segment)
      }
    })
    .join('/')
  return query === undefined ? encoded : `${encoded}?${query}`
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
  const res = await fetch(`${BASE}${encodePath(path)}`, {
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
  // 2xx 但没有响应体：返回 null as T 会让调用方在下一次解引用时崩掉（审查 P1-13），
  // 这里明确报错，让问题停在请求层而不是散落到各个页面。
  if (body == null) {
    throw new ApiError('EMPTY_RESPONSE', `接口返回了空响应体（HTTP ${res.status}）`, res.status)
  }
  return body?.data as T
}
