import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  BarChart3,
  Bell,
  LogOut,
  MonitorPlay,
  QrCode,
  RefreshCw,
  Radio,
  Settings,
} from 'lucide-react'
import React, { useEffect, useState } from 'react'
import type { TraySummary } from '../api/client'
import { api } from '../api/client'

/**
 * 托盘速览面板（Wails 锚定小窗加载 /app/panel）。
 * 与 magpie 同构：主应用同一套用户体系的第二种形态——
 * 自含登录、三分区（监考 / 动态 / 服务）、SSE+轮询双驱动、
 * 快捷动作经 /ui/command 直接驱动原生窗口，全程不经过浏览器。
 */

type PanelTab = 'exam' | 'feed' | 'service'

const MODE_TEXT: Record<string, string> = {
  online: '在线提交',
  noi: 'NOI 全真模拟',
  gesp: 'GESP 机考',
}
const STATUS_TEXT: Record<string, string> = {
  running: '进行中',
  draft: '未开始',
  finished: '已结束',
}
const VERDICT_TEXT: Record<string, string> = {
  accepted: '通过',
  wrong_answer: '答案错误',
  compile_error: '编译错误',
  runtime_error: '运行错误',
  time_limit: '超时',
  memory_limit: '超内存',
  output_limit: '输出超限',
  checker_error: 'SPJ 判错',
  queued: '排队中',
  judging: '评测中',
  submitted: '已提交',
  skipped: '跳过',
}
const VERDICT_COLOR: Record<string, string> = {
  accepted: 'text-emerald-600',
  wrong_answer: 'text-rose-600',
  compile_error: 'text-rose-600',
  runtime_error: 'text-rose-600',
  memory_limit: 'text-rose-600',
  time_limit: 'text-amber-600',
  output_limit: 'text-amber-600',
  queued: 'text-sky-600',
  judging: 'text-sky-600',
  submitted: 'text-sky-600',
}

function relTime(iso?: string, now = Date.now()): string {
  if (!iso) return '—'
  const d = now - new Date(iso).getTime()
  if (Number.isNaN(d)) return '—'
  if (d < 30_000) return '刚刚'
  if (d < 3_600_000) return `${Math.round(d / 60_000)} 分钟前`
  return new Date(iso).toLocaleTimeString()
}

