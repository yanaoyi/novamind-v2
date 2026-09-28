/**
 * 一个「够用就好」的 Markdown ↔ HTML 转换器。
 *
 * 为什么自己写：正文要求以 Markdown 存储（可导出、可 diff、AI 能直接读），
 * 而编辑器要所见即所得。Tiptap 官方把 Markdown 支持放在社区包里，与其再引一个依赖，
 * 不如实现一个覆盖实际写作子集的转换器（标题 / 加粗 / 斜体 / 引用 / 列表 / 分割线 / 代码）。
 * 子集之外的写法按纯文本处理，绝不丢字。
 */

const ESCAPE_MAP: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
}

export function escapeHtml(text: string): string {
  return text.replace(/[&<>]/g, (c) => ESCAPE_MAP[c] ?? c)
}

/** 行内 **加粗** / *斜体* / `代码` → HTML */
function inlineToHtml(text: string): string {
  let out = escapeHtml(text)
  out = out.replace(/`([^`]+)`/g, '<code>$1</code>')
  out = out.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  out = out.replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>')
  return out
}

/** Markdown 文本 → 编辑器 HTML */
export function markdownToHtml(markdown: string): string {
  const lines = (markdown ?? '').replace(/\r\n?/g, '\n').split('\n')
  const html: string[] = []
  let paragraph: string[] = []
  let listType: 'ul' | 'ol' | null = null
  let quote: string[] = []

  const flushParagraph = () => {
    if (paragraph.length === 0) return
    html.push(`<p>${paragraph.map(inlineToHtml).join('<br>')}</p>`)
    paragraph = []
  }
  const flushList = () => {
    if (listType) {
      html.push(`</${listType}>`)
      listType = null
    }
  }
  const flushQuote = () => {
    if (quote.length === 0) return
    html.push(`<blockquote><p>${quote.map(inlineToHtml).join('<br>')}</p></blockquote>`)
    quote = []
  }
  const flushAll = () => {
    flushParagraph()
    flushList()
    flushQuote()
  }

  for (const raw of lines) {
    const line = raw.trimEnd()
    if (line.trim() === '') {
      flushAll()
      continue
    }
    const heading = /^(#{1,6})\s+(.*)$/.exec(line)
    if (heading) {
      flushAll()
      const level = heading[1].length
      html.push(`<h${level}>${inlineToHtml(heading[2].trim())}</h${level}>`)
      continue
    }
    if (/^([-*_])\1{2,}$/.test(line.trim())) {
      flushAll()
      html.push('<hr>')
      continue
    }
    const quoteLine = /^>\s?(.*)$/.exec(line)
    if (quoteLine) {
      flushParagraph()
      flushList()
      quote.push(quoteLine[1])
      continue
    }
    const bullet = /^\s*[-*+]\s+(.*)$/.exec(line)
    if (bullet) {
      flushParagraph()
      flushQuote()
      if (listType !== 'ul') {
        flushList()
        html.push('<ul>')
        listType = 'ul'
      }
      html.push(`<li><p>${inlineToHtml(bullet[1])}</p></li>`)
      continue
    }
    const ordered = /^\s*\d+[.)]\s+(.*)$/.exec(line)
    if (ordered) {
      flushParagraph()
      flushQuote()
      if (listType !== 'ol') {
        flushList()
        html.push('<ol>')
        listType = 'ol'
      }
      html.push(`<li><p>${inlineToHtml(ordered[1])}</p></li>`)
      continue
    }
    flushList()
    flushQuote()
    paragraph.push(line.trim())
  }
  flushAll()
  return html.join('\n')
}

/** 编辑器 HTML → Markdown 文本 */
export function htmlToMarkdown(html: string): string {
  if (typeof DOMParser === 'undefined') {
    return html.replace(/<[^>]+>/g, '')
  }
  const doc = new DOMParser().parseFromString(`<div id="root">${html}</div>`, 'text/html')
  const root = doc.getElementById('root')
  if (!root) return ''
  const blocks: string[] = []

  const inline = (node: Node): string => {
    if (node.nodeType === Node.TEXT_NODE) {
      return (node.textContent ?? '').replace(/([*_`])/g, '\\$1')
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return ''
    const el = node as HTMLElement
    const inner = Array.from(el.childNodes).map(inline).join('')
    switch (el.tagName.toLowerCase()) {
      case 'strong':
      case 'b':
        return `**${inner}**`
      case 'em':
      case 'i':
        return `*${inner}*`
      case 'code':
        return `\`${el.textContent ?? ''}\``
      case 'br':
        return '\n'
      case 'a':
        return `[${inner}](${el.getAttribute('href') ?? ''})`
      default:
        return inner
    }
  }

  const walk = (node: Node): void => {
    if (node.nodeType === Node.TEXT_NODE) {
      const text = (node.textContent ?? '').trim()
      if (text) blocks.push(text)
      return
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return
    const el = node as HTMLElement
    switch (el.tagName.toLowerCase()) {
      case 'h1':
      case 'h2':
      case 'h3':
      case 'h4':
      case 'h5':
      case 'h6': {
        const level = Number(el.tagName[1])
        blocks.push(`${'#'.repeat(level)} ${inline(el).trim()}`)
        return
      }
      case 'p': {
        const text = inline(el).trim()
        if (text) blocks.push(text)
        return
      }
      case 'blockquote': {
        const inner: string[] = []
        Array.from(el.childNodes).forEach((child) => {
          if (child.nodeType === Node.ELEMENT_NODE && (child as HTMLElement).tagName.toLowerCase() === 'p') {
            const t = inline(child).trim()
            if (t) inner.push(t)
            return
          }
          const t = inline(child).trim()
          if (t) inner.push(t)
        })
        if (inner.length) blocks.push(inner.map((l) => `> ${l}`).join('\n'))
        return
      }
      case 'ul':
      case 'ol': {
        const tag = el.tagName.toLowerCase()
        const items = Array.from(el.children).filter((c) => c.tagName.toLowerCase() === 'li')
        items.forEach((li, index) => {
          const marker = tag === 'ol' ? `${index + 1}. ` : '- '
          blocks.push(marker + inline(li).trim().replace(/\n+/g, ' '))
        })
        return
      }
      case 'hr':
        blocks.push('---')
        return
      case 'pre': {
        blocks.push('```\n' + (el.textContent ?? '').replace(/\n$/, '') + '\n```')
        return
      }
      default:
        Array.from(el.childNodes).forEach((child) => walk(child))
    }
  }

  Array.from(root.childNodes).forEach((child) => walk(child))
  return blocks
    .filter((b) => b.trim() !== '')
    .join('\n\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}
