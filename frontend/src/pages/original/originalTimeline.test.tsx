import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useOriginalStore } from '../../stores/originalStore'
import OriginalPlotPage from './OriginalPlotPage'
import OriginalTimelinePage from './OriginalTimelinePage'

// ---------- 假后端 ----------

interface FakeEvent {
  id: string
  original_work_id: string
  title: string
  description: string
  chapter_no: number | null
  time_order: number
  participants: string[]
  location_id: string | null
  location_text: string
  consequences: string
  importance: number
  source: 'MANUAL'
  created_at: string
  updated_at: string
}

let events: FakeEvent[] = []
let timelineItems: Array<{ event_id: string; time_label?: string; duration?: string }> = []
let plotArcs: Array<Record<string, unknown>> = []
let calls: Array<{ method: string; path: string; body?: unknown }> = []

const ISO = '2026-09-27T11:00:00Z'
const WORK_ID = 'work-1'

function envelope(data: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify({ data, error: null, trace_id: 't' }), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

function makeEvent(id: string, title: string): FakeEvent {
  return {
    id,
    original_work_id: WORK_ID,
    title,
    description: '',
    chapter_no: null,
    time_order: 0,
    participants: [],
    location_id: null,
    location_text: '',
    consequences: '',
    importance: 3,
    source: 'MANUAL',
    created_at: ISO,
    updated_at: ISO,
  }
}

function timelineResponse() {
  return {
    id: 'timeline-1',
    name: '主线时间线',
    description: '',
    entries: timelineItems.map((item, index) => ({
      sequence: index + 1,
      time_label: item.time_label ?? '',
      duration: item.duration ?? '',
      event: events.find((e) => e.id === item.event_id),
    })),
  }
}

function fakeFetch(input: RequestInfo | URL, init?: RequestInit) {
  const url = typeof input === 'string' ? input : input.toString()
  const method = (init?.method ?? 'GET').toUpperCase()
  const path = url.split('?')[0]
  const body = init?.body && typeof init.body === 'string' ? JSON.parse(init.body) : undefined
  calls.push({ method, path, body })

  if (path.endsWith(`/original/${WORK_ID}`) && method === 'GET') {
    return envelope({
      id: WORK_ID,
      project_id: 'p1',
      title: '暗涌',
      author: '',
      description: '',
      source_type: 'TXT',
      status: 'PARSED',
      char_count: 100,
      chapter_count: 2,
      created_at: ISO,
      updated_at: ISO,
    })
  }
  if (path.endsWith(`/original/${WORK_ID}/characters`)) {
    return envelope({ items: [{ id: 'c1', name: '林默' }], total: 1, page: 1, page_size: 200 })
  }
  if (path.endsWith(`/original/${WORK_ID}/locations`)) {
    return envelope({ items: [], total: 0 })
  }
  if (path.endsWith(`/original/${WORK_ID}/events`)) {
    if (method === 'GET') {
      return envelope({ items: events, total: events.length, page: 1, page_size: 500 })
    }
    if (method === 'POST') {
      const created = makeEvent('event-new', String((body as { title: string }).title))
      events = [...events, created]
      return envelope(created, 201)
    }
  }
  if (path.endsWith(`/original/${WORK_ID}/timeline`)) {
    if (method === 'GET') return envelope(timelineResponse())
    if (method === 'PUT') {
      timelineItems = (body as { items: typeof timelineItems }).items
      return envelope(timelineResponse())
    }
  }
  if (path.endsWith(`/original/${WORK_ID}/plot-arcs`)) {
    if (method === 'GET') return envelope({ items: plotArcs, total: plotArcs.length })
    if (method === 'POST') {
      const created = { id: 'arc-1', original_work_id: WORK_ID, ...(body as object), created_at: ISO, updated_at: ISO }
      plotArcs = [...plotArcs, created]
      return envelope(created, 201)
    }
  }

  return Promise.resolve(
    new Response(JSON.stringify({ data: null, error: { code: 'NOT_FOUND', message: path }, trace_id: 't' }), {
      status: 404,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
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
  events = [makeEvent('e1', '母亲的意外'), makeEvent('e2', '拆迁通知贴出')]
  timelineItems = []
  plotArcs = []
  calls = []
  useOriginalStore.setState({ workId: WORK_ID, work: null, chapters: [], chapter: null, error: null })
  vi.stubGlobal('fetch', vi.fn(fakeFetch))
})

describe('原著时间线页', () => {
  it('事件入列、顺序调整、时间标签一起保存', async () => {
    renderPage(<OriginalTimelinePage />)

    // 事件库加载
    expect(await screen.findByText('母亲的意外')).toBeInTheDocument()
    expect(screen.getByText('拆迁通知贴出')).toBeInTheDocument()
    expect(screen.getByText(/时间线（0 个事件）/)).toBeInTheDocument()

    // 先把「母亲的意外」加入时间线
    const firstRow = screen.getByText('母亲的意外').closest('tr') as HTMLElement
    fireEvent.click(within(firstRow).getByRole('button', { name: /加入时间线/ }))

    expect(await screen.findByText(/时间线（1 个事件）/)).toBeInTheDocument()

    // 再把「拆迁通知贴出」加入，并把它上移到最前
    const secondRow = screen.getByText('拆迁通知贴出').closest('tr') as HTMLElement
    fireEvent.click(within(secondRow).getByRole('button', { name: /加入时间线/ }))
    expect(await screen.findByText(/时间线（2 个事件）/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '上移：拆迁通知贴出' }))

    // 填时间标签
    const labelInputs = screen.getAllByPlaceholderText('时间标签（如：三年前·梅雨季）')
    fireEvent.change(labelInputs[0], { target: { value: '三年前·梅雨季' } })

    // 保存
    fireEvent.click(screen.getByRole('button', { name: /保存时间线/ }))

    await waitFor(() => {
      const put = calls.find((c) => c.method === 'PUT' && c.path.endsWith('/timeline'))
      expect(put).toBeDefined()
      const items = (put?.body as { items: Array<{ event_id: string; time_label?: string }> }).items
      expect(items).toHaveLength(2)
      // 上移成功后，「拆迁通知贴出」应排在第一位
      expect(items[0].event_id).toBe('e2')
      expect(items[0].time_label).toBe('三年前·梅雨季')
      expect(items[1].event_id).toBe('e1')
    })
  })

  it('新增事件后出现在事件库', async () => {
    renderPage(<OriginalTimelinePage />)
    await screen.findByText('母亲的意外')

    fireEvent.click(screen.getByRole('button', { name: /新增事件/ }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText('事件标题'), { target: { value: '旧信被发现' } })
    fireEvent.click(within(dialog).getByRole('button', { name: /保\s*存/ }))

    await waitFor(() => {
      const post = calls.find((c) => c.method === 'POST' && c.path.endsWith('/events'))
      expect(post).toBeDefined()
      expect(post?.body).toMatchObject({ title: '旧信被发现' })
    })
    expect(await screen.findByText('旧信被发现')).toBeInTheDocument()
  })
})

describe('原著剧情页', () => {
  it('新增剧情弧并展示在列表里', async () => {
    renderPage(<OriginalPlotPage />)

    expect(await screen.findByText(/剧情弧（0）/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /新增剧情弧/ }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText('标题'), { target: { value: '老城改造之争' } })
    fireEvent.click(within(dialog).getByRole('button', { name: /保\s*存/ }))

    await waitFor(() => {
      const post = calls.find((c) => c.method === 'POST' && c.path.endsWith('/plot-arcs'))
      expect(post).toBeDefined()
      expect(post?.body).toMatchObject({ title: '老城改造之争', type: 'main' })
    })
    expect(await screen.findByText('老城改造之争')).toBeInTheDocument()
  })
})
