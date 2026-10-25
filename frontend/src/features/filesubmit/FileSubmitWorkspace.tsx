// 文件提交模式（Lemon）——学生侧工作台。
//
// 复刻 LemonNext 在线提交系统的完整学生端：
//   比赛横幅（标题 + 满分 + 考试剩余倒计时）、开考前锁定态、
//   题目表格（题型徽章 / 状态 / 得分 / 最后提交 / 查看提交代码）、
//   成绩发布横幅（结束后自动轮询）、考试须知 / 消息中心（未读水位）、
//   试题下载、整包上传选手文件夹（CSP 收题格式）与单题作答面板。
//
// 数据来自 mockApi；后端就绪后替换为同接口的真实实现即可。

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  ArrowRight,
  Bell,
  BookOpenCheck,
  ChevronLeft,
  Clock3,
  Copy,
  Download,
  FileQuestion,
  FolderUp,
  Info,
  MailOpen,
  Trophy,
} from 'lucide-react'
import { Badge } from '../../components/ui/badge'
import { Button } from '../../components/ui/button'
import { Dialog } from '../../components/ui/dialog'
import { EmptyState, ErrorState, LoadingBlock } from '../../components/ui/empty-state'
import { useToast } from '../../components/ui/toast'
import { MAX_FOLDER_BYTES, MAX_FOLDER_FILES, formatBytes } from '../student/submissionFiles'
import { MESSAGE_SEEN_KEY, readNumber, writeNumber } from './keys'
import { mockLemonApi as api } from './mockApi'
import { TaskSubmitPanel } from './TaskSubmitPanel'
import type {
  LemonMessage,
  LemonScores,
  LemonSource,
  LemonTasksResponse,
  LemonUploadFolderResult,
} from './types'
import { fmtClock, fmtDateTime, useLemonCountdown, useLocalClock } from './time'

const TYPE_BADGE: Record<string, { label: string; variant: 'info' | 'accent' | 'default' }> = {
  choice: { label: '选择题', variant: 'accent' },
  answers_only: { label: '答案提交', variant: 'info' },
}

function statusLabel(taskType: string, submitted: boolean): string {
  if (taskType === 'choice') return submitted ? '已递交' : '未答题'
  return submitted ? '已提交' : '未提交'
}

function ScoreCellText({ task }: { task: LemonScores['tasks'][number] }) {
  if (task.placementError) return <span className="text-xs font-bold text-rose-600">结构错误 0</span>
  if (task.compileFailed) return <span className="text-xs font-bold text-rose-600">编译失败</span>
  if (task.judged) {
    return (
      <span>
        <b className="font-mono text-sm text-[#14283d]">{task.score}</b>
        <span className="ml-1 text-[11px] text-slate-400">/ {task.totalScore}</span>
      </span>
    )
  }
  return <span className="text-xs font-medium text-sky-600">正在评测…</span>
}

