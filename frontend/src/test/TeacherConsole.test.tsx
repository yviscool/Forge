import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Contest, Group, Problem, ProvinceView, User } from '../api/client'
import { ToastProvider } from '../components/ui/toast'
import { TeacherConsole } from '../views/TeacherConsole'
import { useStore } from '../store/useStore'
import type { PathParams } from 'msw'
import { apiRoutes, HttpResponse, server } from './msw'

const contest: Contest = {
  id: 'contest-1',
  name: 'CSP 模拟复赛',
  description: '考场说明',
  status: 'draft',
  rankingMode: 'oi',
  problemIds: ['problem-1'],
}

const problem: Problem = {
  id: 'problem-1',
  contestId: contest.id,
  code: 'A',
  title: '距离之和',
  statement: '求凸包周长。',
  input: 'n',
  output: 'answer',
  examples: '',
  constraints: 'n <= 1e5',
  timeLimitMs: 1000,
  memoryLimitMib: 512,
  testCases: [],
}

const users: User[] = [
  { id: 'u1', username: 's001', name: 'Alice', role: 'student' },
  { id: 'u2', username: 's002', name: 'Bob', role: 'student' },
]

const groups: Group[] = [{ id: 'g1', name: '高一竞赛A班', userIds: ['u1'] }]

const provinces: ProvinceView[] = [
  { code: 'BJ', name: '北京', sort: 10, enabled: true, users: 0, hasUsers: false },
  { code: 'HA', name: '河南', sort: 170, enabled: true, users: 1, hasUsers: true },
  { code: 'CQ', name: '重庆', sort: 40, enabled: false, users: 0, hasUsers: false },
]

function renderConsole() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <TeacherConsole />
      </ToastProvider>
    </QueryClientProvider>
  )
}

function installBaseline(
  inventory: { files: string[]; missingPairs: string[] },
  overrides: Partial<Record<'sync' | 'deleteFile' | 'deleteProblem', () => Response>> = {}
) {
  let currentInventory = inventory
  const calls = {
    sync: 0,
    deleteFile: [] as string[],
    deleteProblem: 0,
  }
  server.use(
    apiRoutes.get('/contests', () => HttpResponse.json([contest])),
    apiRoutes.get('/contests/contest-1/problems', () => HttpResponse.json([problem])),
    apiRoutes.get('/users', () => HttpResponse.json(users)),
    apiRoutes.get('/provinces', () => HttpResponse.json(provinces)),
    apiRoutes.get('/groups', () => HttpResponse.json(groups)),
    apiRoutes.get('/contests/contest-1/submissions', () => HttpResponse.json([])),
    apiRoutes.get('/contests/contest-1/ranking', () => HttpResponse.json([])),
    apiRoutes.get('/contests/contest-1/statistics', () =>
      HttpResponse.json({
        contestId: contest.id,
        users: 2,
        submissions: 7,
        problems: [
          {
            problemId: problem.id,
            code: problem.code,
            title: problem.title,
            submissions: 7,
            accepted: 2,
            acceptRate: 2 / 7,
            bestScore: 100,
            firstACUser: 'Alice',
          },
        ],
      })
    ),
    apiRoutes.get('/contests/contest-1/problems/problem-1/files', () =>
      HttpResponse.json(currentInventory)
    ),
    apiRoutes.post('/contests/contest-1/problems/problem-1/files/sync', () => {
      calls.sync += 1
      if (overrides.sync) return overrides.sync()
      currentInventory = { files: currentInventory.files, missingPairs: [] }
      return HttpResponse.json({ ...problem, testCases: [{ id: 'a01', inputFile: 'a01.in', outputFile: 'a01.out', score: 100, subtask: 0 }] })
    }),
    apiRoutes.delete(
      '/contests/contest-1/problems/problem-1/files/:name',
      ({ params }) => {
        calls.deleteFile.push(String(params.name))
        return HttpResponse.json({ ok: true })
      }
    ),
    apiRoutes.delete('/contests/contest-1/problems/problem-1', () => {
      calls.deleteProblem += 1
      if (overrides.deleteProblem) return overrides.deleteProblem()
      return HttpResponse.json({ ok: true })
    })
  )
  return calls
}

