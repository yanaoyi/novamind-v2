import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, request } from './client'
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
