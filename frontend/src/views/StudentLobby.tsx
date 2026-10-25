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
  Upload,
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
  onEnterFileSubmit,
}: {
  contest: Contest
  onEnter: () => void
  onEnterFileSubmit: () => void
}) {
  const meta = STATUS_META[contest.status]
  const StatusIcon = meta.icon
  const problemCount = contest.problemIds?.length ?? 0
  const participantCount = contest.participantUserIds?.length ?? 0

  return (
    <Card
      data-testid="contest-card"
      data-contest-id={contest.id}
      className="group flex h-full flex-col overflow-hidden transition duration-300 hover:-translate-y-0.5 hover:border-forge-300 hover:shadow-lift"
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
        <CardTitle className="text-xl leading-7 transition-colors group-hover:text-forge-800">
          {contest.name}
        </CardTitle>
        <p className="line-clamp-2 min-h-10 text-xs leading-5 text-slate-500">
          {contest.description || '暂无赛事说明，进入后查看试题与考场须知。'}
        </p>
      </CardHeader>

      <CardContent className="flex flex-1 flex-col justify-end gap-4 pt-0">
        <div className="grid grid-cols-2 gap-2 rounded-xl bg-[#fbfaf6] p-3 text-[11px] text-slate-500">
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
          {contest.bindIpEnabled && (
            <span className="flex items-center gap-1.5 text-rose-700">
              <ShieldCheck className="h-3.5 w-3.5" />
              机位绑定
            </span>
          )}
        </div>

        <div className="grid grid-cols-1 gap-2">
          <Button
            data-testid="enter-contest"
            onClick={onEnter}
            variant={contest.status === 'running' ? 'primary' : 'outline'}
            className="w-full justify-between"
          >
            {contest.status === 'running' ? '进入考场' : '查看比赛'}
            <ArrowRight className="h-4 w-4" />
          </Button>
          {/* 文件提交模式（Lemon）：轻量网页提交流（在线编辑 / 上传文件 / 整包上传）。
              后端接入前由 mock 数据驱动演示。 */}
          <Button
            data-testid="enter-filesubmit"
            onClick={onEnterFileSubmit}
            variant="ghost"
            size="sm"
            className="w-full justify-between"
          >
            文件提交模式
            <Upload className="h-3.5 w-3.5" />
          </Button>
        </div>
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
    <div className="space-y-6">
      <section className="relative overflow-hidden rounded-[1.75rem] border border-[#e4e7ec] bg-[#14283d] px-6 py-8 text-white shadow-[0_24px_60px_-38px_rgba(20,40,61,0.55)] sm:px-9 sm:py-10">
        <div className="absolute inset-0 forge-grid opacity-[0.14]" aria-hidden="true" />
        <div
          className="absolute -right-20 -top-28 h-80 w-80 rounded-full bg-forge-500/25 blur-3xl"
          aria-hidden="true"
        />
        <div className="relative flex flex-col gap-7 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <p className="text-[10px] font-bold uppercase tracking-[0.22em] text-white/45">
              Contest lobby
            </p>
            <h1 className="font-display mt-2 max-w-full break-words text-[clamp(2.25rem,10vw,3rem)] font-semibold leading-[1.05] tracking-[-0.04em] [overflow-wrap:anywhere] sm:text-5xl">
              你好，{session?.name || session?.username || '选手'}
            </h1>
            <p className="mt-3 max-w-xl text-sm leading-7 text-white/60">
              选择一场比赛进入作答。你的代码草稿会保存在当前浏览器，评测结果与榜单实时更新。
            </p>
          </div>
          <div className="flex items-center gap-5 rounded-2xl border border-white/10 bg-white/[0.06] px-5 py-4 backdrop-blur-sm">
            <div>
              <p className="font-mono text-3xl font-bold text-forge-200">{runningCount}</p>
              <p className="mt-0.5 text-[10px] uppercase tracking-[0.16em] text-white/40">
                进行中
              </p>
            </div>
            <span className="h-10 w-px bg-white/10" />
            <div>
              <p className="font-mono text-3xl font-bold text-white">{contests.length}</p>
              <p className="mt-0.5 text-[10px] uppercase tracking-[0.16em] text-white/40">
                全部赛事
              </p>
            </div>
          </div>
        </div>
      </section>

      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex flex-wrap gap-1 rounded-2xl border border-[#e4e7ec] bg-white/75 p-1.5">
          {FILTERS.map((item) => (
            <button
              key={item.key}
              type="button"
              aria-pressed={filter === item.key}
              onClick={() => setFilter(item.key)}
              className={
                filter === item.key
                  ? 'h-9 rounded-xl bg-[#14283d] px-4 text-xs font-bold text-white'
                  : 'h-9 rounded-xl px-4 text-xs font-bold text-slate-500 transition hover:bg-slate-100 hover:text-slate-900'
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
              onEnter={() => navigate(`/contest/${contest.id}`)}
              onEnterFileSubmit={() => navigate(`/contest/${contest.id}/files`)}
            />
          ))}
        </div>
      )}
    </div>
  )
}
