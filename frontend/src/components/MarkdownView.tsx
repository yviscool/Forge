import React, { useMemo } from 'react'
import MarkdownIt from 'markdown-it'
import katex from 'katex'

interface MarkdownViewProps {
  source?: string
  className?: string
}

const escapeHtml = (value: string) =>
  value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')

function renderMath(source: string): string {
  try {
    return katex.renderToString(source, {
      displayMode: false,
      throwOnError: false,
      strict: 'ignore',
      trust: false,
    })
  } catch {
    return `<code>${escapeHtml(source)}</code>`
  }
}

const md = new MarkdownIt({
  // Problem statements are teacher-authored but can be imported. Disable raw
  // HTML so dangerouslySetInnerHTML only receives Markdown/Katex output.
  html: false,
  linkify: true,
  typographer: true,
  breaks: true,
})

// Math is parsed into tokens before rendering. Injecting KaTeX HTML into the
// Markdown source would be escaped by `html: false` and expose formulas as
// literal tags.
md.block.ruler.before(
  'fence',
  'math_block',
  (state, startLine, endLine, silent) => {
    const start = state.bMarks[startLine] + state.tShift[startLine]
    const lineEnd = state.eMarks[startLine]
    if (start + 2 > lineEnd || state.src.slice(start, start + 2) !== '$$') return false

    const firstLine = state.src.slice(start + 2, lineEnd)
    const sameLineClose = firstLine.indexOf('$$')
    let content: string
    let nextLine = startLine + 1

    if (sameLineClose >= 0) {
      if (firstLine.slice(sameLineClose + 2).trim()) return false
      content = firstLine.slice(0, sameLineClose)
    } else {
      const lines = [firstLine]
      let closed = false
      while (nextLine < endLine) {
        const nextStart = state.bMarks[nextLine] + state.tShift[nextLine]
        const nextEnd = state.eMarks[nextLine]
        const line = state.src.slice(nextStart, nextEnd)
        const close = line.indexOf('$$')
        if (close >= 0) {
          if (line.slice(close + 2).trim()) return false
          const before = line.slice(0, close)
          if (before) lines.push(before)
          nextLine += 1
          closed = true
          break
        }
        lines.push(line)
        nextLine += 1
      }
      if (!closed) return false
      content = lines.join('\n')
    }

    state.line = nextLine
    if (!silent) {
      const token = state.push('math_block', 'math', 0)
      token.block = true
      token.content = content.trim()
      token.map = [startLine, nextLine]
    }
    return true
  },
  { alt: ['paragraph', 'reference', 'blockquote', 'list'] }
)

md.inline.ruler.after('escape', 'math_inline', (state, silent) => {
  const start = state.pos
  if (
    state.src.charCodeAt(start) !== 0x24 ||
    start + 1 >= state.posMax ||
    state.src.charCodeAt(start + 1) === 0x24 ||
    (start > 0 && state.src.charCodeAt(start - 1) === 0x5c)
  ) {
    return false
  }

  for (let end = start + 1; end < state.posMax; end += 1) {
    if (
      state.src.charCodeAt(end) !== 0x24 ||
      state.src.charCodeAt(end - 1) === 0x5c ||
      state.src.charCodeAt(end + 1) === 0x24
    ) {
      continue
    }
    if (end === start + 1 || /\s/.test(state.src[end - 1])) return false

    if (!silent) {
      const token = state.push('math_inline', 'math', 0)
      token.markup = '$'
      token.content = state.src.slice(start + 1, end).trim()
    }
    state.pos = end + 1
    return true
  }
  return false
})

md.renderer.rules.math_inline = (tokens, idx) => renderMath(tokens[idx].content)
md.renderer.rules.math_block = (tokens, idx) =>
  `${katex.renderToString(tokens[idx].content, {
    displayMode: true,
    throwOnError: false,
    strict: 'ignore',
    trust: false,
  })}\n`

const defaultLinkOpen =
  md.renderer.rules.link_open ||
  ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options))

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const target = tokens[idx].attrGet('href') || ''
  if (/^https?:\/\//i.test(target)) {
    // 桌面 WebView 内绝不开 _blank：会弹出系统 Edge 窗口；
    // 浏览器访问时保持新标签行为。
    let inDesktop = false
    try {
      inDesktop = localStorage.getItem('forge_desktop') === '1'
    } catch {
      /* ignore */
    }
    if (!inDesktop) tokens[idx].attrSet('target', '_blank')
    tokens[idx].attrSet('rel', 'noopener noreferrer')
  }
  return defaultLinkOpen(tokens, idx, options, env, self)
}

export function MarkdownView({ source = '', className = '' }: MarkdownViewProps) {
  const html = useMemo(() => {
    if (!source) return ''
    return md.render(source)
  }, [source])

  return (
    <div
      className={`prose-forge max-w-none text-sm leading-7 text-slate-700 ${className}`}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}
