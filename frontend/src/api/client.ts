// 统一 API 前缀：新前端只走 /api/v1，老 /api 由 Go 网关兼容。
const PREFIX = '/api/v1'

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(`${PREFIX}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!r.ok) {
    const body = await r.json().catch(() => ({}))
    throw new Error((body as any).message || `request failed: ${r.status}`)
  }
  return r.json() as Promise<T>
}

export interface Contest {
  id: string
  name: string
  description: string
  status: string
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
  timeLimitMs: number
  memoryLimitMib: number
  locales?: Record<string, any>
}
export interface Submission {
  id: string
  userName: string
  language: string
  verdict: string
  score: number
}
export interface RankEntry {
  userName: string
  score: number
  accepted: number
}

export const api = {
  contests: () => req<Contest[]>('/contests'),
  createContest: (name: string, description: string) =>
    req<Contest>('/contests', { method: 'POST', body: JSON.stringify({ name, description }) }),
  startContest: (cid: string) => req(`/contests/${cid}/start`, { method: 'POST' }),
  finishContest: (cid: string) => req(`/contests/${cid}/finish`, { method: 'POST' }),
  problems: (cid: string) => req<Problem[]>(`/contests/${cid}/problems`),
  createProblem: (cid: string, p: Partial<Problem>) =>
    req<Problem>(`/contests/${cid}/problems`, { method: 'POST', body: JSON.stringify(p) }),
  validateProblem: (cid: string, pid: string) =>
    req<{ valid: boolean; error?: string }>(`/contests/${cid}/problems/${pid}/validate`, { method: 'POST' }),
  users: () => req<any[]>('/users'),
  createUser: (name: string, role: string) =>
    req('/users', { method: 'POST', body: JSON.stringify({ name, role }) }),
  groups: () => req<any[]>('/groups'),
  createGroup: (name: string) => req('/groups', { method: 'POST', body: JSON.stringify({ name }) }),
  submissions: (cid: string) => req<Submission[]>(`/contests/${cid}/submissions`),
  submit: (cid: string, body: any) =>
    req(`/contests/${cid}/submissions`, { method: 'POST', body: JSON.stringify(body) }),
  judge: (id: string, verdict: string, score: number) =>
    req(`/submissions/${id}/judge`, { method: 'POST', body: JSON.stringify({ verdict, score }) }),
  ranking: (cid: string) => req<RankEntry[]>(`/contests/${cid}/ranking`),
  exportUrl: (cid: string, pid: string) => `/api/v1/contests/${cid}/problems/${pid}/export`,
}
