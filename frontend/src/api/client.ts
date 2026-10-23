import { useStore } from '../store/useStore'

const PREFIX = '/api/v1'
export const REQUEST_TIMEOUT_MS = 30_000
export const DOWNLOAD_TIMEOUT_MS = 2 * 60_000

export type Role = 'admin' | 'teacher' | 'student'
export type ContestStatus = 'draft' | 'running' | 'finished'
export type RankingMode = 'oi' | 'acm'
export type TaskType = 'traditional' | 'answers_only' | 'interaction' | 'choice'
export type CompareMode =
  | 'ignore_space'
  | 'exact'
  | 'float_eps'
  | 'token'
  | 'custom_checker'
  | 'line_by_line'
  | 'real_number'
  | 'special_judge'

export type ContestMode = 'online' | 'noi' | 'gesp'

export interface Contest {
  id: string
  name: string
  description: string
  status: ContestStatus
  rankingMode?: RankingMode
  /** 比赛组织方式：online 在线提交 / noi NOI 全真模拟 / gesp GESP 机考。 */
  mode?: ContestMode
  allowedLanguages?: string[]
  problemIds?: string[]
  groupIds?: string[]
  participantUserIds?: string[]
  createdAt?: string
  startedAt?: string
  finishedAt?: string
  bindIpEnabled?: boolean
  boundIps?: Record<string, string>
  windowEnabled?: boolean
  startTime?: string
  endTime?: string
  durationMinutes?: number
  extensions?: Record<string, number>
  notice?: string
  broadcasts?: BroadcastMessage[]
  zipProtected?: boolean
  zipUnlocked?: boolean
}

export interface ChoiceQuestion {
  id: number
  type: 'single' | 'multiple' | 'judge'
  stem: string
  options?: string[]
  // Student paper responses redact the answer key to null.
  answer?: string | string[] | null
  score: number
  explanation?: string
}

export interface TestCase {
  id: string
  inputFile: string
  outputFile: string
  score: number
  subtask: number
  timeLimitMs?: number
  memoryMib?: number
  dependsOn?: number[]
}

export interface Problem {
  id: string
  contestId: string
  code: string
  title: string
  statement: string
  input: string
  output: string
  examples: string
  constraints: string
  locale?: string
  timeLimitMs: number
  memoryLimitMib: number
  taskType?: TaskType
  ioMode?: 'stdio' | 'file'
  inFile?: string
  outFile?: string
  compareMode?: CompareMode
  realEps?: number
  outputLimitKib?: number
  compileTimeoutSec?: number
  checkerLang?: string
  checkerCode?: string
  interactorLang?: string
  interactorCode?: string
  compilerFlags?: string[]
  locales?: Record<string, Partial<Problem>>
  testCases?: TestCase[]
  choiceQuestions?: ChoiceQuestion[]
  choiceKeyJson?: string
  updatedAt?: string
}

export interface User {
  id: string
  username: string
  name: string
  role: Role
  province?: string
  examTicket?: string
  groups?: string[]
  createdAt?: string
}

export interface ProvinceView {
  code: string
  name: string
  sort: number
  enabled: boolean
  users: number
  hasUsers: boolean
}

export interface CodeVersion {
  id: string
  contestId: string
  problemId: string
  userId: string
  seq: number
  language: string
  code: string
  source: 'save' | 'submit' | string
  createdAt: string
}

export interface CSPTicketAssignment {
  user: User
  ticket: string
}

export interface CSPTicketRequest {
  province: string
  group: 'J' | 'S'
  /** 序号补零位宽，缺省 5（HA-J00001）。 */
  numberLength?: number
  /** 与 userIds 二选一：批量创建考生账号数量。 */
  createCount?: number
  /** 与 createCount 二选一：给既有账号发号。 */
  userIds?: string[]
  /** 仅对新建账号生效的统一初始密码。 */
  password?: string
}

export interface Group {
  id: string
  name: string
  description?: string
  userIds?: string[]
  createdAt?: string
}

export interface CaseResult {
  caseIndex: number
  verdict: string
  score: number
  timeMs?: number
  memoryKib?: number
  cpuMs?: number
  message?: string
}

export interface Submission {
  id: string
  contestId: string
  problemID: string
  userId: string
  userName?: string
  language: string
  code: string
  verdict: string
  score: number
  compileMessage?: string
  cases?: CaseResult[]
  submittedAt: string
  judgedAt?: string
}

export interface RankEntry {
  userId: string
  userName: string
  score: number
  accepted: number
  problemScores?: Record<string, number>
}

