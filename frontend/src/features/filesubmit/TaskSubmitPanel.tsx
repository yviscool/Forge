// 文件提交模式（Lemon）——单题作答面板。
//
// 复刻 LemonNext 提交页的全部交互：题面元信息、样例文件下载、
// 在线编辑（语言/字号/字节数/草稿/Ctrl+Enter/覆盖确认）与
// 上传文件（拖拽/点击、扩展名识别语言、64KB 上限）。
// 教师设定的 submitMode 决定界面：editor / file / both（学生自选并记忆）。

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ArrowLeft,
  CloudUpload,
  FileCode2,
  FileUp,
  Gauge,
  Paperclip,
  Send,
} from 'lucide-react'
import { Badge } from '../../components/ui/badge'
import { Button } from '../../components/ui/button'
import { Dialog } from '../../components/ui/dialog'
import { EditorLoading } from '../../components/ui/editor-loading'
import { Select } from '../../components/ui/input'
import { useToast } from '../../components/ui/toast'
import { byteLength, formatBytes, MAX_SOURCE_BYTES } from '../student/submissionFiles'
import { answerModeKey, codeDraftKey } from './keys'
import { mockLemonApi as api } from './mockApi'
import type { LemonTask, LemonTasksResponse } from './types'
import type { LemonCountdown } from './time'
import { fmtClock } from './time'
import { ChoiceAnswerPanel } from './ChoiceAnswerPanel'

const MonacoEditor = React.lazy(() =>
  import('../../components/MonacoEditor').then((module) => ({ default: module.MonacoEditor }))
)

const LANGUAGES: Array<{ value: string; label: string; monaco: string }> = [
  { value: 'cpp', label: 'C++', monaco: 'cpp' },
  { value: 'c', label: 'C', monaco: 'c' },
  { value: 'python', label: 'Python', monaco: 'python' },
  { value: 'pascal', label: 'Pascal', monaco: 'pascal' },
]

const FONT_SIZES = [12, 14, 16, 18]

/** 与 LemonNext 相同的扩展名 → 语言映射。 */
const EXT_TO_LANGUAGE: Record<string, string> = {
  cpp: 'cpp', cc: 'cpp', cxx: 'cpp', cp: 'cpp',
  c: 'c', h: 'cpp',
  py: 'python',
  pas: 'pascal', pp: 'pascal',
}

function languageFromFilename(filename: string): string | null {
  const match = filename.toLowerCase().match(/\.([^.]+)$/)
  return match ? EXT_TO_LANGUAGE[match[1]] || null : null
}

interface TaskSubmitPanelProps {
  data: LemonTasksResponse
  task: LemonTask
  taskIndex: number
  countdown: LemonCountdown
  onBack: () => void
  onSubmitted: (taskId: number, submittedAt: string) => void
}

export function TaskSubmitPanel({
  data,
  task,
  taskIndex,
  countdown,
  onBack,
  onSubmitted,
}: TaskSubmitPanelProps) {
  const toast = useToast()
  const outsideWindow = countdown.state === 'pre' || countdown.state === 'ended'

  // ---- 选择题走独立答题卡 ----
  if (task.taskType === 'choice') {
    return (
      <ChoiceAnswerPanel
        data={data}
        task={task}
        countdown={countdown}
        lastSubmittedAt={task.submittedAt || undefined}
        onBack={onBack}
        onSubmitted={(submittedAt) => onSubmitted(task.id, submittedAt)}
      />
    )
  }

  return (
    <TraditionalPanel
      data={data}
      task={task}
      taskIndex={taskIndex}
      countdown={countdown}
      outsideWindow={outsideWindow}
      onBack={onBack}
      onSubmitted={onSubmitted}
    />
  )
}

