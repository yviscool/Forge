import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api, type Contest, type User } from '../api/client'
import { TeacherSecurityPanel } from '../components/TeacherSecurityPanel'
import { ToastProvider } from '../components/ui/toast'

const users: User[] = [
  { id: 'u1', username: 's001', name: 'Alice', role: 'student' },
  { id: 'u2', username: 's002', name: 'Bob', role: 'student' },
]

function contest(overrides: Partial<Contest> = {}): Contest {
  return {
    id: 'contest-1',
    name: 'Autumn Final',
    description: '',
    status: 'running',
    windowEnabled: false,
    startTime: '2026-10-21T01:00:00.000Z',
    endTime: '2026-10-21T03:30:00.000Z',
    durationMinutes: 150,
    extensions: { u2: 10 },
    notice: '原通告',
    zipProtected: true,
    ...overrides,
  }
}

function renderPanel(
  currentContest = contest(),
  refetchContest: () => void | Promise<unknown> = vi.fn()
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <TeacherSecurityPanel
          contest={currentContest}
          users={users}
          refetchContest={refetchContest}
        />
      </ToastProvider>
    </QueryClientProvider>
  )
  return { refetchContest }
}

describe('TeacherSecurityPanel', () => {
  beforeEach(() => {
    vi.spyOn(api, 'getIpBindings').mockResolvedValue({
      bindIpEnabled: true,
      boundIps: { u1: '192.168.1.10', u2: '192.168.1.11' },
    })
    vi.spyOn(api, 'messages').mockResolvedValue([
      {
        id: 'm1',
        contestId: 'contest-1',
        senderName: '监考教师',
        title: '数据更正',
        content: 'T2 样例已更新。',
        level: 'warning',
        createdAt: '2026-10-21T02:00:00.000Z',
      },
    ])
  })

  it('renders IP bindings and requires confirmation before unbinding a participant', async () => {
    const user = userEvent.setup()
    const unbind = vi.spyOn(api, 'unbindUserIp').mockResolvedValue({ ok: true })
    renderPanel()

    expect(await screen.findByText('192.168.1.10')).toBeInTheDocument()
    expect(screen.getByText('Alice')).toBeInTheDocument()

    const aliceRow = screen.getByText('Alice').closest('div.grid') as HTMLElement
    await user.click(within(aliceRow).getByRole('button', { name: /换机解绑/ }))

    expect(unbind).not.toHaveBeenCalled()
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText(/当前绑定 IP 192.168.1.10/)).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: '解绑机位' }))

    await waitFor(() => expect(unbind).toHaveBeenCalledWith('contest-1', 'u1'))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(await screen.findByText('机位已解绑')).toBeInTheDocument()
  })

  it('saves the local datetime as an ISO instant without changing the clock value', async () => {
    const user = userEvent.setup()
    const setContestWindow = vi
      .spyOn(api, 'setContestWindow')
      .mockResolvedValue(contest())
    const { refetchContest } = renderPanel()

    const startInput = screen.getByLabelText(/开始时间/)
    const endInput = screen.getByLabelText(/截止时间/)
    await user.clear(startInput)
    await user.type(startInput, '2026-10-21T09:00')
    await user.clear(endInput)
    await user.type(endInput, '2026-10-21T11:30')
    await user.click(screen.getByRole('button', { name: '保存时间窗口' }))

    await waitFor(() => expect(setContestWindow).toHaveBeenCalledTimes(1))
    const [, payload] = setContestWindow.mock.calls[0]
    expect(payload.startTime).toBe(new Date('2026-10-21T09:00').toISOString())
    expect(payload.endTime).toBe(new Date('2026-10-21T11:30').toISOString())
    expect(payload.windowEnabled).toBe(false)
    expect(payload.durationMinutes).toBe(150)
    expect(refetchContest).toHaveBeenCalled()
    expect(await screen.findByText('时间窗口已保存')).toBeInTheDocument()
  })

  it('validates extension input and sends the chosen participant and minutes', async () => {
    const user = userEvent.setup()
    const extension = vi.spyOn(api, 'setUserExtension').mockResolvedValue(contest())
    renderPanel()

    await user.selectOptions(screen.getByLabelText('选择考生'), 'u1')
    await user.clear(screen.getByLabelText('加时分钟数'))
    await user.type(screen.getByLabelText('加时分钟数'), '15')
    await user.click(screen.getByRole('button', { name: '应用加时' }))

    await waitFor(() => expect(extension).toHaveBeenCalledWith('contest-1', 'u1', 15))
    expect(await screen.findByText('加时已生效')).toBeInTheDocument()
  })

  it('sends notice, zip password and broadcast updates with typed values', async () => {
    const user = userEvent.setup()
    const notice = vi.spyOn(api, 'updateNotice').mockResolvedValue(contest())
    const zip = vi.spyOn(api, 'setZipPassword').mockResolvedValue(contest())
    const message = vi.spyOn(api, 'sendMessage').mockResolvedValue({
      id: 'm2',
      contestId: 'contest-1',
      senderName: '监考教师',
      title: '紧急',
      content: '请保存代码',
      level: 'urgent',
      createdAt: '2026-10-21T02:10:00.000Z',
    })
    renderPanel()

    const noticeInput = screen.getByLabelText('置顶通告')
    await user.clear(noticeInput)
    await user.type(noticeInput, 'T3 数据已修复')
    await user.click(screen.getByRole('button', { name: '保存通告' }))
    await waitFor(() => expect(notice).toHaveBeenCalledWith('contest-1', 'T3 数据已修复'))

    const zipInput = screen.getByLabelText('试卷包 Zip 解锁口令')
    await user.clear(zipInput)
    await user.type(zipInput, 'NEW-PASS')
    await user.click(screen.getByRole('button', { name: '保存口令' }))
    await waitFor(() => expect(zip).toHaveBeenCalledWith('contest-1', 'NEW-PASS'))
    expect(zipInput).toHaveValue('')

    await user.type(screen.getByPlaceholderText(/广播标题/), '紧急通知')
    await user.type(screen.getByPlaceholderText('广播正文...'), '请保存代码')
    await user.selectOptions(screen.getByLabelText('广播对象'), 'u2')
    await user.selectOptions(screen.getByLabelText('广播级别'), 'urgent')
    await user.click(screen.getByRole('button', { name: '立即下发广播' }))

    await waitFor(() =>
      expect(message).toHaveBeenCalledWith('contest-1', {
        title: '紧急通知',
        content: '请保存代码',
        targetUserId: 'u2',
        level: 'urgent',
      })
    )
    expect(await screen.findByText('广播已下发')).toBeInTheDocument()
  })

  it('never pre-fills an existing ZIP verifier and explains how to rotate it', async () => {
    renderPanel()

    const zipInput = screen.getByLabelText('试卷包 Zip 解锁口令')
    expect(zipInput).toHaveValue('')
    expect(screen.getByText(/当前已配置，服务端不会回显旧口令/)).toBeInTheDocument()
    expect(screen.getByText(/当前状态：已配置（不回显）/)).toBeInTheDocument()
  })
})
