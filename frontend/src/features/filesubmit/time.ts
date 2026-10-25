// 文件提交模式（Lemon）——时间工具与倒计时状态机。
//
// 状态机与 LemonNext app.js 的 contestTimeState 对齐：offset 在拿到
// /tasks 响应时冻结一次（服务端时钟 − 本地时钟），此后每秒只做本地
// 推算，避免反复取 Date.now() 造成的服务器时间漂移假象。

import { useEffect, useMemo, useState } from 'react'
import type { LemonTasksResponse } from './types'

export type LemonWindowState = 'disabled' | 'pre' | 'running' | 'ended'

export interface LemonCountdown {
  state: LemonWindowState
  secs: number
  /** 剩余 HH:MM:SS / 距离开始 HH:MM:SS；ended 为空。 */
  duration: string
  /** 供 UI 用的紧迫度：danger ≤5 分钟、warning ≤30 分钟。 */
  tone: 'pre' | 'warning' | 'danger' | 'ended' | 'normal'
}

const pad = (value: number) => String(value).padStart(2, '0')

export function fmtDuration(secs: number): string {
  const clamped = Math.max(0, Math.floor(secs))
  return `${pad(Math.floor(clamped / 3600))}:${pad(Math.floor((clamped % 3600) / 60))}:${pad(clamped % 60)}`
}

export function fmtClock(iso?: string): string {
  if (!iso) return '—'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

export function fmtDateTime(iso?: string): string {
  if (!iso) return '—'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return `${date.getMonth() + 1}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function computeCountdown(data: LemonTasksResponse, offsetMs: number): LemonCountdown {
  if (!data.windowEnabled) return { state: 'disabled', secs: 0, duration: '', tone: 'normal' }
  const now = Date.now() + offsetMs
  const start = data.startTime ? new Date(data.startTime).getTime() : null
  const end = data.endTime ? new Date(data.endTime).getTime() : null
  if (start && now < start) {
    return { state: 'pre', secs: Math.floor((start - now) / 1000), duration: fmtDuration((start - now) / 1000), tone: 'pre' }
  }
  if (end && now > end) {
    return { state: 'ended', secs: 0, duration: '', tone: 'ended' }
  }
  if (end) {
    const secs = Math.floor((end - now) / 1000)
    return {
      state: 'running',
      secs,
      duration: fmtDuration(secs),
      tone: secs <= 5 * 60 ? 'danger' : secs <= 30 * 60 ? 'warning' : 'normal',
    }
  }
  return { state: 'running', secs: 0, duration: '', tone: 'normal' }
}

/**
 * 比赛倒计时。data 为 /tasks 响应；offset 在首次渲染时冻结。
 * onTransition 在状态发生变化（如 pre → running、running → ended）时回调一次。
 */
export function useLemonCountdown(
  data: LemonTasksResponse | undefined,
  onTransition?: (next: LemonCountdown, previous: LemonCountdown) => void
): LemonCountdown {
  const offsetRef = useMemo(() => {
    if (!data?.serverNow) return 0
    return new Date(data.serverNow).getTime() - Date.now()
    // 只在 data 首次到达时冻结 offset，与 LemonNext 行为一致。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data?.serverNow !== undefined])

  const snapshot = () => (data ? computeCountdown(data, offsetRef) : { state: 'disabled' as const, secs: 0, duration: '', tone: 'normal' as const })

  const [countdown, setCountdown] = useState<LemonCountdown>(snapshot)

  useEffect(() => {
    if (!data) return
    let previous = computeCountdown(data, offsetRef)
    setCountdown(previous)
    const timer = window.setInterval(() => {
      const next = computeCountdown(data, offsetRef)
      if (next.state !== previous.state || next.secs !== previous.secs) {
        const changedState = next.state !== previous.state
        const old = previous
        previous = next
        setCountdown(next)
        if (changedState) onTransition?.(next, old)
      }
    }, 1000)
    return () => window.clearInterval(timer)
    // data/offset 变化时重建计时器；onTransition 由调用方保证稳定。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, offsetRef])

  return countdown
}

/** 页面左侧的本机时钟（HH:MM:SS，每秒走字）。 */
export function useLocalClock(): string {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [])
  const date = new Date(now)
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}
