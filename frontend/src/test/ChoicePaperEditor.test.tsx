import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { ChoiceQuestion } from '../api/client'
import { ChoicePaperEditor } from '../components/ChoicePaperEditor'

function Harness({
  initial = [],
  disabled = false,
}: {
  initial?: ChoiceQuestion[]
  disabled?: boolean
}) {
  const [questions, setQuestions] = useState(initial)
  return (
    <ChoicePaperEditor
      questions={questions}
      onChange={setQuestions}
      disabled={disabled}
    />
  )
}

describe('ChoicePaperEditor', () => {
  it('adds, edits, reorders, and removes objective questions', async () => {
    const user = userEvent.setup()
    render(<Harness />)

    expect(screen.getByText(/还没有客观题/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '新增小题' }))

    const type = screen.getByLabelText('第 1 题题型')
    await user.selectOptions(type, 'multiple')
    const answer = screen.getByLabelText('第 1 题答案')
    await user.clear(answer)
    await user.type(answer, 'a，c')
    await user.tab()
    expect(answer).toHaveValue('A,C')

    await user.type(screen.getByLabelText('第 1 题题干'), 'First objective question')
    await user.click(screen.getByRole('button', { name: '添加选项' }))
    expect(screen.getByLabelText('第 1 题选项 C')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '删除第 1 题选项 C' }))
    expect(screen.queryByLabelText('第 1 题选项 C')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '新增小题' }))
    await user.type(screen.getByLabelText('第 2 题题干'), 'Second objective question')
    await user.click(screen.getByRole('button', { name: '上移第 2 题' }))
    expect(screen.getByLabelText('第 1 题题干')).toHaveValue('Second objective question')

    await user.click(screen.getByRole('button', { name: '删除第 1 题' }))
    expect(screen.getAllByLabelText(/题题干/)).toHaveLength(1)
  })

  it('converts a question to judge mode and respects the disabled state', async () => {
    const user = userEvent.setup()
    const { unmount } = render(
      <Harness
        initial={[
          {
            id: 1,
            type: 'single',
            stem: 'Existing question',
            options: ['A. yes', 'B. no'],
            answer: 'A',
            score: 2,
          },
        ]}
      />
    )

    await user.selectOptions(screen.getByLabelText('第 1 题题型'), 'judge')
    expect(screen.queryByLabelText('第 1 题选项 A')).not.toBeInTheDocument()
    expect(screen.getByLabelText('第 1 题答案')).toHaveValue('T')

    unmount()
    render(
      <Harness
        disabled
        initial={[
          {
            id: 1,
            type: 'single',
            stem: 'Locked question',
            options: ['A. yes', 'B. no'],
            answer: 'A',
            score: 2,
          },
        ]}
      />
    )
    expect(screen.getByLabelText('第 1 题题型')).toBeDisabled()
    expect(screen.getByRole('button', { name: '新增小题' })).toBeDisabled()
  })
})
