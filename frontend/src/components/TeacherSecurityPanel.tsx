import React, { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  AlertCircle,
  AlertTriangle,
  CalendarClock,
  CheckCircle2,
  Clock3,
  Globe2,
  KeyRound,
  Loader2,
  LockKeyhole,
  Megaphone,
  MessageSquareWarning,
  Radio,
  RefreshCw,
  Send,
  ShieldCheck,
  ShieldOff,
  TimerReset,
  Trash2,
  Unplug,
} from 'lucide-react'
import {
  ApiError,
  type BroadcastMessage,
  type Contest,
  type User,
  api,
} from '../api/client'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Card, CardContent } from './ui/card'
import { Dialog } from './ui/dialog'
import { Field, Input, Select, Textarea } from './ui/input'
import { useToast } from './ui/toast'

interface TeacherSecurityPanelProps {
  contest: Contest
  users: User[]
  refetchContest: () => void | Promise<unknown>
}

type BroadcastLevel = BroadcastMessage['level']

interface BroadcastForm {
  title: string
  content: string
  targetUserId: string
  level: BroadcastLevel
}

interface WindowForm {
  windowEnabled: boolean
  startTime: string
  endTime: string
  durationMinutes: string
}

interface ConfirmState {
  title: string
  description: string
  confirmLabel: string
  tone: 'danger' | 'warning'
  action: () => Promise<void>
}

const DEFAULT_DURATION = 150
const MAX_EXTENSION_MINUTES = 24 * 60

function errorMessage(cause: unknown, fallback: string): string {
  if (cause instanceof ApiError) return cause.message
  if (cause instanceof Error && cause.message) return cause.message
  return fallback
}

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

