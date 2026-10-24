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
    // 跨区导航现在是整页跳转（桌面壳内为窗口指令）：stub 掉避免 jsdom 真跳转。
    vi.stubGlobal('location', { ...window.location, assign: vi.fn() })
    localStorage.removeItem('forge_desktop')
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
    // 跨区导航在非桌面浏览器里以整页跳转实现（桌面壳内为原生窗口指令）。
    const teacherLinks = screen.getAllByRole('button', { name: '教师工作台' })
    await user.click(teacherLinks[0])
    expect(window.location.assign).toHaveBeenCalledWith('/teacher')
    // 教师视角下顶部仍提供切换入口（按钮形态）。
    expect(screen.getAllByRole('button', { name: /选手视角|教师工作台/ }).length).toBeGreaterThan(0)

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
