import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest'

import ConsistencyPage from './creative/ConsistencyPage'
import WritingWorkspacePage from './creative/WritingWorkspacePage'
import { useOriginalStore } from '../stores/originalStore'

// ---------- 假后端：原著 → 二创作品 → 卷/章节/版本/一致性问题 ----------

const ORIGINAL_ID = 'orig-1'
const CREATIVE_ID = 'cw-1'
const ISO = '2026-09-27T12:00:00Z'

let volumes: Array<Record<string, unknown>> = []
let chapters: Array<Record<string, unknown>> = []
let versions: Array<Record<string, unknown>> = []
let issues: Array<Record<string, unknown>> = []
let calls: Array<{ method: string; path: string; query: string; body?: unknown }> = []

function envelope(data: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify({ data, error: null, trace_id: 't' }), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

function notFound(path: string) {
  return Promise.resolve(
    new Response(JSON.stringify({ data: null, error: { code: 'NOT_FOUND', message: path }, trace_id: 't' }), {
      status: 404,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

function fakeFetch(input: RequestInfo | URL, init?: RequestInit) {
  const url = typeof input === 'string' ? input : input.toString()
  const method = (init?.method ?? 'GET').toUpperCase()
  const [rawPath, query = ''] = url.split('?')
  const path = rawPath
  const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
  calls.push({ method, path, query, body })

  if (path.endsWith(`/original/${ORIGINAL_ID}/creative-works`)) {
    return envelope({
      items: [
        {
          id: CREATIVE_ID, project_id: 'p2', original_work_id: ORIGINAL_ID,
          title: '暗涌·另一条路', description: '', status: 'ACTIVE',
          divergence_point_id: null, created_at: ISO, updated_at: ISO,
        },
      ],
      total: 1,
    })
  }
  if (path.endsWith(`/creative/${CREATIVE_ID}/volumes`)) {
    return envelope({ items: volumes, total: volumes.length })
  }
  if (path.endsWith(`/creative/${CREATIVE_ID}/chapters`)) {
    return envelope({ items: chapters, total: chapters.length })
  }
  if (path.endsWith(`/creative/${CREATIVE_ID}/consistency/issues`)) {
    const status = new URLSearchParams(query).get('status')
    const items = status ? issues.filter((i) => i.status === status) : issues
    return envelope({ items, total: items.length })
  }
  if (/\/chapters\/[^/]+\/versions$/.test(path)) {
    return envelope({ items: versions, total: versions.length })
  }
  if (/\/chapters\/[^/]+$/.test(path) && method === 'GET') {
    const id = path.split('/').pop()
    const found = chapters.find((c) => c.id === id)
    return found ? envelope(found) : notFound(path)
  }
  if (path.endsWith('/consistency-issues/' + (issues[0]?.id ?? 'x')) && method === 'PUT') {
    return envelope({ updated: true })
  }
  return notFound(path)
}

function renderPage(node: React.ReactElement) {
  return render(
    <ConfigProvider locale={zhCN}>
      <MemoryRouter>{node}</MemoryRouter>
    </ConfigProvider>,
  )
}

beforeEach(() => {
  localStorage.clear()
  vi.stubGlobal('fetch', vi.fn(fakeFetch))
  calls = []
  useOriginalStore.setState({ workId: ORIGINAL_ID, work: null })

  volumes = [
    {
      id: 'vol-1', title: '第一卷 · 梅雨', summary: '老城区的雨季', sequence: 1,
      created_at: ISO, updated_at: ISO,
    },
  ]
  chapters = [
    {
      id: 'ch-1', volume_id: 'vol-1', chapter_no: 1, title: '旧信', summary: '林默发现母亲留下的信',
      content: '雨下了三天。林默在抽屉最里面摸到一封信。', status: 'DRAFT', word_count: 18,
      purpose: '引出母亲意外', conflict: '是否回江城', outcome: '决定留下',
      created_at: ISO, updated_at: ISO,
    },
    {
      id: 'ch-2', volume_id: null, chapter_no: 2, title: '对峙', summary: '',
      content: '', status: 'DRAFT', word_count: 0,
      purpose: '', conflict: '', outcome: '', created_at: ISO, updated_at: ISO,
    },
  ]
  versions = [
    { id: 'v-2', chapter_id: 'ch-1', version_no: 2, word_count: 34, note: '编辑保存', created_at: ISO },
    { id: 'v-1', chapter_id: 'ch-1', version_no: 1, word_count: 18, note: '创建章节', created_at: ISO },
  ]
  issues = [
    {
      id: 'issue-1', creative_work_id: CREATIVE_ID, chapter_id: 'ch-1', severity: 'high', type: 'character',
      description: '林默的说话方式与设定不符', evidence: '原文片段：……', suggestion: '改成更克制的短句',
      status: 'OPEN', created_at: ISO, updated_at: ISO,
    },
  ]
})

afterAll(() => {
  vi.unstubAllGlobals()
})

describe('写作工作台', () => {
  it('列出章节并加载正文到编辑器', async () => {
    renderPage(<WritingWorkspacePage />)

    // 左侧章节列表
    expect(await screen.findByText('1. 旧信')).toBeInTheDocument()
    expect(screen.getByText('2. 对峙')).toBeInTheDocument()
    // 右侧编辑器标题 + 正文
    expect(await screen.findByText('第 1 章 · 旧信')).toBeInTheDocument()
    expect(screen.getByDisplayValue('雨下了三天。林默在抽屉最里面摸到一封信。')).toBeInTheDocument()
    // 一致性问题计数（顶部标签）
    expect(await screen.findByText('一致性问题：1')).toBeInTheDocument()
  })

  it('切到大纲视图显示卷与三要素', async () => {
    renderPage(<WritingWorkspacePage defaultTab="outline" />)

    expect(await screen.findByText(/第一卷 · 梅雨/)).toBeInTheDocument()
    expect(screen.getByText('目的：引出母亲意外')).toBeInTheDocument()
    expect(screen.getByText('冲突：是否回江城')).toBeInTheDocument()
    expect(screen.getByText('结果：决定留下')).toBeInTheDocument()
  })

  it('版本面板列出历史版本', async () => {
    renderPage(<WritingWorkspacePage />)
    await screen.findByText('第 1 章 · 旧信')

    fireEvent.click(screen.getByRole('button', { name: /版\s*本/ }))

    const drawer = await screen.findByRole('dialog')
    await waitFor(() => {
      expect(drawer.textContent).toContain('v2')
    })
    expect(drawer.textContent).toContain('编辑保存')
  })

  it('导出走二进制接口而不是 JSON 接口', async () => {
    const createObjectURL = vi.fn(() => 'blob:fake')
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL, revokeObjectURL: vi.fn() }))
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    renderPage(<WritingWorkspacePage />)
    await screen.findByText('第 1 章 · 旧信')

    fireEvent.click(screen.getByRole('button', { name: /导\s*出/ }))
    fireEvent.click(await screen.findByText('导出 Markdown'))

    await waitFor(() => {
      expect(
        calls.some((c) => c.path.endsWith(`/creative/${CREATIVE_ID}/export`) && c.query.includes('format=md')),
      ).toBe(true)
    })
  })
})

describe('一致性问题页', () => {
  it('按状态筛选并标记解决', async () => {
    renderPage(<ConsistencyPage />)

    expect(await screen.findByText('林默的说话方式与设定不符')).toBeInTheDocument()
    expect(screen.getByText('严重')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /已\s*解\s*决/ }))

    await waitFor(() => {
      expect(calls.some((c) => c.method === 'PUT' && c.path.endsWith('/consistency-issues/issue-1'))).toBe(true)
    })
  })
})
