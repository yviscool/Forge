import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertCircle,
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Check,
  ChevronDown,
  CloudUpload,
  Code2,
  Copy,
  Download,
  FileCode2,
  FileText,
  FolderOpen,
  Gauge,
  History,
  Keyboard,
  LockKeyhole,
  Megaphone,
  Monitor,
  Send,
  Sparkles,
  Trophy,
  Unlock,
  Upload,
  UserRound,
  X,
  ShieldAlert,
} from 'lucide-react'
import {
  api,
  newIdempotencyKey,
  type ACMRankEntry,
  type BroadcastMessage,
  type Problem,
  type RankEntry,
  type Submission,
} from '../api/client'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Dialog } from '../components/ui/dialog'
import {
  EmptyState,
  ErrorState,
  InlineError,
  LoadingBlock,
} from '../components/ui/empty-state'
import { Select } from '../components/ui/input'
import { useToast } from '../components/ui/toast'
import { BroadcastBanner } from '../components/BroadcastBanner'
import { ContestTimer } from '../components/ContestTimer'
import { GESPQuizView } from '../components/GESPQuizView'
import { MarkdownView } from '../components/MarkdownView'
import { Scoreboard } from '../components/Scoreboard'
import { EditorLoading } from '../components/ui/editor-loading'
import { useLocale } from '../i18n/dict'
import { useSSE } from '../realtime/useSSE'
import type { BroadcastAlert } from '../store/useStore'
import { contestProblemDraftKey, useStore } from '../store/useStore'
import {
  MAX_SOURCE_BYTES,
  SOURCE_ACCEPT,
  allowedLanguageOptions,
  byteLength,
  editorLanguage,
  formatBytes,
  languageLabel,
  matchFolderToProblems,
  normalizeLanguage,
  readSourceFile,
  selectSourceFolder,
  type FolderFileMatch,
  type FolderSelection,
  type SupportedLanguage,
} from '../features/student/submissionFiles'

const MonacoEditor = React.lazy(() =>
  import('../components/MonacoEditor').then((module) => ({ default: module.MonacoEditor }))
)
const UbuntuDesktop = React.lazy(() =>
  import('../components/UbuntuDesktop').then((module) => ({ default: module.UbuntuDesktop }))
)

type WorkspacePanel = 'editor' | 'history' | 'ranking'

const verdictMeta: Record<
  string,
  { label: string; variant: 'success' | 'danger' | 'warning' | 'info' | 'default' }
> = {
  accepted: { label: 'Accepted', variant: 'success' },
  wrong_answer: { label: 'Wrong Answer', variant: 'danger' },
  presentation_error: { label: 'Presentation Error', variant: 'warning' },
  time_limit: { label: 'Time Limit', variant: 'warning' },
  memory_limit: { label: 'Memory Limit', variant: 'danger' },
  output_limit: { label: 'Output Limit', variant: 'warning' },
  runtime_error: { label: 'Runtime Error', variant: 'danger' },
  compile_error: { label: 'Compile Error', variant: 'danger' },
  checker_error: { label: 'Checker Error', variant: 'danger' },
  system_error: { label: 'System Error', variant: 'danger' },
  skipped: { label: 'Skipped', variant: 'default' },
  queued: { label: 'Queued', variant: 'info' },
  judging: { label: 'Judging', variant: 'info' },
  // NOI 全真模拟进行中的脱敏判定：只承认「已提交」，细节赛后公布。
  submitted: { label: '已提交 · 赛后公布', variant: 'info' },
}

function VerdictBadge({ submission }: { submission: Pick<Submission, 'verdict' | 'score'> }) {
  const meta = verdictMeta[submission.verdict] ?? {
    label: submission.verdict || 'Unknown',
    variant: 'default' as const,
  }
  return (
    <Badge variant={meta.variant} dot>
      {meta.label}
      {submission.verdict === 'accepted' ? ` · ${submission.score}` : ''}
    </Badge>
  )
}

function formatDateTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function submissionCode(value: string): string {
  try {
    const parsed = JSON.parse(value) as unknown
    if (parsed && typeof parsed === 'object') return JSON.stringify(parsed, null, 2)
  } catch {
    // Regular source code is intentionally not JSON.
  }
  return value
}

export function contestPhase(
  contest?: {
    status?: string
    windowEnabled?: boolean
    startTime?: string
    endTime?: string
    extensions?: Record<string, number>
  },
  userId?: string,
  now = Date.now()
): 'draft' | 'upcoming' | 'running' | 'finished' {
  if (!contest) return 'draft'
  if (contest.status === 'finished') return 'finished'
  if (contest.status !== 'running') return 'draft'
  if (!contest.windowEnabled) return 'running'
  if (contest.startTime && now < new Date(contest.startTime).getTime()) return 'upcoming'
  const extension = (userId ? contest.extensions?.[userId] : 0) ?? 0
  if (contest.endTime && now >= new Date(contest.endTime).getTime() + extension * 60_000) {
    return 'finished'
  }
  return 'running'
}

function NoticeDrawer({
  open,
  onClose,
  notice,
  messages,
}: {
  open: boolean
  onClose: () => void
  notice?: string
  messages: BroadcastMessage[]
}) {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="考场通告"
      eyebrow="Announcements"
      width="md"
      footer={
        <Button variant="secondary" onClick={onClose}>
          关闭
        </Button>
      }
    >
      <div className="space-y-3">
        {notice && (
          <article className="rounded-2xl border border-amber-200 bg-amber-50/80 p-4">
            <p className="app-eyebrow text-amber-700">当前置顶公告</p>
            <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-amber-950">{notice}</p>
          </article>
        )}
        {messages.map((message) => (
          <article
            key={message.id}
            className={
              message.level === 'urgent'
                ? 'rounded-2xl border border-rose-200 bg-rose-50/70 p-4'
                : message.level === 'warning'
                  ? 'rounded-2xl border border-amber-200 bg-amber-50/60 p-4'
                  : 'rounded-2xl border border-[#e4e7ec] bg-white p-4'
            }
          >
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-sm font-extrabold text-[#14283d]">
                {message.title || '考场通知'}
              </p>
              <span className="font-mono text-[11px] text-slate-400">
                {formatDateTime(message.createdAt)}
              </span>
            </div>
            <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-slate-600">
              {message.content}
            </p>
            <p className="mt-2 text-[11px] text-slate-400">发布人 · {message.senderName}</p>
          </article>
        ))}
        {!notice && !messages.length && (
          <EmptyState
            icon={Megaphone}
            title="暂无考场通告"
            description="监考教师发布的消息会实时出现在这里。"
          />
        )}
      </div>
    </Dialog>
  )
}

