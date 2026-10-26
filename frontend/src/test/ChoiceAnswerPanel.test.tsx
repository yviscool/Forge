import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ToastProvider } from '@/components/ui/toast'
import { ChoiceAnswerPanel } from '@/features/filesubmit/ChoiceAnswerPanel'
import { choiceDraftKey } from '@/features/filesubmit/keys'
import { mockLemonApi } from '@/features/filesubmit/mockApi'
import type { LemonTask, LemonTasksResponse } from '@/features/filesubmit/types'
import type { LemonCountdown } from '@/features/filesubmit/time'

// 选择题答题卡：切题渲染、草稿持久化、上次递交回填、部分作答确认递交。

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
  submitMode: 'editor',
  scoresReleased: false,
  latestMessageId: 0,
  tasks: [],
}

const quiz: LemonTask = {
  id: 4, title: 'quiz', sourceFileName: 'quiz', totalScore: 100,
  taskType: 'choice', submittedAt: '', samples: [],
}

const running: LemonCountdown = { state: 'running', secs: 3600, duration: '01:00:00', tone: 'normal' }

function renderPanel(overrides: Partial<Parameters<typeof ChoiceAnswerPanel>[0]> = {}) {
  const onBack = vi.fn()
  const onSubmitted = vi.fn()
  render(
    <ToastProvider>
      <ChoiceAnswerPanel
        data={data}
        task={quiz}
        countdown={running}
        onBack={onBack}
        onSubmitted={onSubmitted}
        {...overrides}
      />
    </ToastProvider>
  )
  return { onBack, onSubmitted }
}

beforeEach(() => {
  localStorage.clear()
})

async function renderLoadedPaper() {
  const result = renderPanel()
  await screen.findByText('#1')
  await screen.findByText('#5')
  return result
}

describe('ChoiceAnswerPanel', () => {
  it('renders the parsed paper with per-question badges', async () => {
    await renderLoadedPaper()
    expect(screen.getAllByText('单选')).toHaveLength(3)
    expect(screen.getByText('多选')).toBeInTheDocument()
    expect(screen.getByText('判断')).toBeInTheDocument()
    expect(screen.getByText(/共有 5 道题目，已完成 0 道/)).toBeInTheDocument()
    expect(screen.getByTestId('choice-submit')).toBeEnabled()
  })

  it('persists the answer sheet draft while selecting', async () => {
    await renderLoadedPaper()
    await userEvent.click(screen.getByRole('radio', { name: /8 个二进制位组成/ }))
    expect(await screen.findByText(/已完成 1 道/)).toBeInTheDocument()
    const draft = localStorage.getItem(choiceDraftKey('HA-J00001', 4))
    expect(JSON.parse(draft || '{}')).toEqual({ 1: ['A'] })
  })

  it('toggles multiple-choice options and supports clearing', async () => {
    await renderLoadedPaper()
    const merge = screen.getByRole('checkbox', { name: /归并排序/ })
    const quick = screen.getByRole('checkbox', { name: /快速排序（平均）/ })
    await userEvent.click(merge)
    await userEvent.click(quick)
    expect(JSON.parse(localStorage.getItem(choiceDraftKey('HA-J00001', 4)) || '{}')).toEqual({
      4: ['A', 'C'],
    })
    await userEvent.click(merge)
    expect(JSON.parse(localStorage.getItem(choiceDraftKey('HA-J00001', 4)) || '{}')).toEqual({
      4: ['C'],
    })
  })

  it('restores the draft from a previous session', async () => {
    localStorage.setItem(choiceDraftKey('HA-J00001', 4), JSON.stringify({ 2: ['C'] }))
    await renderLoadedPaper()
    await waitFor(() =>
      expect(screen.getByRole('radio', { name: /连通图的生成树包含全部顶点/ })).toBeChecked()
    )
    expect(screen.getByText(/已完成 1 道/)).toBeInTheDocument()
  })

  it('backfills answers from the latest submission when no draft exists', async () => {
    await mockLemonApi.submit(4, { source: '1: A\n2: B\n', language: 'txt' })
    await renderLoadedPaper()
    await waitFor(() =>
      expect(screen.getByRole('radio', { name: /有向无环图的拓扑序一定唯一/ })).toBeChecked()
    )
    expect(screen.getByText(/已完成 2 道/)).toBeInTheDocument()
    expect(screen.getByText(/上次递交于/)).toBeInTheDocument()
  })

  it('warns when submitting an empty sheet', async () => {
    await renderLoadedPaper()
    await userEvent.click(screen.getByTestId('choice-submit'))
    expect(await screen.findByText('请先作答任意一题再递交')).toBeInTheDocument()
    expect(screen.queryByText('仍有未作答的题目')).not.toBeInTheDocument()
  })

  it('confirms partial sheets and submits the serialized answers', async () => {
    const { onSubmitted } = await renderLoadedPaper()
    await userEvent.click(screen.getByRole('radio', { name: /8 个二进制位组成/ }))
    await userEvent.click(screen.getByTestId('choice-submit'))
    expect(await screen.findByText('仍有未作答的题目')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /仍然递交/ }))
    await waitFor(() => expect(onSubmitted).toHaveBeenCalledWith(expect.any(String)), { timeout: 5000 })
    const source = await mockLemonApi.source(4)
    expect(source.content).toContain('1: A')
    expect(source.filename).toBe('quiz.txt')
  })

  it('submits directly when every question is answered', async () => {
    localStorage.setItem(
      choiceDraftKey('HA-J00001', 4),
      JSON.stringify({ 1: ['A'], 2: ['C'], 3: ['B'], 4: ['A', 'C'], 5: ['A'] })
    )
    const { onSubmitted } = await renderLoadedPaper()
    await waitFor(() => expect(screen.getByText(/已完成 5 道/)).toBeInTheDocument())
    await userEvent.click(screen.getByTestId('choice-submit'))
    await waitFor(() => expect(onSubmitted).toHaveBeenCalled(), { timeout: 5000 })
    expect(screen.queryByText('仍有未作答的题目')).not.toBeInTheDocument()
  })

  it('locks the paper outside the contest window', async () => {
    renderPanel({ countdown: { state: 'ended', secs: 0, duration: '', tone: 'ended' } })
    await screen.findByText('#1')
    expect(screen.getByTestId('choice-submit')).toBeDisabled()
    expect(screen.getByRole('button', { name: /比赛已结束/ })).toBeDisabled()
  })
})
