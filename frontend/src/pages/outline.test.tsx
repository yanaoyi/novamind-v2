import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import OutlinePage from './creative/OutlinePage'
import { aiOutlineToNodeInputs, countOutlineInputs } from './creative/outlineAi'
import { useOriginalStore } from '../stores/originalStore'

// ---------- 假后端：原著 → 二创作品 → 大纲树 → 落成章节 / AI 候选 ----------

const ORIGINAL_ID = 'orig-1'
const CREATIVE_ID = 'cw-1'
const OUTLINE_ID = 'ol-1'
const ISO = '2026-10-04T12:00:00Z'

let calls: Array<{ method: string; path: string; body?: unknown }> = []

function envelope(data: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify({ data, error: null, trace_id: 't' }), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

const outlineBody = {
  id: OUTLINE_ID,
  creative_work_id: CREATIVE_ID,
  title: '二创大纲 v1',
  summary: '卷节章三层',
  version: 1,
  source: 'MANUAL',
  node_count: 4,
  created_at: ISO,
  updated_at: ISO,
}

const outlineTree = {
  outline: outlineBody,
  nodes: [
    {
      id: 'v1',
      outline_id: OUTLINE_ID,
      parent_id: null,
      level: 1,
      level_name: '卷',
      sequence: 1,
      title: '第一卷 · 梅雨',
      summary: '老城区的雨季',
      purpose: '',
      characters: [],
      location: '',
      conflict: '',
      outcome: '',
      created_at: ISO,
      updated_at: ISO,
      children: [
        {
          id: 's1',
          outline_id: OUTLINE_ID,
          parent_id: 'v1',
          level: 2,
          level_name: '节',
          sequence: 1,
          title: '第一节 · 归乡',
          summary: '',
          purpose: '',
          characters: [],
          location: '',
          conflict: '',
          outcome: '',
          created_at: ISO,
          updated_at: ISO,
          children: [
            {
              id: 'c1',
              outline_id: OUTLINE_ID,
              parent_id: 's1',
              level: 3,
              level_name: '章',
              sequence: 1,
              title: '旧信',
              summary: '林默发现母亲留下的信',
              purpose: '引出母亲意外',
              characters: ['林默'],
              location: '老屋',
              conflict: '是否回江城',
              outcome: '决定留下',
              created_at: ISO,
              updated_at: ISO,
              children: [],
            },
            {
              id: 'c2',
              outline_id: OUTLINE_ID,
              parent_id: 's1',
              level: 3,
              level_name: '章',
              sequence: 2,
              title: '对峙',
              summary: '',
              purpose: '',
              characters: [],
              location: '',
              conflict: '',
              outcome: '',
              created_at: ISO,
              updated_at: ISO,
              children: [],
            },
          ],
        },
      ],
    },
  ],
}

const aiPayload = {
  volumes: [
    {
      title: '第一卷 · 归乡',
      summary: '回到老城区',
      sections: [
        {
          title: '第一节 · 旧信',
          summary: '',
          chapters: [
            { title: '遗物', purpose: '引出母亲意外', characters: ['林默'], location: '老屋', conflict: '查不查', outcome: '决定查' },
            { title: '夜谈', purpose: '补一场对话' },
          ],
        },
      ],
    },
  ],
}

function fakeFetch(input: RequestInfo | URL, init?: RequestInit) {
  const url = typeof input === 'string' ? input : input.toString()
  const method = (init?.method ?? 'GET').toUpperCase()
  const path = url.split('?')[0]
  const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
  calls.push({ method, path, body })

  if (path.endsWith(`/original/${ORIGINAL_ID}/creative-works`)) {
    return envelope({
      items: [
        {
          id: CREATIVE_ID,
          project_id: 'p2',
          original_work_id: ORIGINAL_ID,
          title: '暗涌·另一条路',
          description: '',
          status: 'ACTIVE',
          divergence_point_id: null,
          created_at: ISO,
          updated_at: ISO,
        },
      ],
      total: 1,
    })
  }
  if (path.endsWith(`/creative/${CREATIVE_ID}/outlines`) && method === 'GET') {
    return envelope({ items: [outlineBody], total: 1 })
  }
  if (path.endsWith(`/creative/${CREATIVE_ID}/outlines`) && method === 'POST') {
    return envelope({ outline: { ...outlineBody, id: 'ol-new', source: 'AI' }, nodes: [] }, 201)
  }
  if (path.endsWith(`/outlines/${OUTLINE_ID}`) && method === 'GET') {
    return envelope(outlineTree)
  }
  if (path.endsWith(`/outlines/${OUTLINE_ID}/nodes`) && method === 'POST') {
    return envelope(
      {
        id: 'new-node',
        outline_id: OUTLINE_ID,
        parent_id: body?.parent_id ?? null,
        level: body?.parent_id ? 2 : 1,
        level_name: body?.parent_id ? '节' : '卷',
        sequence: 9,
        title: body?.title ?? '',
        summary: '',
        purpose: '',
        characters: [],
        location: '',
        conflict: '',
        outcome: '',
        children: [],
        created_at: ISO,
        updated_at: ISO,
      },
      201,
    )
  }
  if (path.endsWith(`/outlines/${OUTLINE_ID}/materialize`)) {
    return envelope({ volumes_created: 1, volumes_reused: 0, chapters_created: 2, chapter_ids: ['c1', 'c2'] })
  }
  if (path.endsWith('/ai/generate')) {
    return envelope(aiPayload)
  }
  return envelope({ items: [], total: 0 })
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
  calls = []
  vi.stubGlobal('fetch', vi.fn(fakeFetch))
  useOriginalStore.setState({ workId: ORIGINAL_ID, work: null })
})

