import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import EntityVersions from './EntityVersions'

const ISO = '2026-09-28T10:00:00Z'
const CHAR_ID = 'char-1'

let calls: Array<{ method: string; path: string; body?: unknown }> = []
let versions: Array<Record<string, unknown>> = []

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
  const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
  calls.push({ method, path, body })
  if (path.endsWith(`/creative-characters/${CHAR_ID}/versions`) && method === 'GET') {
    return envelope({ items: versions, total: versions.length })
  }
  if (/\/versions\/\d+$/.test(path) && method === 'GET') {
    const no = Number(path.split('/').pop())
    return envelope({
      ...versions.find((v) => v.version_no === no),
      payload: { name: no === 1 ? '林默' : '林默（改）', importance: 5 },
    })
  }
  if (/\/versions\/\d+\/restore$/.test(path) && method === 'POST') {
    return envelope({ restored: true, version_no: Number(path.split('/')[3]) })
  }
  if (path.endsWith(`/creative-characters/${CHAR_ID}/versions`) && method === 'POST') {
    return envelope({ created: false, reason: '与最新一版相同' })
  }
  return envelope({}, 404)
}

function renderPanel(onRestored = vi.fn()) {
  return render(
    <ConfigProvider locale={zhCN}>
      <EntityVersions entityType="creative_character" entityId={CHAR_ID} onRestored={onRestored} />
    </ConfigProvider>,
  )
}

beforeEach(() => {
  calls = []
  // antd 的 Modal/Drawer 挂在 body 上且不随 unmount 清理，测试之间必须手动清，否则会串台
  document.body.innerHTML = ''
  vi.stubGlobal('fetch', vi.fn(fakeFetch))
  versions = [
    { id: 'v2', entity_type: 'creative_character', entity_id: CHAR_ID, version_no: 2, note: '作者修改人物', created_at: ISO },
    { id: 'v1', entity_type: 'creative_character', entity_id: CHAR_ID, version_no: 1, note: '继承原著人物', created_at: ISO },
  ]
})

describe('通用版本面板', () => {
  it('打开抽屉列出历史版本', async () => {
    renderPanel()
    fireEvent.click(screen.getByRole('button', { name: /版\s*本/ }))
    const drawer = await screen.findByRole('dialog')
    await waitFor(() => {
      expect(within(drawer).getByText('作者修改人物')).toBeInTheDocument()
    })
    expect(within(drawer).getByText('继承原著人物')).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'GET' && c.path.endsWith(`/creative-characters/${CHAR_ID}/versions`))).toBe(true)
  })

  it('预览某个版本会展示快照内容', async () => {
    renderPanel()
    fireEvent.click(screen.getByRole('button', { name: /版\s*本/ }))
    const drawer = await screen.findByRole('dialog')
    await waitFor(() => expect(within(drawer).getByText('作者修改人物')).toBeInTheDocument())

    fireEvent.click(within(drawer).getAllByRole('button', { name: /预\s*览/ })[1])
    await waitFor(() => {
      expect(screen.getByText(/"name": "林默"/)).toBeInTheDocument()
    })
  })

  it('恢复要先确认，成功后回调外层刷新', async () => {
    const onRestored = vi.fn()
    renderPanel(onRestored)
    fireEvent.click(screen.getByRole('button', { name: /版\s*本/ }))
    const drawer = await screen.findByRole('dialog')
    await waitFor(() => expect(within(drawer).getByText('继承原著人物')).toBeInTheDocument())

    fireEvent.click(within(drawer).getAllByRole('button', { name: /恢\s*复/ })[1])
    const titles = await screen.findAllByText('恢复到 v1？')
    const confirm = titles[titles.length - 1].closest('.ant-modal') as HTMLElement
    fireEvent.click(within(confirm).getByRole('button', { name: /确\s*定/ }))

    await waitFor(() => {
      expect(calls.some((c) => c.method === 'POST' && c.path.endsWith('/versions/1/restore'))).toBe(true)
    })
    await waitFor(() => expect(onRestored).toHaveBeenCalled())
  })

  it('手动存档请求会带上备注；后端说没新建版本时列表不变', async () => {
    renderPanel()
    fireEvent.click(screen.getByRole('button', { name: /版\s*本/ }))
    const drawer = await screen.findByRole('dialog')
    await waitFor(() => expect(within(drawer).getByText('继承原著人物')).toBeInTheDocument())

    fireEvent.change(within(drawer).getByPlaceholderText('存档备注（可选）'), {
      target: { value: '定稿前存档' },
    })
    fireEvent.click(within(drawer).getByRole('button', { name: /手动存档/ }))
    await waitFor(() => {
      const post = calls.find(
        (c) => c.method === 'POST' && c.path.endsWith(`/creative-characters/${CHAR_ID}/versions`),
      )
      expect(post?.body).toEqual({ note: '定稿前存档' })
    })
    // 后端返回 created:false → 不新增版本行
    expect(within(drawer).getAllByText(/^v\d$/)).toHaveLength(2)
  })
})
