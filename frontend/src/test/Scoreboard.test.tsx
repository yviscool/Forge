import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { RankEntry } from '../api/client'
import { Scoreboard } from '../components/Scoreboard'

function ranks(): RankEntry[] {
  return [
    { userId: 'u1', userName: 'Alice', score: 300, accepted: 3, problemScores: {} },
    { userId: 'u2', userName: 'Bob', score: 200, accepted: 2, problemScores: {} },
    { userId: 'u3', userName: 'Carol', score: 100, accepted: 1, problemScores: {} },
  ]
}

describe('Scoreboard', () => {
  it('keeps the original rank when filtering the visible rows', async () => {
    const user = userEvent.setup()
    render(<Scoreboard ranks={ranks()} />)

    await user.type(screen.getByPlaceholderText('查找选手'), 'Carol')

    const row = screen.getByText('Carol').closest('tr')
    expect(row).not.toBeNull()
    expect(within(row as HTMLElement).getByText('3')).toBeInTheDocument()
  })
})
