import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  AlertCircle,
  Check,
  CheckCircle2,
  ClipboardCheck,
  FileQuestion,
  RotateCcw,
  Send,
  Sparkles,
  X,
} from 'lucide-react'
import {
  ApiError,
  type CaseResult,
  type ChoiceQuestion,
  type Problem,
  type Submission,
  api,
  newIdempotencyKey,
} from '../api/client'
import { choiceDraftKey, useStore } from '../store/useStore'
import { MarkdownView } from './MarkdownView'
import { Button } from './ui/button'
import { Badge } from './ui/badge'
import { useToast } from './ui/toast'

interface GESPQuizViewProps {
  contestId: string
  problem: Problem
  canSubmit?: boolean
  phase?: 'draft' | 'upcoming' | 'running' | 'finished'
  onSubmitSuccess?: (submission: Submission) => void
}

type AnswerValue = string | string[]
type AnswerSheet = Record<string, AnswerValue>
type QuestionType = ChoiceQuestion['type']

interface DraftEnvelope {
  answers: AnswerSheet
  submission?: Submission
}

const OPTION_KEYS = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H']
const QUESTION_LABELS: Record<QuestionType, string> = {
  single: '单选题',
  multiple: '多选题',
  judge: '判断题',
}

function isAnswerValue(value: unknown): value is AnswerValue {
  return (
    typeof value === 'string' ||
    (Array.isArray(value) && value.every((item) => typeof item === 'string'))
  )
}

function normalizeDraft(raw: string | undefined): DraftEnvelope {
  if (!raw) return { answers: {} }

  try {
    const parsed: unknown = JSON.parse(raw)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      const candidate = parsed as Record<string, unknown>
      // Older drafts stored the answer map directly. Accept both shapes so a
      // browser upgrade never destroys a participant's in-progress work.
      const source =
        candidate.answers &&
        typeof candidate.answers === 'object' &&
        !Array.isArray(candidate.answers)
          ? candidate.answers
          : candidate
      const answers: AnswerSheet = {}
      Object.entries(source).forEach(([key, value]) => {
        if (/^\d+$/.test(key) && isAnswerValue(value)) {
          answers[key] = value
        }
      })
      const submission =
        candidate.submission && typeof candidate.submission === 'object'
          ? (candidate.submission as Submission)
          : undefined
      return { answers, submission }
    }
  } catch {
    // Ignore malformed local data; the server remains the source of truth.
  }
  return { answers: {} }
}

function isAnswered(value: AnswerValue | undefined): boolean {
  if (value == null) return false
  return Array.isArray(value) ? value.length > 0 : value.trim() !== ''
}

function optionText(option: string, index: number): string {
  const key = OPTION_KEYS[index] ?? String(index + 1)
  const stripped = option.replace(/^\s*[A-Ha-h][.、:：)]\s*/, '').trim()
  return stripped || option.trim() || `选项 ${key}`
}

function optionKey(option: string, index: number): string {
  const match = option.match(/^\s*([A-Ha-h])[.、:：)]/)
  return (match?.[1] || OPTION_KEYS[index] || String(index + 1)).toUpperCase()
}

function formatSubmissionTime(value?: string): string {
  if (!value) return '刚刚'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '刚刚'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(date)
}

function SubmissionResultBadge({ submission }: { submission: Submission }) {
  const accepted = submission.verdict === 'accepted'
  return (
    <Badge variant={accepted ? 'success' : 'warning'} className="px-3 py-1 text-xs">
      {accepted ? '全卷通过' : '存在错题'} · {submission.score} 分
    </Badge>
  )
}

function CaseVerdictBadge({ result }: { result?: CaseResult }) {
  if (!result) {
    return <Badge variant="outline">未作答</Badge>
  }
  if (result.verdict === 'AC') {
    return (
      <Badge variant="success" className="gap-1">
        <Check className="h-3 w-3" />
        正确 +{result.score}
      </Badge>
    )
  }
  return (
    <Badge variant="danger" className="gap-1">
      <X className="h-3 w-3" />
      错误 {result.verdict}
    </Badge>
  )
}

