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
      desktopExited: {},
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

  it('autosaves the draft as a code version after the debounce window', async () => {
    const user = userEvent.setup()
    const saveSpy = vi.fn((_info: { request: Request }) =>
      HttpResponse.json({
        id: 'ver-1',
        contestId: 'contest-1',
        problemId: 'answer-problem-1',
        userId: 'user-1',
        seq: 7,
        language: 'answer',
        code: '1/2',
        source: 'save',
        createdAt: new Date().toISOString(),
      })
    )
    installStudentRoutes()
    server.use(
      apiRoutes.post(
        '/contests/contest-1/problems/answer-problem-1/versions',
        saveSpy
      )
    )
    renderWorkspace()

    const answer = await screen.findByRole('textbox', { name: '答案内容' })
    await user.type(answer, '1/2')
    // 2.5s 防抖到期后自动保存一个版本。
    await waitFor(
      () => expect(saveSpy).toHaveBeenCalledTimes(1),
      { timeout: 6000 }
    )
    const body = await saveSpy.mock.calls[0][0].request.clone().json()
    expect(body).toMatchObject({ code: '1/2', source: 'save' })
    expect(await screen.findByText(/已自动保存 v7/)).toBeInTheDocument()
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

  it('hides ranking and shows masked verdicts during a NOI contest', async () => {
    const noiContest: Contest = { ...contest, mode: 'noi' }
    const rankingSpy = vi.fn(() => HttpResponse.json([]))
    server.use(
      apiRoutes.get('/time', () =>
        HttpResponse.json({ serverTime: new Date().toISOString() })
      ),
      apiRoutes.get('/contests/contest-1', () => HttpResponse.json(noiContest)),
      apiRoutes.get('/contests/contest-1/problems', () =>
        HttpResponse.json([answerProblem])
      ),
      apiRoutes.get('/contests/contest-1/submissions', () =>
        HttpResponse.json([{ ...acceptedSubmission, verdict: 'submitted', score: 0 }])
      ),
      apiRoutes.get('/contests/contest-1/ranking', rankingSpy),
      apiRoutes.get('/contests/contest-1/messages', () => HttpResponse.json([]))
    )
    // NOI 默认自动进入仿真桌面；本用例验证的是网页外壳的脱敏行为，
    // 以「学生已手动退出过桌面」的会话状态进入。
    useStore.setState({ desktopExited: { 'contest-1': true } })
    renderWorkspace()

    // NOI 横幅提示赛中不公布结果。
    expect(
      await screen.findByText(/NOI 全真模拟进行中/)
    ).toBeInTheDocument()
    // 切到提交面板，确认只显示脱敏判定。
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /提交 1/ }))
    expect(await screen.findByText(/已提交 · 赛后公布/)).toBeInTheDocument()
    // 榜单请求根本不发（启停门控），切换到榜单页只看到赛后公布提示。
    await user.click(screen.getByRole('button', { name: /排名/ }))
    expect(
      await screen.findByText(/榜单将在比赛结束后统一公布/)
    ).toBeInTheDocument()
    expect(rankingSpy).not.toHaveBeenCalled()
  })

  it('keeps the branded workspace with stat cards for online contests', async () => {
    installStudentRoutes()
    renderWorkspace()

    expect((await screen.findAllByText('最终答案')).length).toBeGreaterThan(0)
    expect(screen.getByText('参赛者')).toBeInTheDocument()
    expect(screen.queryByText(/NOI 全真模拟进行中/)).not.toBeInTheDocument()
  })

  it('switches to the plain exam skin for NOI contests', async () => {
    const noiContest: Contest = { ...contest, mode: 'noi' }
    server.use(
      apiRoutes.get('/time', () =>
        HttpResponse.json({ serverTime: new Date().toISOString() })
      ),
      apiRoutes.get('/contests/contest-1', () => HttpResponse.json(noiContest)),
      apiRoutes.get('/contests/contest-1/problems', () =>
        HttpResponse.json([answerProblem])
      ),
      apiRoutes.get('/contests/contest-1/submissions', () => HttpResponse.json([])),
      apiRoutes.get('/contests/contest-1/ranking', () => HttpResponse.json([])),
      apiRoutes.get('/contests/contest-1/messages', () => HttpResponse.json([]))
    )
    useStore.setState({ desktopExited: { 'contest-1': true } })
    renderWorkspace()

    // 考场皮肤：统计卡整体隐藏（display:none），只留倒计时/题目/提交/通告。
    expect(await screen.findByText(/NOI 全真模拟进行中/)).toBeInTheDocument()
    const statGrid = screen.getByText('参赛者').closest('div.rounded-xl')?.parentElement
    expect(statGrid).toHaveClass('hidden')
  })

  it('auto-enters the NOI Linux desktop sim once per visit', async () => {
    const noiContest: Contest = { ...contest, mode: 'noi' }
    server.use(
      apiRoutes.get('/time', () =>
        HttpResponse.json({ serverTime: new Date().toISOString() })
      ),
      apiRoutes.get('/contests/contest-1', () => HttpResponse.json(noiContest)),
      apiRoutes.get('/contests/contest-1/problems', () =>
        HttpResponse.json([answerProblem])
      ),
      apiRoutes.get('/contests/contest-1/submissions', () => HttpResponse.json([])),
      apiRoutes.get('/contests/contest-1/ranking', () => HttpResponse.json([])),
      apiRoutes.get('/contests/contest-1/messages', () => HttpResponse.json([]))
    )
    renderWorkspace()

    // NOI 作答模式 = 仿真桌面：比赛就绪后自动进入，无需学生手动切换。
    // 进入后整页只剩仿真桌面（懒加载占位），断言会话状态即可。
    await waitFor(() => expect(useStore.getState().desktopMode).toBe(true))
  })
})
