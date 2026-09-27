import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SettingsPage from './SettingsPage'
import TasksPage from './TasksPage'
import OriginalAnalysisPage from './original/OriginalAnalysisPage'
import { useOriginalStore } from '../stores/originalStore'

// ---------- 假后端 ----------

const WORK_ID = 'work-1'
const ISO = '2026-09-27T12:00:00Z'

let providers: Array<Record<string, unknown>> = []
let tasks: Array<Record<string, unknown>> = []
let proposals: Array<Record<string, unknown>> = []
let calls: Array<{ method: string; path: string; body?: unknown }> = []

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
  const path = url.split('?')[0]
  const body = init?.body && typeof init.body === 'string' ? JSON.parse(init.body) : undefined
  calls.push({ method, path, body })

  if (path.endsWith('/model-providers') && method === 'GET') {
    return envelope({ items: providers, total: providers.length })
  }
  if (path.endsWith('/model-providers') && method === 'POST') {
    const created = { id: 'provider-new', has_api_key: true, is_default: false, ...(body as object) }
    providers = [...providers, created]
    return envelope(created, 201)
  }
  if (path.endsWith('/test')) {
    return envelope({ ok: true, model: 'fake-model', reply: '好', latency_ms: 12, total_tokens: 4 })
  }
  if (path.endsWith('/prompts')) {
    return envelope({
      items: [
        { name: 'character_extract', version: 'v1', path: 'character/character_extract.v1.md' },
        { name: 'world_extract', version: 'v1', path: 'world/world_extract.v1.md' },
      ],
      total: 2,
    })
  }
  if (path.endsWith('/tasks') && method === 'GET') {
    return envelope({ items: tasks, total: tasks.length, page: 1, page_size: 50 })
  }
  if (path.endsWith('/cancel')) {
    return envelope({ cancelled: true })
  }
  if (path.endsWith('/retry')) {
    return envelope({ ...tasks[0], status: 'PENDING', attempts: 0 })
  }
  if (path.endsWith(`/original/${WORK_ID}`) && method === 'GET') {
    return envelope({
      id: WORK_ID, project_id: 'p1', title: '暗涌', author: '', description: '',
      source_type: 'TXT', status: 'PARSED', char_count: 100, chapter_count: 3,
      created_at: ISO, updated_at: ISO,
    })
  }
  if (path.endsWith('/analysis/summary')) {
    return envelope({
      pending: proposals.filter((p) => p.status === 'PENDING').length,
      approved: proposals.filter((p) => p.status === 'APPROVED').length,
      rejected: proposals.filter((p) => p.status === 'REJECTED').length,
    })
  }
  if (path.endsWith('/proposals') && method === 'GET') {
    const status = new URLSearchParams(url.split('?')[1] ?? '').get('status')
    const items = status ? proposals.filter((p) => p.status === status) : proposals
    return envelope({ items, total: items.length, page: 1, page_size: 100 })
  }
  if (path.endsWith('/approve')) {
    const id = path.split('/')[path.split('/').length - 2]
    proposals = proposals.map((p) =>
      p.id === id ? { ...p, status: 'APPROVED', applied_id: 'applied-1', payload: body?.payload ?? p.payload } : p,
    )
    return envelope(proposals.find((p) => p.id === id))
  }
  if (path.endsWith('/reject')) {
    const id = path.split('/')[path.split('/').length - 2]
    proposals = proposals.map((p) => (p.id === id ? { ...p, status: 'REJECTED' } : p))
    return envelope(proposals.find((p) => p.id === id))
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
  providers = [
    {
      id: 'provider-1', name: 'DeepSeek 主力', provider: 'OPENAI_COMPATIBLE',
      api_base: 'https://api.deepseek.com/v1', model_name: 'deepseek-chat',
      purpose: 'chat', temperature: 0.7, max_tokens: 4096, timeout_sec: 120,
      enabled: true, is_default: true, notes: '', has_api_key: true,
      created_at: ISO, updated_at: ISO,
    },
  ]
  tasks = [
    {
      id: 'task-running', project_id: 'p1', work_id: WORK_ID, type: 'analysis_character_extract',
      status: 'RUNNING', progress: 40, progress_message: '调用模型', input: { work_id: WORK_ID },
      output: {}, error: '', attempts: 1, max_attempts: 3,
      created_at: ISO, started_at: ISO, finished_at: null, updated_at: ISO,
    },
    {
      id: 'task-failed', project_id: 'p1', work_id: WORK_ID, type: 'original_reparse',
      status: 'FAILED', progress: 0, progress_message: '已失败', input: {},
      output: {}, error: '原著不存在', attempts: 1, max_attempts: 1,
      created_at: ISO, started_at: ISO, finished_at: ISO, updated_at: ISO,
    },
  ]
  proposals = [
    {
      id: 'proposal-1', work_id: WORK_ID, task_id: 'task-1', stage: 'character_extract',
      entity_type: 'character', title: '林默',
      payload: { name: '林默', role: '主角', dna: { personality: { text: '克制', weight: 95 } } },
      evidence: '林默站在月台上。', confidence: 0, status: 'PENDING', review_note: '',
      reviewed_at: null, applied_id: null, created_at: ISO, updated_at: ISO,
    },
  ]
  calls = []
  useOriginalStore.setState({ workId: WORK_ID, work: null, chapters: [], chapter: null, error: null })
  vi.stubGlobal('fetch', vi.fn(fakeFetch))
})

