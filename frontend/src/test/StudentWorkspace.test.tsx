import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api, type Contest, type Problem, type Submission } from '../api/client'
import { ToastProvider } from '../components/ui/toast'
import { useStore } from '../store/useStore'
import { StudentWorkspace } from '../views/StudentWorkspace'
import { apiRoutes, HttpResponse, server } from './msw'

vi.mock('../realtime/useSSE', () => ({
  useSSE: vi.fn(),
}))

const contest: Contest = {
  id: 'contest-1',
  name: '答案题模拟赛',
  description: '只交答案题测试',
  status: 'running',
  rankingMode: 'oi',
}

const answerProblem: Problem = {
  id: 'answer-problem-1',
  contestId: contest.id,
  code: 'A',
  title: '最终答案',
  statement: '提交你的最终答案。',
  input: '',
  output: '',
  examples: '',
  constraints: '',
  timeLimitMs: 1000,
  memoryLimitMib: 256,
  taskType: 'answers_only',
  testCases: [
    {
      id: 'answer',
      inputFile: '',
      outputFile: 'answer',
      score: 100,
      subtask: 0,
    },
  ],
}

const acceptedSubmission: Submission = {
  id: 'submission-1',
  contestId: contest.id,
  problemID: answerProblem.id,
  userId: 'user-1',
  userName: 'Alice',
  language: 'answer',
  code: '1/2',
  verdict: 'accepted',
  score: 100,
  submittedAt: '2026-10-21T08:00:00.000Z',
}

function installStudentRoutes() {
  server.use(
    apiRoutes.get('/time', () =>
      HttpResponse.json({ serverTime: new Date().toISOString() })
    ),
    apiRoutes.get('/contests/contest-1', () => HttpResponse.json(contest)),
    apiRoutes.get('/contests/contest-1/problems', () =>
      HttpResponse.json([answerProblem])
    ),
    apiRoutes.get('/contests/contest-1/submissions', () =>
      HttpResponse.json([])
    ),
    apiRoutes.get('/contests/contest-1/ranking', () => HttpResponse.json([])),
    apiRoutes.get('/contests/contest-1/messages', () => HttpResponse.json([]))
  )
}

function renderWorkspace() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <MemoryRouter initialEntries={['/contests/contest-1']}>
          <Routes>
            <Route path="/contests/:cid" element={<StudentWorkspace />} />
          </Routes>
        </MemoryRouter>
      </ToastProvider>
    </QueryClientProvider>
  )
}

describe('StudentWorkspace answer-only workflow', () => {
  beforeEach(() => {
    useStore.setState({
      session: {
        userId: 'user-1',
        username: 'alice',
        name: 'Alice',
        role: 'student',
      },
      authStatus: 'authenticated',
      drafts: {},
      desktopMode: false,
      unlockedContests: {},
    })
  })

  it('renders a plain answer editor instead of the code language picker', async () => {
    installStudentRoutes()
    renderWorkspace()

    const answer = await screen.findByRole('textbox', { name: '答案内容' })
    expect(answer).toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: '选择提交语言' })).not.toBeInTheDocument()
    expect(screen.queryByText('纯文本答案 · 不编译')).toBeInTheDocument()
  })

  it('loads answer files verbatim and submits them as an answer language', async () => {
    const user = userEvent.setup()
    installStudentRoutes()
    const submit = vi.spyOn(api, 'submit').mockResolvedValue(acceptedSubmission)
    renderWorkspace()

    const answer = await screen.findByRole('textbox', { name: '答案内容' })
    const input = document.querySelector(
      'input[type="file"][accept*=".ans"]'
    ) as HTMLInputElement
    expect(input).not.toBeNull()

    const file = new File(['1/2\n'], 'answer.ans', { type: 'text/plain' })
    await user.upload(input, file)

    await waitFor(() => expect(answer).toHaveValue('1/2\n'))
    await user.click(screen.getAllByRole('button', { name: '提交答案' })[0])

    await waitFor(() =>
      expect(submit).toHaveBeenCalledWith(contest.id, {
        problemID: answerProblem.id,
        language: 'answer',
        code: '1/2\n',
      }, expect.any(String))
    )
  })

  it('reuses an idempotency key when retrying the same failed submission', async () => {
    const user = userEvent.setup()
    installStudentRoutes()
    const submit = vi
      .spyOn(api, 'submit')
      .mockRejectedValueOnce(new Error('network timeout'))
      .mockResolvedValueOnce(acceptedSubmission)
    renderWorkspace()

    const answer = await screen.findByRole('textbox', { name: '答案内容' })
    await user.type(answer, '1/2')
    const submitButton = screen.getAllByRole('button', { name: '提交答案' })[0]

    await user.click(submitButton)
    await waitFor(() => expect(submit).toHaveBeenCalledTimes(1))
    await user.click(submitButton)
    await waitFor(() => expect(submit).toHaveBeenCalledTimes(2))

    expect(submit.mock.calls[0][2]).toEqual(expect.any(String))
    expect(submit.mock.calls[1][2]).toBe(submit.mock.calls[0][2])
  })
})
