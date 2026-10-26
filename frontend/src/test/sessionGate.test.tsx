import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, type User } from '../api/client'
import { RequireEducator, RequireSession } from '../app/ProtectedRoute'
import { SessionGate } from '../app/SessionGate'
import { LoginPage } from '../pages/LoginPage'
import { ToastProvider } from '../components/ui/toast'
import { useStore } from '../store/useStore'

const student: User = {
  id: 'user-1',
  username: 'alice',
  name: 'Alice',
  role: 'student',
}

const teacher: User = {
  id: 'teacher-1',
  username: 'prof',
  name: 'Professor',
  role: 'teacher',
}

function renderApp(initialPath = '/') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <MemoryRouter initialEntries={[initialPath]}>
          <SessionGate>
            <Routes>
              <Route path="/login" element={<LoginPage />} />
              <Route
                path="/"
                element={
                  <RequireSession>
                    <div>student-lobby</div>
                  </RequireSession>
                }
              />
              <Route
                path="/teacher"
                element={
                  <RequireEducator>
                    <div>teacher-console</div>
                  </RequireEducator>
                }
              />
            </Routes>
          </SessionGate>
        </MemoryRouter>
      </ToastProvider>
    </QueryClientProvider>
  )
}

describe('session gate and route protection', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
    useStore.setState({
      session: null,
      authStatus: 'idle',
      serverTimeOffset: 0,
      activeAlert: null,
      unreadBroadcasts: 0,
    })
  })

  it('redirects anonymous visitors to the login page', async () => {
    vi.spyOn(api, 'me').mockRejectedValue(
      new ApiError('login required', 401, 'unauthorized')
    )
    renderApp('/')
    await waitFor(() => expect(screen.getByText('登录工作台')).toBeInTheDocument())
  })

  it('commits a verified session on login and returns to the original page', async () => {
    const user = userEvent.setup()
    vi.spyOn(api, 'login').mockResolvedValue({
      userId: student.id,
      role: 'student',
      username: student.username,
      name: student.name,
    })
    vi.spyOn(api, 'me')
      .mockRejectedValueOnce(new ApiError('login required', 401, 'unauthorized'))
      .mockResolvedValue(student)

    renderApp('/')
    await user.type(await screen.findByLabelText(/账号/), 'Alice')
    await user.type(screen.getByLabelText(/^密码/), 'secret')
    await user.click(screen.getByRole('button', { name: /进入 Forge/ }))

    expect(await screen.findByText('student-lobby')).toBeInTheDocument()
    expect(useStore.getState().session).toMatchObject({
      userId: 'user-1',
      name: 'Alice',
    })
    expect(api.me).toHaveBeenCalledWith()
    expect(localStorage.getItem('forge_session')).toBeNull()
  })

  it('shows a safe credential error for rejected logins without persisting a session', async () => {
    const user = userEvent.setup()
    vi.spyOn(api, 'me').mockRejectedValue(
      new ApiError('login required', 401, 'unauthorized')
    )
    vi.spyOn(api, 'login').mockRejectedValue(new Error('invalid login or password'))

    renderApp('/login')
    await user.type(screen.getByLabelText(/账号/), 'Alice')
    await user.type(screen.getByLabelText(/^密码/), 'wrong')
    await user.click(screen.getByRole('button', { name: /进入 Forge/ }))

    expect(await screen.findByRole('alert')).toHaveTextContent('invalid login or password')
    expect(useStore.getState().session).toBeNull()
    expect(localStorage.getItem('forge_session')).toBeNull()
  })

  it('treats a rejected HttpOnly cookie as anonymous', async () => {
    vi.spyOn(api, 'me').mockRejectedValue(
      new ApiError('session expired', 401, 'session_expired')
    )

    renderApp('/teacher')

    await waitFor(() => expect(useStore.getState().session).toBeNull())
    await waitFor(() =>
      expect(screen.getByText('登录工作台')).toBeInTheDocument()
    )
    expect(api.me).toHaveBeenCalled()
  })

  it('prevents a student from opening the educator console', async () => {
    useStore.setState({
      authStatus: 'authenticated',
      session: {
        userId: student.id,
        username: student.username,
        name: student.name,
        role: student.role,
      },
    })

    renderApp('/teacher')

    expect(await screen.findByText('student-lobby')).toBeInTheDocument()
    expect(screen.queryByText('teacher-console')).not.toBeInTheDocument()
  })

  it('allows an authenticated educator into the console', async () => {
    useStore.setState({
      authStatus: 'authenticated',
      session: {
        userId: teacher.id,
        username: teacher.username,
        name: teacher.name,
        role: teacher.role,
      },
    })

    renderApp('/teacher')

    expect(await screen.findByText('teacher-console')).toBeInTheDocument()
  })

  it('keeps the shell closed and offers retry when identity checks fail offline', async () => {
    vi.spyOn(api, 'me').mockRejectedValue(new Error('network down'))

    renderApp('/')

    expect(await screen.findByRole('alert')).toHaveTextContent(
      '暂时无法确认登录状态'
    )
    expect(screen.getByRole('button', { name: '重新连接' })).toBeInTheDocument()
    expect(screen.queryByText('student-lobby')).not.toBeInTheDocument()
  })
})
