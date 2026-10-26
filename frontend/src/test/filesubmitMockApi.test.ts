import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockLemonApi } from '@/features/filesubmit/mockApi'
import type { LemonUploadFileInput } from '@/features/filesubmit/types'

// mock API（Lemon 版）：数据形状与整包上传校验规则（与 LemonNext
// SubmissionServer 一致）。URL 参数 lemonState 冻结在模块加载时，
// 用动态 import + resetModules 覆盖 pre/ended 两种状态。

afterEach(() => {
  window.history.replaceState({}, '', '/')
  vi.resetModules()
})

describe('mockLemonApi (running state)', () => {
  it('serves the contest skeleton with six tasks', async () => {
    const data = await mockLemonApi.tasks()
    expect(data.contestTitle).toContain('CSP-J')
    expect(data.user).toBe('HA-J00001')
    expect(data.submitMode).toBe('both')
    expect(data.windowEnabled).toBe(true)
    expect(data.preContest).toBe(false)
    expect(data.tasks).toHaveLength(6)
    // 预置提交记录：bracket 已交，color 未交。
    expect(data.tasks[0].submittedAt).not.toBe('')
    expect(data.tasks[1].submittedAt).toBe('')
    expect(data.statements[0].name).toBe('题面.txt')
  })

  it('serves notice text and message history', async () => {
    const notice = await mockLemonApi.notice()
    expect(notice.text).toContain('不得打开其他网页')
    const messages = await mockLemonApi.messages()
    expect(messages.messages).toHaveLength(2)
    expect(messages.latestId).toBe(2)
  })

  it('keeps scores sealed while the contest is running', async () => {
    const scores = await mockLemonApi.scores()
    expect(scores.released).toBe(false)
    expect(scores.tasks).toHaveLength(0)
  })

  it('returns stored sources and rejects unseen ones', async () => {
    const source = await mockLemonApi.source(0)
    expect(source.filename).toBe('bracket.cpp')
    expect(source.content).toContain('std::stack')
    expect(source.bytes).toBe(new TextEncoder().encode(source.content).length)
    await expect(mockLemonApi.source(2)).rejects.toThrow('该题尚未提交')
  })

  it('serves the choice paper only for choice tasks', async () => {
    const meta = await mockLemonApi.choice(4)
    expect(meta.answerFileExt).toBe('txt')
    expect(meta.questions).toHaveLength(5)
    expect(meta.questions[3].type).toBe('multiple')
    expect(meta.paperMarkdown).toContain('{{ select(1) }}')
    await expect(mockLemonApi.choice(0)).rejects.toThrow('该题不是选择题')
  })

  it('records submissions and maps language to file extension', async () => {
    const result = await mockLemonApi.submit(1, { source: 'int main(){}', language: 'cpp' })
    expect(result.submittedAt).toBeTruthy()
    const py = await mockLemonApi.submit(1, { source: 'print(1)', language: 'python' })
    expect(py.submittedAt).toBeTruthy()
    expect((await mockLemonApi.source(1)).filename).toBe('color.py')
    const pas = await mockLemonApi.submit(1, { source: 'begin end.', language: 'pascal' })
    void pas
    expect((await mockLemonApi.source(1)).filename).toBe('color.pas')
    await expect(mockLemonApi.submit(9, { source: 'x', language: 'cpp' })).rejects.toThrow('题目不存在')
  })

  it('validates upload folders like the LemonNext submission server', async () => {
    const files: LemonUploadFileInput[] = [
      { path: 'HA-J00001/bracket.cpp', size: 10 },
      { path: 'HA-J00001/../escape.cpp', size: 5 },
      { path: 'HA-J00001\\color.cpp', size: 5 },
      { path: '/abs/color.cpp', size: 5 },
      { path: 'HA-J00001/CON', size: 5 },
      { path: 'HA-J00001/a/b/c/d/e/f/g/h/i.cpp', size: 5 },
    ]
    const result = await mockLemonApi.uploadFolder('HA-J00001', files)
    expect(result.total).toBe(6)
    expect(result.written).toBe(1)
    expect(result.files[0]).toMatchObject({ ok: true, target: 'source/HA-J00001/bracket.cpp' })
    expect(result.files[1].message).toBe('不允许相对路径片段')
    expect(result.files[2].message).toBe('路径分隔符不合法')
    expect(result.files[3].message).toBe('不允许绝对路径')
    expect(result.files[4].message).toBe('Windows 保留名')
    expect(result.files[5].message).toBe('目录层级超过 8 层')
  })

  it('rejects oversized folders', async () => {
    const many: LemonUploadFileInput[] = Array.from({ length: 513 }, (_, index) => ({
      path: `HA-J00001/f${index}.cpp`, size: 1,
    }))
    await expect(mockLemonApi.uploadFolder('HA-J00001', many)).rejects.toThrow('文件数超过 512')
    const fat: LemonUploadFileInput[] = [
      { path: 'HA-J00001/bracket.cpp', size: 9 * 1024 * 1024 },
    ]
    await expect(mockLemonApi.uploadFolder('HA-J00001', fat)).rejects.toThrow('总体积超过 8 MB')
  })
})

describe('mockLemonApi window states', () => {
  it('locks submissions before the window opens', async () => {
    window.history.replaceState({}, '', '/?lemonState=pre')
    vi.resetModules()
    const { mockLemonApi: pre } = await import('@/features/filesubmit/mockApi')
    await expect(pre.submit(1, { source: 'x', language: 'cpp' })).rejects.toThrow('比赛尚未开始')
    const data = await pre.tasks()
    expect(data.preContest).toBe(true)
    const scores = await pre.scores()
    expect(scores.released).toBe(false)
  })

  it('publishes scores after the window closes', async () => {
    window.history.replaceState({}, '', '/?lemonState=ended')
    vi.resetModules()
    const { mockLemonApi: ended } = await import('@/features/filesubmit/mockApi')
    const scores = await ended.scores()
    expect(scores.released).toBe(true)
    expect(scores.totalPossible).toBe(600)
    expect(scores.totalScore).toBe(320)
    expect(scores.tasks[2].compileFailed).toBe(true)
    expect(scores.tasks[5].placementError).toBe(true)
    const data = await ended.tasks()
    expect(data.scoresReleased).toBe(true)
  })
})