describe('模型设置页', () => {
  it('展示已配置模型与模板清单，并能新增配置与测连通', async () => {
    renderPage(<SettingsPage />)

    expect(await screen.findByText('DeepSeek 主力')).toBeInTheDocument()
    expect(screen.getByText('已保存')).toBeInTheDocument() // 密钥状态
    expect(screen.getByText('character_extract')).toBeInTheDocument() // 模板清单

    // 测连通
    fireEvent.click(screen.getByRole('button', { name: /测\s*试/ }))
    await waitFor(() => {
      expect(calls.some((c) => c.path.endsWith('/provider-1/test'))).toBe(true)
    })

    // 新增配置
    fireEvent.click(screen.getByRole('button', { name: /新增模型配置/ }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText('名称'), { target: { value: '智谱备用' } })
    fireEvent.change(within(dialog).getByLabelText('接口地址'), {
      target: { value: 'https://open.bigmodel.cn/api/paas/v4' },
    })
    fireEvent.change(within(dialog).getByLabelText('模型名'), { target: { value: 'glm-4-plus' } })
    fireEvent.change(within(dialog).getByLabelText('API Key'), { target: { value: 'sk-new-key' } })
    fireEvent.click(within(dialog).getByRole('button', { name: /保\s*存/ }))

    await waitFor(() => {
      const post = calls.find((c) => c.method === 'POST' && c.path.endsWith('/model-providers'))
      expect(post).toBeDefined()
      expect(post?.body).toMatchObject({
        name: '智谱备用',
        model_name: 'glm-4-plus',
        api_key: 'sk-new-key',
      })
    })
  })
})

describe('任务中心', () => {
  it('展示进度与错误，支持取消与重试', async () => {
    renderPage(<TasksPage />)

    expect(await screen.findByText('analysis_character_extract')).toBeInTheDocument()
    expect(screen.getByText('执行中')).toBeInTheDocument()
    expect(screen.getByText('调用模型')).toBeInTheDocument()
    expect(screen.getByText('原著不存在')).toBeInTheDocument() // 失败原因

    // 取消运行中的任务
    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }))
    fireEvent.click(await screen.findByRole('button', { name: '确认取消' }))
    await waitFor(() => {
      expect(calls.some((c) => c.method === 'POST' && c.path.endsWith('/task-running/cancel'))).toBe(true)
    })

    // 重试失败的任务
    fireEvent.click(screen.getByRole('button', { name: /重\s*试/ }))
    await waitFor(() => {
      expect(calls.some((c) => c.method === 'POST' && c.path.endsWith('/task-failed/retry'))).toBe(true)
    })
  })
})

describe('AI 分析审核页', () => {
  it('展示提案与统计，能改内容后通过', async () => {
    renderPage(<OriginalAnalysisPage />)

    // 统计 + 提案（"待审核"既是统计标题也是筛选选项，用 findAll）
    expect((await screen.findAllByText('待审核')).length).toBeGreaterThan(0)
    expect(screen.getByText('林默')).toBeInTheDocument()
    expect(screen.getByText(/依据：林默站在月台上。/)).toBeInTheDocument()

    // 打开审核弹窗
    fireEvent.click(screen.getByRole('button', { name: /审\s*核/ }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('模型给出的原文依据')).toBeInTheDocument()

    // 修改内容后通过
    const textarea = dialog.querySelector('textarea') as HTMLTextAreaElement
    expect(textarea).toBeTruthy()
    fireEvent.change(textarea, {
      target: { value: JSON.stringify({ name: '林默（已校对）', role: '主角' }) },
    })
    fireEvent.click(within(dialog).getByRole('button', { name: /通过并写入原著/ }))

    await waitFor(() => {
      const post = calls.find((c) => c.method === 'POST' && c.path.endsWith('/proposal-1/approve'))
      expect(post).toBeDefined()
      expect(post?.body).toMatchObject({
        payload: { name: '林默（已校对）', role: '主角' },
      })
    })
  })
})
