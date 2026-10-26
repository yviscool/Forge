import { describe, expect, it } from 'vitest'
import {
  computeCountdown,
  fmtClock,
  fmtDateTime,
  fmtDuration,
} from '@/features/filesubmit/time'
import type { LemonTasksResponse } from '@/features/filesubmit/types'

// 倒计时状态机与 LemonNext contestTimeState 对齐的口径。

const pad = (value: number) => String(value).padStart(2, '0')

function build(overrides: Partial<LemonTasksResponse>): LemonTasksResponse {
  return {
    contestTitle: 'C', user: 'u', displayName: 'u', examFolder: 'u',
    windowEnabled: true, serverNow: new Date().toISOString(),
    preContest: false, hasStatement: false, statements: [],
    submitMode: 'both', scoresReleased: false, latestMessageId: 0, tasks: [],
    ...overrides,
  }
}

describe('filesubmit time formatters', () => {
  it('formats durations with clamping', () => {
    expect(fmtDuration(0)).toBe('00:00:00')
    expect(fmtDuration(3723)).toBe('01:02:03')
    expect(fmtDuration(86399)).toBe('23:59:59')
    expect(fmtDuration(-5)).toBe('00:00:00')
    expect(fmtDuration(59.9)).toBe('00:00:59')
  })

  it('renders dashes for missing clocks and echoes invalid input', () => {
    expect(fmtClock()).toBe('—')
    expect(fmtDateTime()).toBe('—')
    expect(fmtClock('not-a-date')).toBe('not-a-date')
  })

  it('formats local clock and date-time', () => {
    const instant = new Date(2026, 0, 2, 15, 4, 5)
    expect(fmtClock(instant.toISOString())).toBe(
      `${pad(instant.getHours())}:${pad(instant.getMinutes())}:${pad(instant.getSeconds())}`
    )
    expect(fmtDateTime(instant.toISOString())).toBe(
      `1-02 ${pad(instant.getHours())}:${pad(instant.getMinutes())}`
    )
  })
})

describe('computeCountdown state machine', () => {
  it('reports disabled when the window is not enforced', () => {
    expect(computeCountdown(build({ windowEnabled: false }), 0)).toEqual({
      state: 'disabled', secs: 0, duration: '', tone: 'normal',
    })
  })

  it('counts down to the start before the window opens', () => {
    const start = Date.now() + 90_000
    const result = computeCountdown(build({ startTime: new Date(start).toISOString() }), 0)
    expect(result.state).toBe('pre')
    expect(result.tone).toBe('pre')
    expect(result.secs).toBeGreaterThanOrEqual(88)
    expect(result.secs).toBeLessThanOrEqual(90)
  })

  it('marks the contest as ended after the window closes', () => {
    const end = Date.now() - 1000
    expect(computeCountdown(build({ startTime: new Date(end - 1000).toISOString(), endTime: new Date(end).toISOString() }), 0).state).toBe('ended')
  })

  it('escalates urgency while running', () => {
    const start = Date.now() - 1000
    const hour = 3600_000
    expect(computeCountdown(build({ startTime: new Date(start).toISOString(), endTime: new Date(start + hour).toISOString() }), 0).tone).toBe('normal')
    expect(computeCountdown(build({ startTime: new Date(start).toISOString(), endTime: new Date(start + 20 * 60_000).toISOString() }), 0).tone).toBe('warning')
    expect(computeCountdown(build({ startTime: new Date(start).toISOString(), endTime: new Date(start + 3 * 60_000).toISOString() }), 0).tone).toBe('danger')
  })

  it('applies the server offset to the local clock', () => {
    const end = Date.now() + 100_000
    const base = build({ startTime: new Date(Date.now() - 1000).toISOString(), endTime: new Date(end).toISOString() })
    // 服务器比本机快 60s：本机视角剩余应少 60s。
    const withOffset = computeCountdown(base, 60_000)
    const withoutOffset = computeCountdown(base, 0)
    expect(withoutOffset.secs - withOffset.secs).toBeGreaterThanOrEqual(59)
  })

  it('stays running with no end time', () => {
    const result = computeCountdown(build({ startTime: new Date(Date.now() - 1000).toISOString(), endTime: undefined }), 0)
    expect(result.state).toBe('running')
    expect(result.tone).toBe('normal')
  })
})
