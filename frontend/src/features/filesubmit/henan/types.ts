// 文件提交模式（河南版）——数据契约。
//
// 形状对齐河南本地收卷系统（ZYZOJ 离线代码收集系统）的学生端 API：
//   GET  /api/user/heartbeat  心跳：phase/banned/confirmed/公告/时间窗/服务器时间
//   GET  /api/user/problems   题目列表（含每题提交记录）
//   POST /api/user/submit     multipart(problem, file)，严格文件名校验
//   POST /api/user/confirm    最终确认（冻结，此后不能再提交）
//   GET  /api/user/score      成绩（发布后带各题得分/测试点/排名）
//   GET  /api/user/files      比赛附件
// 后端接入时把 mockApi 换成同接口的 fetch 实现即可，视图不改。

export type HenanPhase = 'waiting' | 'before' | 'active' | 'ended' | 'publish'

export interface HenanHeartbeat {
  /** 公告文本（空串表示无）。 */
  announcement: string
  /** 账号封禁：只能查看，不能提交/确认。 */
  banned: boolean
  /** 已最终确认：所有题目锁定。 */
  confirmed: boolean
  contestName: string
  /** unix 秒。 */
  startTime: number
  endTime: number
  phase: HenanPhase
  realName: string
  username: string
  serverTimeMs: number
}

export interface HenanSubmitRecord {
  submitCount: number
  filename: string
  submittedAt: string
  /** 最近一次提交的源码（查看代码用）。 */
  code: string
}

export interface HenanProblem {
  name: string
  displayName: string
  sort: number
  /** 有提交时才有。 */
  record?: HenanSubmitRecord
}

export interface HenanCaseResult {
  index: number
  score: number
  /** verdict 文本，如 AC / WA / TLE。 */
  verdict: string
  timeMs?: number
  memoryKib?: number
}

export interface HenanProblemScore {
  score: number
  totalScore: number
  submitCount: number
  /** localJudge=false 时为空：成绩由管理员导入，无本地评测详情。 */
  cases?: HenanCaseResult[]
}

export interface HenanScore {
  confirmed: boolean
  localJudge: boolean
  rank: number
  resultsPublished: boolean
  /** 键为题目英文名。 */
  score: Record<string, HenanProblemScore>
}

export interface HenanAttachment {
  name: string
  size: number
  url: string
}

export interface HenanSubmitResult {
  status: 'success' | 'error'
  message?: string
}

/** 与河南系统一致的错误消息（前端展示原文）。 */
export interface HenanApi {
  heartbeat: () => Promise<HenanHeartbeat>
  problems: () => Promise<HenanProblem[]>
  submit: (problem: string, file: { name: string; size: number; content: string }) => Promise<HenanSubmitResult>
  confirm: () => Promise<HenanSubmitResult>
  score: () => Promise<HenanScore>
  files: () => Promise<HenanAttachment[]>
  changePassword: (current: string, next: string) => Promise<HenanSubmitResult>
}
