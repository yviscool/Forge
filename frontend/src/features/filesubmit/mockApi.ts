// 文件提交模式（Lemon）——mock API。
//
// 后端未接入前的本地实现：数据形状与 LemonNext 服务端一致，含比赛时间窗、
// 提交记录、消息、成绩发布与整包上传校验。真实服务端就绪后，把
// FileSubmitWorkspace 里的 `const api = mockLemonApi` 换成走 fetch 的
// 同接口实现即可，视图层不感知。
//
// 演示三种时间窗状态：地址栏加 ?lemonState=pre（开考前锁定）、
// ?lemonState=ended（已结束 + 成绩发布）、默认 running（进行中）。

import type {
  LemonApi,
  LemonChoiceMeta,
  LemonMessage,
  LemonScoreTask,
  LemonSource,
  LemonTask,
  LemonTasksResponse,
  LemonUploadFolderResult,
} from './types'

const BOOT = Date.now()
const MIN = 60 * 1000

type WindowMode = 'running' | 'pre' | 'ended'

function readWindowMode(): WindowMode {
  try {
    const value = new URLSearchParams(window.location.search).get('lemonState')
    if (value === 'pre' || value === 'ended') return value
  } catch {
    // ignore
  }
  return 'running'
}

const windowMode = readWindowMode()

const WINDOW =
  windowMode === 'pre'
    ? { start: BOOT + 5 * MIN, end: BOOT + 185 * MIN, preContest: true, scoresReleased: false }
    : windowMode === 'ended'
      ? { start: BOOT - 190 * MIN, end: BOOT - 4 * MIN, preContest: false, scoresReleased: true }
      : { start: BOOT - 10 * MIN, end: BOOT + 110 * MIN, preContest: false, scoresReleased: false }

const iso = (ms: number) => new Date(ms).toISOString()

function sampleBlobUrl(name: string, content: string, sizeDelta = 0) {
  const url = URL.createObjectURL(new Blob([content], { type: 'text/plain' }))
  return { name, url, size: new TextEncoder().encode(content).length + sizeDelta }
}

const COLOR_IN = sampleBlobUrl('color1.in', '4\nred blue red green\n')
const COLOR_ANS = sampleBlobUrl('color1.ans', '2\n')

const STATEMENT_URL = URL.createObjectURL(
  new Blob(
    [
      'CSP-J 第二轮 全真模拟（文件提交模式 Lemon 演示）\n\n' +
        '本文件为占位题面。接入后端后，试题下载将指向真实的题面 PDF。\n' +
        '比赛共 6 题，满分 600 分。请遵守考场纪律。\n',
    ],
    { type: 'text/plain' }
  )
)

const BRACKET_CODE = `#include <cstdio>
#include <stack>
#include <string>

int main() {
    int n;
    std::scanf("%d", &n);
    std::stack<char> st;
    // TODO: 括号匹配
    return 0;
}
`

const RANK_CODE = `#include <iostream>
int main() {
    int n;
    std::cin >> n;
    std::cout << n << std::endl;
}
`

const ANSWER_TEXT = '1 42\n2 998244353\n'

interface SubmissionRecord {
  filename: string
  submittedAt: string
  content: string
}

const submissions = new Map<number, SubmissionRecord>([
  [0, { filename: 'bracket.cpp', submittedAt: iso(BOOT - 42 * MIN), content: BRACKET_CODE }],
  [3, { filename: 'rank.cpp', submittedAt: iso(BOOT - 9 * MIN), content: RANK_CODE }],
  [5, { filename: 'answer.txt', submittedAt: iso(BOOT - 70 * MIN), content: ANSWER_TEXT }],
])

const QUIZ_PAPER = `# GESP 选择题演练

本大题共 5 小题，均为必答。请点击底部「递交答案」完成提交。

## 第 1 题

一个字节（byte）由 {{ select(1) }}
- 8 个二进制位组成
- 16 个二进制位组成
- 32 个二进制位组成
- 与操作系统相关，不确定

## 第 2 题

下列关于图的说法正确的是 {{ select(2) }}
- 无向完全图一定存在欧拉回路
- 有向无环图的拓扑序一定唯一
- 连通图的生成树包含全部顶点
- 强连通图一定是无向图

## 第 3 题

表达式 $(a + b) \\times c$ 的后缀形式是 {{ select(3) }}
- $a\\ b\\ +\\ c\\ \\times$
- $a\\ b\\ c\\ +\\ \\times$
- $a\\ +\\ b\\ c\\ \\times$
- $c\\ \\times\\ a\\ b\\ +$

## 第 4 题（多选）

下列时间复杂度为 $O(n \\log n)$ 的排序算法有 {{ select(4) }}
- 归并排序
- 冒泡排序
- 快速排序（平均）
- 插入排序

## 第 5 题

对于任意非空集合，哈希表的查找时间复杂度一定为 $O(1)$。{{ select(5) }}
- 正确
- 错误
`

