import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Maximize2, Radar, ShieldAlert } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import type { MonitorParticipant, MonitorProblemStat } from '../api/client'
import { api } from '../api/client'
import { EditorLoading } from './ui/editor-loading'

const VERDICT_DOT: Record<string, string> = {
  accepted: 'bg-emerald-400',
  wrong_answer: 'bg-rose-400',
  compile_error: 'bg-amber-400',
  runtime_error: 'bg-rose-400',
  time_limit: 'bg-amber-400',
  memory_limit: 'bg-rose-400',
  output_limit: 'bg-amber-400',
  checker_error: 'bg-rose-400',
  presentation_error: 'bg-amber-400',
  queued: 'bg-sky-400',
  judging: 'bg-sky-400',
  skipped: 'bg-slate-500',
  // NOI 全真模拟进行中学生侧是 submitted；教师快照里是真实判定，兜底灰色。
  submitted: 'bg-sky-300',
}

function relativeTime(value?: string, now = Date.now()): string {
  if (!value) return '—'
  const delta = now - new Date(value).getTime()
  if (Number.isNaN(delta)) return '—'
  if (delta < 30_000) return '刚刚'
  if (delta < 5_400_000) return `${Math.round(delta / 60_000)} 分钟前`
  if (delta < 86_400_000) return `${Math.round(delta / 3_600_000)} 小时前`
  return new Date(value).toLocaleTimeString()
}

function VerdictDots({ stat }: { stat: MonitorProblemStat }) {
  const color = VERDICT_DOT[stat.lastVerdict] ?? 'bg-slate-600'
  return (
    <span
      className="inline-flex items-center gap-1 rounded-md border border-white/10 bg-white/[0.04] px-1.5 py-0.5 text-[11px] font-bold text-slate-200"
      title={`${stat.code}. ${stat.title}：${stat.attempts} 次提交，最近 ${stat.lastVerdict || '未作答'}`}
    >
      <i aria-hidden="true" className={`h-1.5 w-1.5 rounded-full ${color}`} />
      {stat.code}
      {stat.attempts > 0 && <span className="text-slate-400">×{stat.attempts}</span>}
    </span>
  )
}

function ParticipantCard({
  p,
  now,
  problemCount,
}: {
  p: MonitorParticipant
  now: number
  problemCount: number
}) {
  const editing = p.lastVersionProblemId && p.lastVersionAt
  return (
    <li
      data-testid="monitor-card"
      className="flex min-w-[15rem] flex-1 flex-col gap-2 rounded-2xl border border-white/10 bg-white/[0.03] p-4"
    >
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-sm font-bold text-white">
          {p.examTicket ? `${p.examTicket} · ` : ''}
          {p.name}
        </span>
        <span
          role="status"
          aria-label={p.online ? '在线' : '离线'}
          className={`inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold ${
            p.online
              ? 'bg-emerald-400/10 text-emerald-300'
              : 'bg-white/5 text-slate-400'
          }`}
        >
          <i
            aria-hidden="true"
            className={`h-1.5 w-1.5 rounded-full ${p.online ? 'bg-emerald-400' : 'bg-slate-500'}`}
          />
          {p.online ? '在线' : '离线'}
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-slate-400">
        <span className="font-mono">{p.username}</span>
        {p.ip && (
          <span
            className={`rounded px-1.5 py-0.5 font-mono ${
              p.ipConflict ? 'bg-rose-500/15 font-bold text-rose-300' : 'bg-white/5'
            }`}
          >
            {p.ip}
          </span>
        )}
        {p.ipConflict && (
          <span className="inline-flex items-center gap-1 rounded bg-rose-500/15 px-1.5 py-0.5 font-bold text-rose-300">
            <ShieldAlert className="h-3 w-3" aria-hidden="true" />
            机位冲突
          </span>
        )}
        {p.extensionMinutes ? (
          <span className="rounded bg-sky-500/15 px-1.5 py-0.5 font-bold text-sky-300">
            +{p.extensionMinutes}min
          </span>
        ) : null}
      </div>

      <div className="flex flex-wrap gap-1.5">
        {p.problems.length > 0 ? (
          p.problems.map((stat) => <VerdictDots key={stat.problemId} stat={stat} />)
        ) : (
          <span className="text-[11px] text-slate-500">
            {problemCount > 0 ? '暂无作答' : '本场暂无试题'}
          </span>
        )}
      </div>

      <div className="mt-auto space-y-0.5 border-t border-white/5 pt-2 text-[11px] text-slate-400">
        <p>
          提交 {p.submissionCount} 次 · 最近活动 {relativeTime(p.lastActivityAt, now)}
        </p>
        {editing ? (
          <p className="font-semibold text-sky-300">
            正在编辑 {p.lastVersionProblemId} v{p.lastVersionSeq}
            {p.lastVersionLines ? `（${p.lastVersionLines} 行）` : ''} ·{' '}
            {relativeTime(p.lastVersionAt, now)}
          </p>
        ) : (
          <p className="text-slate-500">暂无编辑动态</p>
        )}
      </div>
    </li>
  )
}

