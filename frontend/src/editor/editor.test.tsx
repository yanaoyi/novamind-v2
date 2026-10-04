import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'

import RichTextEditor from './RichTextEditor'
import { htmlToMarkdown, markdownToHtml } from './markdown'

describe('Markdown ↔ HTML 转换', () => {
  it('把 Markdown 转成编辑器 HTML', () => {
    const html = markdownToHtml('# 标题\n\n正文 **加粗** 与 *斜体*\n\n- 甲\n- 乙\n\n> 引用\n\n---')
    expect(html).toContain('<h1>标题</h1>')
    expect(html).toContain('<strong>加粗</strong>')
    expect(html).toContain('<em>斜体</em>')
    expect(html).toContain('<ul>')
    expect(html).toContain('<blockquote>')
    expect(html).toContain('<hr>')
  })

  it('把编辑器 HTML 转回 Markdown', () => {
    const md = htmlToMarkdown(
      '<h2>小节</h2><p>一段<strong>重点</strong></p><ul><li><p>甲</p></li><li><p>乙</p></li></ul><blockquote><p>引用</p></blockquote>',
    )
    expect(md).toContain('## 小节')
    expect(md).toContain('**重点**')
    expect(md).toContain('- 甲')
    expect(md).toContain('- 乙')
    expect(md).toContain('> 引用')
  })

  it('Markdown 里的 HTML 特殊字符被转义（不能把正文当标签执行）', () => {
    const html = markdownToHtml('如果 a < b && c > d 就要转义')
    expect(html).toContain('&lt;')
    expect(html).toContain('&amp;')
    expect(html).not.toContain('<b ')
  })

  it('往返转换不丢字', () => {
    const source = '# 第一章\n\n雨下了三天。林默 **摸到一封信**。\n\n- 第一件事\n- 第二件事'
    const roundTrip = htmlToMarkdown(markdownToHtml(source))
    expect(roundTrip).toContain('# 第一章')
    expect(roundTrip).toContain('雨下了三天。林默 **摸到一封信**。')
    expect(roundTrip).toContain('- 第一件事')
    expect(roundTrip).toContain('- 第二件事')
  })
})

function Harness({ initial, onChange }: { initial: string; onChange: (md: string) => void }) {
  const [value, setValue] = useState(initial)
  return (
    <RichTextEditor
      value={value}
      onChange={(md) => {
        setValue(md)
        onChange(md)
      }}
    />
  )
}

describe('富文本编辑器', () => {
  it('把 Markdown 渲染成富文本（加粗是 <strong> 而不是星号）', async () => {
    render(
      <ConfigProvider locale={zhCN}>
        <Harness initial="**加粗的字**" onChange={() => {}} />
      </ConfigProvider>,
    )
    const host = await screen.findByTestId('rich-editor')
    await waitFor(() => {
      expect(host.querySelector('strong')?.textContent).toBe('加粗的字')
    })
    expect(host.textContent).not.toContain('**')
  })

  it('提供标题/列表/引用工具条', async () => {
    render(
      <ConfigProvider locale={zhCN}>
        <Harness initial="正文" onChange={() => {}} />
      </ConfigProvider>,
    )
    await screen.findByTestId('rich-editor')
    for (const label of ['加粗', '斜体', '标题 1', '引用', '无序列表', '有序列表', '分割线']) {
      expect(screen.getByLabelText(label)).toBeInTheDocument()
    }
  })

  it('切到 Markdown 源码模式后编辑会向上抛 Markdown', async () => {
    const onChange = vi.fn()
    render(
      <ConfigProvider locale={zhCN}>
        <Harness initial="原文" onChange={onChange} />
      </ConfigProvider>,
    )
    await screen.findByTestId('rich-editor')
    fireEvent.click(screen.getByRole('button', { name: 'Markdown 源码' }))
    const textarea = document.querySelector('textarea') as HTMLTextAreaElement
    fireEvent.change(textarea, { target: { value: '# 新标题\n\n新正文' } })
    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith('# 新标题\n\n新正文')
    })
  })

  // 审查 P1-14：富文本 ↔ 源码来回切不能给正文叠反斜杠。
  // 之前 htmlToMarkdown 会转义 * _ `，而 markdownToHtml 不还原，
  // 于是每切一次源码模式、正文里的 \ 就翻一倍。
  it('转义字符在 Markdown ↔ HTML 往返中保持稳定', () => {
    const source = 'a*b 与 c_d 还有 `e`'
    const once = htmlToMarkdown(markdownToHtml(source))
    const twice = htmlToMarkdown(markdownToHtml(once))
    expect(twice).toBe(once)
    expect(once).not.toContain('\\\\*') // 不允许出现 \\* 这种双层转义
    // 再转回 HTML 时，字面量星号仍然是文本，而不是变成斜体标记
    const html = markdownToHtml(once)
    expect(html).toContain('a*b')
  })
})