const CHOICE_META: LemonChoiceMeta = {
  paperMarkdown: QUIZ_PAPER,
  answerFileExt: 'txt',
  questions: [
    { id: 1, type: 'single', score: 20, optionCount: 4 },
    { id: 2, type: 'single', score: 20, optionCount: 4 },
    { id: 3, type: 'single', score: 20, optionCount: 4 },
    { id: 4, type: 'multiple', score: 20, optionCount: 4 },
    { id: 5, type: 'truefalse', score: 20 },
  ],
}

const TASKS: LemonTask[] = [
  {
    id: 0,
    title: 'bracket',
    sourceFileName: 'bracket',
    totalScore: 100,
    timeLimitMs: 15000,
    taskType: 'traditional',
    submittedAt: submissions.get(0)?.submittedAt || '',
    samples: [],
  },
  {
    id: 1,
    title: 'color',
    sourceFileName: 'color',
    totalScore: 100,
    timeLimitMs: 17000,
    taskType: 'traditional',
    submittedAt: '',
    samples: [
      { name: COLOR_IN.name, url: COLOR_IN.url, size: COLOR_IN.size },
      { name: COLOR_ANS.name, url: COLOR_ANS.url, size: COLOR_ANS.size },
    ],
  },
  {
    id: 2,
    title: 'part',
    sourceFileName: 'part',
    totalScore: 100,
    timeLimitMs: 14000,
    taskType: 'traditional',
    submittedAt: '',
    samples: [],
  },
  {
    id: 3,
    title: 'rank',
    sourceFileName: 'rank',
    totalScore: 100,
    timeLimitMs: 12000,
    taskType: 'traditional',
    submittedAt: submissions.get(3)?.submittedAt || '',
    samples: [],
  },
  {
    id: 4,
    title: 'quiz',
    sourceFileName: 'quiz',
    totalScore: 100,
    taskType: 'choice',
    submittedAt: '',
    samples: [],
  },
  {
    id: 5,
    title: 'answer',
    sourceFileName: 'answer',
    totalScore: 100,
    taskType: 'answers_only',
    submittedAt: submissions.get(5)?.submittedAt || '',
    samples: [],
  },
]

const MESSAGES: LemonMessage[] = [
  {
    id: 1,
    target: '@all',
    text: '比赛已开始 10 分钟，请仔细核对题目与准考证号。如有疑问请举手示意监考老师。',
    time: iso(BOOT - 30 * MIN),
  },
  {
    id: 2,
    target: 'HA-J00001',
    text: 'HA-J00001 选手：你在 bracket 题的提交已收到，评测以最后一次提交为准。',
    time: iso(BOOT - 8 * MIN),
  },
]

const NOTICE_TEXT = [
  '一、比赛期间只能访问本提交系统，不得打开其他网页或通讯工具。',
  '二、同一题可多次提交，评测以最后一次为准。',
  '三、在线编辑模式下代码会自动保存草稿；上传文件模式请确认文件名与题目源文件名一致。',
  '四、比赛结束前 5 分钟倒计时将变红，请注意留出提交时间。',
  '五、如遇系统异常，立即举手联系监考老师，不要自行刷新重试。',
].join('\n')

const SCORE_FIXTURE: LemonScoreTask[] = [
  { id: 0, title: 'bracket', score: 100, totalScore: 100, judged: true },
  { id: 1, title: 'color', score: 40, totalScore: 100, judged: true },
  { id: 2, title: 'part', score: 0, totalScore: 100, judged: true, compileFailed: true },
  { id: 3, title: 'rank', score: 100, totalScore: 100, judged: true },
  { id: 4, title: 'quiz', score: 80, totalScore: 100, judged: true },
  { id: 5, title: 'answer', score: 0, totalScore: 100, judged: true, placementError: true },
]

const delay = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms))

// ---- 整包上传的服务端校验规则（与 LemonNext SubmissionServer 一致） ----