function TraditionalPanel({
  data,
  task,
  taskIndex,
  countdown,
  outsideWindow,
  onBack,
  onSubmitted,
}: Omit<TaskSubmitPanelProps, 'onSubmitted'> & {
  outsideWindow: boolean
  onSubmitted: (taskId: number, submittedAt: string) => void
}) {
  const toast = useToast()
  const [language, setLanguage] = useState('cpp')
  const [fontSize, setFontSize] = useState(14)
  const [code, setCode] = useState('')
  const [answerMode, setAnswerMode] = useState<'editor' | 'file'>('editor')
  const [uploaded, setUploaded] = useState<{ name: string; content: string; size: number } | null>(null)
  const [dragging, setDragging] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [lastAt, setLastAt] = useState(task.submittedAt || '')
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const submitCountRef = useRef(task.submittedAt ? 1 : 0)

  // 双模式下记忆学生上次的选择。
  useEffect(() => {
    if (data.submitMode === 'both') {
      const saved = localStorage.getItem(answerModeKey(data.user))
      setAnswerMode(saved === 'file' ? 'file' : 'editor')
    } else {
      setAnswerMode(data.submitMode === 'file' ? 'file' : 'editor')
    }
  }, [data.submitMode, data.user])

  // 草稿恢复（在线编辑）。
  useEffect(() => {
    const saved = localStorage.getItem(codeDraftKey(data.user, task.id))
    if (saved) setCode(saved)
  }, [data.user, task.id])

  const currentSource = answerMode === 'editor' ? code : uploaded?.content || ''
  const currentBytes = useMemo(() => byteLength(currentSource), [currentSource])

  const handleCodeChange = useCallback(
    (value: string) => {
      setCode(value)
      try {
        localStorage.setItem(codeDraftKey(data.user, task.id), value)
      } catch {
        // 存储满时静默放弃草稿
      }
    },
    [data.user, task.id]
  )

  const ingestFile = async (file: File) => {
    if (file.size === 0) {
      toast.warning('文件为空')
      return
    }
    if (file.size > MAX_SOURCE_BYTES) {
      toast.error('文件过大', `单个文件不超过 ${formatBytes(MAX_SOURCE_BYTES)}`)
      return
    }
    try {
      const content = await file.text()
      setUploaded({ name: file.name, content, size: file.size })
      const guess = languageFromFilename(file.name)
      if (guess) setLanguage(guess)
    } catch (cause) {
      toast.error('读取文件失败', cause instanceof Error ? cause.message : undefined)
    }
  }

  const submitBlocked =
    outsideWindow || submitting || currentBytes === 0 || currentBytes > MAX_SOURCE_BYTES

  const doSubmit = async () => {
    if (submitBlocked) return
    setSubmitting(true)
    try {
      const result = await api.submit(task.id, { source: currentSource, language })
      submitCountRef.current += 1
      setLastAt(result.submittedAt)
      onSubmitted(task.id, result.submittedAt)
      toast.success(`提交成功 · ${fmtClock(result.submittedAt)}`)
    } catch (cause) {
      toast.error('提交失败', cause instanceof Error ? cause.message : '请稍后重试')
    } finally {
      setSubmitting(false)
      setConfirmOpen(false)
    }
  }

  const handleSubmitClick = () => {
    if (submitBlocked) return
    if (!currentSource.trim()) {
      toast.warning(answerMode === 'file' ? '请选择文件' : '请输入代码')
      return
    }
    // 重复提交需明确确认：评测以最后一次为准。
    if (submitCountRef.current >= 1) setConfirmOpen(true)
    else void doSubmit()
  }

  // Ctrl/Cmd + Enter 全局快捷提交。
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
        event.preventDefault()
        if (!submitBlocked && submitCountRef.current < 1) void doSubmit()
        else if (!submitBlocked) setConfirmOpen(true)
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [submitBlocked, currentSource, language, answerMode])

  const timeLimitLabel = task.timeLimitMs
    ? task.timeLimitMaxMs && task.timeLimitMaxMs !== task.timeLimitMs
      ? `${(task.timeLimitMs / 1000).toFixed(1)}s ~ ${(task.timeLimitMaxMs / 1000).toFixed(1)}s`
      : `${(task.timeLimitMs / 1000).toFixed(1)}s`
    : '—'

  return (
    <div className="space-y-5">
      {/* 题头：返回 + 题号标题 + 元信息 */}
      <div className="flex flex-wrap items-start gap-3">
        <Button variant="ghost" onClick={onBack} aria-label="返回题目列表">
          <ArrowLeft className="h-4 w-4" />
          返回
        </Button>
        <div className="min-w-0">
          <h2 className="font-display text-2xl font-semibold tracking-[-0.03em] text-[#14283d]">
            <span className="mr-2 text-base font-bold text-slate-400">第 {taskIndex + 1} 题</span>
            {task.title}
          </h2>
          <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-slate-500">
            <span className="rounded-lg border border-[#e4e7ec] bg-white px-2.5 py-1 font-semibold">
              分值 <b className="text-slate-800">{task.totalScore}</b>
            </span>
            <span className="rounded-lg border border-[#e4e7ec] bg-white px-2.5 py-1 font-semibold">
              时限 <b className="text-slate-800">{timeLimitLabel}</b>
            </span>
            <span className="rounded-lg border border-[#e4e7ec] bg-white px-2.5 py-1 font-semibold">
              源文件 <b className="font-mono text-slate-800">{task.sourceFileName || '—'}</b>
            </span>
            {task.taskType === 'answers_only' && <Badge variant="info">答案提交</Badge>}
          </div>
        </div>
        {lastAt && (
          <span className="ml-auto text-xs font-medium text-slate-400">
            上次提交于 {fmtClock(lastAt)}
          </span>
        )}
      </div>

      {/* 样例文件 */}
      {task.samples.length > 0 && (
        <div className="app-surface flex flex-wrap items-center gap-x-4 gap-y-2 rounded-2xl px-4 py-3">
          <span className="flex items-center gap-2 text-xs font-bold text-slate-600">
            <Paperclip className="h-3.5 w-3.5 text-forge-600" />
            样例文件
          </span>
          <span className="text-[11px] text-slate-400">点击文件名直接下载</span>
          <div className="flex flex-wrap gap-2">
            {task.samples.map((sample) => (
              <a
                key={sample.name}
                href={sample.url}
                download
                className="rounded-lg border border-forge-200 bg-forge-50 px-2.5 py-1 font-mono text-[11px] font-bold text-forge-800 transition hover:border-forge-400"
              >
                {sample.name}
                {sample.size != null && (
                  <span className="ml-1.5 font-sans font-medium text-forge-600/70">
                    {formatBytes(sample.size)}
                  </span>
                )}
              </a>
            ))}
          </div>
        </div>
      )}

      {/* 双模式切换 */}
      {data.submitMode === 'both' && (
        <div className="flex w-fit gap-1 rounded-2xl border border-[#e4e7ec] bg-white/80 p-1.5">
          {(['editor', 'file'] as const).map((mode) => (
            <button
              key={mode}
              type="button"
              onClick={() => {
                setAnswerMode(mode)
                try {
                  localStorage.setItem(answerModeKey(data.user), mode)
                } catch {
                  // ignore
                }
              }}
              aria-pressed={answerMode === mode}
              className={
                answerMode === mode
                  ? 'flex h-9 items-center gap-1.5 rounded-xl bg-[#14283d] px-4 text-xs font-bold text-white'
                  : 'flex h-9 items-center gap-1.5 rounded-xl px-4 text-xs font-bold text-slate-500 transition hover:bg-slate-100 hover:text-slate-900'
              }
            >
              {mode === 'editor' ? <FileCode2 className="h-3.5 w-3.5" /> : <FileUp className="h-3.5 w-3.5" />}
              {mode === 'editor' ? '在线编辑' : '上传文件'}
            </button>
          ))}
        </div>
      )}

      {answerMode === 'editor' ? (
        <section className="app-surface overflow-hidden rounded-2xl" aria-label="在线编辑">
          <div className="flex flex-wrap items-center gap-3 border-b border-[#e4e7ec] bg-[#fbfaf6] px-4 py-2.5">
            <label className="flex items-center gap-2 text-xs font-semibold text-slate-500">
              语言
              <Select
                value={language}
                onChange={(event) => setLanguage(event.target.value)}
                aria-label="选择语言"
                className="h-8"
              >
                {LANGUAGES.map((item) => (
                  <option key={item.value} value={item.value}>
                    {item.label}
                  </option>
                ))}
              </Select>
            </label>
            <label className="flex items-center gap-2 text-xs font-semibold text-slate-500">
              字号
              <Select
                value={String(fontSize)}
                onChange={(event) => setFontSize(parseInt(event.target.value, 10))}
                aria-label="选择字号"
                className="h-8"
              >
                {FONT_SIZES.map((size) => (
                  <option key={size} value={size}>
                    {size}
                  </option>
                ))}
              </Select>
            </label>
            <span className="ml-auto font-mono text-[11px] text-slate-400">
              字符数 {currentBytes} / {MAX_SOURCE_BYTES}
            </span>
          </div>
          <React.Suspense fallback={<EditorLoading />}>
            <MonacoEditor
              value={code}
              onChange={handleCodeChange}
              language={LANGUAGES.find((item) => item.value === language)?.monaco || 'cpp'}
              fontSize={fontSize}
              height="440px"
              ariaLabel={`${task.title} 代码编辑器`}
            />
          </React.Suspense>
        </section>
      ) : (
        <section className="app-surface rounded-2xl p-5" aria-label="上传文件">
          <h3 className="text-sm font-bold text-[#14283d]">上传源码文件</h3>
          <p className="mt-1 text-xs leading-5 text-slate-500">
            选择本地的源码文件上传即提交，评测以最后一次提交为准；单个文件不超过 {formatBytes(MAX_SOURCE_BYTES)}。
            文件名需与题目源文件名一致（如 {task.sourceFileName}.cpp）。
          </p>
          <input
            ref={fileInputRef}
            type="file"
            accept=".cpp,.cc,.cxx,.c,.h,.py,.pas,.pp,.txt"
            className="hidden"
            onChange={(event) => {
              const file = event.target.files?.[0]
              event.target.value = ''
              if (file) void ingestFile(file)
            }}
          />
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            onDragOver={(event) => {
              event.preventDefault()
              setDragging(true)
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(event) => {
              event.preventDefault()
              setDragging(false)
              const file = event.dataTransfer.files?.[0]
              if (file) void ingestFile(file)
            }}
            className={
              dragging
                ? 'mt-4 flex w-full flex-col items-center justify-center gap-2 rounded-2xl border-2 border-dashed border-forge-500 bg-forge-50 px-6 py-10 transition'
                : uploaded
                  ? 'mt-4 flex w-full flex-col items-center justify-center gap-2 rounded-2xl border-2 border-solid border-emerald-300 bg-emerald-50/60 px-6 py-10 transition hover:border-forge-400'
                  : 'mt-4 flex w-full flex-col items-center justify-center gap-2 rounded-2xl border-2 border-dashed border-[#d0d5dd] bg-[#fbfaf6] px-6 py-10 transition hover:border-forge-400 hover:bg-forge-50'
            }
            aria-label="选择或拖入源码文件"
          >
            <CloudUpload
              className={uploaded ? 'h-8 w-8 text-emerald-600' : 'h-8 w-8 text-slate-400'}
            />
            {uploaded ? (
              <>
                <span className="font-mono text-sm font-bold text-emerald-800">{uploaded.name}</span>
                <span className="text-xs text-emerald-700/80">
                  {uploaded.size} 字节 · 点击或拖入可替换
                </span>
              </>
            ) : (
              <>
                <span className="text-sm font-bold text-slate-600">点击选择文件，或将文件拖到这里</span>
                <span className="text-xs text-slate-400">支持 .cpp / .c / .py / .pas / .txt</span>
              </>
            )}
          </button>
          {uploaded && (
            <p className="mt-3 flex items-center gap-2 text-xs text-slate-500">
              <Gauge className="h-3.5 w-3.5 text-forge-600" />
              当前待提交 {formatBytes(uploaded.size)}，字符数 {currentBytes} / {MAX_SOURCE_BYTES}
            </p>
          )}
        </section>
      )}

      {/* 提交条 */}
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-[#e4e7ec] bg-[#fffefa] px-4 py-3">
        <p className="text-[11px] leading-5 text-slate-400">
          在线编辑内容自动保存草稿；Ctrl + Enter 快捷提交。
          <br />
          同一题可多次提交，评测以最后一次为准。
        </p>
        <Button
          onClick={handleSubmitClick}
          disabled={submitBlocked}
          loading={submitting}
          data-testid="lemon-submit"
        >
          {!submitting && <Send className="h-4 w-4" />}
          {countdown.state === 'pre'
            ? '比赛尚未开始'
            : countdown.state === 'ended'
              ? '比赛已结束'
              : submitting
                ? '提交中…'
                : '提交代码'}
        </Button>
      </div>

      <Dialog
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title="确认覆盖上一次提交"
        eyebrow="重复提交"
        footer={
          <>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>
              再想想
            </Button>
            <Button onClick={() => void doSubmit()} loading={submitting}>
              <Send className="h-4 w-4" />
              确认提交
            </Button>
          </>
        }
      >
        <p className="text-sm leading-6 text-slate-600">
          这是第 {submitCountRef.current + 1} 次提交此题，提交后将覆盖上一次代码（评测以最后一次为准）。确定继续吗？
        </p>
      </Dialog>
    </div>
  )
}