/**
 * 监考大屏：全员在线状态、机位 IP、每题作答动态与实时编辑动态。
 * 快照 10s 轮询兜底；code.saved / submission.* SSE 事件即时触发重取。
 * 教师专用页面（服务端 403 拦截学生）。
 */
export function MonitorBoard({ cid }: { cid: string }) {
  const queryClient = useQueryClient()
  const snapshotQuery = useQuery({
    queryKey: ['monitor', cid],
    queryFn: () => api.monitor(cid),
    enabled: Boolean(cid),
    refetchInterval: 10_000,
  })

  // 监考相关事件到达即重取（提交、判分、代码保存、广播）。
  useEffect(() => {
    if (!cid) return
    const invalidate = () => {
      void queryClient.invalidateQueries({ queryKey: ['monitor', cid] })
    }
    window.addEventListener('forge:monitor-refresh', invalidate)
    return () => window.removeEventListener('forge:monitor-refresh', invalidate)
  }, [cid, queryClient])

  const snapshot = snapshotQuery.data
  const generatedAt = snapshot?.generatedAt
  // 快照到达时固定一次基准时间，避免相对时间随帧漂移。
  const now = useMemo(() => Date.now(), [generatedAt])

  if (!cid) {
    return (
      <p className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-900">
        请先选择一场比赛。
      </p>
    )
  }
  if (snapshotQuery.isLoading) {
    return <EditorLoading />
  }
  if (snapshotQuery.isError) {
    return (
      <div className="space-y-2 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-900">
        <p>监考快照加载失败：{String(snapshotQuery.error)}</p>
        <button
          type="button"
          onClick={() => void snapshotQuery.refetch()}
          className="rounded border border-rose-300 px-2 py-1 font-semibold"
        >
          重试
        </button>
      </div>
    )
  }
  if (!snapshot) return null

  const online = snapshot.participants.filter((p) => p.online).length
  const conflicts = snapshot.participants.filter((p) => p.ipConflict).length

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1.5 text-sm font-bold text-white">
            <Radar className="h-4 w-4 text-emerald-300" aria-hidden="true" />
            {snapshot.name} · 监考大屏
          </span>
          <span className="rounded-full border border-white/10 bg-white/5 px-2 py-0.5 text-[11px] font-bold text-slate-300">
            {snapshot.mode === 'noi' ? 'NOI 全真模拟' : snapshot.mode === 'gesp' ? 'GESP 机考' : '在线提交'}
          </span>
          <span className="rounded-full border border-white/10 bg-white/5 px-2 py-0.5 text-[11px] font-bold text-emerald-300">
            在线 {online}/{snapshot.participants.length}
          </span>
          {conflicts > 0 && (
            <span className="rounded-full bg-rose-500/15 px-2 py-0.5 text-[11px] font-bold text-rose-300">
              机位冲突 {conflicts}
            </span>
          )}
          <span className="text-[11px] text-slate-500">
            快照 {new Date(snapshot.generatedAt).toLocaleTimeString()} · 10s 自动刷新
          </span>
        </div>
        <button
          type="button"
          onClick={() => void document.documentElement.requestFullscreen().catch(() => undefined)}
          className="inline-flex items-center gap-1.5 rounded-lg border border-white/10 bg-white/5 px-2.5 py-1 text-[11px] font-bold text-slate-200 transition hover:bg-white/10"
        >
          <Maximize2 className="h-3.5 w-3.5" aria-hidden="true" />
          全屏投屏
        </button>
      </div>

      {snapshot.participants.length === 0 ? (
        <p className="rounded-2xl border border-dashed border-white/15 px-6 py-10 text-center text-xs text-slate-400">
          本场还没有授权参赛选手。
        </p>
      ) : (
        <ul className="flex flex-wrap gap-3">
          {snapshot.participants.map((p) => (
            <ParticipantCard
              key={p.userId}
              p={p}
              now={now}
              problemCount={snapshot.participants[0]?.problems.length ?? 0}
            />
          ))}
        </ul>
      )}
    </div>
  )
}