function ProblemStatement({
  problem,
  copied,
  onCopy,
  onDownloadPdf,
  onDownloadHtml,
  exporting,
  onSubmit,
  submitting,
  submitLabel,
}: {
  problem: Problem
  copied: boolean
  onCopy: () => void
  onDownloadPdf: () => void
  onDownloadHtml: () => void
  exporting: 'pdf' | 'html' | null
  onSubmit: () => void
  submitting: boolean
  submitLabel: string
}) {
  return (
    <section className="flex min-h-0 flex-col overflow-hidden rounded-2xl border border-[#e4e7ec] bg-[#fffefa]">
      <header className="border-b border-[#eaecf0] px-5 py-4 sm:px-6">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="app-eyebrow">Problem {problem.code}</p>
            <h1 className="mt-1.5 text-xl font-extrabold tracking-tight text-[#14283d] sm:text-2xl">
              {problem.title}
            </h1>
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant="outline">{problem.timeLimitMs} ms</Badge>
            <Badge variant="outline">{problem.memoryLimitMib} MiB</Badge>
            {problem.ioMode === 'file' && (
              <Badge variant="warning">
                文件 IO · {problem.inFile || 'input'} / {problem.outFile || 'output'}
              </Badge>
            )}
            <button
              type="button"
              onClick={onDownloadPdf}
              disabled={exporting != null}
              className="inline-flex h-7 items-center gap-1 rounded-lg border border-[#e4e7ec] bg-white px-2 text-[10px] font-bold text-slate-500 transition hover:border-[#cfd6df] hover:text-slate-800 disabled:opacity-50"
              title="下载认证版 A4 PDF"
            >
              <Download className="h-3 w-3" />
              {exporting === 'pdf' ? '导出中' : 'PDF'}
            </button>
            <button
              type="button"
              onClick={onDownloadHtml}
              disabled={exporting != null}
              className="inline-flex h-7 items-center gap-1 rounded-lg border border-[#e4e7ec] bg-white px-2 text-[10px] font-bold text-slate-500 transition hover:border-[#cfd6df] hover:text-slate-800 disabled:opacity-50"
              title="下载 CCF 排版 HTML"
            >
              <Download className="h-3 w-3" />
              {exporting === 'html' ? '导出中' : 'HTML'}
            </button>
          </div>
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5 scrollbar-slim sm:px-6">
        <div className="space-y-6">
          <div>
            <h2 className="mb-2 text-[11px] font-extrabold uppercase tracking-[0.16em] text-slate-400">
              题目描述
            </h2>
            <MarkdownView source={problem.statement} />
          </div>

          {problem.input && (
            <div>
              <h2 className="mb-2 text-[11px] font-extrabold uppercase tracking-[0.16em] text-slate-400">
                输入格式
              </h2>
              <div className="rounded-xl border border-[#eaecf0] bg-[#fbfaf6] px-4 py-3 text-sm leading-6 text-slate-700">
                <MarkdownView source={problem.input} />
              </div>
            </div>
          )}

          {problem.output && (
            <div>
              <h2 className="mb-2 text-[11px] font-extrabold uppercase tracking-[0.16em] text-slate-400">
                输出格式
              </h2>
              <div className="rounded-xl border border-[#eaecf0] bg-[#fbfaf6] px-4 py-3 text-sm leading-6 text-slate-700">
                <MarkdownView source={problem.output} />
              </div>
            </div>
          )}

          {problem.examples && (
            <div>
              <div className="mb-2 flex items-center justify-between gap-3">
                <h2 className="text-[11px] font-extrabold uppercase tracking-[0.16em] text-slate-400">
                  样例
                </h2>
                <button
                  type="button"
                  onClick={onCopy}
                  className="inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-[11px] font-bold text-slate-500 transition hover:bg-slate-100 hover:text-slate-800"
                >
                  {copied ? (
                    <Check className="h-3.5 w-3.5 text-emerald-600" />
                  ) : (
                    <Copy className="h-3.5 w-3.5" />
                  )}
                  {copied ? '已复制' : '复制样例'}
                </button>
              </div>
              <pre className="overflow-x-auto rounded-xl border border-[#1f3247] bg-[#101f2e] p-4 font-mono text-xs leading-6 text-slate-200">
                {problem.examples}
              </pre>
            </div>
          )}

          {problem.constraints && (
            <div>
              <h2 className="mb-2 text-[11px] font-extrabold uppercase tracking-[0.16em] text-slate-400">
                数据范围与提示
              </h2>
              <div className="rounded-xl border border-amber-200/70 bg-amber-50/55 px-4 py-3">
                <MarkdownView
                  source={problem.constraints}
                  className="text-[13px] leading-6 text-amber-950"
                />
              </div>
            </div>
          )}
        </div>
      </div>

      <footer className="flex items-center justify-between gap-3 border-t border-[#eaecf0] bg-[#fbfaf6] px-5 py-3">
        <p className="hidden text-[11px] text-slate-400 sm:block">
          草稿自动保存在本机；切换试题不会丢失。
        </p>
        <Button size="sm" className="ml-auto gap-1.5" onClick={onSubmit} loading={submitting}>
          <Send className="h-3.5 w-3.5" />
          {submitLabel}
        </Button>
      </footer>
    </section>
  )
}

function HistoryPanel({
  submissions,
  onLoad,
}: {
  submissions: Submission[]
  onLoad: (submission: Submission) => void
}) {
  if (!submissions.length) {
    return (
      <EmptyState
        icon={History}
        title="还没有提交记录"
        description="提交后会在这里保留时间、判定与得分，可一键载回编辑器继续修改。"
      />
    )
  }

  return (
    <div className="space-y-2">
      {submissions.map((submission, index) => (
        <article
          key={submission.id}
          className="rounded-xl border border-[#e4e7ec] bg-white p-3.5 transition hover:border-[#cfd6df]"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <span className="rounded-md bg-[#f2f4f7] px-1.5 py-0.5 font-mono text-[10px] font-bold text-slate-600">
                #{submissions.length - index}
              </span>
              <VerdictBadge submission={submission} />
              <span className="font-mono text-[11px] text-slate-500">{submission.score} pts</span>
            </div>
            <span className="font-mono text-[10px] text-slate-400">
              {formatDateTime(submission.submittedAt)}
            </span>
          </div>
          <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
            <div className="flex flex-wrap gap-2 text-[11px] text-slate-500">
              <span>{languageLabel(submission.language)}</span>
              <span className="text-slate-300">/</span>
              <span>{formatBytes(byteLength(submission.code))}</span>
              {submission.cases?.length ? (
                <>
                  <span className="text-slate-300">/</span>
                  <span>{submission.cases.length} 个测试点</span>
                </>
              ) : null}
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onLoad(submission)}
              className="h-7 px-2 text-[11px]"
            >
              载入编辑器
              <ArrowRight className="h-3 w-3" />
            </Button>
          </div>
        </article>
      ))}
      <p className="px-1 pt-1 text-center text-[10px] text-slate-400">
        最多显示最近 300 条提交记录
      </p>
    </div>
  )
}

