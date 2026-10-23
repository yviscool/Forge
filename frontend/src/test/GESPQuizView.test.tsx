import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { api, type Problem, type Submission } from '../api/client'
import { GESPQuizView } from '../components/GESPQuizView'
import { ToastProvider } from '../components/ui/toast'
import { choiceDraftKey, useStore } from '../store/useStore'

const problem: Problem = {
  id: 'choice-problem-1',
  contestId: 'contest-1',
  code: 'GESP01',
  title: 'GESP 二级客观题',
  statement: '请完成下列客观题。',
  input: '',
  output: '',
  examples: '',
  constraints: '',
  timeLimitMs: 1000,
  memoryLimitMib: 256,
  taskType: 'choice',
  choiceQuestions: [
    {
      id: 1,
      type: 'single',
      stem: '下列哪个是 C++ 关键字？',
      options: ['A. while', 'B. integer', 'C. main', 'D. string'],
      answer: 'A',
      score: 2,
    },
    {
      id: 2,
      type: 'multiple',
      stem: '下列哪些是整型类型？',
      options: ['A. int', 'B. float', 'C. short', 'D. double'],
      answer: ['A', 'C'],
      score: 4,
    },
    {
      id: 3,
      type: 'judge',
      stem: '数组下标默认从 0 开始。',
      answer: 'T',
      score: 2,
    },
  ],
}

function submission(overrides: Partial<Submission> = {}): Submission {
  return {
    id: 'submission-1',
    contestId: 'contest-1',
    problemID: problem.id,
    userId: 'user-1',
    language: 'choice_json',
    code: '{}',
    verdict: 'accepted',
    score: 8,
    cases: [
      { caseIndex: 1, verdict: 'AC', score: 2 },
      { caseIndex: 2, verdict: 'AC', score: 4 },
      { caseIndex: 3, verdict: 'AC', score: 2 },
    ],
    submittedAt: '2026-10-21T08:00:00.000Z',
    ...overrides,
  }
}

