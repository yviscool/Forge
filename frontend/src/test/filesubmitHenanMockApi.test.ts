import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockHenanApi } from '@/features/filesubmit/henan/mockApi'

// 河南版收卷 mock：校验顺序、封禁与冻结语义与河南收卷系统逐条对齐。
// URL 参数 henanPhase / henanBanned 冻结在模块加载时，用动态 import
// + resetModules 覆盖非默认状态。

afterEach(() => {
  window.history.replaceState({}, '', '/')
  vi.resetModules()
})

describe('mockHenanApi (active state)', () => {
  it('heartbeats the active phase', async () => {
    const beat = await mockHenanApi.heartbeat()
    expect(beat.phase).toBe('active')
    expect(beat.banned).toBe(false)
    expect(beat.confirmed).toBe(false)
    expect(beat.username).toBe('test')
    expect(beat.startTime).toBeLessThan(beat.endTime)
    expect(Math.abs(beat.serverTimeMs - Date.now())).toBeLessThan(5000)
  })

  it('lists problems with sort order and seeded records', async () => {
    const problems = await mockHenanApi.problems()
    expect(problems.map((problem) => problem.name)).toEqual(['bracket', 'color', 'part', 'rank'])
    expect(problems[0].record?.submitCount).toBe(1)
    expect(problems[1].record).toBeUndefined()
  })

  it('enforces the henan validation order on submit', async () => {
    // .cpp 后缀 → 文件名全等 → 题目存在性。
    await expect(mockHenanApi.submit('color', { name: 'color.c', size: 1, content: 'x' })).resolves.toMatchObject({
      status: 'error', message: '文件必须以 .cpp 结尾',
    })
    await expect(mockHenanApi.submit('color', { name: 'bracket.cpp', size: 1, content: 'x' })).resolves.toMatchObject({
      status: 'error', message: '文件名必须为 color.cpp',
    })
    await expect(mockHenanApi.submit('ghost', { name: 'ghost.cpp', size: 1, content: 'x' })).resolves.toMatchObject({
      status: 'error', message: '题目不存在',
    })
  })

  it('records submissions and bumps the counter per problem', async () => {
    const first = await mockHenanApi.submit('color', { name: 'color.cpp', size: 13, content: 'int main(){}' })
    expect(first.status).toBe('success')
    const second = await mockHenanApi.submit('color', { name: 'color.cpp', size: 20, content: 'int main(){return 1;}' })
    expect(second.status).toBe('success')
    const problems = await mockHenanApi.problems()
    const color = problems.find((problem) => problem.name === 'color')
    expect(color?.record).toMatchObject({ submitCount: 2, filename: 'color.cpp' })
    expect(color?.record?.code).toContain('return 1;')
  })

  it('freezes every problem after the final confirmation', async () => {
    const confirm = await mockHenanApi.confirm()
    expect(confirm.status).toBe('success')
    await expect(mockHenanApi.confirm()).resolves.toMatchObject({
      status: 'error', message: '您已确认最终提交，不能再次提交代码',
    })
    await expect(mockHenanApi.submit('part', { name: 'part.cpp', size: 1, content: 'x' })).resolves.toMatchObject({
      status: 'error', message: '您已确认最终提交，不能再次提交代码',
    })
    const beat = await mockHenanApi.heartbeat()
    expect(beat.confirmed).toBe(true)
  })

  it('keeps scores unpublished until the publish phase', async () => {
    const score = await mockHenanApi.score()
    expect(score.resultsPublished).toBe(false)
    expect(score.localJudge).toBe(false)
  })

  it('serves the problem attachments', async () => {
    const files = await mockHenanApi.files()
    expect(files[0].name).toBe('题目册.txt')
  })

  it('validates the exam password change', async () => {
    await expect(mockHenanApi.changePassword('wrong', '123456')).resolves.toMatchObject({
      status: 'error', message: '当前密码不正确',
    })
    await expect(mockHenanApi.changePassword('test', '123')).resolves.toMatchObject({
      status: 'error', message: '新密码至少 6 位',
    })
    await expect(mockHenanApi.changePassword('test', '123456')).resolves.toMatchObject({
      status: 'success',
    })
  })
})

describe('mockHenanApi gated states', () => {
  it('blocks submit and confirm for banned accounts', async () => {
    window.history.replaceState({}, '', '/?henanBanned=1')
    vi.resetModules()
    const { mockHenanApi: banned } = await import('@/features/filesubmit/henan/mockApi')
    await expect(banned.submit('color', { name: 'color.cpp', size: 1, content: 'x' })).resolves.toMatchObject({
      status: 'error', message: '账号已被封禁，不能继续作答',
    })
    await expect(banned.confirm()).resolves.toMatchObject({
      status: 'error', message: '账号已被封禁，不能继续作答',
    })
    const beat = await banned.heartbeat()
    expect(beat.banned).toBe(true)
  })

  it('rejects submissions outside the active phase', async () => {
    window.history.replaceState({}, '', '/?henanPhase=waiting')
    vi.resetModules()
    const { mockHenanApi: waiting } = await import('@/features/filesubmit/henan/mockApi')
    await expect(waiting.submit('color', { name: 'color.cpp', size: 1, content: 'x' })).resolves.toMatchObject({
      status: 'error', message: '比赛尚未开始',
    })
    await expect(waiting.confirm()).resolves.toMatchObject({
      status: 'error', message: '比赛尚未开始',
    })

    window.history.replaceState({}, '', '/?henanPhase=ended')
    vi.resetModules()
    const { mockHenanApi: ended } = await import('@/features/filesubmit/henan/mockApi')
    await expect(ended.submit('color', { name: 'color.cpp', size: 1, content: 'x' })).resolves.toMatchObject({
      status: 'error', message: '比赛已结束',
    })
  })

  it('publishes ranked results with per-case detail in the publish phase', async () => {
    window.history.replaceState({}, '', '/?henanPhase=publish')
    vi.resetModules()
    const { mockHenanApi: publish } = await import('@/features/filesubmit/henan/mockApi')
    const score = await publish.score()
    expect(score.resultsPublished).toBe(true)
    expect(score.rank).toBe(3)
    expect(score.localJudge).toBe(true)
    expect(score.score.bracket?.cases).toHaveLength(10)
  })
})
