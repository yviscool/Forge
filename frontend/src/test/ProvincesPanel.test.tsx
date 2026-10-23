import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { ProvinceView } from '../api/client'
import { ProvincesPanel } from '../components/ProvincesPanel'
import { ToastProvider } from '../components/ui/toast'
import { apiRoutes, jsonOk, server } from './msw'

function provinces(): ProvinceView[] {
  return [
    { code: 'BJ', name: '北京', sort: 10, enabled: true, users: 0, hasUsers: false },
    { code: 'HA', name: '河南', sort: 170, enabled: true, users: 129, hasUsers: true },
    { code: 'TW', name: '台湾', sort: 340, enabled: false, users: 0, hasUsers: false },
  ]
}

function renderPanel(overrides: Partial<Parameters<typeof ProvincesPanel>[0]> = {}) {
  const refetch = vi.fn()
  render(
    <ToastProvider>
      <ProvincesPanel
        provinces={provinces()}
        loading={false}
        error={undefined}
        refetch={refetch}
        isAdmin
        {...overrides}
      />
    </ToastProvider>
  )
  return { refetch }
}

describe('ProvincesPanel', () => {
  it('renders the builtin table with codes, user counts and status', () => {
    renderPanel()

    expect(screen.getByText('编号')).toBeInTheDocument()
    expect(screen.getByText('用户数')).toBeInTheDocument()
    expect(screen.getByText('HA')).toBeInTheDocument()
    expect(screen.getByText('河南')).toBeInTheDocument()
    expect(screen.getByText('129')).toBeInTheDocument()
    expect(screen.getByText('台湾')).toBeInTheDocument()
    // 「停用」×3：TW 的状态徽标、HA 与 TW 行的操作按钮（启用行的按钮文案也是「停用」）。
    expect(screen.getAllByText('停用')).toHaveLength(3)
    expect(screen.getByLabelText('启用省份台湾')).toBeInTheDocument()
  })

  it('hides the action column for non-admin viewers', () => {
    renderPanel({ isAdmin: false })

    expect(screen.queryByText('操作')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('停用省份河南')).not.toBeInTheDocument()
  })

  it('toggles a province through the API and refetches', async () => {
    const user = userEvent.setup()
    const putSpy = vi.fn(() => jsonOk({ code: 'HA', name: '河南', sort: 170, enabled: false, users: 129, hasUsers: true }))
    server.use(apiRoutes.put('/provinces/:code', putSpy))
    const { refetch } = renderPanel()

    await user.click(screen.getByLabelText('停用省份河南'))

    await waitFor(() => expect(refetch).toHaveBeenCalled())
    expect(putSpy).toHaveBeenCalled()
  })

  it('shows a retry card when the query failed', async () => {
    const user = userEvent.setup()
    const refetch = vi.fn()
    renderPanel({ error: new Error('boom'), refetch })

    await user.click(screen.getByRole('button', { name: '重试' }))
    expect(refetch).toHaveBeenCalled()
  })
})
