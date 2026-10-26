import React, { useMemo, useState } from 'react'
import { Search, Trophy } from 'lucide-react'
import type { ACMRankEntry, Problem, RankEntry } from '../api/client'
import { Badge } from './ui/badge'
import { EmptyState } from './ui/empty-state'

export type ScoreboardEntry = RankEntry | ACMRankEntry

interface ScoreboardProps {
  ranks: ScoreboardEntry[]
  rankingMode?: 'oi' | 'acm'
  problems?: Problem[]
  highlightUserId?: string
  compact?: boolean
}

function isACMEntry(entry: ScoreboardEntry): entry is ACMRankEntry {
  return 'solved' in entry
}

export function Scoreboard({
  ranks,
  rankingMode = 'oi',
  problems = [],
  highlightUserId,
  compact = false,
}: ScoreboardProps) {
  const [search, setSearch] = useState('')

  const rankedRows = useMemo(
    () => ranks.map((entry, index) => ({ entry, rank: index + 1 })),
    [ranks]
  )

  const rows = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    return keyword
      ? rankedRows.filter(({ entry }) => entry.userName.toLowerCase().includes(keyword))
      : rankedRows
  }, [rankedRows, search])

  if (!ranks.length) {
    return (
      <EmptyState
        icon={Trophy}
        title="榜单还没有数据"
        description="选手提交并完成评测后，排名会在这里实时更新。"
      />
    )
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Badge variant="accent">{rankingMode === 'acm' ? 'ACM 赛制' : 'OI 赛制'}</Badge>
          <span className="text-xs font-medium text-slate-400">{ranks.length} 位选手</span>
        </div>
        {!compact && (
          <label className="relative block w-48">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="查找选手"
              className="h-8 w-full rounded-lg border border-[#e4e7ec] bg-white pl-8 pr-2 text-xs outline-none transition focus:border-forge-600"
            />
          </label>
        )}
      </div>

      <div className="overflow-hidden rounded-xl border border-[#e4e7ec] bg-white">
        <div tabIndex={0} className="overflow-x-auto">
          <table className="w-full min-w-[520px] text-left text-sm">
            <thead className="border-b border-[#eaecf0] bg-[#fbfaf6] text-[11px] uppercase tracking-wider text-slate-500">
              <tr>
                <th className="w-16 px-3 py-3 text-center">#</th>
                <th className="px-4 py-3">选手</th>
                {rankingMode === 'acm' ? (
                  <>
                    <th className="w-20 px-3 py-3 text-center">解题</th>
                    <th className="w-24 px-3 py-3 text-center">罚时</th>
                  </>
                ) : (
                  <>
                    <th className="w-20 px-3 py-3 text-center">总分</th>
                    <th className="w-20 px-3 py-3 text-center">AC</th>
                    {!compact &&
                      problems.map((problem) => (
                        <th
                          key={problem.id}
                          className="w-16 px-2 py-3 text-center"
                          title={problem.title}
                        >
                          {problem.code}
                        </th>
                      ))}
                  </>
                )}
              </tr>
            </thead>
            <tbody className="divide-y divide-[#f2f4f7]">
              {rows.map(({ entry, rank }) => {
                const acm = isACMEntry(entry)
                const highlighted = entry.userId === highlightUserId
                return (
                  <tr
                    key={entry.userId}
                    className={highlighted ? 'bg-forge-50/70' : 'transition-colors hover:bg-[#fbfaf6]'}
                  >
                    <td className="px-3 py-3 text-center">
                      <span
                        className={
                          rank === 1
                            ? 'inline-grid h-7 w-7 place-items-center rounded-full bg-amber-100 font-mono text-xs font-extrabold text-amber-800'
                            : rank === 2
                              ? 'inline-grid h-7 w-7 place-items-center rounded-full bg-slate-200 font-mono text-xs font-extrabold text-slate-700'
                              : rank === 3
                                ? 'inline-grid h-7 w-7 place-items-center rounded-full bg-orange-100 font-mono text-xs font-extrabold text-orange-800'
                                : 'font-mono text-xs text-slate-400'
                        }
                      >
                        {rank}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className="font-bold text-[#14283d]">{entry.userName}</span>
                        {highlighted && (
                          <span className="rounded bg-forge-100 px-1.5 py-0.5 text-[10px] font-bold text-forge-800">
                            我
                          </span>
                        )}
                      </div>
                    </td>
                    {acm ? (
                      <>
                        <td className="px-3 py-3 text-center font-mono font-extrabold text-emerald-700">
                          {entry.solved}
                        </td>
                        <td className="px-3 py-3 text-center font-mono text-xs text-slate-600">
                          {entry.penaltyMin}
                        </td>
                      </>
                    ) : (
                      <>
                        <td className="px-3 py-3 text-center font-mono font-extrabold text-forge-800">
                          {entry.score}
                        </td>
                        <td className="px-3 py-3 text-center font-mono font-bold text-emerald-700">
                          {entry.accepted}
                        </td>
                        {!compact &&
                          problems.map((problem) => {
                            const score = entry.problemScores?.[problem.id] || 0
                            return (
                              <td key={problem.id} className="px-2 py-3 text-center">
                                <span
                                  className={
                                    score > 0
                                      ? 'font-mono text-xs font-bold text-slate-800'
                                      : 'font-mono text-xs text-slate-300'
                                  }
                                >
                                  {score > 0 ? score : '-'}
                                </span>
                              </td>
                            )
                          })}
                      </>
                    )}
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
        {rows.length === 0 && (
          <p className="py-8 text-center text-xs text-slate-400">没有匹配的选手</p>
        )}
      </div>
    </div>
  )
}