describe('二创 · 大纲', () => {
  it('没有原著时引导去原著总览', async () => {
    useOriginalStore.setState({ workId: null, work: null })
    renderPage(<OutlinePage />)

    expect(await screen.findByText(/先到「原著 · 总览」导入一本原文/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '去原著 · 总览' })).toBeInTheDocument()
  })

  it('列出大纲并渲染卷 / 节 / 章三层', async () => {
    renderPage(<OutlinePage />)

    expect(await screen.findByText('二创大纲 v1')).toBeInTheDocument()
    expect(screen.getByText('4 个节点 · v1 · 卷节章三层')).toBeInTheDocument()
    // 树的三层都在，层级标签分别是卷/节/章
    expect(await screen.findByText('第一卷 · 梅雨')).toBeInTheDocument()
    expect(screen.getByText('第一节 · 归乡')).toBeInTheDocument()
    expect(screen.getByText('旧信')).toBeInTheDocument()
    expect(screen.getByText('卷')).toBeInTheDocument()
    expect(screen.getByText('节')).toBeInTheDocument()
    expect(screen.getAllByText('章').length).toBe(2)
  })

  it('章节点下不再提供「子节点」入口（层级到章为止）', async () => {
    renderPage(<OutlinePage />)
    await screen.findByText('旧信')

    // 只有「卷」和「节」可以往下加：两个「子节点」按钮
    // （不能按 /子节点/ 模糊匹配 —— 那会把「在选中节点下加子节点」也数进来）
    expect(screen.getAllByText('子节点').length).toBe(2)
  })

  it('新增卷走 parent_id=null 的节点接口', async () => {
    renderPage(<OutlinePage />)
    await screen.findByText('旧信')

    fireEvent.click(screen.getByRole('button', { name: /新增卷/ }))
    const titleInput = await screen.findByPlaceholderText('例如：第一卷 · 梅雨')
    fireEvent.change(titleInput, { target: { value: '第二卷 · 决堤' } })
    // antd 会在两个汉字的按钮文案里插空格（"保 存"），所以用正则匹配
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }))

    await waitFor(() => {
      const call = calls.find((c) => c.method === 'POST' && c.path.endsWith(`/outlines/${OUTLINE_ID}/nodes`))
      expect(call).toBeTruthy()
      expect((call?.body as { parent_id: string | null }).parent_id).toBeNull()
      expect((call?.body as { title: string }).title).toBe('第二卷 · 决堤')
    })
  })

  it('落成章节调用 materialize 并如实报告结果', async () => {
    renderPage(<OutlinePage />)
    await screen.findByText('旧信')

    fireEvent.click(screen.getByRole('button', { name: /落成章节/ }))

    await waitFor(() => {
      expect(calls.some((c) => c.method === 'POST' && c.path.endsWith(`/outlines/${OUTLINE_ID}/materialize`))).toBe(true)
    })
    expect(await screen.findByText(/已落成：新建 1 卷 \/ 复用 0 卷 \/ 追加 2 章/)).toBeInTheDocument()
  })

  it('AI 只出候选，点采纳才写库（且按嵌套层级提交）', async () => {
    renderPage(<OutlinePage />)
    await screen.findByText('旧信')

    fireEvent.click(screen.getByRole('button', { name: /AI 生成/ }))
    fireEvent.change(await screen.findByPlaceholderText(/三卷结构/), { target: { value: '两卷结构' } })
    fireEvent.click(screen.getByRole('button', { name: /生成候选/ }))

    expect(await screen.findByText('第一卷 · 归乡')).toBeInTheDocument()
    expect(screen.getByText('第一节 · 旧信')).toBeInTheDocument()
    expect(screen.getByText('遗物')).toBeInTheDocument()
    // 只是生成候选，还没有任何写入
    expect(calls.some((c) => c.method === 'POST' && c.path.endsWith(`/creative/${CREATIVE_ID}/outlines`))).toBe(false)

    fireEvent.click(screen.getByRole('button', { name: '采纳为新大纲' }))

    await waitFor(() => {
      const call = calls.find((c) => c.method === 'POST' && c.path.endsWith(`/creative/${CREATIVE_ID}/outlines`))
      expect(call).toBeTruthy()
      const payload = call?.body as { source: string; nodes: Array<Record<string, unknown>> }
      expect(payload.source).toBe('AI')
      expect(payload.nodes[0].title).toBe('第一卷 · 归乡')
      const sections = payload.nodes[0].children as Array<Record<string, unknown>>
      expect(sections[0].title).toBe('第一节 · 旧信')
      expect((sections[0].children as unknown[]).length).toBe(2)
    })
  })
})