/** Convert an ISO timestamp to the local wall-clock string datetime-local expects. */
function isoToLocalInput(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours()
  )}:${pad(date.getMinutes())}`
}

/** datetime-local is intentionally interpreted in the teacher's local timezone. */
function localInputToIso(value: string): string | undefined {
  if (!value) return undefined
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return undefined
  return date.toISOString()
}

function formatDateTime(value?: string): string {
  if (!value) return '未设置'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '格式异常'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function displayUser(user: User | undefined, userId: string): string {
  if (!user) return userId
  return user.name ? `${user.name} · ${user.username}` : user.username || userId
}

function SectionHeading({
  icon: Icon,
  title,
  description,
  action,
}: {
  icon: React.ComponentType<{ className?: string }>
  title: string
  description: string
  action?: React.ReactNode
}) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-3 border-b border-[#eaecf0] bg-[#fbfaf6]/60 px-5 py-4">
      <div className="flex min-w-0 items-start gap-3">
        <span className="grid h-9 w-9 shrink-0 place-items-center rounded-xl border border-[#e4e7ec] bg-white text-[#14283d] shadow-sm">
          <Icon className="h-4 w-4" />
        </span>
        <div className="min-w-0">
          <h3 className="text-sm font-extrabold tracking-tight text-[#14283d]">{title}</h3>
          <p className="mt-0.5 text-[11px] leading-5 text-slate-500">{description}</p>
        </div>
      </div>
      {action}
    </div>
  )
}

function InlineError({ message }: { message: string }) {
  return (
    <div
      role="alert"
      className="flex gap-2.5 rounded-xl border border-rose-200 bg-rose-50 px-3.5 py-3 text-xs font-semibold leading-5 text-rose-700"
    >
      <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{message}</span>
    </div>
  )
}

export const TeacherSecurityPanel: React.FC<TeacherSecurityPanelProps> = ({
  contest,
  users,
  refetchContest,
}) => {
  const toast = useToast()
  const [noticeText, setNoticeText] = useState(contest.notice ?? '')
  // The server never returns an existing password. This field is a one-shot
  // input for setting or rotating the verifier.
  const [zipPassword, setZipPassword] = useState('')
  const [showZipPassword, setShowZipPassword] = useState(false)
  const [windowForm, setWindowForm] = useState<WindowForm>({
    windowEnabled: Boolean(contest.windowEnabled),
    startTime: isoToLocalInput(contest.startTime),
    endTime: isoToLocalInput(contest.endTime),
    durationMinutes: String(contest.durationMinutes || DEFAULT_DURATION),
  })
  const [broadcastForm, setBroadcastForm] = useState<BroadcastForm>({
    title: '',
    content: '',
    targetUserId: '',
    level: 'info',
  })
  const [extensionUserId, setExtensionUserId] = useState('')
  const [extensionMinutes, setExtensionMinutes] = useState('10')
  const [busyAction, setBusyAction] = useState<string | null>(null)
  const [confirmState, setConfirmState] = useState<ConfirmState | null>(null)
  const [confirming, setConfirming] = useState(false)

  const {
    data: ipData,
    isPending: ipPending,
    isError: ipError,
    error: ipErrorCause,
    refetch: refetchIps,
    isFetching: ipFetching,
  } = useQuery({
    queryKey: ['ipBindings', contest.id],
    queryFn: () => api.getIpBindings(contest.id),
    enabled: Boolean(contest.id),
  })

  const {
    data: messages = [],
    isPending: messagesPending,
    isError: messagesError,
    error: messagesErrorCause,
    refetch: refetchMessages,
    isFetching: messagesFetching,
  } = useQuery({
    queryKey: ['messages', contest.id],
    queryFn: () => api.messages(contest.id),
    enabled: Boolean(contest.id),
  })

  useEffect(() => {
    setNoticeText(contest.notice ?? '')
    setZipPassword('')
    setShowZipPassword(false)
    setWindowForm({
      windowEnabled: Boolean(contest.windowEnabled),
      startTime: isoToLocalInput(contest.startTime),
      endTime: isoToLocalInput(contest.endTime),
      durationMinutes: String(contest.durationMinutes || DEFAULT_DURATION),
    })
  }, [
    contest.id,
    contest.notice,
    contest.zipProtected,
    contest.windowEnabled,
    contest.startTime,
    contest.endTime,
    contest.durationMinutes,
  ])

  const bindings = useMemo(() => ipData?.boundIps ?? {}, [ipData?.boundIps])
  const ipEnabled = ipData?.bindIpEnabled ?? Boolean(contest.bindIpEnabled)
  const extensions = useMemo(() => contest.extensions ?? {}, [contest.extensions])

  const boundEntries = useMemo(() => {
    const byId = new Map(users.map((user) => [user.id, user]))
    return Object.entries(bindings)
      .sort(([, a], [, b]) => a.localeCompare(b))
      .map(([userId, ip]) => ({ userId, ip, user: byId.get(userId) }))
  }, [bindings, users])

  const extensionEntries = useMemo(() => {
    const byId = new Map(users.map((user) => [user.id, user]))
    return Object.entries(extensions).map(([userId, minutes]) => ({
      userId,
      minutes,
      user: byId.get(userId),
    }))
  }, [extensions, users])

  const sortedMessages = useMemo(
    () =>
      [...messages].sort(
        (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
      ),
    [messages]
  )

  const reportError = (cause: unknown, fallback: string) => {
    toast.error(fallback, errorMessage(cause, '请稍后重试。'))
  }

  const handleToggleIpBinding = async () => {
    if (busyAction) return
    const nextEnabled = !ipEnabled
    setBusyAction('ip-toggle')
    try {
      await api.setIpBindingEnabled(contest.id, nextEnabled)
      await Promise.all([refetchIps(), refetchContest()])
      toast.success(
        nextEnabled ? '机位强绑定已开启' : '机位强绑定已关闭',
        nextEnabled
          ? '考生首次提交时将自动锁定当前机位 IP。'
          : '关闭期间考生可以从任意机位提交。'
      )
    } catch (cause) {
      reportError(cause, '更新机位绑定开关失败')
    } finally {
      setBusyAction(null)
    }
  }

  const unbindUser = (userId: string) => {
    const user = users.find((item) => item.id === userId)
    setConfirmState({
      title: '解除该考生的机位绑定？',
      description: `${displayUser(user, userId)} 当前绑定 IP ${bindings[userId]}。解绑后，该考生下一次提交会重新锁定当时使用的机位。`,
      confirmLabel: '解绑机位',
      tone: 'warning',
      action: async () => {
        await api.unbindUserIp(contest.id, userId)
        await refetchIps()
        toast.success('机位已解绑', `${displayUser(user, userId)} 可在新机位重新提交。`)
      },
    })
  }

  const unbindAll = () => {
    if (boundEntries.length === 0) return
    setConfirmState({
      title: '重置全考场机位绑定？',
      description: `当前 ${boundEntries.length} 个已锁定机位将全部解绑。所有考生下一次提交时都会重新锁定新机位，请仅在换场或确实需要时操作。`,
      confirmLabel: '全部解绑',
      tone: 'danger',
      action: async () => {
        await api.unbindAllIps(contest.id)
        await refetchIps()
        toast.success('全考场机位已重置', '考生下一次提交会按当前机位重新绑定。')
      },
    })
  }

  const handleSaveWindow = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (busyAction) return

    const startIso = localInputToIso(windowForm.startTime)
    const endIso = localInputToIso(windowForm.endTime)
    const duration = Number(windowForm.durationMinutes)
    if (windowForm.startTime && !startIso) {
      toast.error('开始时间无效', '请重新选择开始时间。')
      return
    }
    if (windowForm.endTime && !endIso) {
      toast.error('截止时间无效', '请重新选择截止时间。')
      return
    }
    if (startIso && endIso && new Date(startIso).getTime() >= new Date(endIso).getTime()) {
      toast.error('时间顺序不正确', '截止时间必须晚于开始时间。')
      return
    }
    if (!Number.isFinite(duration) || duration <= 0) {
      toast.error('考试时长无效', '考试时长必须是大于 0 的分钟数。')
      return
    }

    setBusyAction('window')
    try {
      await api.setContestWindow(contest.id, {
        windowEnabled: windowForm.windowEnabled,
        startTime: startIso,
        endTime: endIso,
        durationMinutes: duration,
      })
      await refetchContest()
      toast.success(
        '时间窗口已保存',
        windowForm.windowEnabled
          ? '提交权限将按服务器时间严格切断。'
          : '当前仅保存时间信息，未启用硬切断。'
      )
    } catch (cause) {
      reportError(cause, '保存考试时间窗口失败')
    } finally {
      setBusyAction(null)
    }
  }

  const handleApplyExtension = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (busyAction) return
    if (!extensionUserId) {
      toast.error('请选择考生', '只有指定考生才能获得单独加时。')
      return
    }
    const minutes = Number(extensionMinutes)
    if (!Number.isInteger(minutes) || minutes <= 0 || minutes > MAX_EXTENSION_MINUTES) {
      toast.error('加时分钟无效', `请输入 1 - ${MAX_EXTENSION_MINUTES} 之间的整数。`)
      return
    }

    const user = users.find((item) => item.id === extensionUserId)
    setBusyAction('extension')
    try {
      await api.setUserExtension(contest.id, extensionUserId, minutes)
      await refetchContest()
      setExtensionUserId('')
      setExtensionMinutes('10')
      toast.success(
        '加时已生效',
        `${displayUser(user, extensionUserId)} 的提交截止时间延长 ${minutes} 分钟。`
      )
    } catch (cause) {
      reportError(cause, '设置考生加时失败')
    } finally {
      setBusyAction(null)
    }
  }

  const removeExtension = (userId: string) => {
    const user = users.find((item) => item.id === userId)
    setConfirmState({
      title: '移除该考生的加时？',
      description: `${displayUser(user, userId)} 当前拥有 +${extensions[userId]} 分钟。移除后将按考试统一截止时间判卷。`,
      confirmLabel: '移除加时',
      tone: 'warning',
      action: async () => {
        await api.removeUserExtension(contest.id, userId)
        await refetchContest()
        toast.success('加时已移除')
      },
    })
  }

  const handleSaveNotice = async () => {
    if (busyAction) return
    setBusyAction('notice')
    try {
      await api.updateNotice(contest.id, noticeText)
      await refetchContest()
      toast.success(
        '考场通告已更新',
        noticeText.trim() ? '新通告会实时显示在考生端。' : '置顶通告已清空。'
      )
    } catch (cause) {
      reportError(cause, '更新考场通告失败')
    } finally {
      setBusyAction(null)
    }
  }

  const handleSendBroadcast = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (busyAction) return
    if (!broadcastForm.title.trim() && !broadcastForm.content.trim()) {
      toast.error('广播内容为空', '请至少填写标题或正文。')
      return
    }

    const wasTargeted = Boolean(broadcastForm.targetUserId)
    setBusyAction('broadcast')
    try {
      await api.sendMessage(contest.id, {
        title: broadcastForm.title.trim(),
        content: broadcastForm.content.trim(),
        targetUserId: broadcastForm.targetUserId || undefined,
        level: broadcastForm.level,
      })
      setBroadcastForm({ title: '', content: '', targetUserId: '', level: 'info' })
      await refetchMessages()
      toast.success(
        '广播已下发',
        wasTargeted ? '定向消息仅目标考生可见。' : '全员消息已实时推送。'
      )
    } catch (cause) {
      reportError(cause, '发送考场广播失败')
    } finally {
      setBusyAction(null)
    }
  }

  const handleSaveZipPassword = async () => {
    if (busyAction) return
    const nextPassword = zipPassword.trim()
    setBusyAction('zip')
    try {
      await api.setZipPassword(contest.id, nextPassword)
      setZipPassword('')
      setShowZipPassword(false)
      await refetchContest()
      toast.success(
        'Zip 解锁口令已保存',
        nextPassword ? '考生需输入口令后解锁试卷。' : '已取消试卷口令保护。'
      )
    } catch (cause) {
      reportError(cause, '保存 Zip 解锁口令失败')
    } finally {
      setBusyAction(null)
    }
  }

  const runConfirmedAction = async () => {
    if (!confirmState || confirming) return
    setConfirming(true)
    try {
      await confirmState.action()
      setConfirmState(null)
    } catch (cause) {
      reportError(cause, '操作失败')
    } finally {
      setConfirming(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="overflow-hidden rounded-2xl border border-[#e4e7ec] bg-[#14283d] text-white shadow-[0_20px_50px_-36px_rgba(20,40,61,0.8)]">
        <div className="relative p-5 sm:p-6">
          <div
            className="pointer-events-none absolute -right-16 -top-20 h-56 w-56 rounded-full bg-forge-500/20 blur-3xl"
            aria-hidden="true"
          />
          <div className="relative flex flex-wrap items-start justify-between gap-4">
            <div className="flex items-start gap-3">
              <span className="grid h-11 w-11 place-items-center rounded-2xl border border-white/10 bg-white/10">
                <ShieldCheck className="h-5 w-5 text-forge-200" />
              </span>
              <div>
                <p className="text-[10px] font-bold uppercase tracking-[0.22em] text-white/65">
                  Exam integrity
                </p>
                <h2 className="mt-1 text-xl font-extrabold tracking-tight">
                  考务安全与机房调度
                </h2>
                <p className="mt-1 max-w-xl text-xs leading-5 text-white/55">
                  机位、时间、加时、通告和广播全部实时同步。所有操作直接写入当前考场。
                </p>
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <Badge variant="outline" className="border-white/15 bg-white/5 text-white/70">
                考场 {contest.name}
              </Badge>
              <Badge
                variant={
                  contest.status === 'running'
                    ? 'success'
                    : contest.status === 'finished'
                      ? 'default'
                      : 'warning'
                }
                dot
              >
                {contest.status === 'running'
                  ? '进行中'
                  : contest.status === 'finished'
                    ? '已结束'
                    : '未开始'}
              </Badge>
            </div>
          </div>
          <div className="relative mt-5 grid gap-2 sm:grid-cols-3">
            <div className="rounded-xl border border-white/10 bg-white/[0.055] px-3.5 py-3">
              <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-white/65">
                机位锁定
              </p>
              <p className="mt-1 font-mono text-base font-black">
                {ipEnabled ? `${boundEntries.length} 台` : '关闭'}
              </p>
            </div>
            <div className="rounded-xl border border-white/10 bg-white/[0.055] px-3.5 py-3">
              <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-white/65">
                单独加时
              </p>
              <p className="mt-1 font-mono text-base font-black">{extensionEntries.length} 人</p>
            </div>
            <div className="rounded-xl border border-white/10 bg-white/[0.055] px-3.5 py-3">
              <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-white/65">
                已发广播
              </p>
              <p className="mt-1 font-mono text-base font-black">{messages.length} 条</p>
            </div>
          </div>
        </div>
      </div>

      <Card className="overflow-hidden">
        <SectionHeading
          icon={ShieldCheck}
          title="机位 IP 强绑定"
          description="首次提交即锁定机位；换机时由监考教师单点解绑。"
          action={
            <div className="flex flex-wrap items-center gap-2">
              <Button
                type="button"
                size="sm"
                variant={ipEnabled ? 'success' : 'outline'}
                loading={busyAction === 'ip-toggle'}
                onClick={handleToggleIpBinding}
                disabled={Boolean(busyAction)}
              >
                {ipEnabled ? (
                  <ShieldCheck className="h-3.5 w-3.5" />
                ) : (
                  <ShieldOff className="h-3.5 w-3.5" />
                )}
                {ipEnabled ? '已开启' : '已关闭'}
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="border-rose-200 text-rose-700 hover:bg-rose-50"
                onClick={unbindAll}
                disabled={Boolean(busyAction) || boundEntries.length === 0}
              >
                <Unplug className="h-3.5 w-3.5" />
                重置全部机位
              </Button>
              <Button
                type="button"
                size="icon"
                variant="ghost"
                aria-label="刷新机位绑定"
                onClick={() => void refetchIps()}
                disabled={ipFetching}
              >
                <RefreshCw className={`h-4 w-4 ${ipFetching ? 'animate-spin' : ''}`} />
              </Button>
            </div>
          }
        />
        <CardContent className="space-y-4 p-5">
          {ipError ? (
            <InlineError message={errorMessage(ipErrorCause, '读取机位绑定失败，请重试。')} />
          ) : ipPending ? (
            <div className="flex items-center gap-2 py-8 text-sm text-slate-400">
              <Loader2 className="h-4 w-4 animate-spin" /> 正在读取机位绑定...
            </div>
          ) : boundEntries.length === 0 ? (
            <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[#d0d5dd] bg-[#fbfaf6]/60 px-6 py-10 text-center">
              <Globe2 className="h-6 w-6 text-slate-300" />
              <p className="mt-3 text-sm font-bold text-slate-600">尚无机位锁定记录</p>
              <p className="mt-1 max-w-md text-xs leading-5 text-slate-400">
                {ipEnabled
                  ? '考生首次提交后将自动绑定所在机位 IP。'
                  : '开启机位强绑定后，考生首次提交将自动锁定 IP。'}
              </p>
            </div>
          ) : (
            <div className="overflow-hidden rounded-xl border border-[#e4e7ec]">
              <div className="hidden grid-cols-[minmax(0,1.25fr)_minmax(0,1fr)_auto_auto] gap-3 border-b border-[#eaecf0] bg-[#f9fafb] px-4 py-2.5 text-[10px] font-bold uppercase tracking-[0.12em] text-slate-400 sm:grid">
                <span>考生</span>
                <span>锁定 IP</span>
                <span>状态</span>
                <span className="text-right">操作</span>
              </div>
              <div className="divide-y divide-[#eaecf0]">
                {boundEntries.map(({ userId, user, ip }) => (
                  <div
                    key={userId}
                    className="grid gap-2 px-4 py-3 text-xs sm:grid-cols-[minmax(0,1.25fr)_minmax(0,1fr)_auto_auto] sm:items-center sm:gap-3"
                  >
                    <div className="min-w-0">
                      <p className="truncate font-bold text-[#14283d]">
                        {user?.name || '未知考生'}
                      </p>
                      <p className="mt-0.5 truncate font-mono text-[10px] text-slate-400">
                        {user?.username || userId}
                      </p>
                    </div>
                    <code className="w-fit rounded-md border border-sky-100 bg-sky-50 px-2 py-1 font-mono text-[11px] font-bold text-sky-700">
                      {ip}
                    </code>
                    <Badge variant="success" dot>
                      已锁定
                    </Badge>
                    <div className="sm:text-right">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="text-rose-700 hover:bg-rose-50"
                        onClick={() => unbindUser(userId)}
                        disabled={Boolean(busyAction)}
                      >
                        <Unplug className="h-3.5 w-3.5" />
                        换机解绑
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-2">
        <Card className="overflow-hidden">
          <SectionHeading
            icon={CalendarClock}
            title="严格考试时间窗口"
            description="启用后按服务器时间硬切断提交，避免本机改表绕过。"
            action={
              <Badge variant={windowForm.windowEnabled ? 'success' : 'default'} dot>
                {windowForm.windowEnabled ? '硬切断开启' : '硬切断关闭'}
              </Badge>
            }
          />
          <CardContent className="p-5">
            <form onSubmit={handleSaveWindow} className="space-y-4">
              <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-[#e4e7ec] bg-[#fbfaf6]/60 p-3.5">
                <input
                  type="checkbox"
                  checked={windowForm.windowEnabled}
                  onChange={(event) =>
                    setWindowForm((current) => ({
                      ...current,
                      windowEnabled: event.target.checked,
                    }))
                  }
                  className="mt-0.5 h-4 w-4 rounded border-[#d0d5dd] accent-forge-700"
                />
                <span>
                  <span className="block text-xs font-bold text-slate-700">
                    启用时间窗口硬切断
                  </span>
                  <span className="mt-0.5 block text-[11px] leading-5 text-slate-400">
                    开始前和截止后，服务端都会拒绝提交。关闭时仅保存计划时间。
                  </span>
                </span>
              </label>
              <div className="grid gap-3 sm:grid-cols-2">
                <Field label="开始时间" hint="本地时区">
                  <Input
                    type="datetime-local"
                    aria-label="开始时间"
                    value={windowForm.startTime}
                    onChange={(event) =>
                      setWindowForm((current) => ({
                        ...current,
                        startTime: event.target.value,
                      }))
                    }
                  />
                </Field>
                <Field label="截止时间" hint="本地时区">
                  <Input
                    type="datetime-local"
                    aria-label="截止时间"
                    value={windowForm.endTime}
                    onChange={(event) =>
                      setWindowForm((current) => ({
                        ...current,
                        endTime: event.target.value,
                      }))
                    }
                  />
                </Field>
              </div>
              <Field label="计划考试时长（分钟）" hint="用于赛程说明">
                <Input
                  type="number"
                  aria-label="计划考试时长"
                  min={1}
                  step={1}
                  value={windowForm.durationMinutes}
                  onChange={(event) =>
                    setWindowForm((current) => ({
                      ...current,
                      durationMinutes: event.target.value,
                    }))
                  }
                />
              </Field>
              <div className="rounded-xl border border-sky-200 bg-sky-50/60 px-3.5 py-3 text-[11px] leading-5 text-sky-800">
                <p className="flex items-center gap-1.5 font-bold">
                  <Clock3 className="h-3.5 w-3.5" />
                  当前计划
                </p>
                <p className="mt-1">
                  {formatDateTime(
                    windowForm.startTime ? localInputToIso(windowForm.startTime) : undefined
                  )}
                  <span className="mx-2 text-sky-400">→</span>
                  {formatDateTime(
                    windowForm.endTime ? localInputToIso(windowForm.endTime) : undefined
                  )}
                </p>
              </div>
              <Button
                type="submit"
                className="w-full"
                loading={busyAction === 'window'}
                disabled={Boolean(busyAction)}
              >
                <CalendarClock className="h-4 w-4" />
                保存时间窗口
              </Button>
            </form>
          </CardContent>
        </Card>

        <Card className="overflow-hidden">
          <SectionHeading
            icon={TimerReset}
            title="故障换机精准加时"
            description="仅对指定考生延长服务端截止时间，其他考生不受影响。"
            action={
              <Badge variant={extensionEntries.length ? 'warning' : 'default'}>
                {extensionEntries.length} 人已加时
              </Badge>
            }
          />
          <CardContent className="space-y-4 p-5">
            <form onSubmit={handleApplyExtension} className="space-y-3">
              <Field label="选择考生">
                <Select
                  aria-label="选择考生"
                  value={extensionUserId}
                  onChange={(event) => setExtensionUserId(event.target.value)}
                >
                  <option value="">请选择需要补偿的考生...</option>
                  {users.map((user) => (
                    <option key={user.id} value={user.id}>
                      {user.name} · {user.username}
                    </option>
                  ))}
                </Select>
              </Field>
              <div className="flex gap-2">
                <Input
                  type="number"
                  min={1}
                  max={MAX_EXTENSION_MINUTES}
                  step={1}
                  aria-label="加时分钟数"
                  value={extensionMinutes}
                  onChange={(event) => setExtensionMinutes(event.target.value)}
                />
                <Button
                  type="submit"
                  className="shrink-0"
                  loading={busyAction === 'extension'}
                  disabled={Boolean(busyAction)}
                >
                  <TimerReset className="h-4 w-4" />
                  应用加时
                </Button>
              </div>
              <div className="flex flex-wrap gap-1.5">
                {[5, 10, 15, 20, 30].map((minutes) => (
                  <button
                    key={minutes}
                    type="button"
                    onClick={() => setExtensionMinutes(String(minutes))}
                    className="rounded-lg border border-[#e4e7ec] bg-white px-2.5 py-1 text-[11px] font-bold text-slate-500 transition hover:border-forge-300 hover:text-forge-700"
                  >
                    +{minutes} 分钟
                  </button>
                ))}
              </div>
            </form>

            <div className="rounded-xl border border-[#e4e7ec]">
              <div className="border-b border-[#eaecf0] bg-[#f9fafb] px-3.5 py-2.5 text-[10px] font-bold uppercase tracking-[0.12em] text-slate-400">
                当前加时名单
              </div>
              {extensionEntries.length === 0 ? (
                <p className="px-3.5 py-8 text-center text-xs text-slate-400">
                  暂无单独加时考生
                </p>
              ) : (
                <div className="max-h-52 divide-y divide-[#eaecf0] overflow-y-auto scrollbar-slim">
                  {extensionEntries.map(({ userId, user, minutes }) => (
                    <div
                      key={userId}
                      className="flex items-center justify-between gap-3 px-3.5 py-3"
                    >
                      <div className="min-w-0">
                        <p className="truncate text-xs font-bold text-[#14283d]">
                          {user?.name || userId}
                        </p>
                        <p className="mt-0.5 text-[10px] text-slate-400">
                          {user?.username || '未知账号'}
                        </p>
                      </div>
                      <div className="flex items-center gap-2">
                        <Badge variant="warning">+{minutes} 分钟</Badge>
                        <Button
                          type="button"
                          size="icon"
                          variant="ghost"
                          aria-label={`移除 ${displayUser(user, userId)} 的加时`}
                          onClick={() => removeExtension(userId)}
                          disabled={Boolean(busyAction)}
                        >
                          <Trash2 className="h-3.5 w-3.5 text-rose-600" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-6 xl:grid-cols-2">
        <Card className="overflow-hidden">
          <SectionHeading
            icon={Radio}
            title="考场置顶通告与 Zip 口令"
            description="置顶通告实时同步到考生工作台；Zip 口令用于试卷包保护。"
          />
          <CardContent className="space-y-5 p-5">
            <div className="space-y-2">
              <Field label="置顶通告" hint="支持纯文本">
                <Textarea
                  aria-label="置顶通告"
                  rows={4}
                  value={noticeText}
                  onChange={(event) => setNoticeText(event.target.value)}
                  placeholder="例如：T3 样例第三行输出已更正，请刷新题面。"
                />
              </Field>
              <div className="flex items-center justify-between gap-3">
                <p className="text-[10px] text-slate-400">留空并保存即可清除置顶通告。</p>
                <Button
                  type="button"
                  size="sm"
                  loading={busyAction === 'notice'}
                  disabled={Boolean(busyAction)}
                  onClick={handleSaveNotice}
                >
                  <Megaphone className="h-3.5 w-3.5" />
                  保存通告
                </Button>
              </div>
            </div>

            <div className="border-t border-[#eaecf0] pt-4">
              <Field
                label="试卷包 Zip 解锁口令"
                hint={
                  contest.zipProtected
                    ? '当前已配置，服务端不会回显旧口令；留空保存即关闭保护'
                    : '留空即关闭保护'
                }
              >
                <div className="relative">
                  <Input
                    aria-label="试卷包 Zip 解锁口令"
                    type={showZipPassword ? 'text' : 'password'}
                    value={zipPassword}
                    onChange={(event) => setZipPassword(event.target.value)}
                    placeholder={
                      contest.zipProtected ? '输入新口令以轮换保护' : '例如：NOIP2026-PASS'
                    }
                    className="pr-10"
                    autoComplete="new-password"
                  />
                  <button
                    type="button"
                    aria-label={showZipPassword ? '隐藏 Zip 口令' : '显示 Zip 口令'}
                    onClick={() => setShowZipPassword((value) => !value)}
                    className="absolute right-1.5 top-1/2 grid h-7 w-7 -translate-y-1/2 place-items-center rounded-lg text-slate-400 transition hover:bg-slate-100 hover:text-slate-700"
                  >
                    {showZipPassword ? (
                      <LockKeyhole className="h-3.5 w-3.5" />
                    ) : (
                      <KeyRound className="h-3.5 w-3.5" />
                    )}
                  </button>
                </div>
              </Field>
              <div className="mt-2 flex items-center justify-between gap-3">
                <p className="text-[10px] text-slate-400">
                  {contest.zipProtected ? '当前状态：已配置（不回显）' : '当前状态：未保护'}
                  ，保存后立即对考生端生效。
                </p>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  loading={busyAction === 'zip'}
                  disabled={Boolean(busyAction)}
                  onClick={handleSaveZipPassword}
                >
                  <KeyRound className="h-3.5 w-3.5" />
                  保存口令
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="overflow-hidden">
          <SectionHeading
            icon={MessageSquareWarning}
            title="下发考场广播"
            description="全员广播或定向提醒，通过 SSE 实时推送到考生端。"
            action={
              <Button
                type="button"
                size="icon"
                variant="ghost"
                aria-label="刷新广播记录"
                onClick={() => void refetchMessages()}
                disabled={messagesFetching}
              >
                <RefreshCw className={`h-4 w-4 ${messagesFetching ? 'animate-spin' : ''}`} />
              </Button>
            }
          />
          <CardContent className="space-y-4 p-5">
            <form onSubmit={handleSendBroadcast} className="space-y-3">
              <div className="grid gap-2 sm:grid-cols-2">
                <Select
                  aria-label="广播对象"
                  value={broadcastForm.targetUserId}
                  onChange={(event) =>
                    setBroadcastForm((current) => ({
                      ...current,
                      targetUserId: event.target.value,
                    }))
                  }
                >
                  <option value="">全场全员广播</option>
                  {users.map((user) => (
                    <option key={user.id} value={user.id}>
                      定向：{user.name} · {user.username}
                    </option>
                  ))}
                </Select>
                <Select
                  aria-label="广播级别"
                  value={broadcastForm.level}
                  onChange={(event) =>
                    setBroadcastForm((current) => ({
                      ...current,
                      level: event.target.value as BroadcastLevel,
                    }))
                  }
                >
                  <option value="info">普通通知</option>
                  <option value="warning">重要提醒</option>
                  <option value="urgent">紧急变更</option>
                </Select>
              </div>
              <Input
                value={broadcastForm.title}
                onChange={(event) =>
                  setBroadcastForm((current) => ({ ...current, title: event.target.value }))
                }
                placeholder="广播标题（例如：T2 数据说明）"
                maxLength={120}
              />
              <Textarea
                rows={3}
                value={broadcastForm.content}
                onChange={(event) =>
                  setBroadcastForm((current) => ({ ...current, content: event.target.value }))
                }
                placeholder="广播正文..."
                maxLength={2000}
              />
              <Button
                type="submit"
                className="w-full"
                loading={busyAction === 'broadcast'}
                disabled={Boolean(busyAction)}
              >
                <Send className="h-4 w-4" />
                立即下发广播
              </Button>
            </form>

            <div className="border-t border-[#eaecf0] pt-4">
              <div className="mb-2.5 flex items-center justify-between">
                <p className="text-xs font-bold text-slate-600">最近广播</p>
                <Badge variant="default">{messages.length} 条</Badge>
              </div>
              {messagesError ? (
                <InlineError
                  message={errorMessage(messagesErrorCause, '读取广播记录失败。')}
                />
              ) : messagesPending ? (
                <div className="flex items-center gap-2 py-5 text-xs text-slate-400">
                  <Loader2 className="h-3.5 w-3.5 animate-spin" /> 正在读取广播...
                </div>
              ) : sortedMessages.length === 0 ? (
                <p className="rounded-xl border border-dashed border-[#d0d5dd] px-4 py-7 text-center text-xs text-slate-400">
                  还没有下发过广播
                </p>
              ) : (
                <div className="max-h-60 space-y-2 overflow-y-auto pr-1 scrollbar-slim">
                  {sortedMessages.map((message) => {
                    const target = users.find((user) => user.id === message.targetUserId)
                    const levelStyle =
                      message.level === 'urgent'
                        ? 'border-rose-200 bg-rose-50/70'
                        : message.level === 'warning'
                          ? 'border-amber-200 bg-amber-50/60'
                          : 'border-[#e4e7ec] bg-[#fbfaf6]'
                    return (
                      <div
                        key={message.id}
                        className={`rounded-xl border px-3.5 py-3 ${levelStyle}`}
                      >
                        <div className="flex items-start justify-between gap-2">
                          <p className="min-w-0 text-xs font-bold text-[#14283d]">
                            {message.title || '无标题广播'}
                          </p>
                          <Badge
                            variant={
                              message.level === 'urgent'
                                ? 'danger'
                                : message.level === 'warning'
                                  ? 'warning'
                                  : 'info'
                            }
                          >
                            {message.level === 'urgent'
                              ? '紧急'
                              : message.level === 'warning'
                                ? '重要'
                                : '通知'}
                          </Badge>
                        </div>
                        {message.content && (
                          <p className="mt-1 whitespace-pre-wrap text-[11px] leading-5 text-slate-600">
                            {message.content}
                          </p>
                        )}
                        <p className="mt-1.5 text-[10px] text-slate-400">
                          {message.senderName || '监考教师'} ·{' '}
                          {target ? `定向 ${target.name}` : '全员'} ·{' '}
                          {formatDateTime(message.createdAt)}
                        </p>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="flex items-start gap-2.5 rounded-xl border border-amber-200 bg-amber-50/60 px-4 py-3 text-[11px] leading-5 text-amber-900">
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />
        <p>
          机位解绑与加时都会即时影响正在考试的考生。执行前请确认考生身份与机房座位，避免误操作。
        </p>
      </div>

      <Dialog
        open={Boolean(confirmState)}
        onClose={() => {
          if (!confirming) setConfirmState(null)
        }}
        title={confirmState?.title || '确认操作'}
        eyebrow="Safety check"
        width="sm"
        dismissible={!confirming}
        footer={
          <>
            <Button
              type="button"
              variant="outline"
              onClick={() => setConfirmState(null)}
              disabled={confirming}
            >
              取消
            </Button>
            <Button
              type="button"
              variant={confirmState?.tone === 'danger' ? 'danger' : 'primary'}
              loading={confirming}
              onClick={() => void runConfirmedAction()}
            >
              <CheckCircle2 className="h-4 w-4" />
              {confirmState?.confirmLabel || '确认'}
            </Button>
          </>
        }
      >
        <div className="flex gap-3">
          <span
            className={`grid h-10 w-10 shrink-0 place-items-center rounded-xl ${
              confirmState?.tone === 'danger'
                ? 'bg-rose-100 text-rose-700'
                : 'bg-amber-100 text-amber-700'
            }`}
          >
            <AlertTriangle className="h-5 w-5" />
          </span>
          <p className="text-sm leading-6 text-slate-600">{confirmState?.description}</p>
        </div>
      </Dialog>
    </div>
  )
}
