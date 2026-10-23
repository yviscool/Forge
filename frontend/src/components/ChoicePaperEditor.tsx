import React, { useEffect, useState } from 'react'
import { ArrowDown, ArrowUp, Check, Plus, Trash2, X } from 'lucide-react'
import type { ChoiceQuestion } from '../api/client'
import { Button } from './ui/button'
import { Input, Select, Textarea } from './ui/input'

interface ChoicePaperEditorProps {
  questions: ChoiceQuestion[]
  onChange: (questions: ChoiceQuestion[]) => void
  disabled?: boolean
}

const optionLetter = (index: number) => String.fromCharCode(65 + index)

function answerText(question: ChoiceQuestion): string {
  if (Array.isArray(question.answer)) return question.answer.join(',')
  return typeof question.answer === 'string' ? question.answer : ''
}

function parseAnswer(value: string, type: ChoiceQuestion['type']): string | string[] {
  const tokens = value
    .toUpperCase()
    .split(/[\s,，;；]+/)
    .filter(Boolean)
  if (type === 'multiple') return tokens
  return tokens[0] || ''
}

function AnswerEditor({
  question,
  onChange,
  disabled,
}: {
  question: ChoiceQuestion
  onChange: (answer: string | string[]) => void
  disabled: boolean
}) {
  const canonical = answerText(question)
  const [draft, setDraft] = useState(canonical)

  // Keep an in-progress separator ("A,") visible while still publishing a
  // normalized answer to the parent. Without this, a controlled array value
  // immediately rewrites the input and prevents entering multiple choices.
  useEffect(() => {
    setDraft((current) => {
      const normalized = answerText({
        ...question,
        answer: parseAnswer(current, question.type),
      })
      return normalized === canonical ? current : canonical
    })
  }, [canonical, question])

  return (
    <Input
      aria-label={`第 ${question.id} 题答案`}
      value={draft}
      disabled={disabled}
      onChange={(event) => {
        setDraft(event.target.value)
        onChange(parseAnswer(event.target.value, question.type))
      }}
      onBlur={() => setDraft(answerText({ ...question, answer: parseAnswer(draft, question.type) }))}
      placeholder={question.type === 'multiple' ? 'A,C' : question.type === 'judge' ? 'T' : 'A'}
      className="h-8 font-mono text-xs"
    />
  )
}

function nextQuestionID(questions: ChoiceQuestion[]): number {
  return questions.reduce((max, question) => Math.max(max, question.id), 0) + 1
}

export function createChoiceQuestion(
  type: ChoiceQuestion['type'] = 'single',
  id = 1
): ChoiceQuestion {
  const base: ChoiceQuestion = {
    id,
    type,
    stem: '',
    answer: type === 'judge' ? 'T' : 'A',
    score: type === 'multiple' ? 4 : 2,
  }
  if (type !== 'judge') {
    base.options = ['A. 选项一', 'B. 选项二']
  }
  return base
}

/**
 * Visual editor for a GESP objective paper. The server remains the final
 * validator; this component only keeps the authoring experience predictable.
 */
