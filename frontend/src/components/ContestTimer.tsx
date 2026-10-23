import React, { useEffect, useRef, useState } from 'react'
import { AlertTriangle, Clock3, PauseCircle, TimerReset } from 'lucide-react'
import { api, type Contest } from '../api/client'
import { useStore } from '../store/useStore'
import { Badge } from './ui/badge'

interface ContestTimerProps {
  contest: Contest
  userId?: string
}

function formatRemaining(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000))
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  return [hours, minutes, seconds].map((value) => String(value).padStart(2, '0')).join(':')
}

function formatClock(ms: number): string {
  return new Date(ms).toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}

export const ContestTimer: React.FC<ContestTimerProps> = ({ contest, userId }) => {
  const { serverTimeOffset, setServerTimeOffset } = useStore()
  const [now, setNow] = useState(() => Date.now() + serverTimeOffset)
  const [syncState, setSyncState] = useState<'syncing' | 'trusted' | 'stale' | 'failed'>(
    'syncing'
  )
  const lastSyncRef = useRef<number | null>(null)

  // Estimate the server clock from a round trip. A one-minute refresh keeps
  // long-running exams stable even when the candidate machine clock drifts.
  useEffect(() => {
    let cancelled = false

    const syncTime = async () => {
      try {
        const startedAt = Date.now()
        const response = await api.time()
        const finishedAt = Date.now()
        if (cancelled) return
        const serverMs = new Date(response.serverTime).getTime()
        if (Number.isNaN(serverMs)) return
        const roundTripHalf = (finishedAt - startedAt) / 2
        const offset = serverMs + roundTripHalf - finishedAt
        setServerTimeOffset(offset)
        setNow(Date.now() + offset)
        lastSyncRef.current = Date.now()
        setSyncState('trusted')
      } catch {
        // Keep the last known offset while offline, but never label an
        // unverified local clock as authoritative server time.
        setSyncState(lastSyncRef.current === null ? 'failed' : 'stale')
      }
    }

    void syncTime()
    const interval = window.setInterval(() => void syncTime(), 60_000)
    return () => {
      cancelled = true
      window.clearInterval(interval)
    }
  }, [setServerTimeOffset])

  useEffect(() => {
    const tick = window.setInterval(() => {
      setNow(Date.now() + serverTimeOffset)
    }, 1000)
    return () => window.clearInterval(tick)
  }, [serverTimeOffset])

  if (!contest.windowEnabled || !contest.endTime) {
    return (
      <div className="inline-flex items-center gap-2 rounded-xl border border-[#e4e7ec] bg-[#fffefa] px-3 py-1.5 text-xs shadow-sm">
        <Clock3 className="h-3.5 w-3.5 text-slate-400" />
        <span className="font-semibold text-slate-500">考场时间</span>
        <span className="font-mono font-bold text-[#14283d]">{formatClock(now)}</span>
      </div>
    )
  }

  const startMs = contest.startTime ? new Date(contest.startTime).getTime() : 0
  const baseEndMs = new Date(contest.endTime).getTime()
  const extensionMinutes = (userId && contest.extensions?.[userId]) || 0
  const effectiveEndMs = baseEndMs + extensionMinutes * 60_000

  const notStarted = startMs > 0 && now < startMs
  const finished = now >= effectiveEndMs
  const remainingMs = Math.max(0, effectiveEndMs - now)
  const urgent = !finished && !notStarted && remainingMs < 15 * 60_000

  const timerTone = finished
    ? 'border-[#e4e7ec] bg-[#f9fafb] text-slate-500'
    : notStarted
      ? 'border-sky-200 bg-sky-50 text-sky-700'
      : urgent
        ? 'border-rose-200 bg-rose-50 text-rose-700 animate-pulse'
        : 'border-emerald-200 bg-emerald-50 text-emerald-800'

  const label = finished
    ? '考试结束'
    : notStarted
      ? '距开考'
      : urgent
        ? '即将结束'
        : '剩余时间'
  const display = finished
    ? '00:00:00'
    : formatRemaining(notStarted ? startMs - now : remainingMs)

  return (
    <div className="flex flex-wrap items-center gap-2">
      {extensionMinutes > 0 && (
        <Badge
          variant="warning"
          className="gap-1"
          title="监考教师已为本账号单独加时"
        >
          <TimerReset className="h-3 w-3" />
          +{extensionMinutes} 分钟考务加时
        </Badge>
      )}

      <div
        className={`inline-flex items-center gap-2 rounded-xl border px-3 py-1.5 shadow-sm transition-colors ${timerTone}`}
        aria-label={`${label} ${display}`}
      >
        {finished ? (
          <PauseCircle className="h-3.5 w-3.5" aria-hidden="true" />
        ) : urgent ? (
          <AlertTriangle className="h-3.5 w-3.5" aria-hidden="true" />
        ) : (
          <Clock3 className="h-3.5 w-3.5" aria-hidden="true" />
        )}
        <span className="text-[11px] font-bold">{label}</span>
        <span className="font-mono text-sm font-black tracking-[0.08em]">{display}</span>
      </div>

      <div className="hidden items-center gap-1.5 text-[11px] text-slate-400 md:flex">
        <span>
          {syncState === 'trusted'
            ? '服务器时间'
            : syncState === 'stale'
              ? '服务器时间（同步中断）'
              : syncState === 'failed'
                ? '本机时间（未校准）'
                : '正在校准服务器时间'}
        </span>
        <span className="font-mono font-semibold text-slate-500">{formatClock(now)}</span>
      </div>
      {syncState !== 'trusted' && syncState !== 'syncing' && (
        <span
          role="status"
          className="inline-flex items-center gap-1 rounded-lg bg-amber-50 px-2 py-1 text-[10px] font-bold text-amber-800"
        >
          <AlertTriangle className="h-3 w-3" aria-hidden="true" />
          {syncState === 'failed'
            ? '服务器时间同步失败，当前倒计时仅供本机参考'
            : '服务器时间同步中断，正在使用最后一次校准结果'}
        </span>
      )}
    </div>
  )
}
