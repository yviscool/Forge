// 文件提交模式（河南版）——学生侧工作台。
//
// 复刻河南本地收卷系统（ZYZOJ 离线代码收集系统）的学生端全部规则：
//   仅 C++、提交文件必须为 <题目英文名>.cpp（前后端双重校验，消息原文一致）、
//   校验顺序 phase → 已确认 → 已选文件 → .cpp 后缀 → 文件名全等、
//   心跳驱动的连接门控（断连时提交/确认暂停）、封禁只读、
//   最终确认冻结（确认并锁定后不能再选择或提交）、
//   成绩发布（总分/排名/各题测试点详情，或管理员导入无详情）、
//   比赛附件下载、修改密码、开赛前自动刷新。
// 数据来自 mockApi；后端就绪后替换为同接口的真实实现即可。

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  Award,
  Ban,
  ChevronLeft,
  Clock3,
  CloudOff,
  Download,
  Eye,
  FileCode2,
  KeyRound,
  Lock,
  LockOpen,
  Paperclip,
  Send,
  ServerCog,
  Trophy,
  Upload,
} from 'lucide-react'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import { Dialog } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { useToast } from '../../../components/ui/toast'
import { formatBytes } from '../../student/submissionFiles'
import { mockHenanApi as api } from './mockApi'
import type {
  HenanAttachment,
  HenanHeartbeat,
  HenanPhase,
  HenanProblem,
  HenanScore,
} from './types'

const PHASE_META: Record<HenanPhase, { label: string; variant: 'default' | 'warning' | 'success' | 'info' | 'accent' }> = {
  waiting: { label: '等待配置', variant: 'default' },
  before: { label: '未开始', variant: 'warning' },
  active: { label: '比赛进行中', variant: 'success' },
  ended: { label: '已结束', variant: 'default' },
  publish: { label: '成绩已发布', variant: 'accent' },
}

const pad = (value: number) => String(value).padStart(2, '0')

function fmtDuration(secs: number): string {
  const clamped = Math.max(0, Math.floor(secs))
  return `${pad(Math.floor(clamped / 3600))}:${pad(Math.floor((clamped % 3600) / 60))}:${pad(clamped % 60)}`
}

function fmtUnix(seconds?: number): string {
  if (!seconds) return '未设置'
  const date = new Date(seconds * 1000)
  const y = date.getFullYear()
  const m = pad(date.getMonth() + 1)
  const d = pad(date.getDate())
  const hh = pad(date.getHours())
  const mm = pad(date.getMinutes())
  const ss = pad(date.getSeconds())
  return `${y}/${m}/${d} ${hh}:${mm}:${ss}`
}

