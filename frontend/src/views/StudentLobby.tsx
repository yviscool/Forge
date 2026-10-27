import React, { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  ArrowRight,
  BookOpenCheck,
  CalendarClock,
  CheckCircle2,
  Clock3,
  PlayCircle,
  Search,
  ShieldCheck,
  Sparkles,
  Trophy,
} from 'lucide-react'
import { api, type Contest, type ContestStatus } from '../api/client'
import { Button } from '../components/ui/button'
import { Badge } from '../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { EmptyState, ErrorState, LoadingBlock } from '../components/ui/empty-state'
import { Input } from '../components/ui/input'
import { useStore } from '../store/useStore'

type ContestFilter = 'all' | ContestStatus

const FILTERS: Array<{ key: ContestFilter; label: string }> = [
  { key: 'all', label: '全部' },
  { key: 'running', label: '进行中' },
  { key: 'draft', label: '即将开始' },
  { key: 'finished', label: '已结束' },
]

const STATUS_META: Record<
  ContestStatus,
  { label: string; variant: 'success' | 'warning' | 'default'; icon: typeof Clock3 }
> = {
  running: { label: '进行中', variant: 'success', icon: PlayCircle },
  draft: { label: '未开始', variant: 'warning', icon: Clock3 },
  finished: { label: '已结束', variant: 'default', icon: CheckCircle2 },
}

