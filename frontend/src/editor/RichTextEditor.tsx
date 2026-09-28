import {
  BoldOutlined,
  ItalicOutlined,
  OrderedListOutlined,
  StrikethroughOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons'
import { EditorContent, useEditor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import { Button, Empty, Space, Tooltip, Typography } from 'antd'
import { useEffect, useRef, useState } from 'react'

import { htmlToMarkdown, markdownToHtml } from './markdown'

interface Props {
  /** Markdown 文本（正文的真实存储形式） */
  value: string
  onChange: (markdown: string) => void
  placeholder?: string
  minRows?: number
}

/**
 * 富文本编辑器（Tiptap / ProseMirror）。
 *
 * 存储仍然是 Markdown：编辑时把 Markdown 转成 HTML 交给编辑器，
 * 编辑器每次变化再转回 Markdown 抛给上层（自动保存、版本、导出全部沿用原逻辑）。
 */
export default function RichTextEditor({ value, onChange, placeholder, minRows = 16 }: Props) {
  const [mode, setMode] = useState<'rich' | 'source'>('rich')
  const lastEmitted = useRef(value)

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: { levels: [1, 2, 3] },
      }),
    ],
    content: markdownToHtml(value),
    editorProps: {
      attributes: {
        class: 'novamind-editor',
        style: `min-height:${minRows * 24}px; outline:none; line-height:1.8;`,
        'data-placeholder': placeholder ?? '在此写正文……',
      },
    },
    onUpdate: ({ editor: ed }) => {
      const md = htmlToMarkdown(ed.getHTML())
      lastEmitted.current = md
      onChange(md)
    },
  })

  // 外部（切换章节、AI 改写、恢复版本）改动文本时同步进编辑器
  useEffect(() => {
    if (!editor) return
    if (value === lastEmitted.current) return
    lastEmitted.current = value
    editor.commands.setContent(markdownToHtml(value), { emitUpdate: false })
  }, [editor, value])

  if (!editor) {
    return <Empty description="编辑器加载中……" />
  }

  const btn = (
    key: string,
    icon: React.ReactNode,
    title: string,
    active: boolean,
    run: () => void,
  ) => (
    <Tooltip title={title} key={key}>
      <Button size="small" type={active ? 'primary' : 'default'} icon={icon} onClick={run} aria-label={title} />
    </Tooltip>
  )

  return (
    <div>
      <Space wrap style={{ marginBottom: 8 }}>
        <Button.Group>
          <Button
            size="small"
            type={mode === 'rich' ? 'primary' : 'default'}
            onClick={() => setMode('rich')}
          >
            富文本
          </Button>
          <Button
            size="small"
            type={mode === 'source' ? 'primary' : 'default'}
            onClick={() => setMode('source')}
          >
            Markdown 源码
          </Button>
        </Button.Group>
        {mode === 'rich' && (
          <>
            <Button.Group>
              {btn('bold', <BoldOutlined />, '加粗', editor.isActive('bold'), () =>
                editor.chain().focus().toggleBold().run(),
              )}
              {btn('italic', <ItalicOutlined />, '斜体', editor.isActive('italic'), () =>
                editor.chain().focus().toggleItalic().run(),
              )}
              {btn('strike', <StrikethroughOutlined />, '删除线', editor.isActive('strike'), () =>
                editor.chain().focus().toggleStrike().run(),
              )}
            </Button.Group>
            <Button.Group>
              {[1, 2, 3].map((level) =>
                btn(
                  `h${level}`,
                  <span>H{level}</span>,
                  `标题 ${level}`,
                  editor.isActive('heading', { level }),
                  () => editor.chain().focus().toggleHeading({ level: level as 1 | 2 | 3 }).run(),
                ),
              )}
            </Button.Group>
            <Button.Group>
              {btn('quote', <span>「」</span>, '引用', editor.isActive('blockquote'), () =>
                editor.chain().focus().toggleBlockquote().run(),
              )}
              {btn('ul', <UnorderedListOutlined />, '无序列表', editor.isActive('bulletList'), () =>
                editor.chain().focus().toggleBulletList().run(),
              )}
              {btn('ol', <OrderedListOutlined />, '有序列表', editor.isActive('orderedList'), () =>
                editor.chain().focus().toggleOrderedList().run(),
              )}
              {btn('hr', <span>―</span>, '分割线', false, () =>
                editor.chain().focus().setHorizontalRule().run(),
              )}
            </Button.Group>
          </>
        )}
        <Typography.Text type="secondary">
          {mode === 'rich' ? '正文以 Markdown 存储，导出与版本不受影响' : '直接编辑 Markdown 文本'}
        </Typography.Text>
      </Space>

      {mode === 'rich' ? (
        <div
          style={{
            border: '1px solid #d9d9d9',
            borderRadius: 6,
            padding: '8px 12px',
            background: '#fff',
          }}
          data-testid="rich-editor"
        >
          <EditorContent editor={editor} />
        </div>
      ) : (
        <textarea
          value={value}
          onChange={(e) => {
            lastEmitted.current = e.target.value
            onChange(e.target.value)
            editor.commands.setContent(markdownToHtml(e.target.value), { emitUpdate: false })
          }}
          spellCheck={false}
          style={{
            width: '100%',
            minHeight: minRows * 24,
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
            lineHeight: 1.7,
            padding: 8,
            border: '1px solid #d9d9d9',
            borderRadius: 6,
          }}
        />
      )}
    </div>
  )
}