const WINDOWS_RESERVED = new Set([
  'CON', 'PRN', 'AUX', 'NUL',
  'COM1', 'COM2', 'COM3', 'COM4', 'COM5', 'COM6', 'COM7', 'COM8', 'COM9',
  'LPT1', 'LPT2', 'LPT3', 'LPT4', 'LPT5', 'LPT6', 'LPT7', 'LPT8', 'LPT9',
])

function validateUploadPath(path: string): string | null {
  if (!path) return '空路径'
  if (path.includes('\\')) return '路径分隔符不合法'
  if (path.startsWith('/')) return '不允许绝对路径'
  const segments = path.split('/')
  if (segments.some((segment) => segment === '.' || segment === '..')) return '不允许相对路径片段'
  if (segments.some((segment) => WINDOWS_RESERVED.has(segment.toUpperCase()))) {
    return 'Windows 保留名'
  }
  if (segments.length > 8) return '目录层级超过 8 层'
  return null
}

export const mockLemonApi: LemonApi = {
  async tasks() {
    await delay(240)
    const response: LemonTasksResponse = {
      contestTitle: 'CSP-J 第二轮 全真模拟',
      user: 'HA-J00001',
      displayName: 'HA-J00001',
      examFolder: 'HA-J00001',
      windowEnabled: true,
      startTime: iso(WINDOW.start),
      endTime: iso(WINDOW.end),
      serverNow: new Date().toISOString(),
      preContest: WINDOW.preContest,
      hasStatement: true,
      statements: [{ kind: 'global', name: '题面.txt', url: STATEMENT_URL }],
      submitMode: 'both',
      scoresReleased: WINDOW.scoresReleased,
      latestMessageId: MESSAGES.length,
      tasks: TASKS.map((task) => ({
        ...task,
        submittedAt: submissions.get(task.id)?.submittedAt || '',
      })),
    }
    return response
  },

  async notice() {
    await delay(120)
    return { text: NOTICE_TEXT }
  },

  async messages() {
    await delay(120)
    return { messages: MESSAGES, latestId: MESSAGES.length }
  },

  async scores() {
    await delay(300)
    if (!WINDOW.scoresReleased) return { released: false, totalScore: 0, totalPossible: 0, tasks: [] }
    const total = SCORE_FIXTURE.reduce((sum, task) => sum + task.score, 0)
    const possible = SCORE_FIXTURE.reduce((sum, task) => sum + task.totalScore, 0)
    return { released: true, totalScore: total, totalPossible: possible, tasks: SCORE_FIXTURE }
  },

  async source(taskId) {
    await delay(180)
    const record = submissions.get(taskId)
    if (!record) throw new Error('该题尚未提交')
    const source: LemonSource = {
      filename: record.filename,
      submittedAt: record.submittedAt,
      bytes: new TextEncoder().encode(record.content).length,
      content: record.content,
    }
    return source
  },

  async choice(taskId) {
    await delay(220)
    if (TASKS[taskId]?.taskType !== 'choice') throw new Error('该题不是选择题')
    return CHOICE_META
  },

  async submit(taskId, body) {
    await delay(360)
    const task = TASKS[taskId]
    if (!task) throw new Error('题目不存在')
    if (WINDOW.preContest) throw new Error('比赛尚未开始')
    const now = new Date().toISOString()
    const extension = body.language === 'python' ? 'py' : body.language === 'pascal' ? 'pas' : body.language
    submissions.set(taskId, {
      filename: `${task.sourceFileName}.${extension}`,
      submittedAt: now,
      content: body.source,
    })
    return { submittedAt: now }
  },

  async uploadFolder(contestant, files) {
    await delay(500)
    if (files.length > 512) throw new Error('文件数超过 512')
    const bytes = files.reduce((sum, file) => sum + file.size, 0)
    if (bytes > 8 * 1024 * 1024) throw new Error('总体积超过 8 MB')
    const result: LemonUploadFolderResult = {
      contestant,
      written: 0,
      total: files.length,
      bytes,
      willJudge: true,
      files: files.map((file) => {
        const error = validateUploadPath(file.path)
        if (error) {
          return { path: file.path, size: file.size, ok: false, message: error }
        }
        const target = `source/${contestant}/${file.path.split('/').slice(1).join('/')}`
        return { path: file.path, target, size: file.size, ok: true }
      }),
    }
    result.written = result.files.filter((file) => file.ok).length
    return result
  },
}