export interface ACMRankEntry {
  userId: string
  userName: string
  solved: number
  penaltyMin: number
}

export interface ProblemStats {
  problemId: string
  code: string
  title: string
  submissions: number
  accepted: number
  bestScore: number
  acceptRate: number
  firstACUser?: string
}

export interface ContestStatistics {
  contestId: string
  users: number
  submissions: number
  problems: ProblemStats[]
}

export interface BroadcastMessage {
  id: string
  contestId: string
  senderName: string
  targetUserId?: string
  title: string
  content: string
  level: 'info' | 'warning' | 'urgent'
  createdAt: string
}

export interface TestFileInventory {
  files: string[]
  missingPairs: string[]
  answerOnly?: boolean
}

export class ApiError extends Error {
  readonly status: number
  readonly code?: string

  constructor(message: string, status: number, code?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

function withQuery(path: string, query?: Record<string, string | undefined>): string {
  if (!query) return path
  const search = new URLSearchParams()
  Object.entries(query).forEach(([key, value]) => {
    if (value != null && value !== '') search.set(key, value)
  })
  const suffix = search.toString()
  return suffix ? `${path}?${suffix}` : path
}

async function parseError(response: Response): Promise<ApiError> {
  let message = `请求失败 (HTTP ${response.status})`
  let code: string | undefined
  try {
    const body = (await response.json()) as {
      message?: string
      error?: string
      code?: string
    }
    message = body.message || body.error || message
    code = body.code
  } catch {
    // Non-JSON responses still produce a useful status-based message.
  }
  return new ApiError(message, response.status, code)
}

function handleUnauthorized(status: number) {
  if (status !== 401 || typeof window === 'undefined') return
  useStore.getState().logout()
  window.dispatchEvent(new CustomEvent('forge:unauthorized'))
}

export function newIdempotencyKey(): string {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID()
  }
  const bytes = new Uint8Array(16)
  globalThis.crypto.getRandomValues(bytes)
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
}

async function request<T>(path: string, init: RequestInit = {}, json = true): Promise<T> {
  const headers = new Headers(init.headers)
  if (json && init.body != null && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const timed = requestSignal(init.signal, REQUEST_TIMEOUT_MS)
  let response: Response
  try {
    response = await fetch(`${PREFIX}${path}`, {
      ...init,
      headers,
      credentials: 'same-origin',
      signal: timed.signal,
    })
  } catch (cause) {
    throw normalizeFetchError(cause, timed.timedOut)
  } finally {
    timed.cleanup()
  }
  if (!response.ok) {
    handleUnauthorized(response.status)
    throw await parseError(response)
  }
  if (response.status === 204) return undefined as T
  return (await response.json()) as T
}

async function requestBlob(path: string): Promise<{ blob: Blob; filename?: string }> {
  const timed = requestSignal(undefined, DOWNLOAD_TIMEOUT_MS)
  let response: Response
  try {
    response = await fetch(`${PREFIX}${path}`, {
      credentials: 'same-origin',
      signal: timed.signal,
    })
  } catch (cause) {
    throw normalizeFetchError(cause, timed.timedOut)
  } finally {
    timed.cleanup()
  }
  if (!response.ok) {
    handleUnauthorized(response.status)
    throw await parseError(response)
  }

  const disposition = response.headers.get('Content-Disposition') || ''
  const utf8Name = disposition.match(/filename\*=UTF-8''([^;]+)/i)?.[1]
  const plainName = disposition.match(/filename="?([^";]+)"?/i)?.[1]
  return {
    blob: await response.blob(),
    filename: utf8Name ? decodeURIComponent(utf8Name) : plainName,
  }
}

function requestSignal(signal: AbortSignal | null | undefined, timeoutMs: number) {
  const controller = new AbortController()
  let timedOut = false
  const timeout = window.setTimeout(() => {
    timedOut = true
    controller.abort()
  }, timeoutMs)
  const forwardAbort = () => controller.abort(signal?.reason)
  if (signal?.aborted) {
    forwardAbort()
  } else {
    signal?.addEventListener('abort', forwardAbort, { once: true })
  }
  return {
    signal: controller.signal,
    get timedOut() {
      return timedOut
    },
    cleanup: () => {
      window.clearTimeout(timeout)
      signal?.removeEventListener('abort', forwardAbort)
    },
  }
}

function normalizeFetchError(cause: unknown, timedOut: boolean): unknown {
  if (timedOut && cause instanceof DOMException && cause.name === 'AbortError') {
    return new ApiError('请求超时，请检查网络后重试', 0, 'timeout')
  }
  return cause
}

function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.rel = 'noopener'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}