function formatDate(value?: string): string {
  if (!value) return '时间待定'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '时间待定'
  return new Intl.DateTimeFormat('zh-CN', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function ContestCard({
  contest,
  onEnter,
}: {
  contest: Contest
  onEnter: (cid: string) => void
}) {
  const meta = STATUS_META[contest.status]
  const StatusIcon = meta.icon
  const problemCount = contest.problemIds?.length ?? 0
  const participantCount = contest.participantUserIds?.length ?? 0
  // 作答模式由比赛配置锁定：教师配置什么，学生进什么，不提供手动切换。
  // online=文件收卷（file 普通 / henan 河南版）；noi=NOI Linux 仿真；gesp=客观题机考。
  const modeLabel =
    contest.mode === 'noi'
      ? 'NOI Linux 仿真'
      : contest.mode === 'gesp'
        ? '客观题机考'
        : contest.answerMode === 'henan'
          ? '文件收卷 · 河南版'
          : '文件收卷'

  return (
    <Card
      data-testid="contest-card"
      data-contest-id={contest.id}
      className="group flex h-full flex-col overflow-hidden transition duration-200 hover:border-[#aab6c2] hover:shadow-lift"
    >
      <CardHeader className="gap-3 pb-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Badge variant={meta.variant} dot>
            <StatusIcon className="h-3 w-3" />
            {meta.label}
          </Badge>
          <span className="font-mono text-[10px] font-bold uppercase tracking-[0.14em] text-slate-400">
            {contest.rankingMode === 'acm' ? 'ACM' : 'OI'}
          </span>
        </div>
        <CardTitle className="text-lg leading-7 transition-colors group-hover:text-forge-800">
          {contest.name}
        </CardTitle>
        <p className="line-clamp-2 min-h-10 text-xs leading-5 text-slate-500">
          {contest.description || '暂无赛事说明，进入后查看试题与考场须知。'}
        </p>
      </CardHeader>

      <CardContent className="flex flex-1 flex-col justify-end gap-4 pt-0">
        <div className="grid grid-cols-2 gap-2 rounded-xl bg-[#f7f9fb] p-3 text-[11px] text-slate-500">
          <span className="flex items-center gap-1.5">
            <BookOpenCheck className="h-3.5 w-3.5 text-forge-600" />
            {problemCount} 道试题
          </span>
          <span className="flex items-center gap-1.5">
            <CalendarClock className="h-3.5 w-3.5 text-teal-700" />
            {formatDate(contest.startTime || contest.startedAt || contest.createdAt)}
          </span>
          <span className="flex items-center gap-1.5">
            <Trophy className="h-3.5 w-3.5 text-amber-600" />
            {participantCount ? `${participantCount} 位选手` : '等待编组'}
          </span>
          <span
            className="flex items-center gap-1.5"
            title={contest.bindIpEnabled ? '机位绑定已启用' : modeLabel}
          >
            {contest.bindIpEnabled ? (
              <>
                <ShieldCheck className="h-3.5 w-3.5 text-rose-700" />
                机位绑定
              </>
            ) : (
              modeLabel
            )}
          </span>
        </div>

        <Button
          data-testid="enter-contest"
          onClick={() => onEnter(contest.id)}
          variant={contest.status === 'running' ? 'accent' : 'outline'}
          className="w-full justify-between"
        >
          {contest.status === 'running' ? '进入考场' : '查看比赛'}
          <ArrowRight className="h-4 w-4" />
        </Button>
      </CardContent>
    </Card>
  )
}

export function StudentLobby() {
  const navigate = useNavigate()
  const session = useStore((state) => state.session)
  const [filter, setFilter] = useState<ContestFilter>('all')
  const [search, setSearch] = useState('')

  const { data: contests = [], isLoading, isError, error, refetch } = useQuery({
    queryKey: ['contests'],
    queryFn: api.contests,
  })

  const filtered = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    return contests.filter((contest) => {
      if (filter !== 'all' && contest.status !== filter) return false
      if (!keyword) return true
      return (
        contest.name.toLowerCase().includes(keyword) ||
        contest.description.toLowerCase().includes(keyword)
      )
    })
  }, [contests, filter, search])

  const runningCount = contests.filter((contest) => contest.status === 'running').length

  return (
    <div className="space-y-5">
      <section className="relative overflow-hidden rounded-2xl border border-[#132a3e] bg-navy-900 px-6 py-7 text-white sm:px-8">
        <div className="absolute inset-0 forge-grid opacity-[0.12]" aria-hidden="true" />
        <div
          className="absolute -right-16 -top-24 h-64 w-64 rounded-full bg-forge-600/15 blur-3xl"
          aria-hidden="true"
        />
        <div className="relative flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
          <div className="min-w-0">
            <h1 className="font-display max-w-full break-words text-[clamp(1.6rem,6vw,2.2rem)] leading-[1.1] [overflow-wrap:anywhere]">
              你好，{session?.name || session?.username || '选手'}
            </h1>
            <p className="mt-2 max-w-xl text-[13px] leading-6 text-white/55">
              选择一场比赛进入作答。代码草稿保存在本机，评测结果与榜单实时更新。
            </p>
          </div>
          <div className="flex shrink-0 items-center gap-5 rounded-xl border border-white/10 bg-white/[0.06] px-5 py-3.5">
            <div>
              <p className="font-mono text-2xl font-bold leading-none text-forge-200">
                {runningCount}
              </p>
              <p className="mt-1.5 text-[10px] font-bold uppercase tracking-[0.14em] text-white/40">
                进行中
              </p>
            </div>
            <span className="h-9 w-px bg-white/10" />
            <div>
              <p className="font-mono text-2xl font-bold leading-none text-white">
                {contests.length}
              </p>
              <p className="mt-1.5 text-[10px] font-bold uppercase tracking-[0.14em] text-white/40">
                全部赛事
              </p>
            </div>
          </div>
        </div>
      </section>

      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div
          className="flex w-fit flex-wrap gap-0.5 rounded-[11px] bg-slate-100/90 p-1"
          role="group"
          aria-label="按状态筛选比赛"
        >
          {FILTERS.map((item) => (
            <button
              key={item.key}
              type="button"
              aria-pressed={filter === item.key}
              onClick={() => setFilter(item.key)}
              className={
                filter === item.key
                  ? 'h-8 rounded-lg bg-white px-3.5 text-xs font-bold text-ink shadow-[0_1px_2px_rgba(19,28,43,0.08)]'
                  : 'h-8 rounded-lg px-3.5 text-xs font-bold text-slate-600 transition hover:text-slate-900'
              }
            >
              {item.label}
            </button>
          ))}
        </div>
        <label className="relative block w-full lg:w-80">
          <span className="sr-only">搜索比赛</span>
          <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="搜索比赛名称或说明"
            className="pl-10"
          />
        </label>
      </div>

      {isLoading ? (
        <LoadingBlock label="正在获取比赛列表..." />
      ) : isError ? (
        <ErrorState
          message={error instanceof Error ? error.message : '比赛列表加载失败'}
          action={<Button onClick={() => void refetch()}>重新加载</Button>}
        />
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={Sparkles}
          title={contests.length ? '没有符合条件的比赛' : '暂时还没有比赛'}
          description={
            contests.length
              ? '尝试切换状态或清空搜索关键词。'
              : '教师发布比赛后，会在这里出现。'
          }
        />
      ) : (
        <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
          {filtered.map((contest) => (
            <ContestCard
              key={contest.id}
              contest={contest}
              onEnter={(cid) => {
                // 作答模式随比赛配置自动路由：online=文件收卷（普通/河南版），
                // noi=NOI Linux 仿真作答，gesp=客观题机考。学生不手动选模式。
                if (contest.mode === 'online') {
                  navigate(`/contest/${cid}/${contest.answerMode === 'henan' ? 'henan' : 'files'}`)
                  return
                }
                navigate(`/contest/${cid}`)
              }}
            />
          ))}
        </div>
      )}
    </div>
  )
}
