import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api, type Contest } from '../api/client'
import { useStore } from '../store/useStore'
import { StudentLobby } from '../views/StudentLobby'

const contests: Contest[] = [
  {
    id: 'contest-running',
    name: 'Autumn Sprint',
    description: 'Live programming contest',
    status: 'running',
    rankingMode: 'oi',
    problemIds: ['p1', 'p2'],
    participantUserIds: ['u1'],
    bindIpEnabled: true,
  },
  {
    id: 'contest-draft',
    name: 'Winter Preview',
    description: 'Draft contest',
    status: 'draft',
    rankingMode: 'acm',
  },
  {
    id: 'contest-finished',
    name: 'Summer Archive',
    description: 'Finished contest',
    status: 'finished',
  },
]

function renderLobby(initialEntries = ['/']) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={initialEntries}>
        <Routes>
          <Route path="/" element={<StudentLobby />} />
          <Route path="/contest/:contestId" element={<div>contest route</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('StudentLobby', () => {
  beforeEach(() => {
    useStore.setState({
      session: {
        userId: 'u1',
        username: 'student',
        name: 'Candidate',
        role: 'student',
      },
      authStatus: 'authenticated',
    })
  })

  it('filters and searches contests, then opens the selected contest', async () => {
    const user = userEvent.setup()
    vi.spyOn(api, 'contests').mockResolvedValue(contests)
    renderLobby()

    expect(await screen.findAllByTestId('contest-card')).toHaveLength(3)
    expect(screen.getByRole('heading', { name: /Candidate/ })).toBeInTheDocument()
    expect(screen.getByText('机位绑定')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '进行中' }))
    expect(screen.getAllByTestId('contest-card')).toHaveLength(1)
    expect(screen.getByText('Autumn Sprint')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '全部' }))
    await user.type(screen.getByPlaceholderText('搜索比赛名称或说明'), 'archive')
    expect(screen.getAllByTestId('contest-card')).toHaveLength(1)
    expect(screen.getByText('Summer Archive')).toBeInTheDocument()

    await user.click(screen.getByTestId('enter-contest'))
    expect(await screen.findByText('contest route')).toBeInTheDocument()
  })

  it('shows an empty state when no contest has been published', async () => {
    vi.spyOn(api, 'contests').mockResolvedValue([])
    renderLobby()

    expect(await screen.findByText('暂时还没有比赛')).toBeInTheDocument()
    expect(screen.getByText(/教师发布比赛后/)).toBeInTheDocument()
  })

  it('retries a failed contest query without a page reload', async () => {
    const user = userEvent.setup()
    vi.spyOn(api, 'contests')
      .mockRejectedValueOnce(new Error('lobby unavailable'))
      .mockResolvedValue([contests[0]])
    renderLobby()

    expect(await screen.findByText('lobby unavailable')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '重新加载' }))
    await waitFor(() => expect(screen.getAllByTestId('contest-card')).toHaveLength(1))
  })
})
