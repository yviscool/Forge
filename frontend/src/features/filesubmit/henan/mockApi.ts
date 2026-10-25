// 文件提交模式（河南版）——mock API。
//
// 规则与河南收卷系统逐条对齐：仅 C++、文件必须为 <题目英文名>.cpp、
// 校验顺序（phase → confirmed → 已选文件 → .cpp 后缀 → 文件名全等）、
// 最终确认后冻结、封禁只读、成绩发布带测试点与排名。
// 演示状态：地址栏 ?henanPhase=waiting|before|active|ended|publish，
// ?henanBanned=1 封禁态。默认 active。

import type {
  HenanApi,
  HenanAttachment,
  HenanHeartbeat,
  HenanPhase,
  HenanProblem,
  HenanScore,
} from './types'

const BOOT = Date.now()
const MIN = 60 * 1000
const SEC = 1000

function readParam(name: string): string | null {
  try {
    return new URLSearchParams(window.location.search).get(name)
  } catch {
    return null
  }
}

const PHASE: HenanPhase = (() => {
  const value = readParam('henanPhase')
  if (value === 'waiting' || value === 'before' || value === 'ended' || value === 'publish') {
    return value
  }
  return 'active'
})()

const BANNED = readParam('henanBanned') === '1'

const WINDOW: { start: number; end: number } =
  PHASE === 'before'
    ? { start: BOOT + 5 * MIN, end: BOOT + 205 * MIN }
    : PHASE === 'ended' || PHASE === 'publish'
      ? { start: BOOT - 300 * MIN, end: BOOT - 6 * MIN }
      : { start: BOOT - 180 * MIN, end: BOOT + 174 * MIN }

const ANNOUNCEMENT = '各位选手：提交前请确认源码文件已保存，文件名必须与题目英文名完全一致。'

const PROBLEMS: Array<{ name: string; displayName: string }> = [
  { name: 'bracket', displayName: 'bracket' },
  { name: 'color', displayName: 'color' },
  { name: 'part', displayName: 'part' },
  { name: 'rank', displayName: 'rank' },
]

const records = new Map<string, { submitCount: number; filename: string; submittedAt: string; code: string }>()

records.set('bracket', {
  submitCount: 1,
  filename: 'bracket.cpp',
  submittedAt: new Date(BOOT - 40 * MIN).toISOString(),
  code: '#include <cstdio>\nint main() { return 0; }\n',
})

const ATTACHMENT_URL = URL.createObjectURL(
  new Blob(['CSP-J 第二轮 全真模拟 题目册（占位附件）\n'], { type: 'text/plain' })
)

const SCORE_FIXTURE: HenanScore = {
  confirmed: false,
  localJudge: true,
  rank: 3,
  resultsPublished: true,
  score: {
    bracket: {
      score: 100,
      totalScore: 100,
      submitCount: 1,
      cases: Array.from({ length: 10 }, (_, index) => ({
        index,
        score: 10,
        verdict: 'AC',
        timeMs: 30 + index * 7,
        memoryKib: 12000 + index * 512,
      })),
    },
    color: {
      score: 70,
      totalScore: 100,
      submitCount: 2,
      cases: Array.from({ length: 10 }, (_, index) => ({
        index,
        score: index < 7 ? 10 : 0,
        verdict: index < 7 ? 'AC' : 'WA',
        timeMs: 40 + index * 5,
        memoryKib: 13000,
      })),
    },
    part: {
      score: 0,
      totalScore: 100,
      submitCount: 1,
      cases: [{ index: 0, score: 0, verdict: 'CE', timeMs: 0 }],
    },
    rank: {
      score: 100,
      totalScore: 100,
      submitCount: 3,
      cases: Array.from({ length: 10 }, (_, index) => ({
        index,
        score: 10,
        verdict: 'AC',
        timeMs: 25 + index * 4,
        memoryKib: 11000,
      })),
    },
  },
}

const delay = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms))

let confirmed = false

export const mockHenanApi: HenanApi = {
  async heartbeat() {
    await delay(150)
    const beat: HenanHeartbeat = {
      announcement: ANNOUNCEMENT,
      banned: BANNED,
      confirmed,
      contestName: 'CSP-J 第二轮 全真模拟（河南收卷）',
      startTime: Math.floor(WINDOW.start / SEC),
      endTime: Math.floor(WINDOW.end / SEC),
      phase: PHASE,
      realName: '测试选手',
      username: 'test',
      serverTimeMs: Date.now(),
    }
    return beat
  },

  async problems() {
    await delay(200)
    const list: HenanProblem[] = PROBLEMS.map((problem, index) => ({
      ...problem,
      sort: index,
      record: records.get(problem.name),
    }))
    return list
  },

  async submit(problem, file) {
    await delay(400)
    // 与河南系统一致的校验顺序与消息原文。
    if (PHASE === 'ended' || PHASE === 'publish') return { status: 'error', message: '比赛已结束' }
    if (PHASE !== 'active') return { status: 'error', message: '比赛尚未开始' }
    if (confirmed) return { status: 'error', message: '您已确认最终提交，不能再次提交代码' }
    if (BANNED) return { status: 'error', message: '账号已被封禁，不能继续作答' }
    if (!file.name.endsWith('.cpp')) return { status: 'error', message: '文件必须以 .cpp 结尾' }
    if (file.name.slice(0, -4) !== problem) {
      return { status: 'error', message: `文件名必须为 ${problem}.cpp` }
    }
    if (!PROBLEMS.some((item) => item.name === problem)) {
      return { status: 'error', message: '题目不存在' }
    }
    const existing = records.get(problem)
    records.set(problem, {
      submitCount: (existing?.submitCount || 0) + 1,
      filename: file.name,
      submittedAt: new Date().toISOString(),
      code: file.content,
    })
    return { status: 'success' }
  },

  async confirm() {
    await delay(400)
    if (confirmed) return { status: 'error', message: '您已确认最终提交，不能再次提交代码' }
    if (BANNED) return { status: 'error', message: '账号已被封禁，不能继续作答' }
    if (PHASE !== 'active') return { status: 'error', message: '比赛尚未开始' }
    confirmed = true
    return { status: 'success' }
  },

  async score() {
    await delay(300)
    if (PHASE !== 'publish') {
      return { confirmed, localJudge: false, rank: 0, resultsPublished: false, score: {} }
    }
    return { ...SCORE_FIXTURE, confirmed }
  },

  async files() {
    await delay(150)
    const files: HenanAttachment[] = [
      { name: '题目册.txt', size: 48, url: ATTACHMENT_URL },
    ]
    return files
  },

  async changePassword(current, next) {
    await delay(300)
    if (current !== 'test') return { status: 'error', message: '当前密码不正确' }
    if (next.length < 6) return { status: 'error', message: '新密码至少 6 位' }
    return { status: 'success' }
  },
}