export function StudentWorkspace() {
  const { cid = '' } = useParams<{ cid: string }>()
  const { t } = useLocale()
  const toast = useToast()
  const queryClient = useQueryClient()
  const {
    session,
    drafts,
    setDraft,
    desktopMode,
    setDesktopMode,
    setActiveAlert,
    serverTimeOffset,
    setContestUnlocked,
  } = useStore()

  const [activePid, setActivePid] = useState('')
  const [language, setLanguage] = useState<SupportedLanguage>('cpp')
  const [copied, setCopied] = useState(false)
  const [activePanel, setActivePanel] = useState<WorkspacePanel>('editor')
  const [notificationsOpen, setNotificationsOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [sourceError, setSourceError] = useState('')
  const [exporting, setExporting] = useState<'pdf' | 'html' | null>(null)
  const [folder, setFolder] = useState<FolderSelection | null>(null)
  const [folderMatches, setFolderMatches] = useState<FolderFileMatch[]>([])
  const [hydratedProblem, setHydratedProblem] = useState('')
  const [lastResult, setLastResult] = useState<Submission | null>(null)
  const [streamState, setStreamState] = useState<'connecting' | 'live' | 'offline'>(
    'connecting'
  )
  const [zipPassword, setZipPassword] = useState('')
  const [zipUnlocking, setZipUnlocking] = useState(false)
  const [zipError, setZipError] = useState('')
  const singleFileRef = useRef<HTMLInputElement>(null)
  const folderFileRef = useRef<HTMLInputElement>(null)
  const answerFileRef = useRef<HTMLInputElement>(null)
  const pendingSubmissionRef = useRef<{ fingerprint: string; key: string } | null>(null)

  const contestQuery = useQuery({
    queryKey: ['contest', cid],
    queryFn: () => api.getContest(cid),
    enabled: Boolean(cid),
  })
  const contest = contestQuery.data
  // Query gating is fail-closed: no paper, submission history, or ranking is
  // requested until the signed server grant says the exam is open. Local
  // storage must never override a missing/expired server grant.
  const zipLocked = Boolean(contest?.zipProtected) && !contest?.zipUnlocked
  const paperReady = Boolean(contest) && !zipLocked
  // NOI 全真模拟进行中：服务端已对判定/榜单脱敏，前端同步隐藏入口并提示。
  const noiMasked = contest?.mode === 'noi' && contest?.status === 'running'
  const problemsQuery = useQuery({
    queryKey: ['problems', cid],
    queryFn: () => api.problems(cid),
    enabled: Boolean(cid) && paperReady,
  })
  const submissionsQuery = useQuery({
    queryKey: ['submissions', cid],
    queryFn: () => api.submissions(cid),
    enabled: Boolean(cid) && paperReady,
  })
  const rankingQuery = useQuery({
    queryKey: ['ranking', cid],
    queryFn: () => api.ranking(cid),
    enabled: Boolean(cid) && paperReady && !noiMasked,
  })
  const messagesQuery = useQuery({
    queryKey: ['messages', cid],
    queryFn: () => api.messages(cid),
    enabled: Boolean(cid),
  })

  const problems = useMemo(
    () => (paperReady ? problemsQuery.data ?? [] : []),
    [paperReady, problemsQuery.data]
  )
  const submissions = useMemo(
    () => (paperReady ? submissionsQuery.data ?? [] : []),
    [paperReady, submissionsQuery.data]
  )
  const ranks = useMemo(
    () => (paperReady ? rankingQuery.data ?? [] : []),
    [paperReady, rankingQuery.data]
  )
  const messages = messagesQuery.data ?? []
  const secondaryFailures = [
    submissionsQuery.isError
      ? { key: 'submissions', label: '提交记录', cause: submissionsQuery.error }
      : null,
    rankingQuery.isError && !noiMasked
      ? { key: 'ranking', label: '排行榜', cause: rankingQuery.error }
      : null,
    messagesQuery.isError
      ? { key: 'messages', label: '通知与广播', cause: messagesQuery.error }
      : null,
  ].filter((item): item is NonNullable<typeof item> => item !== null)
  const retrySecondaryQueries = () => {
    const jobs: Array<Promise<unknown>> = []
    if (submissionsQuery.isError) jobs.push(submissionsQuery.refetch())
    if (rankingQuery.isError && !noiMasked) jobs.push(rankingQuery.refetch())
    if (messagesQuery.isError) jobs.push(messagesQuery.refetch())
    void Promise.all(jobs)
  }

  const activeProblem = useMemo(
    () => problems.find((problem) => problem.id === activePid) ?? problems[0],
    [activePid, problems]
  )

  useEffect(() => {
    if (!activePid && problems.length) setActivePid(problems[0].id)
  }, [activePid, problems])

  const languageOptions = useMemo(
    () => allowedLanguageOptions(contest?.allowedLanguages),
    [contest?.allowedLanguages]
  )

  useEffect(() => {
    if (!activeProblem || activeProblem.taskType === 'choice') return
    if (languageOptions.length && !languageOptions.some((option) => option.value === language)) {
      setLanguage(languageOptions[0].value)
    }
  }, [activeProblem, language, languageOptions])

  const draftKey = activeProblem
    ? contestProblemDraftKey(session?.userId, cid, activeProblem.id)
    : ''
  const draftValue = draftKey ? drafts[draftKey] ?? '' : ''
  const answerOnly = activeProblem?.taskType === 'answers_only'
  const code = useMemo(
    () => (answerOnly ? draftValue : submissionCode(draftValue)),
    [answerOnly, draftValue]
  )
  const codeBytes = byteLength(code)
  const [phaseNow, setPhaseNow] = useState(() => Date.now() + serverTimeOffset)

  useEffect(() => {
    setPhaseNow(Date.now() + serverTimeOffset)
    const tick = window.setInterval(
      () => setPhaseNow(Date.now() + serverTimeOffset),
      1_000
    )
    return () => window.clearInterval(tick)
  }, [serverTimeOffset])

  const phase = contestPhase(contest, session?.userId, phaseNow)
  const hasSubmissionLanguage =
    activeProblem?.taskType === 'answers_only' || languageOptions.length > 0
  const canSubmit =
    phase === 'running' &&
    Boolean(activeProblem) &&
    activeProblem?.taskType !== 'choice' &&
    hasSubmissionLanguage

  // The contest submission feed is intentionally public for live ranking, but
  // source code and history belong to the signed-in candidate. Never derive
  // personal counters or the "load into editor" action from another account.
  const mySubmissions = useMemo(
    () => submissions.filter((submission) => submission.userId === session?.userId),
    [session?.userId, submissions]
  )

  const problemSubmissions = useMemo(
    () =>
      mySubmissions
        .filter((submission) => submission.problemID === activeProblem?.id)
        .sort(
          (a, b) => new Date(b.submittedAt).getTime() - new Date(a.submittedAt).getTime()
        ),
    [activeProblem?.id, mySubmissions]
  )

  const sessionUserId = session?.userId
  const myRank = useMemo(() => {
    if (!sessionUserId) return null
    const index = ranks.findIndex((entry) => entry.userId === sessionUserId)
    return index >= 0 ? index + 1 : null
  }, [ranks, sessionUserId])

  const solvedCount = useMemo(
    () =>
      new Set(
        mySubmissions
          .filter((submission) => submission.verdict === 'accepted')
          .map((submission) => submission.problemID)
      ).size,
    [mySubmissions]
  )

  const messageCount = messages.length + (contest?.notice ? 1 : 0)

  useEffect(() => {
    if (!activeProblem || hydratedProblem === activeProblem.id) return
    setHydratedProblem(activeProblem.id)
    setLastResult(null)
  }, [activeProblem, hydratedProblem])

  useSSE(cid, (type, data) => {
    if (type === 'stream.connected') {
      setStreamState('live')
    } else if (type === 'stream.connecting') {
      setStreamState('connecting')
    } else if (type === 'stream.offline') {
      setStreamState('offline')
    } else if (type === 'stream.unauthorized') {
      setStreamState('offline')
      toast.error('实时连接已失效', '请刷新页面重新登录，评测结果仍会保留在服务端。')
    } else if (type === 'stream.overflow') {
      void queryClient.invalidateQueries()
      toast.warning('实时事件积压', '正在从服务器重新同步最新提交与排名。')
    } else if (type === 'submission.created') {
      void queryClient.invalidateQueries({ queryKey: ['submissions', cid] })
    } else if (type === 'submission.judged') {
      void queryClient.invalidateQueries({ queryKey: ['submissions', cid] })
      void queryClient.invalidateQueries({ queryKey: ['ranking', cid] })
      const judged = data as Submission
      if (judged && judged.userId === session?.userId) setLastResult(judged)
    } else if (type === 'ranking.updated') {
      void queryClient.invalidateQueries({ queryKey: ['ranking', cid] })
    } else if (
      type === 'contest.updated' ||
      type === 'contest.started' ||
      type === 'contest.finished' ||
      type === 'contest.notice' ||
      type === 'contest.window_updated' ||
      type === 'contest.extension_updated'
    ) {
      void queryClient.invalidateQueries({ queryKey: ['contest', cid] })
    } else if (type === 'contest.broadcast') {
      setActiveAlert(data as BroadcastAlert)
      void queryClient.invalidateQueries({ queryKey: ['messages', cid] })
    } else if (
      type === 'problem.created' ||
      type === 'problem.updated' ||
      type === 'problem.deleted' ||
      type === 'problems.reordered'
    ) {
      void queryClient.invalidateQueries({ queryKey: ['problems', cid] })
    }
  })

  const setCode = (value: string) => {
    if (!activeProblem) return
    setDraft(contestProblemDraftKey(session?.userId, cid, activeProblem.id), value)
  }

  const copySamples = async () => {
    if (!activeProblem?.examples) return
    try {
      await navigator.clipboard.writeText(activeProblem.examples)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1800)
    } catch {
      toast.error('无法访问剪贴板', '请手动选择样例文本复制。')
    }
  }

  const handleZipUnlock = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!zipPassword.trim() || zipUnlocking) return
    setZipUnlocking(true)
    setZipError('')
    try {
      await api.unlockZip(cid, zipPassword)
      const fresh = await api.getContest(cid)
      queryClient.setQueryData(['contest', cid], fresh)
      if (!fresh.zipUnlocked) {
        throw new Error('服务端未确认解锁，请重新登录后再试。')
      }
      setContestUnlocked(cid, true)
      setZipPassword('')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['problems', cid] }),
        queryClient.invalidateQueries({ queryKey: ['submissions', cid] }),
        queryClient.invalidateQueries({ queryKey: ['ranking', cid] }),
      ])
    } catch (cause) {
      setZipError(cause instanceof Error ? cause.message : '解锁失败，请重试。')
    } finally {
      setZipUnlocking(false)
    }
  }

  const downloadStatement = async (format: 'pdf' | 'html') => {
    if (!activeProblem || exporting) return
    setExporting(format)
    try {
      if (format === 'pdf') {
        await api.downloadProblemPdf(cid, activeProblem.id, activeProblem.code)
      } else {
        await api.downloadProblemHtml(cid, activeProblem.id, activeProblem.code)
      }
      toast.success('导出已开始', `${activeProblem.code} · ${format.toUpperCase()}`)
    } catch (cause) {
      toast.error(
        '导出失败',
        cause instanceof Error ? cause.message : '请检查登录状态后重试。'
      )
    } finally {
      setExporting(null)
    }
  }

  const commitSubmit = async () => {
    if (!activeProblem) return
    setConfirmOpen(false)
    setSubmitting(true)
    setSourceError('')
    const submitLanguage = activeProblem.taskType === 'answers_only' ? 'answer' : language
    const fingerprint = `${activeProblem.id}\u0000${submitLanguage}\u0000${code}`
    const pending =
      pendingSubmissionRef.current?.fingerprint === fingerprint
        ? pendingSubmissionRef.current
        : { fingerprint, key: newIdempotencyKey() }
    pendingSubmissionRef.current = pending
    try {
      const result = await api.submit(cid, {
        problemID: activeProblem.id,
        language: submitLanguage,
        code,
      }, pending.key)
      pendingSubmissionRef.current = null
      setLastResult(result)
      toast.success(
        '提交成功',
        `${activeProblem.code} · ${verdictMeta[result.verdict]?.label ?? result.verdict}`
      )
      setActivePanel('history')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['submissions', cid] }),
        queryClient.invalidateQueries({ queryKey: ['ranking', cid] }),
      ])
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : '请稍后重试。'
      setSourceError(message)
      toast.error('提交失败', message)
      if (message.includes('ip_mismatch')) {
        toast.error('机位校验失败', '账号已绑定到其他考试机位，请联系监考教师处理。')
      }
    } finally {
      setSubmitting(false)
    }
  }

  const submit = async () => {
    if (!activeProblem || submitting) return
    if (!canSubmit) {
      toast.error('当前不可提交', '比赛尚未开始或已经结束。')
      return
    }
    if (!code.trim()) {
      toast.error(
        answerOnly ? '答案为空' : '代码为空',
        answerOnly ? '请先填写或载入答案再提交。' : '请先编写或载入代码再提交。'
      )
      return
    }
    if (codeBytes > MAX_SOURCE_BYTES) {
      toast.error(
        answerOnly ? '答案超过上限' : '代码超过上限',
        `当前 ${formatBytes(codeBytes)}，上限 ${formatBytes(MAX_SOURCE_BYTES)}。`
      )
      return
    }
    if (problemSubmissions.length > 0) {
      setConfirmOpen(true)
      return
    }
    await commitSubmit()
  }

  const onEditorKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
      event.preventDefault()
      void submit()
    }
  }

  const handleSingleFile = async (file: File | undefined) => {
    if (!file || !activeProblem) return
    setSourceError('')
    try {
      const detected = normalizeLanguage(
        file.name.slice(file.name.lastIndexOf('.') + 1)
      )
      if (!detected) {
        throw new Error('仅支持 .cpp / .cc / .cxx / .c / .py / .go 源文件。')
      }
      if (!languageOptions.some((option) => option.value === detected)) {
        throw new Error(`本场比赛不允许提交 ${languageLabel(detected)}。`)
      }
      const text = await readSourceFile(file)
      setDraft(contestProblemDraftKey(session?.userId, cid, activeProblem.id), text)
      setLanguage(detected)
      setHydratedProblem(activeProblem.id)
      toast.success('已载入源文件', `${file.name} · ${formatBytes(file.size)}`)
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : '源文件读取失败。'
      setSourceError(message)
      toast.error('无法载入源文件', message)
    }
  }

  const handleAnswerFile = async (file: File | undefined) => {
    if (!file || !activeProblem) return
    setSourceError('')
    try {
      if (file.size > MAX_SOURCE_BYTES) {
        throw new Error(
          `答案文件 ${formatBytes(file.size)} 超过单文件上限 ${formatBytes(MAX_SOURCE_BYTES)}。`
        )
      }
      const text = await file.text()
      if (!text.trim()) throw new Error('答案文件为空。')
      setDraft(contestProblemDraftKey(session?.userId, cid, activeProblem.id), text)
      setHydratedProblem(activeProblem.id)
      toast.success('已载入答案文件', `${file.name} · ${formatBytes(file.size)}`)
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : '答案文件读取失败。'
      setSourceError(message)
      toast.error('无法载入答案文件', message)
    }
  }

  const handleFolder = (fileList: FileList | null) => {
    if (!fileList?.length) return
    setSourceError('')
    try {
      const selection = selectSourceFolder(fileList)
      const matches = matchFolderToProblems(selection, problems)
      setFolder(selection)
      setFolderMatches(matches)
      const matched = matches.filter((item) => item.problem).length
      toast.success(
        '文件夹已解析',
        `${selection.sourceCount} 个源码文件 · ${formatBytes(
          selection.totalBytes
        )} · ${matched} 个可自动对应试题`
      )
      if (selection.warnings.length) setSourceError(selection.warnings.join(' '))
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : '文件夹解析失败。'
      setFolder(null)
      setFolderMatches([])
      setSourceError(message)
      toast.error('无法读取文件夹', message)
    }
  }

  const loadFolderFile = async (match: FolderFileMatch) => {
    const problem = match.problem ?? activeProblem
    if (!problem || !match.language) return
    try {
      const text = await readSourceFile(match.file)
      setDraft(contestProblemDraftKey(session?.userId, cid, problem.id), text)
      setLanguage(match.language)
      setActivePid(problem.id)
      setHydratedProblem(problem.id)
      setUploadOpen(false)
      setActivePanel('editor')
      toast.success('已载入试题代码', `${problem.code} · ${match.file.name}`)
    } catch (cause) {
      toast.error('载入失败', cause instanceof Error ? cause.message : '请重试。')
    }
  }

  const loadSubmission = (submission: Submission) => {
    if (!submission.code) {
      toast.error('代码不可用', '该提交未返回源码，无法载入。')
      return
    }
    setDraft(
      contestProblemDraftKey(session?.userId, cid, submission.problemID),
      submission.code
    )
    setActivePid(submission.problemID)
    setHydratedProblem(submission.problemID)
    const detected = normalizeLanguage(submission.language)
    if (detected && languageOptions.some((option) => option.value === detected)) {
      setLanguage(detected)
    }
    setActivePanel('editor')
    toast.success('已载入历史代码', '可以继续修改后再提交。')
  }

  if (contestQuery.isLoading) {
    return <LoadingBlock label="正在进入考场…" />
  }

  if (contestQuery.isError) {
    return (
      <ErrorState
        message={
          contestQuery.error instanceof Error
            ? contestQuery.error.message
            : '无法加载比赛，请检查登录状态。'
        }
        action={
          <Button
            onClick={() => {
              void contestQuery.refetch()
            }}
          >
            重新加载
          </Button>
        }
      />
    )
  }

  if (zipLocked) {
    return (
      <div className="mx-auto flex min-h-[70vh] max-w-xl items-center px-4">
        <section className="w-full overflow-hidden rounded-[1.75rem] border border-amber-200 bg-[#fffefa] shadow-[0_24px_60px_-38px_rgba(120,53,15,0.45)]">
          <div className="relative bg-[#14283d] px-6 py-7 text-white">
            <div className="absolute inset-0 forge-grid opacity-15" aria-hidden="true" />
            <div className="relative">
              <Link
                to="/"
                className="inline-flex items-center gap-1.5 text-[11px] font-bold text-white/55 transition hover:text-white"
              >
                <ArrowLeft className="h-3.5 w-3.5" />
                返回比赛大厅
              </Link>
              <div className="mt-5 flex items-center gap-3">
                <span className="grid h-11 w-11 place-items-center rounded-2xl bg-amber-400/15 text-amber-300">
                  <LockKeyhole className="h-5 w-5" />
                </span>
                <div>
                  <p className="app-eyebrow text-white/40">Encrypted paper</p>
                  <h1 className="font-display mt-0.5 text-2xl font-semibold">
                    {contest?.name || '考场试卷已加密'}
                  </h1>
                </div>
              </div>
            </div>
          </div>
          <form onSubmit={(event) => void handleZipUnlock(event)} className="space-y-4 p-6">
            <p className="text-sm leading-6 text-slate-600">
              本场试题、提交记录与排名处于加密状态。输入监考教师下发的考场口令后，服务端会签发仅限本账号、本场比赛的限时访问凭证。
            </p>
            <label className="block space-y-1.5">
              <span className="text-xs font-bold text-slate-700">考场解锁口令</span>
              <input
                type="password"
                autoFocus
                autoComplete="off"
                value={zipPassword}
                onChange={(event) => {
                  setZipPassword(event.target.value)
                  if (zipError) setZipError('')
                }}
                placeholder="输入监考教师下发的口令"
                aria-invalid={Boolean(zipError)}
                className="app-field text-center font-mono tracking-wider"
              />
            </label>
            {zipError && (
              <div
                role="alert"
                className="rounded-xl border border-rose-200 bg-rose-50 px-3.5 py-2.5 text-xs font-semibold leading-5 text-rose-700"
              >
                {zipError}
              </div>
            )}
            <Button
              type="submit"
              size="lg"
              className="w-full"
              loading={zipUnlocking}
              disabled={!zipPassword.trim()}
            >
              <Unlock className="h-4 w-4" />
              解锁试卷
            </Button>
            <p className="text-center text-[11px] leading-5 text-slate-400">
              口令不会保存在浏览器。解锁凭证最长有效 8 小时，退出登录后立即失效。
            </p>
          </form>
        </section>
      </div>
    )
  }

  if (problemsQuery.isLoading) {
    return <LoadingBlock label="正在展开试卷…" />
  }

  if (problemsQuery.isError) {
    return (
      <ErrorState
        message={
          problemsQuery.error instanceof Error
            ? problemsQuery.error.message
            : '无法加载试题，请检查登录状态。'
        }
        action={
          <Button onClick={() => void problemsQuery.refetch()}>
            重新加载
          </Button>
        }
      />
    )
  }

  if (desktopMode && contest && activeProblem) {
    return (
      <>
        {secondaryFailures.length > 0 && (
          <InlineError
            className="mb-3"
            title="部分考试数据加载失败"
            message={secondaryFailures
              .map(
                (failure) =>
                  `${failure.label}: ${
                    failure.cause instanceof Error ? failure.cause.message : '请重试'
                  }`
              )
              .join('；')}
            action={
              <Button size="sm" variant="outline" onClick={retrySecondaryQueries}>
                重试
              </Button>
            }
          />
        )}
        <React.Suspense fallback={<EditorLoading fullscreen />}>
          <UbuntuDesktop
            contest={contest}
            problems={problems}
            activeProblem={activeProblem}
            setActiveProblem={(problem) => setActivePid(problem.id)}
            code={code}
            setCode={setCode}
            onExitDesktop={() => setDesktopMode(false)}
            onSubmit={() => {
              void submit()
            }}
            submitting={submitting}
          />
        </React.Suspense>
        <BroadcastBanner notice={contest.notice} messages={messages} />
      </>
    )
  }

  return (
    <div className="space-y-4">
      {noiMasked && (
        <div className="mx-auto mb-3 flex max-w-6xl items-start gap-2 rounded-xl border border-amber-200 bg-amber-50 px-4 py-2.5 text-xs leading-5 text-amber-900">
          <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-600" aria-hidden="true" />
          <span>
            NOI 全真模拟进行中：提交已记录（每题以最后一次提交为准），评测结果与榜单将在比赛结束后统一公布。
          </span>
        </div>
      )}
      {secondaryFailures.length > 0 && (
        <InlineError
          title="部分考试数据加载失败"
          message={secondaryFailures
            .map(
              (failure) =>
                `${failure.label}: ${
                  failure.cause instanceof Error ? failure.cause.message : '请重试'
                }`
            )
            .join('；')}
          action={
            <Button size="sm" variant="outline" onClick={retrySecondaryQueries}>
              重试
            </Button>
          }
        />
      )}
      <NoticeDrawer
        open={notificationsOpen}
        onClose={() => setNotificationsOpen(false)}
        notice={contest?.notice}
        messages={messages}
      />
      <BroadcastBanner notice={contest?.notice} messages={messages} />

      <section className="overflow-hidden rounded-2xl border border-[#e4e7ec] bg-[#fffefa] shadow-[0_1px_2px_rgba(16,24,40,0.04),0_18px_40px_-30px_rgba(16,24,40,0.25)]">
        <div className="relative overflow-hidden bg-[#14283d] px-5 py-5 text-white sm:px-6">
          <div className="absolute inset-0 forge-grid opacity-20" aria-hidden="true" />
          <div
            className="absolute -right-16 -top-24 h-64 w-64 rounded-full bg-forge-500/20 blur-3xl"
            aria-hidden="true"
          />
          <div className="relative flex flex-wrap items-start justify-between gap-4">
            <div className="min-w-0">
              <Link
                to="/"
                className="inline-flex items-center gap-1.5 text-[11px] font-bold text-white/55 transition hover:text-white"
              >
                <ArrowLeft className="h-3.5 w-3.5" />
                返回比赛大厅
              </Link>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                <Badge
                  variant={phase === 'running' ? 'success' : phase === 'upcoming' ? 'info' : 'default'}
                  dot
                  className="border-white/15 bg-white/10 text-white"
                >
                  {phase === 'running'
                    ? '进行中'
                    : phase === 'upcoming'
                      ? '即将开始'
                      : phase === 'finished'
                        ? '已结束'
                        : '未开始'}
                </Badge>
                <span className="text-[11px] font-semibold text-white/45">
                  {contest?.rankingMode === 'acm' ? 'ACM 赛制' : 'OI 赛制'}
                </span>
                <span
                  role="status"
                  className={
                    streamState === 'live'
                      ? 'inline-flex items-center gap-1 rounded-full border border-emerald-300/20 bg-emerald-400/10 px-2 py-0.5 text-[10px] font-bold text-emerald-200'
                      : streamState === 'offline'
                        ? 'inline-flex items-center gap-1 rounded-full border border-rose-300/20 bg-rose-400/10 px-2 py-0.5 text-[10px] font-bold text-rose-200'
                        : 'inline-flex items-center gap-1 rounded-full border border-white/10 bg-white/5 px-2 py-0.5 text-[10px] font-bold text-white/50'
                  }
                >
                  <i
                    aria-hidden="true"
                    className={
                      streamState === 'live'
                        ? 'h-1.5 w-1.5 rounded-full bg-emerald-400'
                        : streamState === 'offline'
                          ? 'h-1.5 w-1.5 rounded-full bg-rose-400'
                          : 'h-1.5 w-1.5 animate-pulse rounded-full bg-white/50'
                    }
                  />
                  {streamState === 'live'
                    ? '实时同步'
                    : streamState === 'offline'
                      ? '实时连接中断'
                      : '正在连接'}
                </span>
              </div>
              <h1 className="font-display mt-2 truncate text-2xl font-semibold tracking-[-0.03em] sm:text-3xl">
                {contest?.name || '比赛工作台'}
              </h1>
              {contest?.description && (
                <p className="mt-1.5 line-clamp-1 max-w-2xl text-xs text-white/55">
                  {contest.description}
                </p>
              )}
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <div className="rounded-xl border border-white/10 bg-white/[0.07] px-3 py-2 backdrop-blur-sm">
                {contest ? <ContestTimer contest={contest} userId={session?.userId} /> : null}
              </div>
              <button
                type="button"
                onClick={() => setNotificationsOpen(true)}
                className="relative grid h-10 w-10 place-items-center rounded-xl border border-white/10 bg-white/[0.07] text-white/70 transition hover:bg-white/15 hover:text-white"
                aria-label="查看考场通告"
              >
                <Megaphone className="h-4 w-4" />
                {messageCount > 0 && (
                  <span className="absolute -right-1 -top-1 grid h-4 min-w-4 place-items-center rounded-full bg-forge-500 px-1 font-mono text-[9px] font-bold text-white">
                    {messageCount > 9 ? '9+' : messageCount}
                  </span>
                )}
              </button>
              {activeProblem?.taskType !== 'choice' && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setDesktopMode(true)}
                  className="border-white/15 bg-white/[0.07] text-white hover:border-white/25 hover:bg-white/15"
                >
                  <Monitor className="h-3.5 w-3.5" />
                  Ubuntu 模拟
                </Button>
              )}
            </div>
          </div>

          <div className="relative mt-6 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { icon: BookOpen, label: '题目', value: problems.length },
              { icon: Check, label: '已解决', value: solvedCount },
              { icon: UserRound, label: '我的排名', value: myRank ? `#${myRank}` : '—' },
              {
                icon: Gauge,
                label: '参赛者',
                value: contest?.participantUserIds?.length ?? ranks.length,
              },
            ].map(({ icon: Icon, label, value }) => (
              <div
                key={label}
                className="flex items-center gap-3 rounded-xl border border-white/10 bg-white/[0.055] px-3.5 py-2.5"
              >
                <Icon className="h-4 w-4 text-forge-200" />
                <div>
                  <p className="text-[10px] font-bold uppercase tracking-[0.14em] text-white/40">
                    {label}
                  </p>
                  <p className="font-mono text-sm font-bold text-white">{value}</p>
                </div>
              </div>
            ))}
          </div>
        </div>

        {contest?.notice && (
          <button
            type="button"
            onClick={() => setNotificationsOpen(true)}
            className="flex w-full items-center gap-3 border-b border-amber-200/70 bg-amber-50 px-5 py-2.5 text-left transition hover:bg-amber-100/60 sm:px-6"
          >
            <span className="rounded bg-amber-200/70 px-1.5 py-0.5 text-[10px] font-extrabold text-amber-900">
              置顶公告
            </span>
            <span className="min-w-0 flex-1 truncate text-xs font-semibold text-amber-900">
              {contest.notice}
            </span>
            <ChevronDown className="h-3.5 w-3.5 shrink-0 text-amber-700" />
          </button>
        )}
      </section>

      {sourceError && (
        <div className="flex items-start gap-2.5 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs font-semibold text-rose-800">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <p className="flex-1 leading-5">{sourceError}</p>
          <button
            type="button"
            onClick={() => setSourceError('')}
            aria-label="关闭错误提示"
            className="opacity-50 hover:opacity-100"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {lastResult && (
        <div
          className={
            lastResult.verdict === 'accepted'
              ? 'flex flex-wrap items-center gap-3 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3'
              : 'flex flex-wrap items-center gap-3 rounded-xl border border-sky-200 bg-sky-50 px-4 py-3'
          }
        >
          <Sparkles
            className={
              lastResult.verdict === 'accepted'
                ? 'h-4 w-4 text-emerald-600'
                : 'h-4 w-4 text-sky-600'
            }
          />
          <p className="text-xs font-bold text-slate-800">最近一次评测</p>
          <VerdictBadge submission={lastResult} />
          <span className="font-mono text-xs font-bold text-slate-600">
            {lastResult.score} pts
          </span>
          <span className="text-[11px] text-slate-400">
            {formatDateTime(lastResult.judgedAt || lastResult.submittedAt)}
          </span>
          <button
            type="button"
            onClick={() => setLastResult(null)}
            className="ml-auto text-slate-400 hover:text-slate-700"
            aria-label="关闭评测结果"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {problems.length > 0 && (
        <nav aria-label="试题导航" className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-1 scrollbar-slim">
          {problems.map((problem, index) => {
            const selected = problem.id === activeProblem?.id
            const solved = mySubmissions.some(
              (submission) =>
                submission.problemID === problem.id && submission.verdict === 'accepted'
            )
            return (
              <button
                key={problem.id}
                type="button"
                onClick={() => {
                  setActivePid(problem.id)
                  setActivePanel('editor')
                }}
                aria-current={selected ? 'page' : undefined}
                className={
                  selected
                    ? 'group flex min-w-[9.5rem] shrink-0 items-center gap-2.5 rounded-xl border border-[#14283d] bg-[#14283d] px-3 py-2.5 text-left text-white shadow-sm'
                    : 'group flex min-w-[9.5rem] shrink-0 items-center gap-2.5 rounded-xl border border-[#e4e7ec] bg-[#fffefa] px-3 py-2.5 text-left transition hover:border-[#cfd6df] hover:bg-white'
                }
              >
                <span
                  className={
                    selected
                      ? 'grid h-7 w-7 shrink-0 place-items-center rounded-lg bg-white/15 font-mono text-xs font-extrabold'
                      : solved
                        ? 'grid h-7 w-7 shrink-0 place-items-center rounded-lg bg-emerald-100 font-mono text-xs font-extrabold text-emerald-700'
                        : 'grid h-7 w-7 shrink-0 place-items-center rounded-lg bg-[#f2f4f7] font-mono text-xs font-extrabold text-slate-500'
                  }
                >
                  {problem.code}
                </span>
                <span className="min-w-0">
                  <span
                    className={
                      selected
                        ? 'block truncate text-xs font-bold'
                        : 'block truncate text-xs font-bold text-slate-700'
                    }
                  >
                    {problem.title}
                  </span>
                  <span
                    className={
                      selected
                        ? 'mt-0.5 block text-[10px] text-white/50'
                        : 'mt-0.5 block text-[10px] text-slate-400'
                    }
                  >
                    {problem.taskType === 'choice' ? '客观题' : `第 ${index + 1} 题`}
                    {solved ? ' · 已通过' : ''}
                  </span>
                </span>
              </button>
            )
          })}
        </nav>
      )}

      {!activeProblem ? (
        <EmptyState
          icon={FileText}
          title="比赛暂无试题"
          description="监考教师尚未发布题目。稍后刷新，或返回大厅查看其他比赛。"
          action={
            <Button
              variant="outline"
              onClick={() => {
                void problemsQuery.refetch()
              }}
            >
              刷新题目
            </Button>
          }
        />
      ) : activeProblem.taskType === 'choice' ? (
        <GESPQuizView
          contestId={cid}
          problem={activeProblem}
          canSubmit={phase === 'running'}
          phase={phase}
          onSubmitSuccess={(submission) => {
            setLastResult(submission)
            void queryClient.invalidateQueries({ queryKey: ['submissions', cid] })
            void queryClient.invalidateQueries({ queryKey: ['ranking', cid] })
          }}
        />
      ) : (
        <div className="grid min-h-0 gap-4 xl:grid-cols-[minmax(0,1.06fr)_minmax(24rem,0.94fr)]">
          <ProblemStatement
            problem={activeProblem}
            copied={copied}
            onCopy={() => {
              void copySamples()
            }}
            onDownloadPdf={() => {
              void downloadStatement('pdf')
            }}
            onDownloadHtml={() => {
              void downloadStatement('html')
            }}
            exporting={exporting}
            onSubmit={() => {
              void submit()
            }}
            submitting={submitting}
            submitLabel={answerOnly ? '提交答案' : '提交代码'}
          />

          <section className="flex min-h-0 flex-col overflow-hidden rounded-2xl border border-[#e4e7ec] bg-[#fffefa] xl:sticky xl:top-20 xl:h-[calc(100vh-6.5rem)]">
            <div className="flex items-center justify-between gap-3 border-b border-[#eaecf0] px-3 py-2.5">
              <div className="flex rounded-xl bg-[#f2f4f7] p-1">
                {[
                  {
                    id: 'editor' as const,
                    label: answerOnly ? '答案' : '代码',
                    icon: answerOnly ? FileText : Code2,
                  },
                  {
                    id: 'history' as const,
                    label: `提交 ${problemSubmissions.length}`,
                    icon: History,
                  },
                  { id: 'ranking' as const, label: '排名', icon: Trophy },
                ].map(({ id, label, icon: Icon }) => (
                  <button
                    key={id}
                    type="button"
                    onClick={() => setActivePanel(id)}
                    className={
                      activePanel === id
                        ? 'inline-flex h-8 items-center gap-1.5 rounded-lg bg-white px-2.5 text-[11px] font-extrabold text-[#14283d] shadow-sm'
                        : 'inline-flex h-8 items-center gap-1.5 rounded-lg px-2.5 text-[11px] font-bold text-slate-600 transition hover:text-slate-800'
                    }
                  >
                    <Icon className="h-3.5 w-3.5" />
                    {label}
                  </button>
                ))}
              </div>
              <div className="flex items-center gap-1">
                {!answerOnly && (
                  <button
                    type="button"
                    onClick={() => setUploadOpen(true)}
                    className="inline-flex h-8 items-center gap-1.5 rounded-lg px-2 text-[11px] font-bold text-slate-500 transition hover:bg-slate-100 hover:text-slate-800"
                    title="从本地文件或文件夹载入代码"
                  >
                    <Upload className="h-3.5 w-3.5" />
                    <span className="hidden sm:inline">载入</span>
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => {
                    if (!activeProblem) return
                    setDraft(
                      contestProblemDraftKey(session?.userId, cid, activeProblem.id),
                      ''
                    )
                    setHydratedProblem(activeProblem.id)
                  }}
                  className="inline-flex h-8 items-center rounded-lg px-2 text-[11px] font-bold text-slate-400 transition hover:bg-rose-50 hover:text-rose-600"
                  title="清空当前草稿"
                >
                  清空
                </button>
              </div>
            </div>

            {activePanel === 'editor' && (
              <div className="flex min-h-0 flex-1 flex-col">
                <div className="flex flex-wrap items-center gap-2 border-b border-[#eaecf0] bg-[#fbfaf6] px-3 py-2">
                  {answerOnly ? (
                    <Badge variant="accent">纯文本答案 · 不编译</Badge>
                  ) : languageOptions.length === 0 ? (
                    <Badge variant="danger">本场考试未开放客户端支持的语言</Badge>
                  ) : (
                    <Select
                      aria-label="选择提交语言"
                      value={language}
                      onChange={(event) => setLanguage(event.target.value as SupportedLanguage)}
                      className="h-8 w-auto min-w-[10.5rem] rounded-lg py-0 pl-2.5 pr-8 text-xs font-bold"
                    >
                      {languageOptions.map((option) => (
                        <option key={option.value} value={option.value}>
                          {option.label}
                        </option>
                      ))}
                    </Select>
                  )}
                  <Badge
                    variant={codeBytes > MAX_SOURCE_BYTES ? 'danger' : 'default'}
                    className="font-mono"
                  >
                    {formatBytes(codeBytes)} / 64 KB
                  </Badge>
                  <span className="ml-auto hidden items-center gap-1 text-[10px] font-semibold text-slate-400 sm:flex">
                    <Keyboard className="h-3 w-3" />
                    Ctrl / ⌘ + Enter 提交
                  </span>
                </div>

                <div
                  className="min-h-[22rem] flex-1 overflow-hidden bg-white"
                  onKeyDown={onEditorKeyDown}
                >
                  {answerOnly ? (
                    <div className="flex h-full min-h-[22rem] flex-col">
                      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[#eaecf0] px-3 py-2 text-[11px] text-slate-500">
                        <span>直接输入最终答案，或载入本地文本答案文件。</span>
                        <div>
                          <input
                            ref={answerFileRef}
                            type="file"
                            accept=".txt,.out,.ans,text/plain"
                            aria-label="选择答案文件"
                            className="sr-only"
                            onChange={(event) => {
                              void handleAnswerFile(event.target.files?.[0])
                              event.target.value = ''
                            }}
                          />
                          <Button
                            size="sm"
                            variant="outline"
                            className="h-7 gap-1.5 px-2 text-[10px]"
                            onClick={() => answerFileRef.current?.click()}
                          >
                            <Upload className="h-3 w-3" />
                            载入答案文件
                          </Button>
                        </div>
                      </div>
                      <textarea
                        aria-label="答案内容"
                        value={code}
                        onChange={(event) => setCode(event.target.value)}
                        placeholder="在此输入标准答案对应的最终提交内容…"
                        spellCheck={false}
                        className="min-h-0 flex-1 resize-none border-0 bg-[#fffefa] p-4 font-mono text-sm leading-6 text-[#14283d] outline-none placeholder:text-slate-300"
                      />
                    </div>
                  ) : (
                    <React.Suspense fallback={<EditorLoading />}>
                      <MonacoEditor
                        value={code}
                        onChange={setCode}
                        language={editorLanguage(language)}
                        height="100%"
                        ariaLabel="源代码编辑器"
                      />
                    </React.Suspense>
                  )}
                </div>

                {phase !== 'running' && (
                  <div className="flex items-center gap-2 border-t border-amber-200 bg-amber-50 px-3 py-2 text-[11px] font-semibold text-amber-900">
                    <LockKeyhole className="h-3.5 w-3.5" />
                    {phase === 'upcoming'
                      ? '考试尚未开始，当前为只读预览。'
                      : phase === 'finished'
                        ? '考试已结束，提交通道已关闭。'
                        : '比赛尚未启动。'}
                  </div>
                )}

                <div className="flex items-center gap-2 border-t border-[#eaecf0] bg-[#fbfaf6] px-3 py-3">
                  <div className="hidden min-w-0 flex-1 items-center gap-1.5 text-[10px] text-slate-400 sm:flex">
                    <CloudUpload className="h-3.5 w-3.5" />
                    {answerOnly ? '答案自动保存于本机草稿' : '自动保存于本机草稿'}
                  </div>
                  <Button
                    onClick={() => {
                      void submit()
                    }}
                    disabled={!canSubmit || !code.trim() || codeBytes > MAX_SOURCE_BYTES}
                    loading={submitting}
                    className="ml-auto gap-2"
                  >
                    <Send className="h-4 w-4" />
                    {answerOnly ? '提交答案' : t('submit_code')}
                  </Button>
                </div>
              </div>
            )}

            {activePanel === 'history' && (
              <div className="min-h-0 flex-1 overflow-y-auto bg-[#fbfaf6] p-3 scrollbar-slim">
                <HistoryPanel submissions={problemSubmissions} onLoad={loadSubmission} />
              </div>
            )}

            {activePanel === 'ranking' && (
              <div className="min-h-0 flex-1 overflow-y-auto bg-[#fbfaf6] p-3 scrollbar-slim">
                {noiMasked && (
                  <p className="mb-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs leading-5 text-amber-900">
                    NOI 全真模拟：排名区在比赛结束后开放。
                  </p>
                )}
                <Scoreboard
                  ranks={ranks as RankEntry[] | ACMRankEntry[]}
                  rankingMode={contest?.rankingMode}
                  problems={problems}
                  highlightUserId={session?.userId}
                />
              </div>
            )}
          </section>
        </div>
      )}

      <Dialog
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title={`再次提交 ${activeProblem?.code ?? ''}`}
        eyebrow="Submit confirmation"
        width="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmOpen(false)}>
              取消
            </Button>
            <Button
              onClick={() => {
                void commitSubmit()
              }}
              loading={submitting}
              className="gap-2"
            >
              <Send className="h-4 w-4" />
              确认提交
            </Button>
          </>
        }
      >
        <div className="space-y-4 text-sm leading-6 text-slate-600">
          <p>
            这是你第{' '}
            <span className="font-mono font-extrabold text-[#14283d]">
              {problemSubmissions.length + 1}
            </span>{' '}
            次提交
            <span className="font-bold text-[#14283d]"> {activeProblem?.title}</span>。
          </p>
          <div className="rounded-xl border border-[#e4e7ec] bg-[#fbfaf6] px-4 py-3 text-xs">
            <p>
              {activeProblem?.taskType === 'answers_only'
                ? '提交类型 · 纯文本答案'
                : `语言 · ${languageLabel(language)}`}
            </p>
            <p className="mt-1">
              {activeProblem?.taskType === 'answers_only' ? '答案大小' : '代码大小'} ·{' '}
              {formatBytes(codeBytes)}
            </p>
            <p className="mt-1 text-slate-400">以本次提交的评测结果为准。</p>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={uploadOpen}
        onClose={() => setUploadOpen(false)}
        title="从本地载入代码"
        eyebrow="Local source"
        width="lg"
        footer={
          <Button variant="secondary" onClick={() => setUploadOpen(false)}>
            完成
          </Button>
        }
      >
        <div className="space-y-5">
          <div className="grid gap-3 sm:grid-cols-2">
            <button
              type="button"
              onClick={() => singleFileRef.current?.click()}
              className="group rounded-2xl border border-[#e4e7ec] bg-white p-5 text-left transition hover:border-forge-300 hover:bg-forge-50/30"
            >
              <span className="grid h-10 w-10 place-items-center rounded-xl bg-forge-50 text-forge-700">
                <FileCode2 className="h-5 w-5" />
              </span>
              <p className="mt-3 text-sm font-extrabold text-[#14283d]">选择单个源文件</p>
              <p className="mt-1 text-[11px] leading-5 text-slate-500">
                读取 .cpp / .c / .py / .go，自动识别语言并映射到当前试题。单文件不超过 64 KB。
              </p>
            </button>
            <button
              type="button"
              onClick={() => folderFileRef.current?.click()}
              className="group rounded-2xl border border-[#e4e7ec] bg-white p-5 text-left transition hover:border-teal-300 hover:bg-teal-50/30"
            >
              <span className="grid h-10 w-10 place-items-center rounded-xl bg-teal-50 text-teal-700">
                <FolderOpen className="h-5 w-5" />
              </span>
              <p className="mt-3 text-sm font-extrabold text-[#14283d]">选择整个考试文件夹</p>
              <p className="mt-1 text-[11px] leading-5 text-slate-500">
                仅在浏览器本地解析，不提交服务器。按目录 / 文件名对应题目，最多 512 个文件、8 MB。
              </p>
            </button>
          </div>

          <input
            ref={singleFileRef}
            type="file"
            accept={SOURCE_ACCEPT}
            aria-label="选择单个源代码文件"
            className="sr-only"
            onChange={(event) => {
              void handleSingleFile(event.target.files?.[0])
              event.target.value = ''
            }}
          />
          <input
            ref={folderFileRef}
            type="file"
            aria-label="选择考试源代码文件夹"
            multiple
            className="sr-only"
            // webkitdirectory is non-standard but supported by Chromium and Safari.
            {...({ webkitdirectory: '', directory: '' } as Record<string, string>)}
            onChange={(event) => {
              handleFolder(event.target.files)
              event.target.value = ''
            }}
          />

          <div className="rounded-2xl border border-dashed border-[#d0d5dd] bg-[#fbfaf6] p-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs font-extrabold text-slate-700">文件夹预览</p>
                <p className="mt-0.5 text-[11px] text-slate-400">
                  只读取源码；不会提交文件夹，也不会写入服务器。
                </p>
              </div>
              {folder && (
                <Badge variant="accent">
                  {folder.sourceCount} / {folder.totalCount} 个源码
                </Badge>
              )}
            </div>

            {!folder ? (
              <div className="mt-4 flex items-center gap-2 rounded-xl border border-[#eaecf0] bg-white px-3 py-6 text-xs text-slate-400">
                <FolderOpen className="h-4 w-4" />
                尚未选择文件夹
              </div>
            ) : (
              <div className="mt-4 space-y-2">
                <div className="flex flex-wrap gap-2 text-[11px] text-slate-500">
                  <span className="font-mono font-bold text-slate-700">{folder.root}</span>
                  <span>·</span>
                  <span>{formatBytes(folder.totalBytes)}</span>
                  <span>·</span>
                  <span>{folderMatches.filter((item) => item.problem).length} 个已匹配</span>
                </div>
                <div className="max-h-64 space-y-1.5 overflow-y-auto pr-1 scrollbar-slim">
                  {folderMatches.map((match) => (
                    <div
                      key={match.path}
                      className="flex flex-wrap items-center gap-2 rounded-xl border border-[#eaecf0] bg-white px-3 py-2"
                    >
                      <FileCode2 className="h-3.5 w-3.5 shrink-0 text-slate-400" />
                      <span className="min-w-0 flex-1 truncate font-mono text-[10px] text-slate-600">
                        {match.path}
                      </span>
                      {match.problem ? (
                        <Badge variant="success">{match.problem.code}</Badge>
                      ) : (
                        <Badge variant="warning">未匹配</Badge>
                      )}
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-7 px-2 text-[10px]"
                        onClick={() => {
                          void loadFolderFile(match)
                        }}
                      >
                        载入
                      </Button>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          <p className="flex items-start gap-2 text-[11px] leading-5 text-slate-400">
            <Sparkles className="mt-0.5 h-3.5 w-3.5 shrink-0 text-forge-500" />
            提示：这里只做浏览器端读取和本地草稿保存。正式提交仍使用当前编辑器中的源码，并记录到评测服务。
          </p>
        </div>
      </Dialog>
    </div>
  )
}