describe('GESPQuizView', () => {
  beforeEach(() => {
    useStore.setState({ drafts: {} })
  })

  it('tracks single, multiple and judge answers and restores them from a draft', async () => {
    const user = userEvent.setup()
    const { unmount } = render(
      <ToastProvider>
        <GESPQuizView contestId="contest-1" problem={problem} />
      </ToastProvider>
    )

    expect(screen.getByRole('progressbar', { name: '答题进度' })).toHaveAttribute(
      'aria-valuenow',
      '0'
    )
    await user.click(screen.getByRole('radio', { name: /while/ }))
    await user.click(screen.getByRole('checkbox', { name: /int/ }))
    await user.click(screen.getByRole('checkbox', { name: /short/ }))
    await user.click(screen.getAllByRole('radio', { name: /正确/ })[0])

    expect(screen.getByRole('progressbar', { name: '答题进度' })).toHaveAttribute(
      'aria-valuenow',
      '100'
    )
    const draftEntries = Object.entries(useStore.getState().drafts)
    expect(
      draftEntries.some(
        ([key, value]) => key.includes(problem.id) && key.includes('choice-sheet') && value
      )
    ).toBe(true)

    unmount()
    render(
      <ToastProvider>
        <GESPQuizView contestId="contest-1" problem={problem} />
      </ToastProvider>
    )

    expect(screen.getByText('已恢复本地草稿')).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /while/ })).toHaveAttribute(
      'aria-checked',
      'true'
    )
    expect(screen.getByRole('checkbox', { name: /int/ })).toHaveAttribute(
      'aria-checked',
      'true'
    )
    expect(screen.getByRole('checkbox', { name: /short/ })).toHaveAttribute(
      'aria-checked',
      'true'
    )
  })

  it('blocks submission while questions are unanswered', async () => {
    const user = userEvent.setup()
    render(
      <ToastProvider>
        <GESPQuizView contestId="contest-1" problem={problem} />
      </ToastProvider>
    )

    await user.click(screen.getByRole('button', { name: '提交答题卡' }))

    expect(
      screen.getByRole('alert')
    ).toHaveTextContent('还有 3 题未作答')
  })

  it.each([
    ['upcoming', '考试尚未开始，当前仅可阅读题面，答题卡将在开考后解锁。'],
    ['finished', '考试已经结束，答题卡与提交入口均已锁定。'],
  ] as const)('locks answer controls and submission while the phase is %s', async (phase, message) => {
    const user = userEvent.setup()
    const submit = vi.spyOn(api, 'submit')
    render(
      <ToastProvider>
        <GESPQuizView
          contestId="contest-1"
          problem={problem}
          canSubmit={false}
          phase={phase}
        />
      </ToastProvider>
    )

    expect(screen.getByRole('status')).toHaveTextContent('答题卡已锁定')
    expect(screen.getByRole('status')).toHaveTextContent(message)
    expect(screen.getByRole('radio', { name: /while/ })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: /int/ })).toBeDisabled()
    const submitButton = screen.getByRole('button', { name: '答题卡已锁定' })
    expect(submitButton).toBeDisabled()
    expect(screen.getByRole('button', { name: '清空本题草稿' })).toBeDisabled()

    await user.click(submitButton)
    expect(submit).not.toHaveBeenCalled()
  })

  it('submits a complete answer sheet and renders backend AC/WA case verdicts', async () => {
    const user = userEvent.setup()
    const submit = vi.spyOn(api, 'submit').mockResolvedValue(
      submission({
        verdict: 'wrong_answer',
        score: 6,
        cases: [
          { caseIndex: 1, verdict: 'AC', score: 2 },
          { caseIndex: 2, verdict: 'WA', score: 0 },
          { caseIndex: 3, verdict: 'AC', score: 2 },
        ],
      })
    )
    const onSuccess = vi.fn()

    render(
      <ToastProvider>
        <GESPQuizView contestId="contest-1" problem={problem} onSubmitSuccess={onSuccess} />
      </ToastProvider>
    )

    await user.click(screen.getByRole('radio', { name: /while/ }))
    await user.click(screen.getByRole('checkbox', { name: /int/ }))
    await user.click(screen.getByRole('checkbox', { name: /short/ }))
    await user.click(screen.getAllByRole('radio', { name: /正确/ })[0])
    await user.click(screen.getByRole('button', { name: '提交答题卡' }))

    await waitFor(() => {
      expect(submit).toHaveBeenCalledWith('contest-1', {
        problemID: problem.id,
        language: 'choice_json',
        code: JSON.stringify({ '1': 'A', '2': ['A', 'C'], '3': 'T' }),
      }, expect.any(String))
    })
    expect(await screen.findByText('存在错题 · 6 分')).toBeInTheDocument()
    expect(screen.getAllByText('正确 +2').length).toBeGreaterThan(0)
    expect(screen.getByText('错误 WA')).toBeInTheDocument()
    expect(onSuccess).toHaveBeenCalledWith(expect.objectContaining({ score: 6 }))
  })

  it('reuses the idempotency key when retrying the same answer sheet', async () => {
    const user = userEvent.setup()
    const submit = vi
      .spyOn(api, 'submit')
      .mockRejectedValueOnce(new Error('network offline'))
      .mockResolvedValueOnce(submission())

    render(
      <ToastProvider>
        <GESPQuizView contestId="contest-1" problem={problem} />
      </ToastProvider>
    )

    await user.click(screen.getByRole('radio', { name: /while/ }))
    await user.click(screen.getByRole('checkbox', { name: /int/ }))
    await user.click(screen.getByRole('checkbox', { name: /short/ }))
    await user.click(screen.getAllByRole('radio', { name: /正确/ })[0])
    await user.click(screen.getByRole('button', { name: '提交答题卡' }))
    await waitFor(() => expect(submit).toHaveBeenCalledTimes(1))
    await user.click(screen.getByRole('button', { name: '提交答题卡' }))

    await waitFor(() => expect(submit).toHaveBeenCalledTimes(2))
    expect(submit.mock.calls[0][2]).toBeTruthy()
    expect(submit.mock.calls[0][2]).toBe(submit.mock.calls[1][2])
  })

  it('does not restore another account choice sheet after identity changes', async () => {
    const user = userEvent.setup()
    useStore.setState({
      session: {
        userId: 'user-1',
        username: 'alice',
        name: 'Alice',
        role: 'student',
      },
      drafts: {},
    })

    const first = render(
      <ToastProvider>
        <GESPQuizView contestId="contest-1" problem={problem} />
      </ToastProvider>
    )
    await user.click(screen.getByRole('radio', { name: /while/ }))
    expect(screen.getByRole('radio', { name: /while/ })).toHaveAttribute(
      'aria-checked',
      'true'
    )
    first.unmount()

    useStore.getState().logout()
    useStore.getState().setSession({
      userId: 'user-2',
      username: 'bob',
      name: 'Bob',
      role: 'student',
    })

    render(
      <ToastProvider>
        <GESPQuizView contestId="contest-1" problem={problem} />
      </ToastProvider>
    )

    expect(screen.getByRole('radio', { name: /while/ })).toHaveAttribute(
      'aria-checked',
      'false'
    )
    expect(screen.queryByText(/宸叉仮澶嶆湰鍦拌崏绋/)).not.toBeInTheDocument()
    expect(
      useStore.getState().drafts[choiceDraftKey('user-1', 'contest-1', problem.id)]
    ).toContain('A')
    expect(
      useStore.getState().drafts[choiceDraftKey('user-2', 'contest-1', problem.id)]
    ).toBeUndefined()
  })
})