async function openProblemEditor(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('tab', { name: /试题工坊/ }))
  await user.click(await screen.findByRole('button', { name: /编辑题面/ }))
  expect(
    await screen.findByText(/(测试数据文件仓|只交答案题 · 标准答案)/)
  ).toBeInTheDocument()
}

describe('TeacherConsole test data workflow', () => {
  beforeEach(() => {
    localStorage.clear()
    useStore.setState({
      session: {
        userId: 'teacher-1',
        username: 'prof',
        name: 'Professor',
        role: 'teacher',
      },
      authStatus: 'authenticated',
      drafts: {},
    })
  })

  it('supports roving keyboard navigation across semantic tabs', async () => {
    installBaseline({ files: [], missingPairs: [] })
    renderConsole()

    const contestsTab = await screen.findByRole('tab', { name: '比赛概览' })
    expect(contestsTab).toHaveAttribute('aria-selected', 'true')
    contestsTab.focus()
    await userEvent.keyboard('{ArrowRight}')

    const usersTab = screen.getByRole('tab', { name: '用户与编组' })
    await waitFor(() => expect(usersTab).toHaveAttribute('aria-selected', 'true'))
    expect(usersTab).toHaveFocus()
    expect(screen.getByRole('tabpanel', { name: '用户与编组' })).toBeInTheDocument()
  })

  it('shows the province table read-only for teachers and edits user province', async () => {
    const user = userEvent.setup()
    const lastEdit: { current: { id: string; body: Record<string, unknown> } | null } = { current: null }
    const updateUserSpy = vi.fn(async ({ params, request }: { params: PathParams; request: Request }) => {
      const body = (await request.json()) as Record<string, unknown>
      lastEdit.current = { id: String(params.id), body }
      return HttpResponse.json({ ...users[0], ...body })
    })
    installBaseline({ files: [], missingPairs: [] })
    server.use(apiRoutes.put('/users/:id', updateUserSpy))
    renderConsole()

    // 教师视角：省份表只读，没有启停操作列。
    await user.click(await screen.findByRole('tab', { name: '省份与考号' }))
    expect(await screen.findByText('河南')).toBeInTheDocument()
    expect(screen.getByText('重庆')).toBeInTheDocument()
    expect(screen.queryByRole('columnheader', { name: '操作' })).not.toBeInTheDocument()

    // 用户名册：编辑 Alice，省份改河南。
    await user.click(screen.getByRole('tab', { name: '用户与编组' }))
    const row = (await screen.findByText('Alice')).closest('tr') as HTMLElement
    await user.click(within(row).getByRole('button', { name: '编辑' }))
    const dialog = await screen.findByRole('dialog', { name: '编辑账号' })
    await user.selectOptions(within(dialog).getByLabelText('编辑省份'), 'HA')
    await user.click(within(dialog).getByRole('button', { name: '保存' }))

    await waitFor(() => expect(lastEdit.current).not.toBeNull())
    expect(lastEdit.current?.id).toBe('u1')
    expect(lastEdit.current?.body).toMatchObject({ name: 'Alice', username: 's001', province: 'HA' })
  })

  it('blocks syncing until every test data file is paired', async () => {
    const user = userEvent.setup()
    const calls = installBaseline({ files: ['a01.in', 'a02.in', 'a02.out'], missingPairs: ['a01.in'] })
    renderConsole()
    await openProblemEditor(user)

    expect(await screen.findByText(/缺少配对：a01.in/)).toBeInTheDocument()
    const syncButton = screen.getByRole('button', { name: /同步为测试点/ })
    expect(syncButton).toBeDisabled()

    await user.click(syncButton)
    expect(calls.sync).toBe(0)
  })

  it('syncs paired files into scored test cases', async () => {
    const user = userEvent.setup()
    const calls = installBaseline({ files: ['a01.in', 'a01.out'], missingPairs: [] })
    renderConsole()
    await openProblemEditor(user)

    const syncButton = screen.getByRole('button', { name: /同步为测试点/ })
    await waitFor(() => expect(syncButton).toBeEnabled())
    await user.click(syncButton)

    await waitFor(() => expect(calls.sync).toBe(1))
    expect(
      await screen.findByText(/已同步 1 组测试点。自动评测是否执行取决于服务端 FORGE_AUTOJUDGE 配置/)
    ).toBeInTheDocument()
    expect(screen.getByText(/当前测试点/)).toHaveTextContent('当前测试点 1 组')
  })

  it('surfaces a sync failure and keeps the previous test cases', async () => {
    const user = userEvent.setup()
    installBaseline(
      { files: ['a01.in', 'a01.out'], missingPairs: [] },
      { sync: () => HttpResponse.json({ message: 'judge unavailable' }, { status: 500 }) }
    )
    renderConsole()
    await openProblemEditor(user)

    await user.click(screen.getByRole('button', { name: /同步为测试点/ }))

    expect(await screen.findByText('同步测试点失败')).toBeInTheDocument()
    expect(await screen.findByRole('alert')).toHaveTextContent('judge unavailable')
  })

  it('saves a single inline answer key for answer-only tasks', async () => {
    const user = userEvent.setup()
    let savedAnswer = ''
    installBaseline({ files: [], missingPairs: [] })
    server.use(
      apiRoutes.put('/contests/contest-1/problems/problem-1', async ({ request }) => {
        const payload = (await request.json()) as {
          testCases?: Array<{ outputFile?: string }>
        }
        savedAnswer = payload.testCases?.[0]?.outputFile || ''
        return HttpResponse.json({
          ...problem,
          taskType: 'answers_only',
          testCases: [
            {
              id: 'answer',
              inputFile: '',
              outputFile: savedAnswer,
              score: 100,
              subtask: 0,
            },
          ],
        })
      })
    )
    renderConsole()
    await openProblemEditor(user)

    await user.selectOptions(screen.getByRole('combobox', { name: '题型分类' }), 'answers_only')
    const answer = await screen.findByRole('textbox', { name: '只交答案题标准答案' })
    await user.type(answer, '42')
    await user.click(screen.getByRole('button', { name: '保存标准答案' }))

    await waitFor(() => expect(savedAnswer).toBe('42'))
    expect(await screen.findByText('标准答案已保存')).toBeInTheDocument()
    expect(screen.getByText(/当前答案 key 1 个/)).toBeInTheDocument()
  })

  it('keeps inline answer text containing slashes instead of treating it as a file path', async () => {
    const user = userEvent.setup()
    installBaseline({ files: [], missingPairs: [] })
    server.use(
      apiRoutes.get('/contests/contest-1/problems', () =>
        HttpResponse.json([
          {
            ...problem,
            taskType: 'answers_only',
            testCases: [
              {
                id: 'answer',
                inputFile: '',
                outputFile: '1/2',
                score: 100,
                subtask: 0,
              },
            ],
          },
        ])
      )
    )
    renderConsole()
    await openProblemEditor(user)

    const answer = await screen.findByRole('textbox', { name: '只交答案题标准答案' })
    await waitFor(() => expect(answer).toHaveValue('1/2'))
  })

  it('loads an answer key from the file inventory only when the basename exists', async () => {
    const user = userEvent.setup()
    let loadedFile = ''
    installBaseline({ files: ['key.ans'], missingPairs: [] })
    server.use(
      apiRoutes.get('/contests/contest-1/problems', () =>
        HttpResponse.json([
          {
            ...problem,
            taskType: 'answers_only',
            testCases: [
              {
                id: 'answer',
                inputFile: '',
                outputFile: 'answers/key.ans',
                score: 100,
                subtask: 0,
              },
            ],
          },
        ])
      ),
      apiRoutes.get(
        '/contests/contest-1/problems/problem-1/files/:name',
        ({ params }) => {
          loadedFile = String(params.name)
          return new HttpResponse('42\n', {
            headers: { 'Content-Type': 'text/plain; charset=utf-8' },
          })
        }
      )
    )
    renderConsole()
    await openProblemEditor(user)

    const answer = await screen.findByRole('textbox', { name: '只交答案题标准答案' })
    await waitFor(() => expect(answer).toHaveValue('42\n'))
    expect(loadedFile).toBe('key.ans')
  })

  it('blocks saving an interaction task without interactor source', async () => {
    const user = userEvent.setup()
    installBaseline({ files: [], missingPairs: [] })
    renderConsole()
    await openProblemEditor(user)

    await user.selectOptions(screen.getByRole('combobox', { name: '题型分类' }), 'interaction')
    expect(await screen.findByRole('textbox', { name: '交互器源码' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '保存题面' }))

    expect(await screen.findByText('交互题缺少交互器')).toBeInTheDocument()
  })

  it('persists grouped subtask score and limits through the dedicated API', async () => {
    const user = userEvent.setup()
    installBaseline({ files: ['a01.in', 'a01.out'], missingPairs: [] })
    let subtaskPayload: unknown
    server.use(
      apiRoutes.put(
        '/contests/contest-1/problems/problem-1/subtasks',
        async ({ request }) => {
          subtaskPayload = await request.json()
          return HttpResponse.json({
            ...problem,
            testCases: [
              {
                id: 'a01',
                inputFile: 'a01.in',
                outputFile: 'a01.out',
                score: 100,
                subtask: 0,
              },
            ],
          })
        }
      )
    )
    renderConsole()
    await openProblemEditor(user)
    await user.click(screen.getByRole('button', { name: /同步为测试点/ }))

    expect(await screen.findByText('子任务分值、限额与依赖')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '保存子任务配置' }))

    await waitFor(() =>
      expect(subtaskPayload).toEqual({
        subtasks: [
          { subtask: 0, score: 100, timeLimitMs: 0, memoryMib: 0, dependsOn: [] },
        ],
      })
    )
    expect(await screen.findByText('子任务配置已保存')).toBeInTheDocument()
  })

  it('renders server-side contest statistics in the analytics workspace', async () => {
    const user = userEvent.setup()
    installBaseline({ files: [], missingPairs: [] })
    renderConsole()

    await user.click(await screen.findByRole('tab', { name: /学情分析/ }))
    expect(await screen.findByText('本场学情总览')).toBeInTheDocument()
    expect((await screen.findAllByText('28.6%')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('距离之和').length).toBeGreaterThan(0)
    expect(screen.getByText('Alice')).toBeInTheDocument()
  })

  it('validates and imports a contest bundle as a new draft', async () => {
    const user = userEvent.setup()
    let imported = false
    installBaseline({ files: [], missingPairs: [] })
    server.use(
      apiRoutes.post('/contests/import', async ({ request }) => {
        const body = (await request.json()) as { contest?: { name?: string } }
        imported = body.contest?.name === 'CSP 原卷'
        return HttpResponse.json({
          ...contest,
          id: 'contest-imported',
          name: 'CSP 原卷 (imported)',
        })
      }),
      apiRoutes.get('/contests/contest-imported/problems', () =>
        HttpResponse.json([problem])
      ),
      apiRoutes.get('/contests/contest-imported/submissions', () =>
        HttpResponse.json([])
      ),
      apiRoutes.get('/contests/contest-imported/ranking', () =>
        HttpResponse.json([])
      )
    )
    renderConsole()

    await user.click(await screen.findByRole('button', { name: /导入比赛包/ }))
    const input = document.querySelector(
      'input[type="file"][accept*="json"]'
    ) as HTMLInputElement
    expect(input).not.toBeNull()
    const bundle = new File(
      [
        JSON.stringify({
          contest: { name: 'CSP 原卷', description: '导入测试' },
          problems: [{ code: 'A', title: '题' }],
        }),
      ],
      'contest.json',
      { type: 'application/json' }
    )
    await user.upload(input, bundle)

    expect(await screen.findByText('CSP 原卷')).toBeInTheDocument()
    expect(screen.getByText('1 道题')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '导入为新草稿' }))

    await waitFor(() => expect(imported).toBe(true))
    expect(await screen.findByText('比赛包已导入')).toBeInTheDocument()
  })

  it('deletes a single test data file after confirmation-free click', async () => {
    const user = userEvent.setup()
    const calls = installBaseline({ files: ['a01.in', 'a01.out'], missingPairs: [] })
    renderConsole()
    await openProblemEditor(user)

    const row = (await screen.findByText('a01.in')).closest('tr') as HTMLElement
    await user.click(within(row).getByRole('button', { name: /删除/ }))

    await waitFor(() => expect(calls.deleteFile).toEqual(['a01.in']))
  })

  it('deletes a problem only after the teacher confirms', async () => {
    const user = userEvent.setup()
    const calls = installBaseline({ files: [], missingPairs: [] })
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    renderConsole()
    await user.click(await screen.findByRole('tab', { name: /试题工坊/ }))

    const card = (await screen.findByText('距离之和')).closest('div.rounded-2xl') as HTMLElement
    await user.click(within(card).getByRole('button', { name: /删除/ }))
    expect(calls.deleteProblem).toBe(0)
    expect(confirm).toHaveBeenCalled()

    confirm.mockReturnValue(true)
    await user.click(within(card).getByRole('button', { name: /删除/ }))
    await waitFor(() => expect(calls.deleteProblem).toBe(1))
  })

  it('never prints a guessed password for an existing account on the seat card', async () => {
    const user = userEvent.setup()
    installBaseline({ files: [], missingPairs: [] })
    renderConsole()

    await user.click(await screen.findByRole('tab', { name: /用户与编组/ }))
    await user.click(screen.getByRole('button', { name: /智能准考证生成/ }))

    const dialog = await screen.findByRole('dialog', { name: /智能准考证/ })
    expect(within(dialog).getAllByText(/密码未知，请由教师重置/).length).toBeGreaterThan(0)
    expect(within(dialog).queryByText('123456')).not.toBeInTheDocument()
  })

  it('previews collection warnings before downloading the collect zip', async () => {
    const user = userEvent.setup()
    installBaseline({ files: [], missingPairs: [] })
    const createObjectURL = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue('blob:collect-zip')
    const revokeObjectURL = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    server.use(
      apiRoutes.get('/contests/contest-1/collect', ({ request }) => {
        const url = new URL(request.url)
        if (url.searchParams.get('format') === 'json') {
          return HttpResponse.json({
            contestId: contest.id,
            contestName: contest.name,
            season: '2026',
            group: 'J',
            entries: [
              {
                userId: 'u1',
                userName: 'Alice',
                username: 's001',
                ticket: 'HA-J00001',
                folder: 'HA-J00001',
                problemId: problem.id,
                problemCode: 'A',
                slug: 'poker',
                fileName: 'poker.cpp',
                language: 'cpp',
                code: 'int main(){}',
                verdict: 'accepted',
                score: 100,
              },
            ],
            warnings: ['s002 has no J ticket for season 2026; collected under username s002'],
          })
        }
        return new HttpResponse(new Uint8Array([0x50, 0x4b]), {
          headers: {
            'Content-Type': 'application/zip',
            'Content-Disposition': 'attachment; filename=collect-2026-J.zip',
          },
        })
      })
    )
    renderConsole()

    await user.click(await screen.findByRole('tab', { name: /实时提交/ }))
    await user.click(screen.getByRole('button', { name: /导出收题包/ }))

    // 下载前先把告警讲清楚：缺号的学生会落在登录名目录下。
    expect(await screen.findByText('收题前请注意')).toBeInTheDocument()
    expect(
      await screen.findByText(/收题包已导出/)
    ).toBeInTheDocument()
    expect(
      screen.getByText(/共 1 份答卷（2026\/J）。/)
    ).toBeInTheDocument()
    expect(createObjectURL).toHaveBeenCalled()
    revokeObjectURL.mockRestore()
    createObjectURL.mockRestore()
  })

  it('saves the ticket scope from the contest settings card', async () => {
    const user = userEvent.setup()
    installBaseline({ files: [], missingPairs: [] })
    let savedScope: Record<string, unknown> | null = null
    server.use(
      apiRoutes.put('/contests/contest-1/settings', async ({ request }) => {
        savedScope = (await request.json()) as Record<string, unknown>
        return HttpResponse.json({
          ...contest,
          ...(savedScope as object),
        })
      })
    )
    renderConsole()

    const seasonInput = await screen.findByRole('textbox', { name: '准考证赛季' })
    await user.clear(seasonInput)
    await user.type(seasonInput, '2026')
    await user.selectOptions(screen.getByRole('combobox', { name: '认证组别' }), 'J')
    await user.click(screen.getByRole('button', { name: '保存收题口径' }))

    await waitFor(() => expect(savedScope).not.toBeNull())
    expect(savedScope).toMatchObject({ season: '2026', ticketGroup: 'J' })
    expect(await screen.findByText('收题口径已保存')).toBeInTheDocument()
  })
})