export function FileSubmitWorkspace() {
  const { cid } = useParams<{ cid: string }>()
  const toast = useToast()
  const clock = useLocalClock()

  const [data, setData] = useState<LemonTasksResponse | null>(null)
  const [loadError, setLoadError] = useState('')
  const [reloadToken, setReloadToken] = useState(0)
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null)
  const [noticeOpen, setNoticeOpen] = useState(false)
  const [noticeText, setNoticeText] = useState('')
  const [messagesOpen, setMessagesOpen] = useState(false)
  const [messages, setMessages] = useState<LemonMessage[]>([])
  const [latestMessageId, setLatestMessageId] = useState(0)
  const [scores, setScores] = useState<LemonScores | null>(null)
  const [codeTaskId, setCodeTaskId] = useState<number | null>(null)
  const [codeSource, setCodeSource] = useState<LemonSource | null>(null)
  const [codeLoading, setCodeLoading] = useState(false)
  const scoresTimerRef = useRef<number | null>(null)

  const reload = useCallback(() => setReloadToken((token) => token + 1), [])

  // ---- /tasks 加载 ----
  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const response = await api.tasks()
        if (!cancelled) {
          setData(response)
          setLoadError('')
          if (response.scoresReleased) setScores(await api.scores())
        }
      } catch (cause) {
        if (!cancelled) setLoadError(cause instanceof Error ? cause.message : '加载失败')
      }
    })()
    return () => {
      cancelled = true
    }
  }, [reloadToken])

  const countdown = useLemonCountdown(data ?? undefined, (next) => {
    // 开考瞬间自动刷新：锁定态解除，题目列表出现。
    if (next.state === 'running') reload()
    // 结束瞬间开始轮询成绩（评测陆续完成，晚到的判定要能刷出来）。
    if (next.state === 'ended') {
      void api.scores().then(setScores).catch(() => undefined)
    }
  })

  // ended 后每 6 秒刷新成绩。
  useEffect(() => {
    if (countdown.state !== 'ended') return
    scoresTimerRef.current = window.setInterval(() => {
      api.scores().then(setScores).catch(() => undefined)
    }, 6000)
    return () => {
      if (scoresTimerRef.current) window.clearInterval(scoresTimerRef.current)
    }
  }, [countdown.state])

  // ---- 消息轮询（10s）与未读水位 ----
  useEffect(() => {
    let cancelled = false
    const poll = async () => {
      try {
        const response = await api.messages()
        if (!cancelled) {
          setMessages(response.messages)
          setLatestMessageId(response.latestId)
        }
      } catch {
        // 网络抖动忽略，下轮再取
      }
    }
    void poll()
    const timer = window.setInterval(poll, 10000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [])

  const unreadCount = useMemo(() => {
    const seen = readNumber(MESSAGE_SEEN_KEY)
    return messages.filter((message) => message.id > seen).length
  }, [messages, messagesOpen])

  const openMessages = () => {
    setMessagesOpen(true)
    if (latestMessageId > readNumber(MESSAGE_SEEN_KEY)) {
      writeNumber(MESSAGE_SEEN_KEY, latestMessageId)
    }
  }

  // ---- 考试须知（首次打开时拉取并缓存） ----
  const openNotice = async () => {
    setNoticeOpen(true)
    if (!noticeText) {
      try {
        const response = await api.notice()
        setNoticeText(response.text)
      } catch {
        setNoticeText('须知加载失败，请稍后重试。')
      }
    }
  }

  // ---- 查看提交代码 ----
  const openCode = async (taskId: number) => {
    setCodeTaskId(taskId)
    setCodeSource(null)
    setCodeLoading(true)
    try {
      setCodeSource(await api.source(taskId))
    } catch (cause) {
      toast.error('加载失败', cause instanceof Error ? cause.message : undefined)
      setCodeTaskId(null)
    } finally {
      setCodeLoading(false)
    }
  }

  const selectedTask = useMemo(
    () => (selectedTaskId == null ? null : data?.tasks.find((task) => task.id === selectedTaskId) || null),
    [data, selectedTaskId]
  )

  if (loadError && !data) {
    return (
      <ErrorState
        message={loadError}
        action={<Button onClick={reload}>重新加载</Button>}
      />
    )
  }

  if (!data) return <LoadingBlock label="正在进入文件提交模式..." />

  const totalScore = data.tasks.reduce((sum, task) => sum + task.totalScore, 0)
  const scoreByTask = new Map(scores?.tasks.map((task) => [task.id, task]) || [])
  const selectedTaskIndex = selectedTask ? data.tasks.findIndex((task) => task.id === selectedTask.id) : -1

  // ---------- 答题页 ----------
  if (selectedTask && selectedTaskIndex >= 0) {
    return (
      <div className="space-y-5">
        <TaskSubmitPanel
          key={selectedTask.id}
          data={data}
          task={selectedTask}
          taskIndex={selectedTaskIndex}
          countdown={countdown}
          onBack={() => setSelectedTaskId(null)}
          onSubmitted={(taskId, submittedAt) => {
            setData((current) =>
              current
                ? {
                    ...current,
                    tasks: current.tasks.map((task) =>
                      task.id === taskId ? { ...task, submittedAt } : task
                    ),
                  }
                : current
            )
          }}
        />
        <LemonChrome
          clock={clock}
          countdown={countdown}
          data={data}
          unreadCount={unreadCount}
          onNotice={() => void openNotice()}
          onMessages={openMessages}
          compact
        />
        <WorkspaceDialogs
          noticeOpen={noticeOpen}
          noticeText={noticeText}
          onCloseNotice={() => setNoticeOpen(false)}
          messagesOpen={messagesOpen}
          messages={messages}
          onCloseMessages={() => setMessagesOpen(false)}
          codeTaskId={codeTaskId}
          codeSource={codeSource}
          codeLoading={codeLoading}
          onCloseCode={() => setCodeTaskId(null)}
        />
      </div>
    )
  }

  // ---------- 题目列表页 ----------
  return (
    <div className="space-y-6">
      {/* 比赛横幅：标题 + 元信息 + 倒计时 + 本机时钟 */}
      <section className="relative overflow-hidden rounded-[1.75rem] border border-[#e4e7ec] bg-[#14283d] px-6 py-7 text-white shadow-[0_24px_60px_-38px_rgba(20,40,61,0.55)] sm:px-9">
        <div className="absolute inset-0 forge-grid opacity-[0.12]" aria-hidden="true" />
        <div className="relative flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div className="min-w-0">
            <p className="flex items-center gap-2 text-[10px] font-bold uppercase tracking-[0.22em] text-white/45">
              <FileQuestion className="h-3.5 w-3.5" />
              文件提交模式 · Lemon
            </p>
            <h1 className="font-display mt-2 break-words text-[clamp(1.9rem,6vw,2.75rem)] font-semibold leading-[1.08] tracking-[-0.04em]">
              {data.contestTitle}
            </h1>
            <p className="mt-3 text-xs leading-6 text-white/55">
              {data.tasks.length} 题 · 满分 {totalScore} · 选手 {data.displayName}
              {data.submitMode === 'both' && ' · 在线编辑 / 上传文件均可'}
            </p>
          </div>
          <LemonCountdownBlock countdown={countdown} clock={clock} />
        </div>
      </section>

      {/* 开考前锁定态 */}
      {data.preContest ? (
        <EmptyState
          icon={Clock3}
          title="比赛尚未开始"
          description="题目列表将在开始时间到达后自动出现，请耐心等待。"
        />
      ) : (
        <>
          {/* 成绩横幅 */}
          {scores?.released && (
            <section className="app-surface rounded-2xl p-5" data-testid="lemon-scores">
              <div className="flex flex-wrap items-center gap-3">
                <Trophy className="h-5 w-5 text-amber-500" />
                <h2 className="text-sm font-bold text-[#14283d]">您的成绩</h2>
                <span className="ml-auto font-mono text-2xl font-bold text-[#14283d]">
                  {scores.totalScore}
                  <span className="text-sm font-medium text-slate-400"> / {scores.totalPossible}</span>
                </span>
              </div>
              <ul className="mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
                {scores.tasks.map((task, index) => (
                  <li
                    key={task.id}
                    className="flex items-center gap-3 rounded-xl border border-[#e4e7ec] bg-[#fbfaf6] px-3 py-2 text-sm"
                  >
                    <span className="font-mono text-xs font-bold text-slate-400">{index + 1}.</span>
                    <span className="min-w-0 flex-1 truncate font-semibold text-slate-700">{task.title}</span>
                    <ScoreCellText task={task} />
                  </li>
                ))}
              </ul>
            </section>
          )}

          {/* 题目表格 */}
          <section className="app-surface overflow-hidden rounded-2xl" aria-label="题目列表">
            <div className="overflow-x-auto">
              <table className="w-full min-w-[820px] text-left text-sm" data-testid="lemon-task-table">
                <thead>
                  <tr className="border-b border-[#e4e7ec] bg-[#fbfaf6] text-[11px] font-bold uppercase tracking-[0.08em] text-slate-400">
                    <th className="w-14 px-4 py-3 text-center">序号</th>
                    <th className="px-4 py-3">试题名称</th>
                    <th className="w-16 px-4 py-3 text-center">分值</th>
                    <th className="w-24 px-4 py-3">状态</th>
                    <th className="w-28 px-4 py-3">得分</th>
                    <th className="w-24 px-4 py-3">最后提交</th>
                    <th className="w-16 px-4 py-3">查看</th>
                    <th className="w-12 px-4 py-3" aria-label="操作" />
                  </tr>
                </thead>
                <tbody>
                  {data.tasks.length === 0 && (
                    <tr>
                      <td colSpan={8} className="px-4 py-10 text-center text-slate-400">
                        本场比赛尚无题目
                      </td>
                    </tr>
                  )}
                  {data.tasks.map((task, index) => {
                    const submitted = Boolean(task.submittedAt)
                    const badge = TYPE_BADGE[task.taskType]
                    const scoreTask = scoreByTask.get(task.id)
                    return (
                      <tr
                        key={task.id}
                        tabIndex={0}
                        role="button"
                        aria-label={`进入题目 ${index + 1} ${task.title}`}
                        onClick={() => setSelectedTaskId(task.id)}
                        onKeyDown={(event) => {
                          // 行内按钮/链接（查看、样例）自己处理 Enter，不触发行导航。
                          if ((event.target as HTMLElement).closest('button, a')) return
                          if (event.key === 'Enter' || event.key === ' ') {
                            event.preventDefault()
                            setSelectedTaskId(task.id)
                          }
                        }}
                        className="cursor-pointer border-b border-[#eef0f3] transition last:border-b-0 hover:bg-[#fbfaf6] focus-visible:bg-[#fbfaf6] focus-visible:outline-none"
                      >
                        <td className="px-4 py-3 text-center font-mono text-xs font-bold text-slate-400">
                          {index + 1}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex flex-wrap items-center gap-2">
                            {badge && <Badge variant={badge.variant}>{badge.label}</Badge>}
                            <span className="font-semibold text-[#14283d]">{task.title}</span>
                          </div>
                          {task.samples.length > 0 && (
                            <div className="mt-1 flex flex-wrap items-center gap-2 text-[11px]">
                              <span className="text-slate-400">📎 样例：</span>
                              {task.samples.map((sample) => (
                                <a
                                  key={sample.name}
                                  href={sample.url}
                                  download
                                  onClick={(event) => event.stopPropagation()}
                                  className="font-mono font-bold text-forge-700 hover:underline"
                                >
                                  {sample.name}
                                </a>
                              ))}
                            </div>
                          )}
                        </td>
                        <td className="px-4 py-3 text-center font-mono text-xs text-slate-500">
                          {task.totalScore}
                        </td>
                        <td className="px-4 py-3">
                          <Badge variant={submitted ? 'success' : 'default'} dot>
                            {statusLabel(task.taskType, submitted)}
                          </Badge>
                        </td>
                        <td className="px-4 py-3">
                          {scoreTask ? (
                            <ScoreCellText task={scoreTask} />
                          ) : (
                            <span className="text-slate-300">—</span>
                          )}
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-slate-400">
                          {submitted ? fmtClock(task.submittedAt) : '—'}
                        </td>
                        <td className="px-4 py-3">
                          {submitted ? (
                            <button
                              type="button"
                              onClick={(event) => {
                                event.stopPropagation()
                                void openCode(task.id)
                              }}
                              className="text-xs font-bold text-forge-700 hover:underline"
                            >
                              查看
                            </button>
                          ) : (
                            <span className="text-slate-300">—</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-slate-300" aria-hidden="true">
                          <ArrowRight className="h-4 w-4" />
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </section>

          <FolderUploadCard countdown={countdown} examFolder={data.examFolder} />
        </>
      )}

      <LemonChrome
        clock={clock}
        countdown={countdown}
        data={data}
        unreadCount={unreadCount}
        onNotice={() => void openNotice()}
        onMessages={openMessages}
      />

      <WorkspaceDialogs
        noticeOpen={noticeOpen}
        noticeText={noticeText}
        onCloseNotice={() => setNoticeOpen(false)}
        messagesOpen={messagesOpen}
        messages={messages}
        onCloseMessages={() => setMessagesOpen(false)}
        codeTaskId={codeTaskId}
        codeSource={codeSource}
        codeLoading={codeLoading}
        onCloseCode={() => setCodeTaskId(null)}
      />
      {/* cid 留给后端接入时使用（提交会带比赛标识）。 */}
      <span className="hidden" data-contest-id={cid} />
    </div>
  )
}

/** 倒计时方块：数字随紧迫度变色（≤30 分钟琥珀、≤5 分钟红）。 */
function LemonCountdownBlock({
  countdown,
  clock,
}: {
  countdown: ReturnType<typeof useLemonCountdown>
  clock: string
}) {
  if (countdown.state === 'disabled') return null
  const toneClass =
    countdown.tone === 'danger'
      ? 'text-rose-300'
      : countdown.tone === 'warning'
        ? 'text-amber-300'
        : 'text-white'
  return (
    <div className="flex items-center gap-5 rounded-2xl border border-white/10 bg-white/[0.06] px-5 py-4 backdrop-blur-sm">
      <div className="text-right">
        <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-white/40">
          {countdown.state === 'pre' ? '距离开始' : countdown.state === 'ended' ? '比赛状态' : '考试剩余'}
        </p>
        <p className={`font-mono mt-1 text-3xl font-bold tabular-nums ${toneClass}`} data-testid="lemon-countdown">
          {countdown.state === 'ended' ? '已结束' : countdown.duration}
        </p>
      </div>
      <span className="h-10 w-px bg-white/10" />
      <div>
        <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-white/40">本机时间</p>
        <p className="mt-1 font-mono text-lg font-semibold tabular-nums text-white/85">{clock}</p>
      </div>
    </div>
  )
}

/** 功能导航条：考试须知 / 试题下载 / 消息（未读徽标）。 */
function LemonChrome({
  clock: _clock,
  countdown: _countdown,
  data,
  unreadCount,
  onNotice,
  onMessages,
  compact,
}: {
  clock: string
  countdown: ReturnType<typeof useLemonCountdown>
  data: LemonTasksResponse
  unreadCount: number
  onNotice: () => void
  onMessages: () => void
  compact?: boolean
}) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Button variant="outline" size="sm" onClick={onNotice}>
        <Info className="h-3.5 w-3.5" />
        考试须知
      </Button>
      {data.hasStatement ? (
        data.statements.map((statement) => (
          <a
            key={statement.url}
            href={statement.url}
            target="_blank"
            rel="noopener noreferrer"
            download
            className="inline-flex h-9 items-center gap-1.5 rounded-xl border border-[#e4e7ec] bg-white px-3 text-xs font-bold text-slate-600 transition hover:border-[#d0d5dd] hover:text-slate-900"
          >
            <Download className="h-3.5 w-3.5" />
            {compact ? '试题下载' : statement.name}
          </a>
        ))
      ) : (
        <Button variant="outline" size="sm" disabled title="本场没有下发题面文件">
          <Download className="h-3.5 w-3.5" />
          试题下载
        </Button>
      )}
      <Button variant="outline" size="sm" onClick={onMessages} data-testid="lemon-messages-button">
        {unreadCount > 0 ? (
          <Bell className="h-3.5 w-3.5 text-amber-600" />
        ) : (
          <MailOpen className="h-3.5 w-3.5" />
        )}
        {unreadCount > 0 ? `${unreadCount} 条未读消息` : '消息'}
      </Button>
      <Link
        to="/"
        className="ml-auto inline-flex h-9 items-center gap-1 text-xs font-bold text-slate-400 transition hover:text-slate-700"
      >
        <ChevronLeft className="h-3.5 w-3.5" />
        返回大厅
      </Link>
    </div>
  )
}

/** 整包上传选手文件夹（CSP-J/S 收题格式：<准考证号>/<题目>/<题目>.cpp）。 */
function FolderUploadCard({
  countdown,
  examFolder,
}: {
  countdown: ReturnType<typeof useLemonCountdown>
  examFolder: string
}) {
  const toast = useToast()
  const inputRef = useRef<HTMLInputElement | null>(null)
  const [picked, setPicked] = useState<{ root: string; files: File[]; bytes: number } | null>(null)
  const [pickedError, setPickedError] = useState('')
  const [uploading, setUploading] = useState(false)
  const [result, setResult] = useState<LemonUploadFolderResult | null>(null)

  const blocked = countdown.state === 'pre' || countdown.state === 'ended'
  const blockedLabel = countdown.state === 'pre' ? '比赛尚未开始' : '比赛已结束'

  const handlePick = () => inputRef.current?.click()

  const handleFiles = (fileList: FileList | null) => {
    setResult(null)
    setPickedError('')
    setPicked(null)
    const files = Array.from(fileList || [])
    if (!files.length) return
    const relatives = files.map((file) => file.webkitRelativePath || '')
    if (!relatives[0]) {
      setPickedError('当前浏览器不支持文件夹上传，请改用 Chrome / Edge。')
      return
    }
    const roots = new Set(relatives.map((path) => path.split('/')[0]))
    if (roots.size !== 1) {
      setPickedError('检测到多个顶层文件夹，请只选中你自己的准考证号文件夹。')
      return
    }
    if (files.length > MAX_FOLDER_FILES) {
      setPickedError(`文件数 ${files.length} 超过上限 ${MAX_FOLDER_FILES}。`)
      return
    }
    const bytes = files.reduce((sum, file) => sum + file.size, 0)
    setPicked({ root: relatives[0].split('/')[0], files, bytes })
    if (bytes > MAX_FOLDER_BYTES * 0.7) {
      setPickedError(`文件夹约 ${formatBytes(bytes)}，超过上限 ${formatBytes(MAX_FOLDER_BYTES)}。`)
    }
  }

  const doUpload = async () => {
    if (!picked || uploading) return
    if (picked.bytes > MAX_FOLDER_BYTES * 0.7) {
      toast.warning('文件夹过大，未上传')
      return
    }
    setUploading(true)
    try {
      const response = await api.uploadFolder(
        picked.root,
        picked.files.map((file) => ({
          path: file.webkitRelativePath || `${picked.root}/${file.name}`,
          size: file.size,
        }))
      )
      setResult(response)
      toast.success(`已写入 ${response.written}/${response.total} 个文件`)
    } catch (cause) {
      toast.error('上传失败', cause instanceof Error ? cause.message : undefined)
    } finally {
      setUploading(false)
    }
  }

  return (
    <section className="app-surface rounded-2xl p-5" aria-label="整包上传选手文件夹">
      <div className="flex flex-wrap items-center gap-3">
        <FolderUp className="h-5 w-5 text-forge-700" />
        <div className="min-w-0">
          <h2 className="text-sm font-bold text-[#14283d]">整包上传选手文件夹</h2>
          <p className="mt-0.5 text-xs text-slate-500">
            选择以准考证号命名的文件夹（{examFolder}/bracket/bracket.cpp 这类 CSP 收题结构）。
            限制：≤ {MAX_FOLDER_FILES} 个文件、≤ {formatBytes(MAX_FOLDER_BYTES)}。
          </p>
        </div>
      </div>
      <input
        ref={inputRef}
        type="file"
        className="hidden"
        multiple
        {...({ webkitdirectory: '', directory: '' } as Record<string, string>)}
        onChange={(event) => {
          handleFiles(event.target.files)
          // 重置 value，否则再次选择同一个文件夹不会触发 change。
          event.target.value = ''
        }}
      />
      <div className="mt-4 flex flex-wrap items-center gap-3">
        <Button variant="outline" onClick={handlePick} disabled={blocked}>
          选择文件夹…
        </Button>
        <span className="text-xs text-slate-400">
          {picked
            ? `已选择 ${picked.root} · ${picked.files.length} 个文件 · ${formatBytes(picked.bytes)}`
            : '尚未选择文件夹'}
        </span>
        {pickedError && <span className="text-xs font-semibold text-rose-600">{pickedError}</span>}
        <Button
          className="ml-auto"
          onClick={() => void doUpload()}
          disabled={blocked || !picked || uploading}
          loading={uploading}
          data-testid="lemon-folder-upload"
        >
          {!uploading && <FolderUp className="h-4 w-4" />}
          {uploading ? '上传中…' : blocked ? blockedLabel : '上传'}
        </Button>
      </div>
      {result && (
        <div className="mt-4 rounded-xl border border-[#e4e7ec] bg-[#fbfaf6] p-4" data-testid="lemon-folder-result">
          <div className="flex flex-wrap items-center gap-2 text-xs font-semibold text-slate-600">
            <BookOpenCheck className="h-3.5 w-3.5 text-emerald-600" />
            <b className="font-mono">{result.contestant}</b>
            写入 {result.written}/{result.total} 个文件 · {formatBytes(result.bytes)}
            {result.willJudge ? (
              <Badge variant="success">已自动触发评测</Badge>
            ) : (
              <Badge variant="warning">未自动评测，请老师在桌面端执行「全部评测」</Badge>
            )}
          </div>
          <ul className="mt-2 max-h-44 overflow-y-auto font-mono text-[11px] leading-5">
            {result.files.map((file) => (
              <li
                key={file.path}
                className={
                  file.ok
                    ? 'flex items-center gap-2 text-slate-600'
                    : 'flex items-center gap-2 text-rose-600 line-through'
                }
              >
                <span className="min-w-0 flex-1 truncate">{file.target || file.path}</span>
                <span className="shrink-0 text-slate-400">{formatBytes(file.size)}</span>
                {!file.ok && <span className="shrink-0">{file.message || '未写入'}</span>}
              </li>
            ))}
          </ul>
          {result.written < result.total && (
            <p className="mt-2 text-[11px] text-slate-400">
              {result.total - result.written} 个文件未写入，其余已成功。
            </p>
          )}
        </div>
      )}
    </section>
  )
}

/** 工作台的全部弹窗：须知 / 消息 / 查看提交代码。 */
function WorkspaceDialogs({
  noticeOpen,
  noticeText,
  onCloseNotice,
  messagesOpen,
  messages,
  onCloseMessages,
  codeTaskId,
  codeSource,
  codeLoading,
  onCloseCode,
}: {
  noticeOpen: boolean
  noticeText: string
  onCloseNotice: () => void
  messagesOpen: boolean
  messages: LemonMessage[]
  onCloseMessages: () => void
  codeTaskId: number | null
  codeSource: LemonSource | null
  codeLoading: boolean
  onCloseCode: () => void
}) {
  const toast = useToast()
  return (
    <>
      <Dialog open={noticeOpen} onClose={onCloseNotice} title="考试须知" eyebrow="来自比赛主办方">
        <div className="space-y-2 text-sm leading-7 text-slate-600">
          {noticeText
            ? noticeText.split(/\r?\n/).map((line, index) =>
                line.trim() ? <p key={index}>{line}</p> : <br key={index} />
              )
            : '加载中…'}
        </div>
      </Dialog>

      <Dialog
        open={messagesOpen}
        onClose={onCloseMessages}
        title="消息"
        eyebrow="全体广播与定向通知"
        width="md"
      >
        {messages.length === 0 ? (
          <p className="py-6 text-center text-sm text-slate-400">暂无消息</p>
        ) : (
          <div className="max-h-96 space-y-3 overflow-y-auto">
            {[...messages]
              .sort((a, b) => b.id - a.id)
              .map((message) => (
                <article key={message.id} className="rounded-xl border border-[#e4e7ec] bg-[#fbfaf6] p-3.5">
                  <header className="flex items-center gap-2 text-[11px]">
                    <Badge variant={message.target === '@all' ? 'info' : 'accent'}>
                      {message.target === '@all' ? '全体' : message.target}
                    </Badge>
                    <span className="ml-auto font-mono text-slate-400">{fmtDateTime(message.time)}</span>
                  </header>
                  <p className="mt-2 text-sm leading-6 text-slate-700">{message.text}</p>
                </article>
              ))}
          </div>
        )}
      </Dialog>

      <Dialog
        open={codeTaskId != null}
        onClose={onCloseCode}
        title="查看提交代码"
        eyebrow="最近一次提交"
        width="lg"
        footer={
          <Button
            variant="outline"
            onClick={() => {
              if (!codeSource) return
              navigator.clipboard
                ?.writeText(codeSource.content)
                .then(() => toast.success('已复制到剪贴板'))
                .catch(() => toast.error('复制失败'))
            }}
          >
            <Copy className="h-4 w-4" />
            复制
          </Button>
        }
      >
        {codeLoading || !codeSource ? (
          <LoadingBlock label="正在读取提交..." />
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <span className="rounded-lg border border-[#e4e7ec] bg-white px-2.5 py-1 font-semibold">
                文件 <b className="font-mono">{codeSource.filename}</b>
              </span>
              <span className="rounded-lg border border-[#e4e7ec] bg-white px-2.5 py-1 font-semibold">
                提交于 <b className="font-mono">{fmtDateTime(codeSource.submittedAt)}</b>
              </span>
              <span className="rounded-lg border border-[#e4e7ec] bg-white px-2.5 py-1 font-semibold">
                <b className="font-mono">{codeSource.bytes}</b> 字节
              </span>
            </div>
            <pre className="mt-3 max-h-[50vh] overflow-auto rounded-xl bg-[#14283d] p-4 font-mono text-xs leading-5 text-slate-100">
              {codeSource.content}
            </pre>
          </>
        )}
      </Dialog>
    </>
  )
}