describe('AI 大纲候选 → 节点树', () => {
  it('按卷 → 节 → 章转换，并保留三要素与人物', () => {
    const nodes = aiOutlineToNodeInputs(aiPayload)
    expect(nodes).toHaveLength(1)
    expect(nodes[0].title).toBe('第一卷 · 归乡')
    expect(nodes[0].children?.[0].title).toBe('第一节 · 旧信')
    expect(nodes[0].children?.[0].children?.[0]).toMatchObject({
      title: '遗物',
      purpose: '引出母亲意外',
      characters: ['林默'],
      location: '老屋',
      conflict: '查不查',
      outcome: '决定查',
    })
    expect(countOutlineInputs(nodes)).toEqual({ volumes: 1, sections: 1, chapters: 2 })
  })

  it('模型省略「节」时补一层「正文」，不让章节错位成节', () => {
    const nodes = aiOutlineToNodeInputs({
      volumes: [{ title: '第一卷', chapters: [{ title: '第一章' }] }],
    })
    expect(nodes[0].children?.[0].title).toBe('正文')
    expect(nodes[0].children?.[0].children?.[0].title).toBe('第一章')
    expect(countOutlineInputs(nodes)).toEqual({ volumes: 1, sections: 1, chapters: 1 })
  })

  it('认不出的输出返回空数组，由界面提示重新生成', () => {
    expect(aiOutlineToNodeInputs(null)).toEqual([])
    expect(aiOutlineToNodeInputs({ foo: 'bar' })).toEqual([])
    expect(aiOutlineToNodeInputs({ volumes: '不是数组' })).toEqual([])
  })
})
