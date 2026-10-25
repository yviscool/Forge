// 文件提交模式（Lemon）——选择题作答面板。
//
// 复刻 LemonNext ChoiceView：paper.md 按 {{ select(N) }} 占位符切题，
// 题干走 Markdown/KaTeX 渲染，四种题型（单选/多选/判断/填空），
// 草稿实时落 localStorage、无草稿时回填最后一次递交，序列化为
// 「题号: 答案」纯文本提交（与 LemonNext choicejudge 的解析格式一致）。

import { useEffect, useMemo, useRef, useState } from 'react'
import { ArrowLeft, ClipboardCheck, Send } from 'lucide-react'
import { MarkdownView } from '../../components/MarkdownView'
import { Badge } from '../../components/ui/badge'
import { Button } from '../../components/ui/button'
import { Dialog } from '../../components/ui/dialog'
import { InlineError, LoadingBlock } from '../../components/ui/empty-state'
import { Input } from '../../components/ui/input'
import { useToast } from '../../components/ui/toast'
import { choiceDraftKey } from './keys'
import { mockLemonApi as api } from './mockApi'
import type { LemonChoiceMeta, LemonChoiceQuestionType, LemonTask, LemonTasksResponse } from './types'
import type { LemonCountdown } from './time'
import { fmtClock } from './time'

const TYPE_LABEL: Record<LemonChoiceQuestionType, string> = {
  single: '单选',
  multiple: '多选',
  truefalse: '判断',
  fill: '填空',
}

const OPTION_LETTERS = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L']

interface PaperBlock {
  id: number
  stemMd: string
  options: string[]
}

/** 与 LemonNext 相同的切题规则：占位符前的正文归入本题题干，其后的 bullet 列表是选项。 */
function parsePaperIntoBlocks(markdown: string): PaperBlock[] {
  const slotRe = /\{\{\s*select\s*\(\s*(\d+)\s*\)\s*\}\}/
  const bulletRe = /^\s*[-*+]\s+(.+?)\s*$/
  const lines = String(markdown || '').split(/\r?\n/)
  const blocks: PaperBlock[] = []
  let stem: string[] = []
  let index = 0
  while (index < lines.length) {
    const match = lines[index].match(slotRe)
    if (!match) {
      stem.push(lines[index])
      index += 1
      continue
    }
    const id = parseInt(match[1], 10)
    const before = lines[index].slice(0, match.index)
    if (before.trim()) stem.push(before)
    let cursor = index + 1
    while (cursor < lines.length && lines[cursor].trim() === '') cursor += 1
    const options: string[] = []
    while (cursor < lines.length) {
      const line = lines[cursor]
      if (line.trim() === '') {
        if (options.length > 0) break
        cursor += 1
        continue
      }
      const bullet = line.match(bulletRe)
      if (!bullet) break
      options.push(bullet[1].trim())
      cursor += 1
    }
    while (stem.length && stem[stem.length - 1].trim() === '') stem.pop()
    blocks.push({ id, stemMd: stem.join('\n'), options })
    stem = []
    index = cursor
  }
  return blocks
}

type AnswerSheet = Record<number, string[]>

function serializeAnswers(answers: AnswerSheet): string {
  const ids = Object.keys(answers)
    .map(Number)
    .filter(Number.isFinite)
    .sort((a, b) => a - b)
  const body = ids
    .map((id) => {
      // 与 LemonNext 一致：多选按选项字母顺序序列化（与点击顺序无关）。
      const values = (answers[id] || []).slice().sort()
      return `${id}: ${values.join(',')}`
    })
    .join('\n')
  return body ? `${body}\n` : ''
}

function parseAnswerText(text: string): AnswerSheet {
  const out: AnswerSheet = {}
  if (!text) return out
  const numbered = /^\s*(\d+)\s*[:.\-、)\s]\s*(.+?)\s*$/
  String(text).split(/\r?\n/).forEach((line) => {
    const match = line.match(numbered)
    if (!match) return
    const id = parseInt(match[1], 10)
    const parts = match[2].trim().split(/[\s,;、，;|/]+/).filter(Boolean)
    out[id] = parts.length ? parts : [match[2].trim()]
  })
  return out
}

interface ChoiceAnswerPanelProps {
  data: LemonTasksResponse
  task: LemonTask
  countdown: LemonCountdown
  lastSubmittedAt?: string
  onBack: () => void
  onSubmitted: (submittedAt: string) => void
}

