import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useOriginalStore } from '../../stores/originalStore'
import OriginalCharactersPage from './OriginalCharactersPage'
import OriginalWorldPage from './OriginalWorldPage'

// ---------- 假后端 ----------

interface FakeCharacter {
  id: string
  original_work_id: string
  name: string
  aliases: string[]
  role: string
  gender: string
  age: string
  appearance: string
  personality: string
  motivation: string
  values: string
  fears: string
  desires: string
  behavior_patterns: string
  speech_style: string
  abilities: string
  first_appearance: string
  last_appearance: string
  dna: Record<string, { text: string; weight: number }>
  importance: number
  source: 'MANUAL'
  notes: string
  created_at: string
  updated_at: string
}

let characters: FakeCharacter[] = []
let relationships: Array<Record<string, unknown>> = []
let world: Record<string, unknown> | null = null
let rules: Array<Record<string, unknown>> = []
let locations: Array<Record<string, unknown>> = []
let factions: Array<Record<string, unknown>> = []
let calls: Array<{ method: string; path: string; body?: unknown }> = []

const ISO = '2026-09-27T10:00:00Z'
const WORK_ID = 'work-1'

function envelope(data: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify({ data, error: null, trace_id: 'test' }), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

function notFound(code: string, message: string) {
  return Promise.resolve(
    new Response(JSON.stringify({ data: null, error: { code, message }, trace_id: 't' }), {
      status: 404,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

function makeCharacter(id: string, name: string, dnaWeight = 0): FakeCharacter {
  return {
    id,
    original_work_id: WORK_ID,
    name,
    aliases: [],
    role: '主角',
    gender: '女',
    age: '24',
    appearance: '',
    personality: '外冷内热',
    motivation: '',
    values: '',
    fears: '',
    desires: '',
    behavior_patterns: '',
    speech_style: '',
    abilities: '',
    first_appearance: '',
    last_appearance: '',
    dna: { personality: { text: '克制', weight: dnaWeight } },
    importance: 5,
    source: 'MANUAL',
    notes: '',
    created_at: ISO,
    updated_at: ISO,
  }
}

function fakeFetch(input: RequestInfo | URL, init?: RequestInit) {
  const url = typeof input === 'string' ? input : input.toString()
  const method = (init?.method ?? 'GET').toUpperCase()
  const path = url.split('?')[0]
  const body = init?.body && typeof init.body === 'string' ? JSON.parse(init.body) : undefined
  calls.push({ method, path, body })

  // 原著详情（store.loadWork 会调）
  if (path.endsWith(`/original/${WORK_ID}`) && method === 'GET') {
    return envelope({
      id: WORK_ID,
      project_id: 'p1',
      title: '暗涌',
      author: '',
      description: '',
      source_type: 'TXT',
      status: 'PARSED',
      char_count: 1851,
      chapter_count: 5,
      created_at: ISO,
      updated_at: ISO,
    })
  }

  // 人物
  if (path.endsWith(`/original/${WORK_ID}/characters`)) {
    if (method === 'GET') {
      return envelope({ items: characters, total: characters.length, page: 1, page_size: 200 })
    }
    if (method === 'POST') {
      const created = makeCharacter('char-new', String((body as { name: string }).name))
      created.dna = (body as { dna: FakeCharacter['dna'] }).dna ?? created.dna
      characters = [...characters, created]
      return envelope(created, 201)
    }
  }
  if (path.includes('/characters/') && method === 'DELETE') {
    const id = path.split('/').pop() as string
    characters = characters.filter((c) => c.id !== id)
    return envelope({ deleted: true })
  }

  // 人物关系
  if (path.endsWith(`/original/${WORK_ID}/relationships`)) {
    if (method === 'GET') return envelope({ items: relationships, total: relationships.length })
    if (method === 'POST') {
      const created = { id: 'rel-1', ...(body as object), source: 'MANUAL', created_at: ISO, updated_at: ISO }
      relationships = [...relationships, created]
      return envelope(created, 201)
    }
  }

  // 世界观
  if (path.endsWith(`/original/${WORK_ID}/world`)) {
    if (method === 'GET') {
      return world ? envelope(world) : notFound('WORLD_NOT_FOUND', '世界观不存在')
    }
    if (method === 'PUT') {
      world = {
        id: 'world-1',
        original_work_id: WORK_ID,
        name: (body as { name?: string }).name ?? '',
        description: (body as { description?: string }).description ?? '',
        rule_count: rules.length,
        location_count: locations.length,
        faction_count: factions.length,
        created_at: ISO,
        updated_at: ISO,
      }
      return envelope(world)
    }
  }
  if (path.endsWith(`/original/${WORK_ID}/rules`)) {
    if (method === 'GET') return envelope({ items: rules, total: rules.length })
    if (method === 'POST') {
      const created = { id: 'rule-1', world_id: 'world-1', ...(body as object), created_at: ISO, updated_at: ISO }
      rules = [...rules, created]
      return envelope(created, 201)
    }
  }
  if (path.endsWith(`/original/${WORK_ID}/locations`)) {
    if (method === 'GET') return envelope({ items: locations, total: locations.length })
  }
  if (path.endsWith(`/original/${WORK_ID}/factions`)) {
    if (method === 'GET') return envelope({ items: factions, total: factions.length })
  }

  return notFound('NOT_FOUND', `未处理 ${method} ${path}`)
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
  characters = [makeCharacter('char-1', '林默', 90), makeCharacter('char-2', '陈述')]
  relationships = []
  world = null
  rules = []
  locations = []
  factions = []
  calls = []
  useOriginalStore.setState({ workId: WORK_ID, work: null, chapters: [], chapter: null, error: null })
  vi.stubGlobal('fetch', vi.fn(fakeFetch))
})

describe('原著人物页', () => {
  it('展示人物与 DNA 摘要，支持新增（提交 DNA 权重）与删除', async () => {
    renderPage(<OriginalCharactersPage />)

    // 列表 + DNA 摘要
    expect(await screen.findByText('林默')).toBeInTheDocument()
    expect(screen.getByText(/人格 90%/)).toBeInTheDocument()
    expect(screen.getByText('陈述')).toBeInTheDocument()

    // 新增人物，并把「人格」权重设成 85
    fireEvent.click(screen.getByRole('button', { name: /新增人物/ }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText('姓名'), { target: { value: '苏晚' } })

    const weights = within(dialog).getAllByRole('spinbutton')
    fireEvent.change(weights[0], { target: { value: '85' } })
    fireEvent.click(within(dialog).getByRole('button', { name: /保\s*存/ }))

    await waitFor(() => {
      const post = calls.find((c) => c.method === 'POST' && c.path.endsWith('/characters'))
      expect(post).toBeDefined()
      expect(post?.body).toMatchObject({ name: '苏晚' })
      expect((post?.body as { dna: { personality: { weight: number } } }).dna.personality.weight).toBe(85)
    })
    expect(await screen.findByText('苏晚')).toBeInTheDocument()

    // 删除人物
    const row = screen.getByText('陈述').closest('tr') as HTMLElement
    fireEvent.click(within(row).getByRole('button', { name: /删\s*除/ }))
    fireEvent.click(await screen.findByRole('button', { name: '确认删除' }))
    await waitFor(() => {
      expect(calls.some((c) => c.method === 'DELETE' && c.path.includes('/characters/char-2'))).toBe(true)
    })
  })
})

describe('原著世界观页', () => {
  it('保存世界设定并新增规则，页面上能看到统计与规则', async () => {
    renderPage(<OriginalWorldPage />)

    // 初始没有世界：表单为空、计数为 0
    const nameInput = await screen.findByLabelText('世界名称')
    fireEvent.change(nameInput, { target: { value: '九州' } })
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }))

    await waitFor(() => {
      const put = calls.find((c) => c.method === 'PUT' && c.path.endsWith('/world'))
      expect(put).toBeDefined()
      expect(put?.body).toMatchObject({ name: '九州' })
    })

    // 切到规则标签已在默认页；新增规则
    fireEvent.click(screen.getByRole('button', { name: /新增规则/ }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText('规则名称'), { target: { value: '灵力不可凭空产生' } })
    fireEvent.click(within(dialog).getByRole('button', { name: /保\s*存/ }))

    await waitFor(() => {
      const post = calls.find((c) => c.method === 'POST' && c.path.endsWith('/rules'))
      expect(post).toBeDefined()
      expect(post?.body).toMatchObject({ name: '灵力不可凭空产生', importance: 3 })
    })
    expect(await screen.findByText('灵力不可凭空产生')).toBeInTheDocument()
    expect(screen.getByText('世界规则（1）')).toBeInTheDocument()
  })
})
