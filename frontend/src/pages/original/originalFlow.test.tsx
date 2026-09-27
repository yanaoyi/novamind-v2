import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import App from '../../App'
import { useOriginalStore } from '../../stores/originalStore'
import { useProjectStore } from '../../stores/projectStore'

// ---------- 假后端：把"工程 → 原著 → 章节"的关系存在内存里 ----------

interface FakeProject {
  id: string
  name: string
  description: string
  type: 'ORIGINAL' | 'CREATIVE'
  status: 'ACTIVE'
  created_at: string
  updated_at: string
}

let projects: FakeProject[] = []
let works: Record<string, Record<string, unknown>> = {} // projectId -> work
let chapters: Record<string, Array<Record<string, unknown>>> = {} // workId -> chapters
let importCalls: Array<{ isFormData: boolean; fileName: string }> = []

const ISO = '2026-09-27T09:00:00Z'

function envelope(data: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify({ data, error: null, trace_id: 'test' }), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

function errorEnvelope(status: number, code: string, message: string) {
  return Promise.resolve(
    new Response(JSON.stringify({ data: null, error: { code, message }, trace_id: 'test' }), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

function buildChapters(workId: string, count: number) {
  return Array.from({ length: count }, (_, i) => ({
    chapter_no: i + 1,
    title: i === 0 ? '第一章 初遇' : `第${i + 1}章 测试章节`,
    char_count: 458,
    summary: '',
    content: `这是第${i + 1}章的正文内容，用于验证阅读页。`,
    start_position: i * 1000,
    end_position: (i + 1) * 1000,
    work_id: workId,
  }))
}

function fakeFetch(input: RequestInfo | URL, init?: RequestInit) {
  const url = typeof input === 'string' ? input : input.toString()
  const method = (init?.method ?? 'GET').toUpperCase()
  const path = url.split('?')[0]

  // GET /projects?type=ORIGINAL
  if (path.endsWith('/projects') && method === 'GET') {
    return envelope({
      items: projects,
      total: projects.length,
      page: 1,
      page_size: 100,
    })
  }

  // /projects/:id/original
  const byProject = /\/projects\/([^/]+)\/original$/.exec(path)
  if (byProject) {
    const projectId = byProject[1]
    if (method === 'GET') {
      const work = works[projectId]
      return work ? envelope(work) : errorEnvelope(404, 'ORIGINAL_NOT_FOUND', '原著不存在')
    }
    if (method === 'POST') {
      const body = JSON.parse(String(init?.body ?? '{}')) as { title: string; author?: string }
      const work = {
        id: 'work-1',
        project_id: projectId,
        title: body.title,
        author: body.author ?? '',
        description: '',
        source_type: 'MANUAL',
        status: 'DRAFT',
        char_count: 0,
        chapter_count: 0,
        created_at: ISO,
        updated_at: ISO,
      }
      works[projectId] = work
      return envelope(work, 201)
    }
  }

  const importMatch = /\/original\/([^/]+)\/import$/.exec(path)
  if (importMatch && method === 'POST') {
    const workId = importMatch[1]
    const form = init?.body as FormData
    const file = form instanceof FormData ? (form.get('file') as File) : null
    importCalls.push({
      isFormData: form instanceof FormData,
      fileName: file?.name ?? '',
    })
    const built = buildChapters(workId, 4)
    chapters[workId] = built
    const work = Object.values(works).find((w) => w.id === workId) as Record<string, unknown>
    Object.assign(work, { status: 'PARSED', chapter_count: 4, char_count: 1851, source_type: 'TXT' })
    return envelope({
      file_id: 'file-1',
      file_name: file?.name ?? 'unknown.txt',
      size_bytes: 3401,
      encoding: 'GB18030',
      char_count: 1851,
      chapter_count: 4,
      chapters: built.map((c) => ({
        chapter_no: c.chapter_no,
        title: c.title,
        char_count: c.char_count,
        summary: '',
      })),
    })
  }

  const chapterDetail = /\/original\/([^/]+)\/chapters\/(\d+)$/.exec(path)
  if (chapterDetail) {
    const list = chapters[chapterDetail[1]] ?? []
    const found = list.find((c) => c.chapter_no === Number(chapterDetail[2]))
    return found ? envelope(found) : errorEnvelope(404, 'CHAPTER_NOT_FOUND', '章节不存在')
  }

  const chapterList = /\/original\/([^/]+)\/chapters$/.exec(path)
  if (chapterList) {
    const list = chapters[chapterList[1]] ?? []
    return envelope({
      items: list.map((c) => ({
        chapter_no: c.chapter_no,
        title: c.title,
        char_count: c.char_count,
        summary: '',
      })),
      total: list.length,
      page: 1,
      page_size: 50,
    })
  }

  const workDetail = /\/original\/([^/]+)$/.exec(path)
  if (workDetail && method === 'GET') {
    const found = Object.values(works).find((w) => w.id === workDetail[1])
    return found ? envelope(found) : errorEnvelope(404, 'ORIGINAL_NOT_FOUND', '原著不存在')
  }

  return errorEnvelope(404, 'NOT_FOUND', `未处理的请求 ${method} ${path}`)
}

function renderAtOriginal() {
  return render(
    <ConfigProvider locale={zhCN}>
      <MemoryRouter
        initialEntries={[{ pathname: '/original/overview', state: { projectId: 'p1' } }]}
      >
        <App />
      </MemoryRouter>
    </ConfigProvider>,
  )
}

beforeEach(() => {
  localStorage.clear()
  projects = [
    {
      id: 'p1',
      name: '暗涌工程',
      description: '',
      type: 'ORIGINAL',
      status: 'ACTIVE',
      created_at: ISO,
      updated_at: ISO,
    },
  ]
  works = {}
  chapters = {}
  importCalls = []
  useOriginalStore.setState({
    workId: null,
    work: null,
    chapters: [],
    chaptersTotal: 0,
    chapter: null,
    error: null,
  })
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

describe('原著工作流', () => {
  it('从工程进入 → 创建原著 → 上传导入 → 看目录 → 读章节', async () => {
    renderAtOriginal()

    // 1) 自动定位到传入的工程，因为没有原著，出现创建表单
    const createCard = await screen.findByText('该工程还没有原著，创建一个')
    expect(createCard).toBeInTheDocument()

    // 标题应预填工程名
    const titleInput = screen.getByLabelText('原著标题')
    expect(titleInput).toHaveValue('暗涌工程')

    fireEvent.click(screen.getByRole('button', { name: /创\s*建\s*原\s*著/ }))

    // 2) 创建成功后进入详情视图
    expect(await screen.findByText('导入原文')).toBeInTheDocument()
    expect(screen.getByText('待导入')).toBeInTheDocument()

    // 3) 选择文件 → 导入
    const file = new File(['第一章 初遇\n正文'], '暗涌.txt', { type: 'text/plain' })
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    fireEvent.change(fileInput, { target: { files: [file] } })

    fireEvent.click(screen.getByRole('button', { name: /开始导入/ }))

    await waitFor(() => {
      expect(importCalls).toHaveLength(1)
    })
    // 关键：上传必须是 multipart（FormData），而不是 JSON
    expect(importCalls[0].isFormData).toBe(true)
    expect(importCalls[0].fileName).toBe('暗涌.txt')

    // 导入后详情刷新：章节数 4、状态已导入
    expect(await screen.findByText('已导入')).toBeInTheDocument()
    const toChapters = await screen.findByRole('link', { name: /查看章节/ })
    expect(toChapters).toBeInTheDocument()

    // 4) 进入目录
    fireEvent.click(toChapters)

    const firstChapter = await screen.findByText('第一章 初遇')
    expect(firstChapter).toBeInTheDocument()
    expect(screen.getByText('共 4 章')).toBeInTheDocument()

    // 5) 点「阅读」看正文
    const row = firstChapter.closest('tr') as HTMLElement
    fireEvent.click(within(row).getByRole('button', { name: /阅\s*读/ }))

    expect(await screen.findByText(/这是第1章的正文内容/)).toBeInTheDocument()
    expect(screen.getByText(/第 1 \/ 4 章/)).toBeInTheDocument()
  })
})