function fmtClock(iso?: string): string {
  if (!iso) return '—'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

/** 心跳时间窗：offset 在每次心跳到达时校准，秒级走字在本地推算。 */
function useHenanClock(beat: HenanHeartbeat | null) {
  const [nowMs, setNowMs] = useState(() => Date.now())
  const offsetRef = useRef(0)
  useEffect(() => {
    if (beat) offsetRef.current = beat.serverTimeMs - Date.now()
  }, [beat])
  useEffect(() => {
    const timer = window.setInterval(() => setNowMs(Date.now() + offsetRef.current), 1000)
    return () => window.clearInterval(timer)
  }, [])
  return nowMs
}

export function HenanWorkspace() {
  const { cid } = useParams<{ cid: string }>()
  const toast = useToast()

  const [beat, setBeat] = useState<HenanHeartbeat | null>(null)
  const [connection, setConnection] = useState<'ok' | 'paused'>('ok')
  const [problems, setProblems] = useState<HenanProblem[]>([])
  const [score, setScore] = useState<HenanScore | null>(null)
  const [attachments, setAttachments] = useState<HenanAttachment[]>([])
  const [picked, setPicked] = useState<Record<string, File>>({})
  const [submitting, setSubmitting] = useState<string | null>(null)
  const [codeProblem, setCodeProblem] = useState<HenanProblem | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [scoreOpenProblem, setScoreOpenProblem] = useState<string | null>(null)

  const refreshProblems = useCallback(async () => {
    try {
      setProblems(await api.problems())
    } catch {
      // 列表失败交给心跳横幅提示
    }
  }, [])

  // ---- 心跳：比赛状态/公告/连接门控的唯一来源 ----
  const reloadOnce = useRef(false)
  useEffect(() => {
    let cancelled = false
    let previousPhase: HenanPhase | null = null
    const poll = async () => {
      try {
        const next = await api.heartbeat()
        if (cancelled) return
        setBeat(next)
        setConnection('ok')
        if (previousPhase && previousPhase !== next.phase) {
          // 开赛瞬间整页刷新，题目列表与提交区随服务端状态就位。
          if (previousPhase === 'before' && next.phase === 'active' && !reloadOnce.current) {
            reloadOnce.current = true
            window.location.reload()
            return
          }
        }
        previousPhase = next.phase
      } catch {
        if (!cancelled) setConnection('paused')
      }
    }
    void poll()
    const timer = window.setInterval(poll, 15000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [])

  useEffect(() => {
    void refreshProblems()
    void api.files().then(setAttachments).catch(() => undefined)
  }, [refreshProblems])

  useEffect(() => {
    if (beat?.phase === 'publish') {
      void api.score().then(setScore).catch(() => undefined)
    }
  }, [beat?.phase])

  const nowMs = useHenanClock(beat)
  const phase = beat?.phase || 'waiting'
  const phaseMeta = PHASE_META[phase]
  const banned = Boolean(beat?.banned)
  const confirmed = Boolean(beat?.confirmed)
  const operationsPaused =
    connection === 'paused' || phase !== 'active' || confirmed || banned

  const remainingSecs = useMemo(() => {
    if (!beat) return 0
    const end = beat.endTime * 1000
    return Math.floor((end - nowMs) / 1000)
  }, [beat, nowMs])

  const progressPercent = useMemo(() => {
    if (!beat || !beat.startTime || !beat.endTime) return 0
    const total = (beat.endTime - beat.startTime) * 1000
    const elapsed = nowMs - beat.startTime * 1000
    return Math.max(0, Math.min(100, (elapsed / total) * 100))
  }, [beat, nowMs])

  const submittedCount = problems.filter((problem) => problem.record).length
  const totalCount = problems.length

  const handlePick = (problem: string, file: File | undefined) => {
    setPicked((current) => {
      const next = { ...current }
      if (file) next[problem] = file
      else delete next[problem]
      return next
    })
  }

  const handleSubmit = async (problem: HenanProblem) => {
    if (submitting) return
    const file = picked[problem.name]
    // 与河南系统一致的客户端校验链（服务端同样兜底）。
    if (phase === 'ended' || phase === 'publish') {
      toast.error('比赛已结束')
      return
    }
    if (phase !== 'active') {
      toast.error('比赛尚未开始')
      return
    }
    if (connection === 'paused') {
      toast.error('服务器连接已中断，请等待连接恢复')
      return
    }
    if (confirmed) {
      toast.error('您已确认最终提交，不能再次提交代码')
      return
    }
    if (banned) {
      toast.error('账号已被封禁，不能继续作答')
      return
    }
    if (!file) {
      toast.error('请先选择文件')
      return
    }
    if (!file.name.endsWith('.cpp')) {
      toast.error('文件必须以 .cpp 结尾')
      return
    }
    if (file.name.slice(0, -4) !== problem.name) {
      toast.error(`文件名必须为 ${problem.name}.cpp`)
      return
    }
    setSubmitting(problem.name)
    try {
      const content = await file.text()
      const result = await api.submit(problem.name, { name: file.name, size: file.size, content })
      if (result.status !== 'success') {
        toast.error('提交失败', result.message)
        return
      }
      toast.success(`${problem.name}.cpp 提交成功`)
      handlePick(problem.name, undefined)
      await refreshProblems()
    } catch (cause) {
      toast.error('提交失败', cause instanceof Error ? cause.message : undefined)
    } finally {
      setSubmitting(null)
    }
  }

  const handleConfirm = async () => {
    if (confirming) return
    setConfirming(true)
    try {
      const result = await api.confirm()
      if (result.status !== 'success') {
        toast.error('确认失败', result.message)
        return
      }
      toast.success('已确认最终提交')
      setConfirmOpen(false)
      setBeat((current) => (current ? { ...current, confirmed: true } : current))
    } finally {
      setConfirming(false)
    }
  }

  if (!beat) {
    return (
      <div className="flex flex-col items-center gap-3 py-24 text-slate-400">
        <ServerCog className="h-8 w-8 animate-pulse" />
        <p className="text-sm font-semibold">正在同步服务器时间...</p>
      </div>
    )
  }

  const totalScore = score
    ? Object.values(score.score).reduce((sum, item) => sum + item.score, 0)
    : 0
  const totalPossible = score
    ? Object.values(score.score).reduce((sum, item) => sum + item.totalScore, 0)
    : 0

  return (
    <div className="space-y-5">
      {/* 比赛头卡：名称 + 阶段 + 时间窗 + 倒计时 + 进度条 */}
      <section className="relative overflow-hidden rounded-[1.75rem] border border-[#e4e7ec] bg-[#14283d] px-6 py-7 text-white shadow-[0_24px_60px_-38px_rgba(20,40,61,0.55)] sm:px-9">
        <div className="absolute inset-0 forge-grid opacity-[0.12]" aria-hidden="true" />
        <div className="relative flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div className="min-w-0">
            <p className="flex items-center gap-2 text-[10px] font-bold uppercase tracking-[0.22em] text-white/45">
              <Upload className="h-3.5 w-3.5" />
              文件提交模式 · 河南版
            </p>
            <h1 className="font-display mt-2 break-words text-[clamp(1.9rem,6vw,2.75rem)] font-semibold leading-[1.08] tracking-[-0.04em]">
              {beat.contestName}
            </h1>
            <p className="mt-3 text-xs leading-6 text-white/55">
              选择与题目英文名同名的 C++ 源代码文件并提交。选手 {beat.realName}（{beat.username}）
            </p>
            <div className="mt-3 flex flex-wrap items-center gap-2 text-[11px] text-white/60">
              <Badge variant={phaseMeta.variant} dot>
                {phaseMeta.label}
              </Badge>
              <span className="font-mono">{fmtUnix(beat.startTime)} 开始</span>
              <span className="text-white/30">·</span>
              <span className="font-mono">{fmtUnix(beat.endTime)} 结束</span>
            </div>
          </div>
          <div className="flex items-center gap-5 rounded-2xl border border-white/10 bg-white/[0.06] px-5 py-4 backdrop-blur-sm">
            <div className="text-right">
              <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-white/40">
                {phase === 'before' ? '距离开始' : '剩余'}
              </p>
              <p
                className={`mt-1 font-mono text-3xl font-bold tabular-nums ${
                  remainingSecs <= 5 * 60 ? 'text-rose-300' : remainingSecs <= 30 * 60 ? 'text-amber-300' : 'text-white'
                }`}
                data-testid="henan-countdown"
              >
                {phase === 'before'
                  ? fmtDuration(remainingSecs - (beat.endTime - beat.startTime))
                  : phase === 'ended' || phase === 'publish'
                    ? '已结束'
                    : fmtDuration(remainingSecs)}
              </p>
            </div>
            <span className="h-10 w-px bg-white/10" />
            <div>
              <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-white/40">提交进度</p>
              <p className="mt-1 font-mono text-3xl font-bold text-white">
                {submittedCount}
                <span className="text-base font-medium text-white/50"> / {totalCount}</span>
              </p>
            </div>
          </div>
        </div>
        <div className="relative mt-5 h-1.5 overflow-hidden rounded-full bg-white/10" aria-label="比赛时间进度">
          <div
            className="h-full rounded-full bg-gradient-to-r from-forge-400 to-amber-300 transition-all duration-1000"
            style={{ width: `${phase === 'before' ? 0 : progressPercent}%` }}
          />
        </div>
      </section>

      {/* 公告 */}
      {beat.announcement && (
        <div className="flex items-start gap-3 rounded-2xl border border-sky-200 bg-sky-50 px-4 py-3 text-sm leading-6 text-sky-900">
          <Paperclip className="mt-0.5 h-4 w-4 shrink-0 text-sky-600" />
          <p>{beat.announcement}</p>
        </div>
      )}

      {/* 连接中断：提交与确认暂停 */}
      {connection === 'paused' && (
        <div className="flex items-start gap-3 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm leading-6 text-rose-900">
          <CloudOff className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />
          <p>与服务器的连接已中断，提交和确认操作已暂停。恢复后将继续倒计时与作答。</p>
        </div>
      )}

      {/* 封禁：只读 */}
      {banned && (
        <div className="flex items-start gap-3 rounded-2xl border border-rose-300 bg-rose-50 px-4 py-3 text-sm font-semibold leading-6 text-rose-900">
          <Ban className="mt-0.5 h-4 w-4 shrink-0" />
          <p>账号已被封禁，当前只能查看已开放的比赛内容，不能提交或确认。</p>
        </div>
      )}

      {/* 已最终确认：全部锁定 */}
      {confirmed && (
        <div className="flex items-start gap-3 rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm leading-6 text-emerald-900">
          <Lock className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />
          <p>已确认最终提交，所有题目已锁定。</p>
        </div>
      )}

      {/* 成绩发布 */}
      {phase === 'publish' && score?.resultsPublished && (
        <section className="app-surface rounded-2xl p-5" data-testid="henan-scores">
          <div className="flex flex-wrap items-center gap-3">
            <Trophy className="h-5 w-5 text-amber-500" />
            <h2 className="text-sm font-bold text-[#14283d]">比赛成绩</h2>
            <Badge variant="accent">成绩已发布</Badge>
            <span className="ml-auto flex items-center gap-4">
              <span className="flex items-center gap-1.5 text-xs font-bold text-slate-500">
                <Award className="h-3.5 w-3.5 text-teal-700" />
                总排名 第 {score.rank} 名
              </span>
              <span className="font-mono text-2xl font-bold text-[#14283d]">
                {totalScore}
                <span className="text-sm font-medium text-slate-400"> / {totalPossible}</span>
              </span>
            </span>
          </div>
          <p className="mt-2 text-xs text-slate-400">成绩已经发布，以下为本次比赛的最终成绩。</p>
          <div className="mt-4 space-y-2">
            {Object.entries(score.score).map(([name, item]) => {
              const open = scoreOpenProblem === name
              return (
                <div key={name} className="overflow-hidden rounded-xl border border-[#e4e7ec]">
                  <button
                    type="button"
                    onClick={() => setScoreOpenProblem(open ? null : name)}
                    aria-expanded={open}
                    className="flex w-full items-center gap-3 bg-[#fbfaf6] px-4 py-3 text-left transition hover:bg-[#f5f2e9]"
                  >
                    <FileCode2 className="h-4 w-4 text-forge-700" />
                    <span className="font-mono text-sm font-bold text-[#14283d]">{name}</span>
                    <Badge variant={item.score === item.totalScore ? 'success' : item.score > 0 ? 'warning' : 'danger'}>
                      {item.score} / {item.totalScore}
                    </Badge>
                    <span className="text-[11px] text-slate-400">已提交 {item.submitCount} 次</span>
                    {item.cases && item.cases.length > 0 && (
                      <span className="text-[11px] text-slate-400">· {item.cases.length} 个测试点</span>
                    )}
                    <span className="ml-auto text-xs font-bold text-slate-400">{open ? '收起' : '评测详情'}</span>
                  </button>
                  {open && (
                    <div className="border-t border-[#e4e7ec] bg-white px-4 py-3">
                      {item.cases && item.cases.length > 0 ? (
                        <ul className="grid gap-1.5 sm:grid-cols-2 xl:grid-cols-3">
                          {item.cases.map((testCase) => (
                            <li
                              key={testCase.index}
                              className="flex items-center gap-2 rounded-lg bg-[#fbfaf6] px-2.5 py-1.5 font-mono text-[11px]"
                            >
                              <span className="text-slate-400">#{testCase.index + 1}</span>
                              <span
                                className={
                                  testCase.verdict === 'AC'
                                    ? 'font-bold text-emerald-600'
                                    : 'font-bold text-rose-600'
                                }
                              >
                                {testCase.verdict}
                              </span>
                              <span className="text-slate-400">{testCase.score} 分</span>
                              {testCase.timeMs != null && (
                                <span className="ml-auto text-slate-400">{testCase.timeMs} ms</span>
                              )}
                            </li>
                          ))}
                        </ul>
                      ) : (
                        <p className="text-xs text-slate-400">成绩由管理员导入，无本地评测详情。</p>
                      )}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </section>
      )}

      {/* 开考前 / 未配置 */}
      {(phase === 'before' || phase === 'waiting') && (
        <div className="flex flex-col items-center gap-3 rounded-2xl border border-[#e4e7ec] bg-white px-6 py-14 text-center">
          <Clock3 className="h-8 w-8 text-slate-300" />
          <p className="text-base font-bold text-slate-700">
            {phase === 'waiting' ? '等待管理员配置' : '比赛尚未开始'}
          </p>
          <p className="max-w-md text-xs leading-6 text-slate-400">
            {phase === 'waiting'
              ? '管理员尚未添加比赛题目。'
              : '请等待比赛开始，页面将在开赛时自动刷新。'}
          </p>
        </div>
      )}

      {/* 题目提交 + 侧栏 */}
      {phase !== 'waiting' && (
        <div className="grid gap-5 xl:grid-cols-[1fr_340px]">
          <section className="space-y-3" aria-label="题目提交">
            <div className="flex flex-wrap items-center gap-3">
              <h2 className="text-sm font-bold text-[#14283d]">题目提交</h2>
              <span className="text-xs text-slate-400">以每道题最新一次提交作为最终代码</span>
              <span className="ml-auto flex items-center gap-2">
                {confirmed ? (
                  <Badge variant="success">
                    <Lock className="h-3 w-3" />
                    已确认最终提交
                  </Badge>
                ) : (
                  <Badge variant={phase === 'active' ? 'info' : 'default'}>
                    <LockOpen className="h-3 w-3" />
                    {phase === 'active' ? '仍可更新代码' : PHASE_META[phase].label}
                  </Badge>
                )}
              </span>
            </div>
            {problems.length === 0 && (
              <div className="rounded-2xl border border-[#e4e7ec] bg-white px-6 py-10 text-center text-sm text-slate-400">
                管理员尚未发布题目
              </div>
            )}
            {problems.map((problem) => {
              const file = picked[problem.name]
              const record = problem.record
              return (
                <article
                  key={problem.name}
                  className="app-surface rounded-2xl p-4"
                  data-problem={problem.name}
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="grid h-9 w-9 place-items-center rounded-xl bg-[#14283d] font-mono text-xs font-bold text-white">
                      {problem.displayName.slice(0, 2).toUpperCase()}
                    </span>
                    <div className="min-w-0">
                      <p className="font-mono text-sm font-bold text-[#14283d]">{problem.displayName}</p>
                      <p className="mt-0.5 text-[11px] text-slate-400">
                        题目英文名 <code className="rounded bg-[#f1f0ea] px-1 py-0.5 font-mono">{problem.name}</code>
                        ，提交文件必须为{' '}
                        <code className="rounded bg-[#f1f0ea] px-1 py-0.5 font-mono">{problem.name}.cpp</code>
                      </p>
                    </div>
                    <span className="ml-auto flex items-center gap-2">
                      {record ? (
                        <>
                          <Badge variant="success" dot>
                            已提交 {record.submitCount} 次
                          </Badge>
                          <button
                            type="button"
                            onClick={() => setCodeProblem(problem)}
                            className="inline-flex items-center gap-1 text-xs font-bold text-forge-700 hover:underline"
                          >
                            <Eye className="h-3.5 w-3.5" />
                            查看代码
                          </button>
                        </>
                      ) : (
                        <Badge variant="default" dot>
                          未提交
                        </Badge>
                      )}
                    </span>
                  </div>
                  <div className="mt-3 flex flex-wrap items-center gap-3 border-t border-[#eef0f3] pt-3">
                    <label className="inline-flex h-10 cursor-pointer items-center gap-2 rounded-xl border border-[#e4e7ec] bg-white px-4 text-sm font-bold text-slate-600 transition hover:border-[#d0d5dd] hover:text-slate-900">
                      <FileCode2 className="h-4 w-4" />
                      选择 {problem.name}.cpp
                      <input
                        type="file"
                        accept=".cpp"
                        className="hidden"
                        disabled={operationsPaused}
                        onChange={(event) => {
                          handlePick(problem.name, event.target.files?.[0])
                          event.target.value = ''
                        }}
                      />
                    </label>
                    {file ? (
                      <span className="text-xs font-semibold text-slate-500">
                        已选择 <b className="font-mono">{file.name}</b>（{formatBytes(file.size)}）
                      </span>
                    ) : (
                      <span className="text-xs text-slate-300">尚未选择文件</span>
                    )}
                    <Button
                      className="ml-auto"
                      onClick={() => void handleSubmit(problem)}
                      disabled={operationsPaused || !file}
                      loading={submitting === problem.name}
                      data-testid={`henan-submit-${problem.name}`}
                    >
                      {!submitting && <Send className="h-4 w-4" />}
                      提交
                    </Button>
                  </div>
                </article>
              )
            })}
          </section>

          {/* 侧栏：附件 + 最终确认 */}
          <div className="space-y-5">
            <section className="app-surface rounded-2xl p-5" aria-label="比赛附件">
              <h2 className="flex items-center gap-2 text-sm font-bold text-[#14283d]">
                <Paperclip className="h-4 w-4 text-forge-700" />
                比赛附件
              </h2>
              <p className="mt-1 text-[11px] leading-5 text-slate-400">
                点击后请查看浏览器下载列表，无需刷新或重复点击。
              </p>
              <div className="mt-3 space-y-2">
                {attachments.length === 0 && <p className="text-xs text-slate-300">暂无附件</p>}
                {attachments.map((file) => (
                  <a
                    key={file.name}
                    href={file.url}
                    download
                    className="flex items-center gap-2 rounded-xl border border-[#e4e7ec] bg-[#fbfaf6] px-3 py-2 text-xs transition hover:border-forge-300"
                  >
                    <Download className="h-3.5 w-3.5 text-forge-700" />
                    <span className="min-w-0 flex-1 truncate font-semibold text-slate-700">{file.name}</span>
                    <span className="text-slate-400">{formatBytes(file.size)}</span>
                  </a>
                ))}
              </div>
            </section>

            <section className="app-surface rounded-2xl border-amber-200/70 p-5" aria-label="最终确认">
              <h2 className="flex items-center gap-2 text-sm font-bold text-[#14283d]">
                <Lock className="h-4 w-4 text-amber-600" />
                最终确认
              </h2>
              <p className="mt-1 text-[11px] leading-5 text-slate-400">确认后将无法继续更新任何题目</p>
              <div className="mt-3 flex items-center justify-between rounded-xl bg-[#fbfaf6] px-3 py-2.5 text-xs font-semibold text-slate-600">
                已提交题目
                <span className="font-mono text-base font-bold text-[#14283d]">
                  {submittedCount} / {totalCount}
                </span>
              </div>
              <Button
                variant="outline"
                className="mt-3 w-full"
                disabled={operationsPaused || confirming}
                onClick={() => setConfirmOpen(true)}
                data-testid="henan-confirm-button"
              >
                <Lock className="h-4 w-4" />
                {confirmed ? '已确认最终提交' : '确认最终提交'}
              </Button>
            </section>

            <Button variant="ghost" size="sm" className="w-full" onClick={() => setPasswordOpen(true)}>
              <KeyRound className="h-3.5 w-3.5" />
              修改密码
            </Button>
          </div>
        </div>
      )}

      <div className="flex items-center">
        <Link
          to="/"
          className="inline-flex h-9 items-center gap-1 text-xs font-bold text-slate-400 transition hover:text-slate-700"
        >
          <ChevronLeft className="h-3.5 w-3.5" />
          返回大厅
        </Link>
      </div>
      {/* cid 留给后端接入时使用。 */}
      <span className="hidden" data-contest-id={cid} />

      {/* 查看代码 */}
      <Dialog
        open={codeProblem != null}
        onClose={() => setCodeProblem(null)}
        title={codeProblem ? `查看 ${codeProblem.name} 代码` : ''}
        eyebrow="当前保存的最新提交代码"
        width="lg"
      >
        {codeProblem?.record ? (
          <>
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <span className="rounded-lg border border-[#e4e7ec] bg-white px-2.5 py-1 font-semibold">
                文件 <b className="font-mono">{codeProblem.record.filename}</b>
              </span>
              <span className="rounded-lg border border-[#e4e7ec] bg-white px-2.5 py-1 font-semibold">
                提交于 <b className="font-mono">{fmtClock(codeProblem.record.submittedAt)}</b>
              </span>
              <span className="rounded-lg border border-[#e4e7ec] bg-white px-2.5 py-1 font-semibold">
                已提交 <b className="font-mono">{codeProblem.record.submitCount}</b> 次
              </span>
            </div>
            <pre className="mt-3 max-h-[55vh] overflow-auto rounded-xl bg-[#14283d] p-4 font-mono text-xs leading-5 text-slate-100">
              {codeProblem.record.code}
            </pre>
          </>
        ) : (
          <p className="py-6 text-center text-sm text-slate-400">本题未提交代码。</p>
        )}
      </Dialog>

      {/* 最终确认对话框 */}
      <Dialog
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title="确认最终提交"
        eyebrow="最终确认"
        footer={
          <>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>
              再检查一下
            </Button>
            <Button onClick={() => void handleConfirm()} loading={confirming} data-testid="henan-confirm-ok">
              <Lock className="h-4 w-4" />
              确认并锁定
            </Button>
          </>
        }
      >
        <p className="text-sm leading-6 text-slate-600">
          确认后将不能再次选择或提交代码，请检查所有题目均已提交正确文件。
        </p>
        <p className="mt-3 rounded-xl bg-[#fbfaf6] px-3 py-2.5 text-xs font-semibold text-slate-600">
          已提交题目 {submittedCount} / {totalCount}
        </p>
      </Dialog>

      {/* 修改密码 */}
      <PasswordDialog open={passwordOpen} onClose={() => setPasswordOpen(false)} />
    </div>
  )
}

function PasswordDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const toast = useToast()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) {
      setCurrent('')
      setNext('')
      setConfirm('')
    }
  }, [open])

  const submit = async () => {
    if (busy) return
    if (next !== confirm) {
      toast.error('两次输入的新密码不一致')
      return
    }
    setBusy(true)
    try {
      const result = await api.changePassword(current, next)
      if (result.status !== 'success') {
        toast.error('修改失败', result.message)
        return
      }
      toast.success('密码已修改')
      onClose()
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="修改密码"
      eyebrow="账号安全"
      width="sm"
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button onClick={() => void submit()} loading={busy}>
            修改密码
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <label className="block text-xs font-bold text-slate-500">
          当前密码
          <Input
            type="password"
            autoComplete="current-password"
            value={current}
            onChange={(event) => setCurrent(event.target.value)}
            className="mt-1.5"
          />
        </label>
        <label className="block text-xs font-bold text-slate-500">
          新密码
          <Input
            type="password"
            autoComplete="new-password"
            value={next}
            onChange={(event) => setNext(event.target.value)}
            className="mt-1.5"
          />
        </label>
        <label className="block text-xs font-bold text-slate-500">
          确认新密码
          <Input
            type="password"
            autoComplete="new-password"
            value={confirm}
            onChange={(event) => setConfirm(event.target.value)}
            className="mt-1.5"
          />
        </label>
      </div>
    </Dialog>
  )
}
