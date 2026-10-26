import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ToastProvider } from '@/components/ui/toast'
import { TaskSubmitPanel } from '@/features/filesubmit/TaskSubmitPanel'
import { codeDraftKey } from '@/features/filesubmit/keys'
import { mockLemonApi } from '@/features/filesubmit/mockApi'
import type { LemonTask, LemonTasksResponse } from '@/features/filesubmit/types'
import type { LemonCountdown } from '@/features/filesubmit/time'

// 单题作答面板：元信息、草稿恢复、编辑/上传双模式、提交与覆盖确认。

const data: LemonTasksResponse = {
  contestTitle: 'CSP-J 第二轮 全真模拟',
  user: 'HA-J00001',
  displayName: 'HA-J00001',
  examFolder: 'HA-J00001',
  windowEnabled: true,
  startTime: new Date(Date.now() - 600_000).toISOString(),
  endTime: new Date(Date.now() + 3_600_000).toISOString(),
  serverNow: new Date().toISOString(),
  preContest: false,
  hasStatement: true,
  statements: [],
  submitMode: 'both',
  scoresReleased: false,
  latestMessageId: 0,
  tasks: [],
}

const running: LemonCountdown = { state: 'running', secs: 3600, duration: '01:00:00', tone: 'normal' }
const pre: LemonCountdown = { state: 'pre', secs: 600, duration: '00:10:00', tone: 'pre' }

const bracket: LemonTask = {
  id: 0, title: 'bracket', sourceFileName: 'bracket', totalScore: 100,
  timeLimitMs: 15000, taskType: 'traditional', submittedAt: '', samples: [],
}
const answer: LemonTask = {
  id: 5, title: 'answer', sourceFileName: 'answer', totalScore: 100,
  taskType: 'answers_only', submittedAt: '', samples: [],
}

function renderPanel(overrides: Partial<Parameters<typeof TaskSubmitPanel>[0]> = {}) {
  const onBack = vi.fn()
  const onSubmitted = vi.fn()
  const props = {
    data, task: bracket, taskIndex: 0, countdown: running, onBack, onSubmitted,
    ...overrides,
  }
  render(
    <ToastProvider>
      <TaskSubmitPanel {...props} />
    </ToastProvider>
  )
  return { onBack, onSubmitted }
}

beforeEach(() => {
  localStorage.clear()
})

describe('TaskSubmitPanel traditional tasks', () => {
  it('renders task metadata and header', () => {
    renderPanel()
    expect(screen.getByRole('heading', { name: /第 1 题/ })).toHaveTextContent('bracket')
    expect(screen.getByText('分值')).toBeInTheDocument()
    expect(screen.getByText('15.0s')).toBeInTheDocument()
    expect(screen.getByText('bracket', { selector: 'b.font-mono' })).toBeInTheDocument()
  })

  it('restores the code draft and submits it', async () => {
    localStorage.setItem(codeDraftKey('HA-J00001', 0), 'int main(){}')
    const { onSubmitted } = renderPanel()
    const submit = screen.getByTestId('lemon-submit')
    expect(submit).toBeEnabled()
    await userEvent.click(submit)
    await waitFor(() => expect(onSubmitted).toHaveBeenCalledWith(0, expect.any(String)), { timeout: 5000 })
    expect(await screen.findByText(/提交成功/)).toBeInTheDocument()
    expect(screen.getByText(/上次提交于/)).toBeInTheDocument()
  })

  it('asks for confirmation before overwriting a previous submission', async () => {
    localStorage.setItem(codeDraftKey('HA-J00001', 0), 'int main(){}')
    renderPanel({ task: { ...bracket, submittedAt: new Date().toISOString() } })
    await userEvent.click(screen.getByTestId('lemon-submit'))
    expect(await screen.findByText('确认覆盖上一次提交')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /确认提交/ }))
    await waitFor(() => expect(screen.queryByText('确认覆盖上一次提交')).not.toBeInTheDocument(), { timeout: 5000 })
  })

  it('blocks submission with an empty source', () => {
    renderPanel()
    expect(screen.getByTestId('lemon-submit')).toBeDisabled()
  })

  it('disables submission outside the contest window', () => {
    renderPanel({ countdown: pre })
    expect(screen.getByRole('button', { name: /比赛尚未开始/ })).toBeDisabled()
  })

  it('submits via Ctrl+Enter', async () => {
    localStorage.setItem(codeDraftKey('HA-J00001', 0), 'int main(){}')
    const { onSubmitted } = renderPanel()
    fireEvent.keyDown(window, { ctrlKey: true, key: 'Enter' })
    await waitFor(() => expect(onSubmitted).toHaveBeenCalled(), { timeout: 5000 })
  })

  it('marks answer-only tasks and defaults to text language', () => {
    renderPanel({ task: answer, taskIndex: 5 })
    expect(screen.getByText('答案提交')).toBeInTheDocument()
    expect(screen.getByLabelText('选择语言')).toHaveValue('txt')
  })

  it('uploads a file, guesses the language, and submits its content', async () => {
    renderPanel({ data: { ...data, submitMode: 'file' } })
    expect(screen.getByText('上传源码文件')).toBeInTheDocument()
    expect(screen.queryByLabelText('在线编辑')).not.toBeInTheDocument()
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const file = new File(['print(1)'], 'bracket.py', { type: 'text/x-python' })
    fireEvent.change(input, { target: { files: [file] } })
    expect(await screen.findByText('bracket.py')).toBeInTheDocument()
    // 文件模式没有语言选择器：语言由扩展名内部推断，直接以提交产物验证。
    await userEvent.click(screen.getByTestId('lemon-submit'))
    await waitFor(() => expect(mockLemonApi.source(0)).resolves.toMatchObject({ filename: 'bracket.py' }), { timeout: 5000 })
  })

  it('rejects empty and oversized uploads with a toast', async () => {
    renderPanel({ data: { ...data, submitMode: 'file' } })
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    fireEvent.change(input, { target: { files: [new File([], 'empty.cpp')] } })
    expect(await screen.findByText('文件为空')).toBeInTheDocument()
    fireEvent.change(input, {
      target: { files: [new File(['x'.repeat(65 * 1024)], 'big.cpp')] },
    })
    expect(await screen.findByText(/文件过大/)).toBeInTheDocument()
  })
})
