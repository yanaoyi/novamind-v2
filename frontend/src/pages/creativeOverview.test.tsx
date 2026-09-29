import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CreativeOverviewPage from './creative/CreativeOverviewPage'
import { useOriginalStore } from '../stores/originalStore'

const ISO = '2026-09-28T12:00:00Z'

let calls: Array<{ method: string; path: string; body?: unknown; hasFile?: boolean }> = []
let originalWorks: Record<string, Record<string, unknown>> = {}
let creativeWorks: Array<Record<string, unknown>> = []

function envelope(data: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify({ data, error: null, trace_id: 't' }), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

function fakeFetch(input: RequestInfo | URL, init?: RequestInit) {
  const url = typeof input === 'string' ? input : input.toString()
  const method = (init?.method ?? 'GET').toUpperCase()
  const path = url.split('?')[0]
  const hasFile = typeof FormData !== 'undefined' && init?.body instanceof FormData
  const body = !hasFile && typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
  calls.push({ method, path, body, hasFile })

  // 建工程
  if (path.endsWith('/projects') && method === 'POST') {
    const type = (body as { type: string }).type
    const id = type === 'ORIGINAL' ? 'proj-orig' : 'proj-creative'
    return envelope({
      id, name: (body as { name: string }).name, type, status: 'ACTIVE',
      description: '', created_at: ISO, updated_at: ISO,
    })
  }
  // 建原著
  if (/\/projects\/[\w-]+\/original$/.test(path) && method === 'POST') {
    const id = 'orig-1'
    originalWorks[id] = {
      id, project_id: 'proj-orig', title: (body as { title: string }).title, author: '',
      description: '', source_type: 'TXT', status: 'PARSED', char_count: 0, chapter_count: 0,
      created_at: ISO, updated_at: ISO,
    }
    return envelope(originalWorks[id], 201)
  }
  // 导入原文（multipart）
  if (/\/original\/[\w-]+\/import$/.test(path) && method === 'POST') {
    return envelope({ encoding: 'UTF-8', chapter_count: 3, char_count: 1234, chapters: [] })
  }
  // 从原著创建同人作品
  if (/\/original\/[\w-]+\/create-creative$/.test(path) && method === 'POST') {
    const created = {
      id: 'cw-1', project_id: (body as { project_id: string }).project_id, original_work_id: 'orig-1',
      title: (body as { title: string }).title, description: '', status: 'DRAFT',
      divergence_point_id: null, created_at: ISO, updated_at: ISO,
    }
    creativeWorks = [created]
    return envelope(created, 201)
  }
  // 原著人物（用于批量继承）
  if (path.endsWith('/original/orig-1/characters')) {
    return envelope({
      items: [
        { id: 'ch-a', original_work_id: 'orig-1', name: '林默', role: '主角', importance: 5, dna: {} },
        { id: 'ch-b', original_work_id: 'orig-1', name: '陈述', role: '配角', importance: 3, dna: {} },
      ],
      total: 2,
    })
  }
  // 二创作品下的继承
  if (/\/creative\/[\w-]+\/characters\/inherit$/.test(path) && method === 'POST') {
    return envelope({ id: `cc-${(body as { source_character_id: string }).source_character_id}`, name: 'x' }, 201)
  }
  if (/\/creative\/[\w-]+\/world\/inherit$/.test(path) && method === 'POST') {
    return envelope({ id: 'world-1', name: '江城' })
  }
  // 列表与详情
  if (path.endsWith('/original/orig-1/creative-works')) {
    return envelope({ items: creativeWorks, total: creativeWorks.length })
  }
  if (/\/original\/[\w-]+$/.test(path) && method === 'GET') {
    const id = path.split('/').pop() as string
    return envelope(originalWorks[id] ?? null)
  }
  if (/\/creative\/[\w-]+$/.test(path) && method === 'GET') {
    return envelope({ id: 'cw-1', title: '暗涌·同人', descripction: '', status: 'DRAFT' })
  }
  return envelope({ items: [], total: 0 })
}

function renderPage() {
  return render(
    <ConfigProvider locale={zhCN}>
      <MemoryRouter>
        <CreativeOverviewPage />
      </MemoryRouter>
    </ConfigProvider>,
  )
}

beforeEach(() => {
  localStorage.clear()
  document.body.innerHTML = ''
  calls = []
  originalWorks = {}
  creativeWorks = []
  useOriginalStore.setState({ workId: null, work: null })
  vi.stubGlobal('fetch', vi.fn(fakeFetch))
})

describe('二创 · 总览（同人坊首页）', () => {
  it('没有原著时给出导入引导', async () => {
    renderPage()
    expect(await screen.findByText('还没有选中的原著')).toBeInTheDocument()
    expect(screen.getByText('导入并创建同人作品')).toBeInTheDocument()
    expect(screen.getByText('点击或把原著文件拖到这里')).toBeInTheDocument()
  })

  it('上传一本书后按顺序完成：建原著工程 → 导原文 → 建同人作品 → 继承人物与世界观', async () => {
    renderPage()
    await screen.findByText('还没有选中的原著')

    // 选文件（用文件名自动填书名）
    const file = new File(['第一章 初遇\n\n海面很平。'], '暗涌.pdf', { type: 'application/pdf' })
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    fireEvent.change(input, { target: { files: [file] } })
    await waitFor(() => {
      expect((screen.getByLabelText('书名') as HTMLInputElement).value).toBe('暗涌')
    })

    fireEvent.click(screen.getByRole('button', { name: /导入并创建同人作品/ }))

    await waitFor(() => {
      expect(calls.some((c) => c.method === 'POST' && c.path.endsWith('/original/orig-1/import') && c.hasFile)).toBe(true)
    })
    const order = calls.map((c) => `${c.method} ${c.path}`)
    const idxProjects = order.findIndex((o) => o === 'POST /api/v1/projects')
    const idxOriginal = order.findIndex((o) => o.endsWith('/projects/proj-orig/original'))
    const idxImport = order.findIndex((o) => o.endsWith('/original/orig-1/import'))
    const idxCreative = order.findIndex((o) => o.endsWith('/original/orig-1/create-creative'))
    expect(idxProjects).toBeGreaterThanOrEqual(0)
    expect(idxOriginal).toBeGreaterThan(idxProjects)
    expect(idxImport).toBeGreaterThan(idxOriginal)
    expect(idxCreative).toBeGreaterThan(idxImport)

    // 两个人物都被继承，世界观按 FULL 继承
    await waitFor(() => {
      const inherits = calls.filter((c) => c.path.endsWith('/characters/inherit'))
      expect(inherits).toHaveLength(2)
      expect(inherits.map((c) => (c.body as { source_character_id: string }).source_character_id).sort()).toEqual([
        'ch-a',
        'ch-b',
      ])
    })
    expect(calls.some((c) => c.path.endsWith('/world/inherit'))).toBe(true)
    // 继承权重是"全维度 100%"
    const firstInherit = calls.find((c) => c.path.endsWith('/characters/inherit'))
    expect(Object.values((firstInherit?.body as { weights: Record<string, number> }).weights).every((v) => v === 100)).toBe(
      true,
    )
  })

  it('已有原著时可以直接新建同人作品（不再要求重新导入书）', async () => {
    useOriginalStore.setState({
      workId: 'orig-1',
      work: {
        id: 'orig-1', project_id: 'proj-orig', title: '暗涌', author: '', description: '',
        source_type: 'TXT', status: 'PARSED', char_count: 0, chapter_count: 0,
        created_at: ISO, updated_at: ISO,
      },
    })
    originalWorks['orig-1'] = {
      id: 'orig-1', project_id: 'proj-orig', title: '暗涌', author: '', description: '',
      source_type: 'TXT', status: 'PARSED', char_count: 0, chapter_count: 0,
      created_at: ISO, updated_at: ISO,
    }
    renderPage()

    expect(await screen.findByText(/当前原著的同人作品/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /从当前原著新建/ }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(dialog.querySelector('input') as HTMLInputElement, {
      target: { value: '暗涌·另一条路' },
    })
    fireEvent.click(dialog.querySelector('.ant-btn-primary') as HTMLButtonElement)

    await waitFor(() => {
      expect(calls.some((c) => c.path.endsWith('/original/orig-1/create-creative'))).toBe(true)
    })
    expect(calls.filter((c) => c.method === 'POST' && c.path.endsWith('/projects'))).toHaveLength(1)
  })
})