function fmtCountdown(ms: number): string {
  if (ms < 0) ms = 0
  const h = Math.floor(ms / 3_600_000)
  const m = Math.floor((ms % 3_600_000) / 60_000)
  const s = Math.floor((ms % 60_000) / 1000)
  return (h > 0 ? `${h}:` : '') + `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function InlineLogin({ onDone }: { onDone: () => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!username.trim() || !password) {
      setError('请输入账号与密码')
      return
    }
    setBusy(true)
    setError('')
    try {
      await api.login(username.trim(), password)
      onDone()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '登录失败，请稍后重试。')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-1 flex-col justify-center gap-3 px-6 py-8">
      <div>
        <p className="font-display text-lg font-bold text-[#14283d]">Forge 监考面板</p>
        <p className="mt-1 text-xs text-slate-500">登录教师账号后此处显示考场实时状态。</p>
      </div>
      <form onSubmit={submit} className="space-y-2.5">
        <input
          aria-label="面板登录账号"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="账号"
          autoComplete="username"
          className="h-9 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm outline-none focus:border-[#405dff]"
        />
        <input
          aria-label="面板登录密码"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="密码"
          autoComplete="current-password"
          className="h-9 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm outline-none focus:border-[#405dff]"
        />
        {error && <p className="text-xs font-semibold text-rose-600">{error}</p>}
        <button
          type="submit"
          disabled={busy}
          className="h-9 w-full rounded-lg bg-[#14283d] text-sm font-bold text-white transition hover:bg-[#1d3a58] disabled:opacity-50"
        >
          {busy ? '登录中…' : '登录'}
        </button>
      </form>
    </div>
  )
}

export function TrayPanel() {
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<PanelTab>('exam')
  const [now, setNow] = useState(() => Date.now())
  const [quitting, setQuitting] = useState(false)
  const [quitError, setQuitError] = useState('')

  // 面板会话自检：未登录时给内联登录（不跳出面板）。
  const sessionQuery = useQuery({
    queryKey: ['panel-session'],
    queryFn: async () => {
      const r = await fetch('/api/v1/auth/me')
      if (r.status === 401) return { authed: false, role: '' }
      const body = (await r.json()) as { role?: string }
      return { authed: r.ok, role: body.role ?? '' }
    },
    staleTime: 30_000,
  })
  const authed = sessionQuery.data?.authed === true

  const summaryQuery = useQuery({
    queryKey: ['tray-summary'],
    queryFn: () => api.traySummary(),
    enabled: authed,
    refetchInterval: 5_000,
  })
  const summary: TraySummary | undefined = summaryQuery.data

  // 倒计时每秒本地跳动；快照到达时校准基准时间。
  useEffect(() => {
    setNow(Date.now())
  }, [summary?.generatedAt])
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(t)
  }, [])

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['tray-summary'] })
    void sessionQuery.refetch()
  }

  const quit = async () => {
    if (!window.confirm('确定退出 Forge 服务？考生将无法继续作答。')) return
    setQuitting(true)
    setQuitError('')
    try {
      await api.shutdownServer()
      setQuitting(false)
    } catch (cause) {
      setQuitting(false)
      setQuitError(cause instanceof Error ? cause.message : '退出失败，请稍后重试。')
    }
  }

  const command = async (action: Parameters<typeof api.uiCommand>[0]) => {
    try {
      await api.uiCommand(action)
    } catch {
      // 非 Wails 构建（浏览器打开 /app/panel）没有原生窗口可驱动，静默忽略。
    }
  }

  if (quitting) {
    return (
      <div className="flex h-screen flex-col items-center justify-center gap-2 bg-[#fbfaf6] px-6 text-center">
        <p className="font-display text-lg font-bold text-[#14283d]">服务已停止</p>
        <p className="text-xs text-slate-500">可以关闭本窗口。重新启动：forge-desktop.exe</p>
      </div>
    )
  }

  if (!authed) {
    return (
      <div className="flex h-screen flex-col bg-[#fbfaf6]">
        <PanelHeader tab={tab} onTab={setTab} onRefresh={refresh} summary={undefined} />
        <InlineLogin onDone={refresh} />
        <PanelFooter onQuit={quit} quitError={quitError} authed={false} />
      </div>
    )
  }

  const contest = summary?.contest
  const endsAt = contest?.status === 'running' ? contest?.windowEndsAt : undefined
  const endsMs = endsAt ? new Date(endsAt).getTime() - now : null
  const alerts: string[] = []
  if ((summary?.conflicts ?? 0) > 0) {
    alerts.push(`${summary?.conflicts} 个机位 IP 冲突——请在监考大屏核对`)
  }
  if ((summary?.queued ?? 0) > 20) {
    alerts.push(`评测队列积压 ${summary?.queued} 题，判题可能变慢`)
  }

  return (
    <div className="flex h-screen flex-col bg-[#fbfaf6]">
      <PanelHeader tab={tab} onTab={setTab} onRefresh={refresh} summary={summary} />

      <main className="min-h-0 flex-1 overflow-y-auto px-3.5 py-3">
        {summaryQuery.isError && (
          <p className="mb-3 rounded-lg bg-rose-50 px-3 py-2 text-xs text-rose-700">
            状态获取失败，正在自动重试…
          </p>
        )}

        {tab === 'exam' && (
          <div className="space-y-3">
            <section className="rounded-xl border border-[#e4e7ec] bg-white p-3.5 shadow-[0_1px_2px_rgba(16,24,40,0.04)]">
              {!contest ? (
                <div>
                  <p className="text-sm font-bold text-[#14283d]">当前没有进行中的比赛</p>
                  <p className="mt-1 text-xs text-slate-500">
                    在教师台创建并启动一场比赛后，这里会实时显示考场状态。
                  </p>
                </div>
              ) : (
                <>
                  <div className="flex items-start justify-between gap-2">
                    <p className="break-words text-sm font-bold leading-5 text-[#14283d]">
                      {contest.name}
                    </p>
                    <span className="shrink-0 rounded-full border border-[#e4e7ec] bg-[#f2f4f7] px-2 py-0.5 text-[10px] font-bold text-slate-600">
                      {STATUS_TEXT[contest.status] || contest.status}
                    </span>
                  </div>
                  <p className="mt-1 text-[11px] text-slate-500">
                    {MODE_TEXT[contest.mode] || contest.mode}
                  </p>
                  {contest.status === 'running' && (
                    <p className="mt-2 flex items-baseline gap-2">
                      <span className="text-[11px] text-slate-500">剩余时间</span>
                      <span
                        className={`font-mono text-2xl font-extrabold ${
                          endsMs != null && endsMs < 5 * 60_000 ? 'text-amber-600' : 'text-[#405dff]'
                        }`}
                      >
                        {endsMs != null ? fmtCountdown(endsMs) : '不限时'}
                      </span>
                    </p>
                  )}
                </>
              )}
            </section>

            {alerts.map((a) => (
              <p
                key={a}
                className="flex items-start gap-1.5 rounded-lg bg-rose-50 px-3 py-2 text-xs font-semibold leading-5 text-rose-700"
              >
                <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                {a}
              </p>
            ))}

            <div className="grid grid-cols-2 gap-2">
              {[
                { label: '在线 / 参赛', value: `${summary?.online ?? 0} / ${summary?.participants ?? 0}` },
                { label: '累计提交', value: summary?.submissions ?? 0 },
                { label: '待评测', value: summary?.queued ?? 0 },
                { label: '机位冲突', value: summary?.conflicts ?? 0 },
              ].map((s) => (
                <div
                  key={s.label}
                  className="rounded-xl border border-[#e4e7ec] bg-white p-3 shadow-[0_1px_2px_rgba(16,24,40,0.04)]"
                >
                  <b className="block font-mono text-xl font-extrabold text-[#14283d]">{s.value}</b>
                  <span className="text-[11px] text-slate-500">{s.label}</span>
                </div>
              ))}
            </div>

            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => void command('open-bigscreen')}
                className="flex items-center justify-center gap-1.5 rounded-xl bg-[#14283d] px-3 py-2.5 text-xs font-bold text-white transition hover:bg-[#1d3a58]"
              >
                <MonitorPlay className="h-3.5 w-3.5" aria-hidden="true" /> 监考大屏
              </button>
              <button
                type="button"
                onClick={() => void command('open-join')}
                className="flex items-center justify-center gap-1.5 rounded-xl border border-[#e4e7ec] bg-white px-3 py-2.5 text-xs font-bold text-[#14283d] transition hover:bg-slate-50"
              >
                <QrCode className="h-3.5 w-3.5" aria-hidden="true" /> 学生扫码加入
              </button>
            </div>
          </div>
        )}

        {tab === 'feed' && (
          <div className="rounded-xl border border-[#e4e7ec] bg-white shadow-[0_1px_2px_rgba(16,24,40,0.04)]">
            {(summary?.recent ?? []).length === 0 ? (
              <p className="px-3.5 py-6 text-center text-xs text-slate-400">暂无提交动态</p>
            ) : (
              (summary?.recent ?? []).map((e, i) => (
                <div
                  key={`${e.userName}-${e.submittedAt}-${i}`}
                  className="flex items-center justify-between gap-2 border-b border-slate-100 px-3.5 py-2 last:border-b-0 text-xs"
                >
                  <span className="min-w-0">
                    <span className="font-bold text-[#14283d]">{e.userName || '考生'}</span>
                    <span className="ml-1.5 text-slate-500">{e.problemCode || '—'} 题</span>
                  </span>
                  <span className={`shrink-0 font-semibold ${VERDICT_COLOR[e.verdict] || 'text-slate-500'}`}>
                    {VERDICT_TEXT[e.verdict] || e.verdict}
                    <span className="ml-1.5 font-normal text-slate-400">
                      {relTime(e.submittedAt, now)}
                    </span>
                  </span>
                </div>
              ))
            )}
          </div>
        )}

        {tab === 'service' && (
          <div className="space-y-3">
            <section className="rounded-xl border border-[#e4e7ec] bg-white p-3.5 text-xs leading-5 shadow-[0_1px_2px_rgba(16,24,40,0.04)]">
              <p className="mb-1 text-sm font-bold text-[#14283d]">服务运行中</p>
              <p className="text-slate-500">
                服务地址：<span className="font-mono">{window.location.host}</span>
              </p>
              <p className="text-slate-500">
                数据目录：data\（SQLite · 本地优先，不上传任何数据）
              </p>
              <p className="text-slate-500">
                快照时间：{summary ? new Date(summary.generatedAt).toLocaleTimeString() : '—'}
              </p>
            </section>
            <section className="rounded-xl border border-[#e4e7ec] bg-white p-3.5 text-xs leading-5 shadow-[0_1px_2px_rgba(16,24,40,0.04)]">
              <p className="mb-1.5 text-sm font-bold text-[#14283d]">考场建议</p>
              <p className="text-slate-500">· 考前用「学生扫码加入」把二维码投到机房大屏</p>
              <p className="text-slate-500">· 开启比赛时间窗口与机位绑定，防跨机位作答</p>
              <p className="text-slate-500">· NOI 全真模拟模式下，赛后才会公布判定与榜单</p>
            </section>
          </div>
        )}
      </main>

      <PanelFooter onQuit={quit} quitError={quitError} authed={authed} />
    </div>
  )
}

function PanelHeader({
  tab,
  onTab,
  onRefresh,
  summary,
}: {
  tab: PanelTab
  onTab: (t: PanelTab) => void
  onRefresh: () => void
  summary?: TraySummary
}) {
  const tabs: Array<{ key: PanelTab; label: string; icon: typeof Radio }> = [
    { key: 'exam', label: '监考', icon: Radio },
    { key: 'feed', label: '动态', icon: Bell },
    { key: 'service', label: '服务', icon: Settings },
  ]
  return (
    <header
      className="shrink-0 border-b border-[#e4e7ec] bg-white px-3.5 pb-0 pt-2.5"
      style={{ ['--wails-draggable' as string]: 'drag' }}
    >
      <div className="flex items-center justify-between pb-2">
        <span className="flex items-center gap-1.5 text-sm font-extrabold text-[#14283d]">
          <span
            className="inline-block h-2 w-2 rounded-full bg-emerald-500"
            aria-hidden="true"
            title="服务运行中"
          />
          Forge 监考面板
        </span>
        <span className="flex items-center gap-2 text-slate-400">
          <button
            type="button"
            aria-label="刷新状态"
            onClick={onRefresh}
            className="rounded p-1 transition hover:bg-slate-100 hover:text-slate-600"
          >
            <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
          </button>
          <button
            type="button"
            aria-label="打开教师台"
            onClick={() => window.open('/teacher', '_blank')}
            className="rounded p-1 transition hover:bg-slate-100 hover:text-slate-600"
          >
            <BarChart3 className="h-3.5 w-3.5" aria-hidden="true" />
          </button>
        </span>
      </div>
      <nav className="flex gap-1" role="tablist" aria-label="面板分区">
        {tabs.map(({ key, label, icon: Icon }) => {
          const selected = tab === key
          return (
            <button
              key={key}
              type="button"
              role="tab"
              aria-selected={selected}
              onClick={() => onTab(key)}
              className={
                selected
                  ? 'flex items-center gap-1 rounded-t-lg border-b-2 border-[#405dff] px-3 py-1.5 text-xs font-bold text-[#14283d]'
                  : 'flex items-center gap-1 rounded-t-lg px-3 py-1.5 text-xs font-bold text-slate-500 transition hover:text-slate-800'
              }
            >
              <Icon className="h-3.5 w-3.5" aria-hidden="true" />
              {label}
            </button>
          )
        })}
        {summary && (
          <span className="ml-auto self-center pr-1 text-[10px] text-slate-400">
            {relTime(summary.generatedAt)}
          </span>
        )}
      </nav>
    </header>
  )
}

function PanelFooter({
  onQuit,
  quitError,
  authed,
}: {
  onQuit: () => void
  quitError: string
  authed: boolean
}) {
  return (
    <footer className="shrink-0 border-t border-[#e4e7ec] bg-white px-3.5 py-2.5">
      {quitError && <p className="mb-1 text-[11px] font-semibold text-rose-600">{quitError}</p>}
      <div className="flex items-center justify-between text-xs">
        <a
          href="/app"
          target="_blank"
          rel="noreferrer"
          className="font-semibold text-slate-500 transition hover:text-[#14283d]"
        >
          学生大厅
        </a>
        <span className="flex items-center gap-3">
          <a
            href="/teacher"
            target="_blank"
            rel="noreferrer"
            className="font-bold text-[#405dff] transition hover:text-[#2c43cc]"
          >
            打开教师台
          </a>
          {authed && (
            <button
              type="button"
              onClick={onQuit}
              className="flex items-center gap-1 font-semibold text-slate-500 transition hover:text-rose-600"
            >
              <LogOut className="h-3.5 w-3.5" aria-hidden="true" /> 退出服务
            </button>
          )}
        </span>
      </div>
    </footer>
  )
}
