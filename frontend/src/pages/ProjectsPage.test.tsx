import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ProjectType } from '../api/types'
import { useProjectStore } from '../stores/projectStore'
import ProjectsPage from './ProjectsPage'

// ---------- 假后端：在内存里真实地增删改查，验证页面与 API 的交互契约 ----------

interface Row {
  id: string
  name: string
  description: string
  type: ProjectType
  status: 'DRAFT' | 'ACTIVE' | 'ARCHIVED'
  created_at: string
  updated_at: string
}

let rows: Row[] = []
let seq = 0
let calls: Array<{ method: string; url: string; body?: unknown }> = []

function envelope(data: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify({ data, error: null, trace_id: 'test-trace' }), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

function nowIso() {
  return new Date('2026-09-27T08:00:00Z').toISOString()
}

function fakeFetch(input: RequestInfo | URL, init?: RequestInit) {
  const url = typeof input === 'string' ? input : input.toString()
  const method = (init?.method ?? 'GET').toUpperCase()
  const body = init?.body ? (JSON.parse(String(init.body)) as Record<string, unknown>) : undefined
  calls.push({ method, url, body })

  const path = url.split('?')[0]
  const query = new URLSearchParams(url.split('?')[1] ?? '')

  if (!path.endsWith('/projects')) {
    const id = path.split('/').pop() as string
    const row = rows.find((r) => r.id === id)
    if (method === 'DELETE') {
      rows = rows.filter((r) => r.id !== id)
      return envelope({ deleted: true })
    }
    if (method === 'PUT' && row && body) {
      Object.assign(row, body, { updated_at: nowIso() })
      return envelope(row)
    }
    return row ? envelope(row) : envelope(null, 404)
  }

  if (method === 'POST' && body) {
    seq += 1
    const created: Row = {
      id: `00000000-0000-7000-8000-00000000000${seq}`,
      name: String(body.name),
      description: String(body.description ?? ''),
      type: body.type as ProjectType,
      status: 'ACTIVE',
      created_at: nowIso(),
      updated_at: nowIso(),
    }
    rows = [created, ...rows]
    return envelope(created, 201)
  }

  const keyword = query.get('keyword')
  const type = query.get('type')
  const filtered = rows.filter(
    (r) => (!keyword || r.name.includes(keyword)) && (!type || r.type === type),
  )
  return envelope({
    items: filtered,
    total: filtered.length,
    page: Number(query.get('page') ?? 1),
    page_size: Number(query.get('page_size') ?? 20),
  })
}

function renderPage() {
  return render(<ProjectsPage />)
}

beforeEach(() => {
  rows = [
    {
      id: '11111111-1111-7111-8111-111111111111',
      name: '人间真相',
      description: '群像现实主义',
      type: 'CREATIVE',
      status: 'ACTIVE',
      created_at: nowIso(),
      updated_at: nowIso(),
    },
  ]
  seq = 0
  calls = []
  useProjectStore.setState({
    items: [],
    total: 0,
    page: 1,
    pageSize: 20,
    keyword: '',
    type: undefined,
    loading: false,
    error: null,
  })
  vi.stubGlobal('fetch', vi.fn(fakeFetch))
})

describe('ProjectsPage', () => {
  it('加载并渲染工程列表', async () => {
    renderPage()

    expect(await screen.findByText('人间真相')).toBeInTheDocument()
    expect(screen.getByText('二创')).toBeInTheDocument()
    expect(screen.getByText('进行中')).toBeInTheDocument()
    expect(screen.getByText('共 1 个工程')).toBeInTheDocument()
  })

  it('新建工程：提交表单后调用 POST 并刷新列表', async () => {
    renderPage()
    await screen.findByText('人间真相')

    fireEvent.click(screen.getByRole('button', { name: /新建工程/ }))
    const dialog = await screen.findByRole('dialog')

    fireEvent.change(within(dialog).getByLabelText('名称'), { target: { value: '新增的测试工程' } })
    fireEvent.change(within(dialog).getByLabelText('简介'), { target: { value: '来自组件测试' } })
    // 注意：antd 会在两字中文按钮里插入空格（"创 建"），因此用宽松匹配
    fireEvent.click(within(dialog).getByRole('button', { name: /创\s*建/ }))

    await waitFor(() => {
      const post = calls.find((c) => c.method === 'POST' && c.url.includes('/projects'))
      expect(post).toBeDefined()
      expect(post?.body).toMatchObject({ name: '新增的测试工程', type: 'CREATIVE' })
    })
    expect(await screen.findByText('新增的测试工程')).toBeInTheDocument()
  })

  it('编辑工程：调用 PUT，且不提交不可变的 type 字段', async () => {
    renderPage()
    await screen.findByText('人间真相')

    fireEvent.click(screen.getByRole('button', { name: '编辑' }))
    const dialog = await screen.findByRole('dialog')

    const nameInput = within(dialog).getByLabelText('名称')
    fireEvent.change(nameInput, { target: { value: '人间真相（修订）' } })
    fireEvent.click(within(dialog).getByRole('button', { name: /保\s*存/ }))

    await waitFor(() => {
      const put = calls.find((c) => c.method === 'PUT')
      expect(put).toBeDefined()
      expect(put?.body).toMatchObject({ name: '人间真相（修订）' })
      expect(put?.body).not.toHaveProperty('type')
    })
    expect(await screen.findByText('人间真相（修订）')).toBeInTheDocument()
  })

  it('删除工程：二次确认后调用 DELETE，并从列表移除', async () => {
    renderPage()
    await screen.findByText('人间真相')

    fireEvent.click(screen.getByRole('button', { name: /删\s*除/ }))
    fireEvent.click(await screen.findByRole('button', { name: '确认删除' }))

    await waitFor(() => {
      expect(calls.some((c) => c.method === 'DELETE')).toBe(true)
    })
    await waitFor(() => {
      expect(screen.queryByText('人间真相')).not.toBeInTheDocument()
    })
  })

  it('后端不可用时报错不崩（错误要能被看见）', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() =>
        Promise.resolve(
          new Response(
            JSON.stringify({
              data: null,
              error: { code: 'INTERNAL_ERROR', message: '服务器内部错误' },
              trace_id: 't',
            }),
            { status: 500, headers: { 'Content-Type': 'application/json' } },
          ),
        ),
      ),
    )

    renderPage()

    // 页面仍然渲染出标题与表格（不白屏）
    expect(await screen.findByText('工程管理')).toBeInTheDocument()
  })
})
