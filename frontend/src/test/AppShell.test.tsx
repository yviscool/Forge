import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../api/client'
import { AppShell } from '../app/AppShell'
import { ToastProvider } from '../components/ui/toast'
import { useStore } from '../store/useStore'

function LocationProbe() {
  const location = useLocation()
  return <span data-testid="current-path">{location.pathname}</span>
}

function renderShell(initialPath = '/', role = 'teacher') {
  useStore.setState({
    session: {
      userId: 'user-1',
      username: 'teacher',
      name: 'Teacher One',
      role,
    },
    authStatus: 'authenticated',
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[initialPath]}>
        <ToastProvider>
          <AppShell>
            <LocationProbe />
          </AppShell>
        </ToastProvider>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('AppShell', () => {
  beforeEach(() => {
    useStore.setState({
      session: null,
      authStatus: 'idle',
      activeAlert: null,
    })
  })

  it('navigates educator areas and opens the account security dialog', async () => {
    const user = userEvent.setup()
    renderShell()

    expect(screen.getByRole('navigation', { name: '主导航' })).toBeInTheDocument()
    expect(screen.getByText('Teacher One')).toBeInTheDocument()
    // 导航项与移动端图标都可到达教师台，取第一个即可。
    await user.click(screen.getAllByRole('link', { name: '教师工作台' })[0])
    expect(screen.getByTestId('current-path')).toHaveTextContent('/teacher')
    // 选手大厅与教师工作台在分段导航中互为回路，替代旧的「选手视角」快捷键。
    expect(screen.getByRole('link', { name: '选手大厅' })).toBeInTheDocument()

    await user.click(screen.getByTestId('change-password-button'))
    const dialog = screen.getByRole('dialog', { name: '修改登录密码' })
    expect(dialog).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '取消' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('logs out locally even when session revocation fails', async () => {
    const user = userEvent.setup()
    const logout = vi.spyOn(api, 'logout').mockRejectedValue(new Error('network down'))
    renderShell()

    await user.click(screen.getByTestId('logout-button'))
    await waitFor(() => expect(logout).toHaveBeenCalledOnce())
    await waitFor(() => expect(useStore.getState().session).toBeNull())
    expect(useStore.getState().authStatus).toBe('anonymous')
  })

  it('does not expose educator navigation to student accounts', () => {
    renderShell('/', 'student')
    expect(screen.queryByRole('link', { name: '教师工作台' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /教师工作台/ })).not.toBeInTheDocument()
    expect(screen.getByText('选手')).toBeInTheDocument()
  })
})