export function ChoiceAnswerPanel({
  data,
  task,
  countdown,
  lastSubmittedAt,
  onBack,
  onSubmitted,
}: ChoiceAnswerPanelProps) {
  const toast = useToast()
  const [meta, setMeta] = useState<LemonChoiceMeta | null>(null)
  const [loadError, setLoadError] = useState('')
  const [answers, setAnswers] = useState<AnswerSheet>({})
  const [submitting, setSubmitting] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [lastAt, setLastAt] = useState(lastSubmittedAt || '')
  const paperRef = useRef<HTMLDivElement | null>(null)

  const outsideWindow = countdown.state === 'pre' || countdown.state === 'ended'

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const loaded = await api.choice(task.id)
        if (cancelled) return
        setMeta(loaded)
      } catch (cause) {
        if (!cancelled) setLoadError(cause instanceof Error ? cause.message : '题目加载失败')
      }
    })()
    return () => {
      cancelled = true
    }
  }, [task.id])

  // 草稿恢复：本地草稿优先，否则回填最后一次递交的答案文本。
  useEffect(() => {
    if (!meta) return
    let cancelled = false
    const draftKey = choiceDraftKey(data.user, task.id)
    try {
      const raw = localStorage.getItem(draftKey)
      if (raw) {
        const parsed = JSON.parse(raw) as AnswerSheet
        if (parsed && typeof parsed === 'object') {
          setAnswers(parsed)
          return
        }
      }
    } catch {
      // 损坏的草稿当不存在
    }
    ;(async () => {
      try {
        const source = await api.source(task.id)
        if (cancelled) return
        const parsed = parseAnswerText(source.content || '')
        if (Object.keys(parsed).length) {
          setAnswers(parsed)
          setLastAt(source.submittedAt)
        }
      } catch {
        // 未提交过
      }
    })()
    return () => {
      cancelled = true
    }
  }, [meta, data.user, task.id])

  const blocks = useMemo(() => parsePaperIntoBlocks(meta?.paperMarkdown || ''), [meta])
  const questionById = useMemo(() => {
    const map = new Map<number, LemonChoiceMeta['questions'][number]>()
    meta?.questions.forEach((question) => map.set(question.id, question))
    return map
  }, [meta])

  const answeredCount = Object.keys(answers).length
  const totalCount = meta?.questions.length || 0
  const progressPercent = totalCount === 0 ? 0 : Math.round((answeredCount * 100) / totalCount)

  const persist = (next: AnswerSheet) => {
    setAnswers(next)
    try {
      localStorage.setItem(choiceDraftKey(data.user, task.id), JSON.stringify(next))
    } catch {
      // 存储满时静默放弃草稿
    }
  }

  const setQuestionAnswer = (questionId: number, values: string[]) => {
    const next = { ...answers }
    if (values.length) next[questionId] = values
    else delete next[questionId]
    persist(next)
  }

  const scrollToQuestion = (questionId: number) => {
    paperRef.current
      ?.querySelector(`[data-question-id="${questionId}"]`)
      ?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  const doSubmit = async () => {
    if (submitting) return
    setSubmitting(true)
    try {
      const result = await api.submit(task.id, {
        source: serializeAnswers(answers),
        language: meta?.answerFileExt || 'txt',
      })
      setLastAt(result.submittedAt)
      onSubmitted(result.submittedAt)
      toast.success(`已递交 · ${fmtClock(result.submittedAt)}`)
    } catch (cause) {
      toast.error('递交失败', cause instanceof Error ? cause.message : '请稍后重试')
    } finally {
      setSubmitting(false)
      setConfirmOpen(false)
    }
  }

  const handleSubmitClick = () => {
    if (outsideWindow || submitting) return
    if (answeredCount === 0) {
      toast.warning('请先作答任意一题再递交')
      return
    }
    if (answeredCount < totalCount) setConfirmOpen(true)
    else void doSubmit()
  }

  if (loadError) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <Button variant="ghost" onClick={onBack} aria-label="返回题目列表">
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h2 className="font-display text-xl font-semibold text-[#14283d]">{task.title}</h2>
        </div>
        <InlineError message={loadError} />
      </div>
    )
  }

  if (!meta) return <LoadingBlock label="正在加载答题卡..." />

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="ghost" onClick={onBack} aria-label="返回题目列表">
          <ArrowLeft className="h-4 w-4" />
          返回
        </Button>
        <h2 className="font-display text-xl font-semibold tracking-[-0.02em] text-[#14283d]">
          {task.title}
        </h2>
        <Badge variant="accent">选择题</Badge>
        <span className="text-xs text-slate-400">{task.totalScore} 分</span>
        {lastAt && (
          <span className="ml-auto text-xs font-medium text-slate-400">
            上次递交于 {fmtClock(lastAt)}
          </span>
        )}
      </div>

      <div ref={paperRef} className="space-y-4">
        {blocks.map((block) => {
          const question = questionById.get(block.id)
          if (!question) {
            return (
              <div
                key={`missing-${block.id}`}
                className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs font-semibold text-amber-800"
              >
                第 {block.id} 题缺少答案配置，请联系监考老师
              </div>
            )
          }
          return (
            <section
              key={block.id}
              data-question-id={block.id}
              className="app-surface scroll-mt-24 rounded-2xl p-5"
            >
              <header className="mb-3 flex items-center gap-2">
                <span className="font-mono text-xs font-bold text-slate-400">#{block.id}</span>
                <Badge variant={question.type === 'multiple' ? 'info' : 'default'}>
                  {TYPE_LABEL[question.type]}
                </Badge>
                <span className="text-[11px] font-bold text-slate-400">{question.score} 分</span>
              </header>
              <MarkdownView source={block.stemMd} className="prose-forge" />
              {question.type === 'fill' ? (
                <Input
                  maxLength={256}
                  autoComplete="off"
                  disabled={outsideWindow}
                  value={(answers[block.id] || [])[0] || ''}
                  onChange={(event) =>
                    setQuestionAnswer(block.id, event.target.value ? [event.target.value] : [])
                  }
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') event.preventDefault()
                  }}
                  placeholder="在此输入答案"
                  className="mt-3 max-w-md"
                  aria-label={`第 ${block.id} 题答案`}
                />
              ) : (
                <div className="mt-3 grid gap-2">
                  {(question.type === 'truefalse'
                    ? [['A', block.options[0] || '正确'], ['B', block.options[1] || '错误']]
                    : Array.from(
                        { length: block.options.length || question.optionCount || 4 },
                        (_, index) => [
                          OPTION_LETTERS[index] || String(index + 1),
                          block.options[index] || '',
                        ] as [string, string]
                      )
                  ).map(([value, text]) => {
                    const checked = (answers[block.id] || []).includes(value)
                    return (
                      <label
                        key={value}
                        className={
                          checked
                            ? 'flex cursor-pointer items-start gap-3 rounded-xl border border-forge-300 bg-forge-50 px-3.5 py-2.5 transition'
                            : 'flex cursor-pointer items-start gap-3 rounded-xl border border-[#e4e7ec] bg-white px-3.5 py-2.5 transition hover:border-forge-200 hover:bg-[#fbfaf6]'
                        }
                      >
                        <input
                          type={question.type === 'multiple' ? 'checkbox' : 'radio'}
                          name={`q-${block.id}`}
                          value={value}
                          checked={checked}
                          disabled={outsideWindow}
                          onChange={() => {
                            if (question.type === 'multiple') {
                              const current = answers[block.id] || []
                              setQuestionAnswer(
                                block.id,
                                current.includes(value)
                                  ? current.filter((item) => item !== value)
                                  : [...current, value]
                              )
                            } else {
                              setQuestionAnswer(block.id, [value])
                            }
                          }}
                          className="mt-0.5 h-4 w-4 accent-[#c2410c]"
                        />
                        <span className="grid h-5 w-5 shrink-0 place-items-center rounded-md bg-[#14283d] font-mono text-[10px] font-bold text-white">
                          {value}
                        </span>
                        {text && <MarkdownView source={text} className="min-w-0 text-sm leading-6 text-slate-700" />}
                      </label>
                    )
                  })}
                </div>
              )}
            </section>
          )
        })}
      </div>

      {/* 底部递交栏：进度 + 题号网格 + 递交按钮（sticky 跟随滚动，不遮页脚） */}
      <div className="sticky bottom-4 z-30 rounded-2xl border border-[#e4e7ec] bg-[#fffefa]/95 shadow-lift backdrop-blur-xl">
        <div className="flex w-full flex-wrap items-center gap-4 px-4 py-3 sm:px-5">
          <div className="min-w-40 flex-1">
            <div className="flex items-center justify-between text-[11px] font-semibold text-slate-500">
              <span>
                共有 {totalCount} 道题目，已完成 {answeredCount} 道
              </span>
              <span className="font-mono">{progressPercent}%</span>
            </div>
            <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-slate-200/70">
              <div
                className="h-full rounded-full bg-forge-600 transition-all duration-500"
                style={{ width: `${progressPercent}%` }}
              />
            </div>
          </div>
          <div className="flex max-w-full flex-wrap gap-1" aria-label="题号导航">
            {meta.questions.map((question) => (
              <button
                key={question.id}
                type="button"
                onClick={() => scrollToQuestion(question.id)}
                className={
                  answers[question.id]
                    ? 'h-8 w-8 rounded-lg bg-[#14283d] font-mono text-xs font-bold text-white transition'
                    : 'h-8 w-8 rounded-lg border border-[#e4e7ec] bg-white font-mono text-xs font-bold text-slate-400 transition hover:border-forge-300 hover:text-slate-700'
                }
                aria-label={`跳到第 ${question.id} 题`}
              >
                {question.id}
              </button>
            ))}
          </div>
          <Button
            onClick={handleSubmitClick}
            disabled={outsideWindow || submitting}
            data-testid="choice-submit"
          >
            {submitting ? (
              '递交中…'
            ) : countdown.state === 'pre' ? (
              '比赛尚未开始'
            ) : countdown.state === 'ended' ? (
              '比赛已结束'
            ) : (
              <>
                <Send className="h-4 w-4" />
                递交答案
              </>
            )}
          </Button>
        </div>
      </div>

      <Dialog
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title="仍有未作答的题目"
        eyebrow="递交确认"
        footer={
          <>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>
              再检查一下
            </Button>
            <Button onClick={() => void doSubmit()} loading={submitting}>
              <ClipboardCheck className="h-4 w-4" />
              仍然递交
            </Button>
          </>
        }
      >
        <p className="text-sm leading-6 text-slate-600">
          还有 {totalCount - answeredCount} 道题未作答，递交后将覆盖上一次答案（评测以最后一次为准）。确定继续吗？
        </p>
      </Dialog>
    </div>
  )
}
