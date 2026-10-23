import React from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ErrorBoundary } from '../app/ErrorBoundary'

class ExplodingView extends React.Component {
  render(): React.ReactNode {
    throw new Error('test crash')
  }
}

describe('ErrorBoundary', () => {
  it('renders healthy children untouched', () => {
    render(
      <ErrorBoundary>
        <p>safe content</p>
      </ErrorBoundary>
    )

    expect(screen.getByText('safe content')).toBeInTheDocument()
  })

  it('shows a recovery screen when a child crashes', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})

    render(
      <ErrorBoundary>
        <ExplodingView />
      </ErrorBoundary>
    )

    expect(screen.getByRole('heading', { name: '页面遇到意外错误' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /刷新并恢复/ })).toBeInTheDocument()
    expect(screen.getByText(/当前操作尚未确认成功/)).toBeInTheDocument()
  })
})