export const GESPQuizView: React.FC<GESPQuizViewProps> = ({
  contestId,
  problem,
  canSubmit = true,
  phase = 'running',
  onSubmitSuccess,
}) => {
  const { drafts, setDraft, session } = useStore()
  const toast = useToast()
  const [answers, setAnswers] = useState<AnswerSheet>({})
  const [lastSubmission, setLastSubmission] = useState<Submission | null>(null)
  const [restored, setRestored] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [activeQuestion, setActiveQuestion] = useState<number | null>(null)
  const questionRefs = useRef(new Map<number, HTMLElement>())
  const pendingSubmissionRef = useRef<{ fingerprint: string; key: string } | null>(null)

  const questions = useMemo(() => problem.choiceQuestions ?? [], [problem.choiceQuestions])
  const draftKey = choiceDraftKey(session?.userId, contestId, problem.id)
  const storedDraft = drafts[draftKey]
  const draftRef = useRef(storedDraft)

  useEffect(() => {
    draftRef.current = storedDraft
  }, [storedDraft])

  const totalScore = useMemo(
    () => questions.reduce((sum, question) => sum + (question.score > 0 ? question.score : 2), 0),
    [questions]
  )

  const answeredCount = useMemo(
    () =>
      questions.reduce((count, question) => {
        return count + (isAnswered(answers[String(question.id)]) ? 1 : 0)
      }, 0),
    [answers, questions]
  )

  const progress =
    questions.length === 0 ? 0 : Math.round((answeredCount / questions.length) * 100)
  const unansweredQuestions = useMemo(
    () => questions.filter((question) => !isAnswered(answers[String(question.id)])),
    [answers, questions]
  )

  const resultByQuestion = useMemo(() => {
    const map = new Map<number, CaseResult>()
    lastSubmission?.cases?.forEach((result) => map.set(result.caseIndex, result))
    return map
  }, [lastSubmission])

  const correctCount =
    lastSubmission?.cases?.filter((result) => result.verdict === 'AC').length ?? 0

  useEffect(() => {
    const restore = normalizeDraft(draftRef.current)
    setAnswers(restore.answers)
    setLastSubmission(restore.submission ?? null)
    setError('')
    setActiveQuestion(questions[0]?.id ?? null)
    setRestored(Object.keys(restore.answers).length > 0 || Boolean(restore.submission))
    // Only restore when the active paper changes. The local draft must not
    // overwrite live edits on every store update.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [problem.id])

  const persist = useCallback(
    (nextAnswers: AnswerSheet, submission: Submission | null = lastSubmission) => {
      const envelope: DraftEnvelope = { answers: nextAnswers }
      if (submission) envelope.submission = submission
      setDraft(draftKey, JSON.stringify(envelope))
    },
    [draftKey, lastSubmission, setDraft]
  )

  const selectSingle = useCallback(
    (question: ChoiceQuestion, value: string) => {
      if (!canSubmit) return
      const next = { ...answers, [String(question.id)]: value }
      setAnswers(next)
      persist(next)
      setError('')
      setRestored(false)
    },
    [answers, canSubmit, persist]
  )

  const toggleMultiple = useCallback(
    (question: ChoiceQuestion, value: string) => {
      if (!canSubmit) return
      const key = String(question.id)
      const current = Array.isArray(answers[key]) ? answers[key] : []
      const nextSelection = current.includes(value)
        ? current.filter((item) => item !== value)
        : [...current, value].sort()
      const next = { ...answers, [key]: [...nextSelection].sort() }
      setAnswers(next)
      persist(next)
      setError('')
      setRestored(false)
    },
    [answers, canSubmit, persist]
  )

  const jumpToQuestion = useCallback((questionId: number) => {
    setActiveQuestion(questionId)
    const node = questionRefs.current.get(questionId)
    node?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    node?.focus({ preventScroll: true })
  }, [])

  const handleSubmit = async () => {
    if (submitting || questions.length === 0) return
    if (!canSubmit) {
      setError('当前不在可提交时段，答题卡已锁定。')
      return
    }

    if (unansweredQuestions.length > 0) {
      const first = unansweredQuestions[0]
      setError(
        `还有 ${unansweredQuestions.length} 题未作答，请先完成第 ${
          questions.indexOf(first) + 1
        } 题。`
      )
      jumpToQuestion(first.id)
      return
    }

    setSubmitting(true)
    setError('')
    const fingerprint = JSON.stringify({ problemID: problem.id, answers })
    const pending =
      pendingSubmissionRef.current?.fingerprint === fingerprint
        ? pendingSubmissionRef.current
        : { fingerprint, key: newIdempotencyKey() }
    pendingSubmissionRef.current = pending
    try {
      const submission = await api.submit(contestId, {
        problemID: problem.id,
        language: 'choice_json',
        code: JSON.stringify(answers),
      }, pending.key)
      pendingSubmissionRef.current = null
      setLastSubmission(submission)
      persist(answers, submission)
      onSubmitSuccess?.(submission)
      if (submission.verdict === 'accepted') {
        toast.success('全卷通过', `本次答题获得 ${submission.score} / ${totalScore} 分。`)
      } else {
        toast.notify(
          'warning',
          '答题卡已提交',
          `本次获得 ${submission.score} / ${totalScore} 分，可查看逐题结果后再次提交。`
        )
      }
    } catch (cause) {
      const message =
        cause instanceof ApiError
          ? cause.message
          : cause instanceof Error
            ? cause.message
            : '提交失败，请检查网络后重试。'
      setError(message)
      toast.error('提交失败', message)
    } finally {
      setSubmitting(false)
    }
  }

  const resetAnswers = () => {
    if (!canSubmit) return
    setAnswers({})
    setLastSubmission(null)
    setError('')
    setRestored(false)
    setDraft(draftKey, JSON.stringify({ answers: {} }))
    toast.notify('info', '已清空答题卡', '本题的本地草稿已同步清除。')
  }

  if (questions.length === 0) {
    return (
      <div className="flex min-h-[28rem] flex-col items-center justify-center gap-4 p-8 text-center">
        <div className="grid h-14 w-14 place-items-center rounded-2xl bg-forge-50 text-forge-700">
          <FileQuestion className="h-6 w-6" />
        </div>
        <div>
          <h2 className="text-lg font-extrabold text-[#14283d]">答题卡尚未配置</h2>
          <p className="mt-2 max-w-md text-sm leading-6 text-slate-500">
            监考教师还没有为这道题录入客观题。请刷新题目，或举手联系监考教师。
          </p>
        </div>
      </div>
    )
  }

  const locked = !canSubmit
  const lockMessage =
    phase === 'upcoming'
      ? '考试尚未开始，当前仅可阅读题面，答题卡将在开考后解锁。'
      : phase === 'finished'
        ? '考试已经结束，答题卡与提交入口均已锁定。'
        : '比赛尚未启动，答题卡暂不可用。'

  return (
    <div className="grid gap-6 p-5 sm:p-6 xl:grid-cols-[minmax(0,1fr)_21rem] xl:p-8">
      <main className="min-w-0 space-y-5">
        {locked && (
          <div
            role="status"
            className="flex items-start gap-3 rounded-2xl border border-amber-300 bg-amber-50 px-4 py-3 text-xs font-semibold leading-5 text-amber-900"
          >
            <span className="mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-full bg-amber-200 font-mono text-[10px]">
              !
            </span>
            <div>
              <p>答题卡已锁定</p>
              <p className="mt-0.5 font-medium text-amber-800">{lockMessage}</p>
            </div>
          </div>
        )}
        <header className="relative overflow-hidden rounded-2xl border border-[#e4e7ec] bg-[#fffefa] p-5 shadow-[0_1px_2px_rgba(16,24,40,0.04)] sm:p-6">
          <div
            className="pointer-events-none absolute -right-14 -top-20 h-52 w-52 rounded-full bg-forge-100/70 blur-3xl"
            aria-hidden="true"
          />
          <div className="relative flex flex-wrap items-start justify-between gap-4">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="accent">CCF GESP 客观题</Badge>
                <span className="font-mono text-xs font-bold text-slate-400">{problem.code}</span>
                {restored && (
                  <Badge variant="info" className="gap-1">
                    <RotateCcw className="h-3 w-3" />
                    已恢复本地草稿
                  </Badge>
                )}
              </div>
              <h1 className="mt-3 text-2xl font-black tracking-[-0.03em] text-[#14283d] sm:text-3xl">
                {problem.title}
              </h1>
              <p className="mt-2 text-xs leading-6 text-slate-500">
                共 {questions.length} 题，满分 {totalScore} 分。选择会在本机自动保存，交卷后立即判分。
              </p>
            </div>
            <div className="rounded-2xl border border-[#eaecf0] bg-[#fbfaf6] px-4 py-3 text-right">
              <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-slate-400">
                作答进度
              </p>
              <p className="mt-1 font-mono text-xl font-black text-[#14283d]">
                {answeredCount}
                <span className="text-sm font-bold text-slate-400"> / {questions.length}</span>
              </p>
            </div>
          </div>
          {problem.statement && (
            <div className="relative mt-5 border-t border-[#eaecf0] pt-4">
              <MarkdownView source={problem.statement} className="text-[13px] leading-6" />
            </div>
          )}
        </header>

        <section className="space-y-4" aria-label="客观题答题卡">
          {questions.map((question, index) => {
            const answer = answers[String(question.id)]
            const result = resultByQuestion.get(question.id)
            const correct = result?.verdict === 'AC'
            const wrong = Boolean(result) && !correct
            const currentType: QuestionType = (
              ['single', 'multiple', 'judge'] as const
            ).includes(question.type)
              ? question.type
              : 'single'
            const options =
              currentType === 'judge'
                ? [
                    { key: 'T', text: '正确' },
                    { key: 'F', text: '错误' },
                  ]
                : (question.options?.length ? question.options : OPTION_KEYS.slice(0, 4)).map(
                    (option, optionIndex) => ({
                      key: optionKey(option, optionIndex),
                      text: optionText(option, optionIndex),
                    })
                  )

            return (
              <article
                key={question.id}
                ref={(node) => {
                  if (node) questionRefs.current.set(question.id, node)
                  else questionRefs.current.delete(question.id)
                }}
                id={`choice-question-${question.id}`}
                tabIndex={-1}
                aria-labelledby={`choice-question-title-${question.id}`}
                className={`scroll-mt-28 rounded-2xl border bg-[#fffefa] p-5 shadow-[0_1px_2px_rgba(16,24,40,0.035)] transition sm:p-6 ${
                  correct
                    ? 'border-emerald-300 ring-1 ring-emerald-100'
                    : wrong
                      ? 'border-rose-300 ring-1 ring-rose-100'
                      : activeQuestion === question.id
                        ? 'border-forge-300 ring-4 ring-forge-700/5'
                        : 'border-[#e4e7ec] hover:border-[#d0d5dd]'
                }`}
                onFocus={() => setActiveQuestion(question.id)}
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="flex min-w-0 items-start gap-3">
                    <span
                      className={`grid h-8 w-8 shrink-0 place-items-center rounded-xl font-mono text-xs font-black ${
                        correct
                          ? 'bg-emerald-100 text-emerald-800'
                          : wrong
                            ? 'bg-rose-100 text-rose-800'
                            : 'bg-[#14283d] text-white'
                      }`}
                    >
                      {String(index + 1).padStart(2, '0')}
                    </span>
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <h2
                          id={`choice-question-title-${question.id}`}
                          className="text-sm font-extrabold text-[#14283d]"
                        >
                          第 {index + 1} 题
                        </h2>
                        <Badge variant="default">{QUESTION_LABELS[currentType]}</Badge>
                        <span className="font-mono text-[11px] font-bold text-slate-400">
                          {question.score > 0 ? question.score : 2} 分
                        </span>
                      </div>
                      <div className="mt-3 text-[13px] leading-6">
                        <MarkdownView source={question.stem} />
                      </div>
                    </div>
                  </div>
                  {result ? <CaseVerdictBadge result={result} /> : null}
                </div>

                <div
                  className={`mt-4 grid gap-2.5 ${
                    currentType === 'judge'
                      ? 'sm:grid-cols-2'
                      : options.length > 2
                        ? 'sm:grid-cols-2'
                        : ''
                  }`}
                  role={currentType === 'multiple' ? 'group' : 'radiogroup'}
                  aria-label={`第 ${index + 1} 题选项`}
                >
                  {options.map((option) => {
                    const multiple = currentType === 'multiple'
                    const checked = multiple
                      ? Array.isArray(answer) && answer.includes(option.key)
                      : String(answer ?? '').toUpperCase() === option.key

                    return (
                      <button
                        key={`${question.id}-${option.key}`}
                        type="button"
                        role={multiple ? 'checkbox' : 'radio'}
                        aria-checked={checked}
                        disabled={locked}
                        onClick={() =>
                          multiple
                            ? toggleMultiple(question, option.key)
                            : selectSingle(question, option.key)
                        }
                        className={`group flex min-h-12 w-full items-center gap-3 rounded-xl border px-3.5 py-3 text-left text-[13px] font-semibold leading-6 transition focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-forge-700/15 disabled:cursor-not-allowed disabled:opacity-75 ${
                          checked
                            ? 'border-forge-600 bg-forge-50 text-forge-950 shadow-[inset_0_0_0_1px_rgba(194,65,12,0.12)]'
                            : 'border-[#e4e7ec] bg-white text-slate-700 hover:border-forge-300 hover:bg-[#fffaf5]'
                        }`}
                      >
                        <span
                          className={`grid h-6 w-6 shrink-0 place-items-center border text-[10px] font-black transition ${
                            multiple ? 'rounded-md' : 'rounded-full'
                          } ${
                            checked
                              ? 'border-forge-600 bg-forge-600 text-white'
                              : 'border-[#d0d5dd] bg-[#fbfaf6] text-slate-500 group-hover:border-forge-300'
                          }`}
                        >
                          {checked ? <Check className="h-3.5 w-3.5" /> : option.key}
                        </span>
                        <span className="min-w-0 flex-1">{option.text}</span>
                      </button>
                    )
                  })}
                </div>

                {currentType === 'multiple' && (
                  <p className="mt-2.5 text-[11px] font-medium text-slate-400">
                    多选题可以勾选多个选项；全对才得分。
                  </p>
                )}

                {correct && question.explanation && (
                  <div className="mt-4 flex gap-2.5 rounded-xl border border-emerald-200 bg-emerald-50/70 px-3.5 py-3 text-xs leading-5 text-emerald-900">
                    <Sparkles className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />
                    <div>
                      <p className="font-bold">解析</p>
                      <MarkdownView
                        source={question.explanation}
                        className="mt-1 text-xs text-emerald-900"
                      />
                    </div>
                  </div>
                )}
              </article>
            )
          })}
        </section>
      </main>

      <aside className="min-w-0 xl:sticky xl:top-24 xl:self-start">
        <div className="overflow-hidden rounded-2xl border border-[#e4e7ec] bg-[#fffefa] shadow-[0_20px_50px_-36px_rgba(16,24,40,0.4)]">
          <div className="border-b border-[#eaecf0] bg-[#14283d] p-5 text-white">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-[10px] font-bold uppercase tracking-[0.2em] text-white/45">
                  Answer sheet
                </p>
                <h2 className="mt-1 text-base font-extrabold">答题卡</h2>
              </div>
              <ClipboardCheck className="h-5 w-5 text-forge-200" />
            </div>
            <div className="mt-4 flex items-baseline justify-between gap-3">
              <span className="text-xs text-white/55">已作答</span>
              <span className="font-mono text-sm font-black">
                {answeredCount}
                <span className="text-white/40"> / {questions.length}</span>
              </span>
            </div>
            <div
              className="mt-2 h-1.5 overflow-hidden rounded-full bg-white/10"
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={progress}
              aria-label="答题进度"
            >
              <div
                className="h-full rounded-full bg-gradient-to-r from-forge-400 to-amber-300 transition-[width] duration-300"
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>

          <div className="space-y-5 p-5">
            <div>
              <div className="mb-2.5 flex items-center justify-between">
                <p className="text-xs font-bold text-slate-600">题目导航</p>
                <p className="text-[10px] text-slate-400">点击跳转</p>
              </div>
              <div className="grid grid-cols-6 gap-2">
                {questions.map((question, index) => {
                  const answered = isAnswered(answers[String(question.id)])
                  const result = resultByQuestion.get(question.id)
                  const correct = result?.verdict === 'AC'
                  const wrong = Boolean(result) && !correct
                  return (
                    <button
                      key={question.id}
                      type="button"
                      onClick={() => jumpToQuestion(question.id)}
                      aria-label={`跳转到第 ${index + 1} 题${
                        correct ? '，正确' : wrong ? '，错误' : answered ? '，已作答' : '，未作答'
                      }`}
                      aria-current={activeQuestion === question.id ? 'step' : undefined}
                      className={`grid h-9 place-items-center rounded-lg border font-mono text-[11px] font-bold transition focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-forge-700/15 ${
                        correct
                          ? 'border-emerald-500 bg-emerald-500 text-white'
                          : wrong
                            ? 'border-rose-500 bg-rose-500 text-white'
                            : answered
                              ? 'border-forge-600 bg-forge-600 text-white'
                              : 'border-[#e4e7ec] bg-white text-slate-400 hover:border-forge-300 hover:text-forge-700'
                      } ${
                        activeQuestion === question.id
                          ? 'ring-2 ring-forge-700/25 ring-offset-1'
                          : ''
                      }`}
                    >
                      {index + 1}
                    </button>
                  )
                })}
              </div>
              <div className="mt-3 flex flex-wrap gap-x-3 gap-y-1.5 text-[10px] text-slate-400">
                <span className="flex items-center gap-1">
                  <i className="h-2 w-2 rounded-sm bg-forge-600" />
                  已答
                </span>
                <span className="flex items-center gap-1">
                  <i className="h-2 w-2 rounded-sm border border-[#d0d5dd] bg-white" />
                  未答
                </span>
                {lastSubmission && (
                  <>
                    <span className="flex items-center gap-1">
                      <i className="h-2 w-2 rounded-sm bg-emerald-500" />
                      正确
                    </span>
                    <span className="flex items-center gap-1">
                      <i className="h-2 w-2 rounded-sm bg-rose-500" />
                      错误
                    </span>
                  </>
                )}
              </div>
            </div>

            {lastSubmission && (
              <div className="rounded-xl border border-[#e4e7ec] bg-[#fbfaf6] p-3.5">
                <div className="flex items-center justify-between gap-2">
                  <p className="text-xs font-bold text-slate-700">最近一次判分</p>
                  <SubmissionResultBadge submission={lastSubmission} />
                </div>
                <dl className="mt-3 grid grid-cols-2 gap-2 text-center">
                  <div className="rounded-lg border border-white bg-white px-2 py-2">
                    <dt className="text-[10px] text-slate-400">正确</dt>
                    <dd className="mt-0.5 font-mono text-base font-black text-emerald-700">
                      {correctCount}
                    </dd>
                  </div>
                  <div className="rounded-lg border border-white bg-white px-2 py-2">
                    <dt className="text-[10px] text-slate-400">总分</dt>
                    <dd className="mt-0.5 font-mono text-base font-black text-[#14283d]">
                      {lastSubmission.score}
                      <span className="text-[10px] text-slate-400"> / {totalScore}</span>
                    </dd>
                  </div>
                </dl>
                <p className="mt-2.5 text-[10px] text-slate-400">
                  提交于 {formatSubmissionTime(lastSubmission.submittedAt)}
                </p>
              </div>
            )}

            {error && (
              <div
                role="alert"
                className="flex gap-2.5 rounded-xl border border-rose-200 bg-rose-50 px-3.5 py-3 text-xs font-semibold leading-5 text-rose-700"
              >
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>{error}</span>
              </div>
            )}

            <div className="space-y-2.5">
              <Button
                data-testid="submit-choice-sheet"
                type="button"
                size="lg"
                className="w-full"
                loading={submitting}
                onClick={handleSubmit}
                disabled={submitting || locked}
              >
                {locked ? (
                  '答题卡已锁定'
                ) : submitting ? (
                  '正在判分...'
                ) : (
                  <>
                    <Send className="h-4 w-4" />
                    提交答题卡
                  </>
                )}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="w-full text-slate-500"
                onClick={resetAnswers}
                disabled={submitting || locked || (answeredCount === 0 && !lastSubmission)}
              >
                <RotateCcw className="h-3.5 w-3.5" />
                清空本题草稿
              </Button>
            </div>

            {lastSubmission ? (
              <div className="flex gap-2.5 rounded-xl border border-emerald-200 bg-emerald-50/60 px-3.5 py-3 text-[11px] leading-5 text-emerald-800">
                <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" />
                <p>
                  {locked
                    ? '逐题结果已保留。当前时段不可再修改答题卡。'
                    : '逐题结果已更新。答错题目可重新选择并再次提交，以最后一次成绩为准。'}
                </p>
              </div>
            ) : (
              <p className="text-center text-[10px] leading-5 text-slate-400">
                作答内容自动保存在本机浏览器，刷新或短暂断网不会丢失。
              </p>
            )}
          </div>
        </div>
      </aside>
    </div>
  )
}