export function ChoicePaperEditor({
  questions,
  onChange,
  disabled = false,
}: ChoicePaperEditorProps) {
  const update = (index: number, patch: Partial<ChoiceQuestion>) => {
    onChange(questions.map((question, i) => (i === index ? { ...question, ...patch } : question)))
  }

  const changeType = (index: number, type: ChoiceQuestion['type']) => {
    const current = questions[index]
    const next = { ...current, type }
    if (type === 'judge') {
      next.options = []
      next.answer = 'T'
    } else {
      next.options =
        current.options && current.options.length >= 2
          ? current.options
          : ['A. 选项一', 'B. 选项二']
      next.answer = type === 'multiple' ? ['A'] : 'A'
    }
    update(index, next)
  }

  const addOption = (index: number) => {
    const question = questions[index]
    const options = [...(question.options || [])]
    const letter = optionLetter(options.length)
    options.push(`${letter}. 选项${options.length + 1}`)
    update(index, { options })
  }

  const updateOption = (questionIndex: number, optionIndex: number, value: string) => {
    const question = questions[questionIndex]
    const options = [...(question.options || [])]
    options[optionIndex] = value
    update(questionIndex, { options })
  }

  const removeOption = (questionIndex: number, optionIndex: number) => {
    const question = questions[questionIndex]
    const options = (question.options || []).filter((_, index) => index !== optionIndex)
    update(questionIndex, { options })
  }

  const add = () => {
    onChange([...questions, createChoiceQuestion('single', nextQuestionID(questions))])
  }

  const move = (index: number, direction: -1 | 1) => {
    const target = index + direction
    if (target < 0 || target >= questions.length) return
    const next = [...questions]
    ;[next[index], next[target]] = [next[target], next[index]]
    onChange(next.map((question, i) => ({ ...question, id: i + 1 })))
  }

  const remove = (index: number) => {
    onChange(questions.filter((_, i) => i !== index).map((question, i) => ({ ...question, id: i + 1 })))
  }

  return (
    <section className="space-y-3 rounded-xl border border-sky-200 bg-sky-50/40 p-3.5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p className="text-xs font-extrabold text-sky-950">客观题答题卡编辑器</p>
          <p className="mt-0.5 text-[11px] leading-5 text-sky-800/75">
            支持单选、多选、判断题。答案会在服务端脱敏，仅教师可见。
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="rounded-full bg-white px-2.5 py-1 font-mono text-[10px] font-bold text-sky-700">
            {questions.length} 题 · {questions.reduce((sum, q) => sum + (Number(q.score) || 0), 0)} 分
          </span>
          <Button size="sm" variant="outline" onClick={add} disabled={disabled} className="h-8 gap-1 text-xs">
            <Plus className="h-3.5 w-3.5" /> 新增小题
          </Button>
        </div>
      </div>

      {questions.length === 0 ? (
        <div className="rounded-lg border border-dashed border-sky-300 bg-white px-4 py-7 text-center text-xs text-slate-500">
          还没有客观题。先新增小题，或使用上一层的“载入 GESP 样卷”快速起步。
        </div>
      ) : (
        <div className="space-y-3">
          {questions.map((question, index) => (
            <article key={`${question.id}-${index}`} className="rounded-xl border border-slate-200 bg-white p-3.5 shadow-sm">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <span className="grid h-7 w-7 place-items-center rounded-lg bg-sky-100 font-mono text-xs font-extrabold text-sky-800">
                    {index + 1}
                  </span>
                  <Select
                    aria-label={`第 ${index + 1} 题题型`}
                    value={question.type}
                    disabled={disabled}
                    onChange={(event) => changeType(index, event.target.value as ChoiceQuestion['type'])}
                    className="h-8 w-auto py-0 text-xs"
                  >
                    <option value="single">单选题</option>
                    <option value="multiple">多选题</option>
                    <option value="judge">判断题</option>
                  </Select>
                </div>
                <div className="flex items-center gap-1">
                  <Button
                    size="sm"
                    variant="ghost"
                    aria-label={`上移第 ${index + 1} 题`}
                    onClick={() => move(index, -1)}
                    disabled={disabled || index === 0}
                    className="h-7 w-7 p-0"
                  >
                    <ArrowUp className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    aria-label={`下移第 ${index + 1} 题`}
                    onClick={() => move(index, 1)}
                    disabled={disabled || index === questions.length - 1}
                    className="h-7 w-7 p-0"
                  >
                    <ArrowDown className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    aria-label={`删除第 ${index + 1} 题`}
                    onClick={() => remove(index)}
                    disabled={disabled}
                    className="h-7 gap-1 px-2 text-[10px] text-rose-600 hover:bg-rose-50 hover:text-rose-700"
                  >
                    <Trash2 className="h-3 w-3" /> 删除
                  </Button>
                </div>
              </div>

              <label className="mt-3 block space-y-1">
                <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">题干</span>
                <Textarea
                  aria-label={`第 ${index + 1} 题题干`}
                  rows={2}
                  value={question.stem}
                  disabled={disabled}
                  onChange={(event) => update(index, { stem: event.target.value })}
                  placeholder="支持 Markdown，例如：下列哪个是 C++ 关键字？"
                  className="text-xs"
                />
              </label>

              {question.type !== 'judge' && (
                <div className="mt-3 space-y-1.5">
                  <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">选项</span>
                  {(question.options || []).map((option, optionIndex) => (
                    <div key={optionIndex} className="flex items-center gap-2">
                      <span className="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-slate-100 font-mono text-[10px] font-extrabold text-slate-600">
                        {optionLetter(optionIndex)}
                      </span>
                      <Input
                        aria-label={`第 ${index + 1} 题选项 ${optionLetter(optionIndex)}`}
                        value={option}
                        disabled={disabled}
                        onChange={(event) => updateOption(index, optionIndex, event.target.value)}
                        className="h-8 text-xs"
                      />
                      <Button
                        size="sm"
                        variant="ghost"
                        aria-label={`删除第 ${index + 1} 题选项 ${optionLetter(optionIndex)}`}
                        onClick={() => removeOption(index, optionIndex)}
                        disabled={disabled || (question.options || []).length <= 2}
                        className="h-7 w-7 p-0 text-slate-400 hover:text-rose-600"
                      >
                        <X className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  ))}
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => addOption(index)}
                    disabled={disabled || (question.options || []).length >= 8}
                    className="h-7 gap-1 px-2 text-[10px] text-sky-700"
                  >
                    <Plus className="h-3 w-3" /> 添加选项
                  </Button>
                </div>
              )}

              <div className="mt-3 grid gap-2 sm:grid-cols-[1fr_7rem_7rem]">
                <label className="space-y-1">
                  <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">
                    {question.type === 'multiple' ? '正确答案（逗号分隔）' : question.type === 'judge' ? '正确答案（T/F）' : '正确答案'}
                  </span>
                  <AnswerEditor
                    question={{ ...question, id: index + 1 }}
                    onChange={(answer) => update(index, { answer })}
                    disabled={disabled}
                  />
                </label>
                <label className="space-y-1">
                  <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">分值</span>
                  <Input
                    aria-label={`第 ${index + 1} 题分值`}
                    type="number"
                    min={1}
                    value={question.score}
                    disabled={disabled}
                    onChange={(event) => update(index, { score: Number(event.target.value) || 0 })}
                    className="h-8 text-xs"
                  />
                </label>
                <label className="space-y-1">
                  <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">题解</span>
                  <Input
                    aria-label={`第 ${index + 1} 题题解`}
                    value={question.explanation || ''}
                    disabled={disabled}
                    onChange={(event) => update(index, { explanation: event.target.value })}
                    placeholder="可选"
                    className="h-8 text-xs"
                  />
                </label>
              </div>
              <div className="mt-2 flex justify-end text-[10px] text-emerald-700">
                <Check className="mr-1 h-3 w-3" /> 仅教师端可见答案
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  )
}
