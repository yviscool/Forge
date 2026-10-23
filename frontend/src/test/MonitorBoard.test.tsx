import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, expect, it, vi } from 'vitest'
import type { MonitorSnapshot } from '../api/client'
import { MonitorBoard } from '../components/MonitorBoard'
import { apiRoutes, HttpResponse, server } from './msw'

const snapshot: MonitorSnapshot = {
  contestId: 'contest-1',
  name: '监考赛',
  mode: 'noi',
  status: 'running',
  generatedAt: new Date().toISOString(),
  participants: [
    {
      userId: 'u1',
      name: '张三',
      username: 'zhangsan',
      examTicket: 'HA-J00001',
      ip: '192.168.1.20',
      ipConflict: true,
      online: true,
      submissionCount: 3,
      problems: [
        {
          problemId: 'p1',
          code: 'A',
          title: '题一',
          attempts: 2,
          lastVerdict: 'accepted',
          lastScore: 100,
        },
        {
          problemId: 'p2',
          code: 'B',
          title: '题二',
          attempts: 1,
          lastVerdict: 'wrong_answer',
          lastScore: 0,
        },
      ],
      lastVersionProblemId: 'p1',
      lastVersionSeq: 5,
      lastVersionSource: 'save',
      lastVersionLines: 42,
      lastVersionAt: new Date().toISOString(),
      lastActivityAt: new Date().toISOString(),
    },
    {
      userId: 'u2',
      name: '李四',
      username: 'lisi',
      online: false,
      submissionCount: 0,
      problems: [],
    },
  ],
}

function renderBoard() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MonitorBoard cid="contest-1" />
    </QueryClientProvider>
  )
}

describe('MonitorBoard', () => {
  it('renders participant cards with online state, IP conflict and edit activity', async () => {
    server.use(
      apiRoutes.get('/contests/contest-1/monitor', () => HttpResponse.json(snapshot))
    )
    renderBoard()

    // 张三：在线 + 机位冲突 + 编辑动态。
    expect(await screen.findByText('HA-J00001 · 张三')).toBeInTheDocument()
    expect(screen.getByText('在线')).toBeInTheDocument()
    expect(screen.getByText('离线')).toBeInTheDocument()
    expect(screen.getAllByText('机位冲突')).toHaveLength(1)
    expect(screen.getByText('192.168.1.20')).toBeInTheDocument()
    expect(screen.getByText(/正在编辑 p1 v5/)).toBeInTheDocument()
    // 每题作答动态：A×2（AC）、B×1（WA）。
    expect(screen.getByTitle(/A\. 题一：2 次提交，最近 accepted/)).toBeInTheDocument()
    expect(screen.getByTitle(/B\. 题二：1 次提交，最近 wrong_answer/)).toBeInTheDocument()
    expect(screen.getByText('NOI 全真模拟')).toBeInTheDocument()
  })

  it('shows the empty state when no candidate is authorized', async () => {
    server.use(
      apiRoutes.get('/contests/contest-1/monitor', () =>
        HttpResponse.json({ ...snapshot, participants: [] })
      )
    )
    renderBoard()
    expect(await screen.findByText(/还没有授权参赛选手/)).toBeInTheDocument()
  })

  it('shows a retry card when the snapshot fails', async () => {
    const retry = vi.fn()
    server.use(
      apiRoutes.get('/contests/contest-1/monitor', () =>
        HttpResponse.json({ message: 'boom' }, { status: 500 })
      )
    )
    renderBoard()
    expect(await screen.findByText(/监考快照加载失败/)).toBeInTheDocument()
    await screen.findByRole('button', { name: '重试' })
    void retry
  })
})
