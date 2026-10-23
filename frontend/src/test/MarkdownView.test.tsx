import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { MarkdownView } from '../components/MarkdownView'

describe('MarkdownView', () => {
  it('escapes raw HTML instead of inserting executable elements', () => {
    const { container } = render(
      <MarkdownView
        source={[
          '<img src=x onerror="window.__forgeXss = 1">',
          '<script>window.__forgeXss = 2</script>',
          '<strong>not markdown html</strong>',
          '',
          '**safe markdown**',
        ].join('\n')}
      />
    )

    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('strong')).not.toBeNull()
    expect(container.querySelector('strong')).toHaveTextContent('safe markdown')
    expect(container.textContent).toContain('<img src=x onerror=')
    expect(container.textContent).toContain('window.__forgeXss = 1')
    expect(container.textContent).toContain('<script>window.__forgeXss = 2</script>')
  })

  it('opens external links safely while leaving internal links in place', () => {
    const { container } = render(
      <MarkdownView source={'[external](https://example.com/docs) [internal](/docs)'} />
    )
    const links = container.querySelectorAll('a')

    expect(links).toHaveLength(2)
    expect(links[0]).toHaveAttribute('href', 'https://example.com/docs')
    expect(links[0]).toHaveAttribute('target', '_blank')
    expect(links[0]).toHaveAttribute('rel', 'noopener noreferrer')
    expect(links[1]).toHaveAttribute('href', '/docs')
    expect(links[1]).not.toHaveAttribute('target')
  })

  it('renders inline and display math without exposing KaTeX markup as text', () => {
    const { container } = render(
      <MarkdownView
        source={['已知 $x^2 + y^2 = z^2$。', '', '$$', '\\frac{1}{2}', '$$'].join('\n')}
      />
    )

    expect(container.querySelectorAll('.katex')).toHaveLength(2)
    expect(container.textContent).not.toContain('$x^2')
    expect(container.textContent).not.toContain('<span class="katex"')
  })
})