async function triggerProtectedDownload(path: string, fallbackFilename: string) {
  const { blob, filename } = await requestBlob(path)
  triggerDownload(blob, filename || fallbackFilename)
}

const encoded = (value: string) => encodeURIComponent(value)

export const api = {
  time: () => request<{ serverTime: string }>('/time'),

  contests: () => request<Contest[]>('/contests'),
  getContest: (cid: string) => request<Contest>(`/contests/${encoded(cid)}`),
  createContest: (
    name: string,
    description: string,
    rankingMode: RankingMode = 'oi',
    allowedLanguages: string[] = [],
    mode: ContestMode = 'online'
  ) =>
    request<Contest>('/contests', {
      method: 'POST',
      body: JSON.stringify({ name, description, rankingMode, allowedLanguages, mode }),
    }),
  startContest: (cid: string) =>
    request<Contest>(`/contests/${encoded(cid)}/start`, { method: 'POST' }),
  finishContest: (cid: string) =>
    request<Contest>(`/contests/${encoded(cid)}/finish`, { method: 'POST' }),
  updateContestSettings: (
    cid: string,
    settings: { rankingMode?: RankingMode; allowedLanguages?: string[] }
  ) =>
    request<Contest>(`/contests/${encoded(cid)}/settings`, {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),

  getIpBindings: (cid: string) =>
    request<{ bindIpEnabled: boolean; boundIps: Record<string, string> }>(
      `/contests/${encoded(cid)}/ip-bindings`
    ),
  setIpBindingEnabled: (cid: string, enabled: boolean) =>
    request<Contest>(`/contests/${encoded(cid)}/ip-bindings`, {
      method: 'PUT',
      body: JSON.stringify({ enabled }),
    }),
  unbindUserIp: (cid: string, uid: string) =>
    request<{ ok: boolean }>(
      `/contests/${encoded(cid)}/participants/${encoded(uid)}/unbind-ip`,
      { method: 'POST' }
    ),
  unbindAllIps: (cid: string) =>
    request<{ ok: boolean }>(`/contests/${encoded(cid)}/unbind-all-ips`, {
      method: 'POST',
    }),

  setContestWindow: (
    cid: string,
    window: {
      windowEnabled: boolean
      startTime?: string
      endTime?: string
      durationMinutes?: number
    }
  ) =>
    request<Contest>(`/contests/${encoded(cid)}/window`, {
      method: 'PUT',
      body: JSON.stringify(window),
    }),
  setUserExtension: (cid: string, userId: string, minutes: number) =>
    request<Contest>(`/contests/${encoded(cid)}/extensions`, {
      method: 'POST',
      body: JSON.stringify({ userId, minutes }),
    }),
  removeUserExtension: (cid: string, userId: string) =>
    request<Contest>(`/contests/${encoded(cid)}/extensions/${encoded(userId)}`, {
      method: 'DELETE',
    }),

  updateNotice: (cid: string, notice: string) =>
    request<Contest>(`/contests/${encoded(cid)}/notice`, {
      method: 'PUT',
      body: JSON.stringify({ notice }),
    }),
  messages: (cid: string) =>
    request<BroadcastMessage[]>(`/contests/${encoded(cid)}/messages`),
  sendMessage: (
    cid: string,
    msg: {
      title: string
      content: string
      targetUserId?: string
      level?: BroadcastMessage['level']
    }
  ) =>
    request<BroadcastMessage>(`/contests/${encoded(cid)}/messages`, {
      method: 'POST',
      body: JSON.stringify(msg),
    }),

  setZipPassword: (cid: string, password: string) =>
    request<Contest>(`/contests/${encoded(cid)}/zip-password`, {
      method: 'PUT',
      body: JSON.stringify({ password }),
    }),
  unlockZip: (cid: string, password: string) =>
    request<{ unlocked: boolean; expiresAt?: string }>(`/contests/${encoded(cid)}/unlock-zip`, {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),

  problems: (cid: string) => request<Problem[]>(`/contests/${encoded(cid)}/problems`),
  getProblem: (cid: string, pid: string) =>
    request<Problem>(`/contests/${encoded(cid)}/problems/${encoded(pid)}`),
  createProblem: (cid: string, problem: Partial<Problem>) =>
    request<Problem>(`/contests/${encoded(cid)}/problems`, {
      method: 'POST',
      body: JSON.stringify(problem),
    }),
  updateProblem: (cid: string, pid: string, problem: Partial<Problem>) =>
    request<Problem>(`/contests/${encoded(cid)}/problems/${encoded(pid)}`, {
      method: 'PUT',
      body: JSON.stringify(problem),
    }),
  reorderProblems: (cid: string, problemIds: string[]) =>
    request<Problem[]>(`/contests/${encoded(cid)}/problems/reorder`, {
      method: 'PUT',
      body: JSON.stringify({ problemIds }),
    }),
  validateProblem: (cid: string, pid: string) =>
    request<{ valid: boolean; error?: string }>(
      `/contests/${encoded(cid)}/problems/${encoded(pid)}/validate`,
      { method: 'POST' }
    ),
  rejudgeProblem: (cid: string, pid: string) =>
    request<{ total: number; enqueued: number }>(
      `/contests/${encoded(cid)}/problems/${encoded(pid)}/rejudge`,
      { method: 'POST' }
    ),
  configureSubtasks: (
    cid: string,
    pid: string,
    subtasks: Array<{
      subtask: number
      score: number
      timeLimitMs?: number
      memoryMib?: number
      dependsOn?: number[]
    }>
  ) =>
    request<Problem>(`/contests/${encoded(cid)}/problems/${encoded(pid)}/subtasks`, {
      method: 'PUT',
      body: JSON.stringify({ subtasks }),
    }),
  syncTestCases: (cid: string, pid: string) =>
    request<Problem>(
      `/contests/${encoded(cid)}/problems/${encoded(pid)}/files/sync`,
      { method: 'POST' }
    ),
  deleteProblem: (cid: string, pid: string) =>
    request<{ ok: boolean }>(
      `/contests/${encoded(cid)}/problems/${encoded(pid)}`,
      { method: 'DELETE' }
    ),

  testFiles: (cid: string, pid: string) =>
    request<TestFileInventory>(
      `/contests/${encoded(cid)}/problems/${encoded(pid)}/files`
    ),
  uploadTestFile: (cid: string, pid: string, name: string, content: Blob) =>
    request<{ ok: boolean; size: number }>(
      `/contests/${encoded(cid)}/problems/${encoded(pid)}/files/${encoded(name)}`,
      { method: 'PUT', body: content },
      false
    ),
  deleteTestFile: (cid: string, pid: string, name: string) =>
    request<{ ok: boolean }>(
      `/contests/${encoded(cid)}/problems/${encoded(pid)}/files/${encoded(name)}`,
      { method: 'DELETE' }
    ),
  fetchTestFile: (cid: string, pid: string, name: string) =>
    requestBlob(`/contests/${encoded(cid)}/problems/${encoded(pid)}/files/${encoded(name)}`),

  users: () => request<User[]>('/users'),
  createUser: (name: string, username = '', role: Role = 'student', password = '', province = '') =>
    request<User>('/users', {
      method: 'POST',
      body: JSON.stringify({ name, username, role, password, province }),
    }),
  updateUser: (
    uid: string,
    payload: { name: string; username: string; role?: string; province?: string }
  ) =>
    request<User>(`/users/${encoded(uid)}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),
  batchSetProvince: (userIds: string[], province: string) =>
    request<{ updated: number }>('/users/batch-province', {
      method: 'POST',
      body: JSON.stringify({ userIds, province }),
    }),
  generateCSPTickets: (req: CSPTicketRequest) =>
    request<{ assignments: CSPTicketAssignment[] }>('/users/csp-tickets', {
      method: 'POST',
      body: JSON.stringify(req),
    }),
  provinces: () => request<ProvinceView[]>('/provinces'),
  saveCodeVersion: (
    cid: string,
    pid: string,
    code: string,
    language: string,
    source: 'save' | 'submit' = 'save'
  ) =>
    request<CodeVersion>(`/contests/${encoded(cid)}/problems/${encoded(pid)}/versions`, {
      method: 'POST',
      body: JSON.stringify({ code, language, source }),
    }),
  codeVersions: (cid: string, pid: string, userId?: string) =>
    request<CodeVersion[]>(
      `/contests/${encoded(cid)}/problems/${encoded(pid)}/versions${
        userId ? `?userId=${encoded(userId)}` : ''
      }`
    ),
  setProvinceEnabled: (code: string, enabled: boolean) =>
    request<ProvinceView>(`/provinces/${encoded(code)}`, {
      method: 'PUT',
      body: JSON.stringify({ enabled }),
    }),
  resetUserPassword: (uid: string, password: string) =>
    request<{ ok: boolean }>(`/users/${encoded(uid)}/password`, {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),
  importUsersCsv: (csv: string) =>
    request<{ created: number; errors: string[] }>('/users/import', {
      method: 'POST',
      body: JSON.stringify({ csv }),
    }),

  groups: () => request<Group[]>('/groups'),
  createGroup: (name: string) =>
    request<Group>('/groups', { method: 'POST', body: JSON.stringify({ name }) }),
  addGroupMember: (gid: string, userId: string) =>
    request<{ ok: boolean }>(`/groups/${encoded(gid)}/members`, {
      method: 'POST',
      body: JSON.stringify({ userId }),
    }),
  removeGroupMember: (gid: string, userId: string) =>
    request<{ ok: boolean }>(`/groups/${encoded(gid)}/members`, {
      method: 'DELETE',
      body: JSON.stringify({ userId }),
    }),
  bindGroupToContest: (cid: string, groupId: string) =>
    request<{ ok: boolean }>(`/contests/${encoded(cid)}/groups`, {
      method: 'POST',
      body: JSON.stringify({ groupId }),
    }),
  bindParticipantToContest: (cid: string, userId: string) =>
    request<{ ok: boolean }>(`/contests/${encoded(cid)}/participants`, {
      method: 'POST',
      body: JSON.stringify({ userId }),
    }),

  submissions: (cid: string, limit = 300) =>
    request<Submission[]>(
      withQuery(`/contests/${encoded(cid)}/submissions`, { limit: String(limit) })
    ),
  submit: (
    cid: string,
    body: { problemID: string; language: string; code: string },
    idempotencyKey = newIdempotencyKey()
  ) =>
    request<Submission>(`/contests/${encoded(cid)}/submissions`, {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify(body),
    }),
  judge: (id: string, verdict: string, score: number) =>
    request<Submission>(`/submissions/${encoded(id)}/judge`, {
      method: 'POST',
      body: JSON.stringify({ verdict, score }),
    }),
  judgeCases: (id: string, cases: CaseResult[], subtasks: Record<number, number> = {}) =>
    request<Submission>(`/submissions/${encoded(id)}/cases`, {
      method: 'POST',
      body: JSON.stringify({ cases, subtasks }),
    }),

  ranking: (cid: string) =>
    request<RankEntry[] | ACMRankEntry[]>(`/contests/${encoded(cid)}/ranking`),
  statistics: (cid: string) =>
    request<ContestStatistics>(`/contests/${encoded(cid)}/statistics`),

  login: (login: string, password = '') =>
    request<{
      userId: string
      role: Role
      username: string
      name: string
    }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ login, password }),
    }),
  logout: () => request<{ ok: boolean }>('/auth/logout', { method: 'POST' }),
  me: () => request<User>('/auth/me'),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<{ ok: boolean }>('/auth/password', {
      method: 'POST',
      body: JSON.stringify({ oldPassword, newPassword }),
    }),

  eventsTicket: (cid: string) =>
    request<{ expiresAt: string }>(
      `/events/ticket?contestId=${encoded(cid)}`,
      { method: 'POST' }
    ),

  importContestBundle: (bundle: unknown) =>
    request<Contest>('/contests/import', {
      method: 'POST',
      body: JSON.stringify(bundle),
    }),

  download: async (path: string, fallbackFilename: string) => {
    await triggerProtectedDownload(path, fallbackFilename)
  },
  downloadUsersCsv: () => triggerProtectedDownload('/users/export', 'forge-users.csv'),
  downloadStatisticsCsv: (cid: string) =>
    triggerProtectedDownload(
      `/contests/${encoded(cid)}/statistics/export`,
      `statistics-${cid}.csv`
    ),
  downloadContestPdf: (cid: string) =>
    triggerProtectedDownload(`/contests/${encoded(cid)}/pdf`, `contest-${cid}.pdf`),
  downloadContestHtml: (cid: string) =>
    triggerProtectedDownload(`/contests/${encoded(cid)}/export`, `contest-${cid}.html`),
  downloadProblemPdf: (cid: string, pid: string, code: string) =>
    triggerProtectedDownload(
      `/contests/${encoded(cid)}/problems/${encoded(pid)}/pdf`,
      `${code}.pdf`
    ),
  downloadProblemHtml: (cid: string, pid: string, code: string) =>
    triggerProtectedDownload(
      `/contests/${encoded(cid)}/problems/${encoded(pid)}/export`,
      `${code}.html`
    ),
  downloadBundle: (cid: string) =>
    triggerProtectedDownload(`/contests/${encoded(cid)}/bundle`, `bundle-${cid}.json`),
}

export function eventStreamUrl(contestId?: string | null): string {
  return withQuery(`${PREFIX}/events`, {
    contestId: contestId || undefined,
  })
}
