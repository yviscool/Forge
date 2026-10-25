// 文件提交模式（Lemon）——数据契约。
//
// 形状对齐 LemonNext 在线提交服务（upstream/LemonNext-Windows/src/server）：
// 前端按这些类型开发，后端接入时把 mockApi 换成真实 fetch 即可，视图不改。

/** LemonNext 任务类型：traditional 传统编程 / answers_only 只交答案 / choice 选择题。 */
export type LemonTaskType = 'traditional' | 'answers_only' | 'choice'

/** 教师设定的答题界面：editor 仅在线编辑 / file 仅上传文件 / both 学生自选。 */
export type LemonSubmitMode = 'editor' | 'file' | 'both'

export interface LemonSampleFile {
  name: string
  size?: number
  url: string
}

export interface LemonTask {
  id: number
  title: string
  /** 源文件基名（如 bracket），提交文件应命名为 bracket.cpp。 */
  sourceFileName: string
  totalScore: number
  timeLimitMs?: number
  timeLimitMaxMs?: number
  taskType: LemonTaskType
  /** 最近一次提交时间（ISO）；空串表示未提交。 */
  submittedAt: string
  samples: LemonSampleFile[]
}

export interface LemonStatementFile {
  kind: 'global' | string
  name: string
  url: string
}

export interface LemonTasksResponse {
  contestTitle: string
  user: string
  displayName: string
  /** 整包上传时的顶层目录名（准考证号）。 */
  examFolder: string
  windowEnabled: boolean
  startTime?: string
  endTime?: string
  /** 服务端时钟（ISO），前端用它冻结本地与服务端的时差。 */
  serverNow: string
  /** true = 开考前锁定态：题目列表不展示，到点自动刷新出现。 */
  preContest: boolean
  hasStatement: boolean
  statements: LemonStatementFile[]
  submitMode: LemonSubmitMode
  scoresReleased: boolean
  latestMessageId: number
  tasks: LemonTask[]
}

export interface LemonScoreTask {
  id: number
  title: string
  score: number
  totalScore: number
  judged: boolean
  compileFailed?: boolean
  placementError?: boolean
}

export interface LemonScores {
  released: boolean
  totalScore: number
  totalPossible: number
  tasks: LemonScoreTask[]
}

export interface LemonSource {
  filename: string
  submittedAt: string
  bytes: number
  content: string
}

export interface LemonMessage {
  id: number
  /** '@all' = 全体广播，其余为准考证号定向消息。 */
  target: string
  text: string
  time: string
}

export type LemonChoiceQuestionType = 'single' | 'multiple' | 'truefalse' | 'fill'

export interface LemonChoiceQuestion {
  id: number
  type: LemonChoiceQuestionType
  score: number
  /** 无 paper.md 选项文本时按该数量生成 A-N 空选项。 */
  optionCount?: number
}

export interface LemonChoiceMeta {
  paperMarkdown: string
  answerFileExt: string
  questions: LemonChoiceQuestion[]
}

export interface LemonSubmitBody {
  source: string
  language: string
}

export interface LemonSubmitResult {
  submittedAt: string
}

export interface LemonUploadFileInput {
  path: string
  size: number
}

export interface LemonUploadFileResult {
  path: string
  target?: string
  size: number
  ok: boolean
  message?: string
}

export interface LemonUploadFolderResult {
  contestant: string
  written: number
  total: number
  bytes: number
  /** true = 服务端已自动触发整包评测。 */
  willJudge: boolean
  files: LemonUploadFileResult[]
}

/** 文件提交模式的 API 面。后端接入时实现同一接口即可无缝替换 mock。 */
export interface LemonApi {
  tasks: () => Promise<LemonTasksResponse>
  notice: () => Promise<{ text: string }>
  messages: () => Promise<{ messages: LemonMessage[]; latestId: number }>
  scores: () => Promise<LemonScores>
  source: (taskId: number) => Promise<LemonSource>
  choice: (taskId: number) => Promise<LemonChoiceMeta>
  submit: (taskId: number, body: LemonSubmitBody) => Promise<LemonSubmitResult>
  uploadFolder: (
    contestant: string,
    files: LemonUploadFileInput[]
  ) => Promise<LemonUploadFolderResult>
}
