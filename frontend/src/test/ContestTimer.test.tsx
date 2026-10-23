import { act, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api, type Contest } from '../api/client'
import { ContestTimer } from '../components/ContestTimer'
import { useStore } from '../store/useStore'

const LOCAL_NOW = new Date('2026-10-21T08:00:00.000Z')

function contest(overrides: Partial<Contest> = {}): Contest {
  return {
    id: 'contest-1',
    name: 'Forge Contest',
    description: '',
    status: 'running',
    windowEnabled: true,
    startTime: new Date(LOCAL_NOW.getTime() - 60 * 60 * 1000).toISOString(),
    endTime: new Date(LOCAL_NOW.getTime() + 10 * 60 * 1000).toISOString(),
    extensions: {},
    ...overrides,
  }
}

describe('ContestTimer', () => {
  beforeEach(() => {
    useStore.setState({ serverTimeOffset: 0 })
    vi.useFakeTimers()
    vi.setSystemTime(LOCAL_NOW)
    vi.spyOn(api, 'time').mockResolvedValue({ serverTime: LOCAL_NOW.toISOString() })
  })

  it('syncs to the server clock and refreshes the countdown every second', async () => {
    const serverNow = LOCAL_NOW.getTime() + 90_000
    vi.spyOn(api, 'time').mockResolvedValue({
      serverTime: new Date(serverNow).toISOString(),
    })

    render(
      <ContestTimer
        contest={contest({
          endTime: new Date(serverNow + 10 * 60 * 1000).toISOString(),
        })}
      />
    )

    await act(async () => {
      await Promise.resolve()
    })

    expect(useStore.getState().serverTimeOffset).toBe(90_000)

    act(() => {
      vi.advanceTimersByTime(1000)
    })

    expect(screen.getByText('00:09:59')).toBeInTheDocument()
  })

  it('includes a participant extension and shows the urgent state', async () => {
    await act(async () => {
      render(
        <ContestTimer
          contest={contest({
            endTime: new Date(LOCAL_NOW.getTime() + 4 * 60 * 1000).toISOString(),
            extensions: { 'user-1': 5 },
          })}
          userId="user-1"
        />
      )
      await Promise.resolve()
    })

    const countdown = screen.getByText('00:09:00')
    expect(countdown).toBeInTheDocument()
    expect(screen.getByText(/\+5/)).toBeInTheDocument()
    expect(countdown.parentElement).toHaveClass('animate-pulse')
  })

  it('clamps a finished contest to zero', async () => {
    await act(async () => {
      render(
        <ContestTimer
          contest={contest({
            endTime: new Date(LOCAL_NOW.getTime() - 60_000).toISOString(),
          })}
        />
      )
      await Promise.resolve()
    })

    expect(screen.getByText('00:00:00')).toBeInTheDocument()
  })

  it('marks the countdown as untrusted when the first clock sync fails', async () => {
    vi.spyOn(api, 'time').mockRejectedValueOnce(new Error('offline'))

    await act(async () => {
      render(<ContestTimer contest={contest()} />)
      await Promise.resolve()
    })

    expect(screen.getByText('本机时间（未校准）')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('服务器时间同步失败')
  })
})
