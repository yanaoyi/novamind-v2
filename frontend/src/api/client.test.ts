import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, errorMessage, request } from './client'
import { TOKEN_REQUIRED_EVENT, setToken } from './token'

function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }),
  )
}

function envelope(data: unknown) {
  return { data, error: null, trace_id: 't' }
}

beforeEach(() => {
  localStorage.clear()
})

describe('api client 的访问令牌处理', () => {
  it('保存过令牌时自动带上 Authorization 头', async () => {
    setToken('tok-123')
    const fetchMock = vi.fn(() => jsonResponse(envelope({ ok: true })))
    vi.stubGlobal('fetch', fetchMock)

    await request('/projects')

    const [, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer tok-123')
  })

  it('没有令牌时不带 Authorization 头（开发环境后端允许匿名）', async () => {
    const fetchMock = vi.fn(() => jsonResponse(envelope({ ok: true })))
    vi.stubGlobal('fetch', fetchMock)

    await request('/projects')

    const [, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect((init.headers as Record<string, string>).Authorization).toBeUndefined()
  })

  it('401 时抛出 UNAUTHORIZED 并广播"需要令牌"事件（界面据此弹窗）', async () => {
    const fetchMock = vi.fn(() =>
      jsonResponse({ data: null, error: { code: 'UNAUTHORIZED', message: '需要访问令牌' }, trace_id: 't' }, 401),
    )
    vi.stubGlobal('fetch', fetchMock)

    let notified = false
    const listener = () => {
      notified = true
    }
    window.addEventListener(TOKEN_REQUIRED_EVENT, listener)

    await expect(request('/projects')).rejects.toBeInstanceOf(ApiError)
    await expect(request('/projects')).rejects.toMatchObject({ code: 'UNAUTHORIZED', status: 401 })
    expect(notified).toBe(true)

    window.removeEventListener(TOKEN_REQUIRED_EVENT, listener)
  })
})

describe('路径参数编码与错误消息提取', () => {
  it('路径段做了编码，查询串保持原样', async () => {
    const fetchMock = vi.fn(() => jsonResponse(envelope({ ok: true })))
    vi.stubGlobal('fetch', fetchMock)

    await request('/original/张三 的书/chapters?page=1&kw=a b')

    const [url] = fetchMock.mock.calls[0] as unknown as [string]
    expect(url).toContain('/original/%E5%BC%A0%E4%B8%89%20%E7%9A%84%E4%B9%A6/chapters')
    expect(url.endsWith('?page=1&kw=a b')).toBe(true) // 查询串不动，交给调用方自己 encode
  })

  it('已经是编码形态的路径段不会被二次编码', async () => {
    const fetchMock = vi.fn(() => jsonResponse(envelope({ ok: true })))
    vi.stubGlobal('fetch', fetchMock)

    await request('/original/%E5%BC%A0%E4%B8%89/chapters')

    const [url] = fetchMock.mock.calls[0] as unknown as [string]
    expect(url).toContain('/original/%E5%BC%A0%E4%B8%89/chapters')
    expect(url).not.toContain('%25')
  })

  it('errorMessage 能处理 ApiError / Error / 字符串 / 未知值', () => {
    expect(errorMessage(new ApiError('BAD_REQUEST', '参数不对', 400))).toBe('参数不对')
    expect(errorMessage(new Error('炸了'))).toBe('炸了')
    expect(errorMessage('纯字符串错误')).toBe('纯字符串错误')
    expect(errorMessage({ weird: true })).toBe('未知错误')
    expect(errorMessage(undefined)).toBe('未知错误')
  })

  // v3 复审指出这条分支缺测试：2xx 但响应体为空时必须显式报错，
  // 不能把 null 当数据返回（调用方解引用时会崩，且崩在离现场很远的地方）。
  it('2xx 但空响应体时抛 EMPTY_RESPONSE', async () => {
    const fetchMock = vi.fn(() => Promise.resolve(new Response('', { status: 200 })))
    vi.stubGlobal('fetch', fetchMock)

    await expect(request('/projects')).rejects.toMatchObject({
      code: 'EMPTY_RESPONSE',
      status: 200,
    })
  })
})
