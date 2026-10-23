import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, REQUEST_TIMEOUT_MS, api } from '../api/client'
import { useStore } from '../store/useStore'
import { apiRoutes, HttpResponse, jsonError, server } from './msw'

const session = {
  userId: 'user-1',
  username: 'alice',
  name: 'Alice',
  role: 'student',
}

describe('api client', () => {
  beforeEach(() => {
    localStorage.clear()
    useStore.setState({
      session: null,
      authStatus: 'idle',
      serverTimeOffset: 0,
      unreadBroadcasts: 0,
      activeAlert: null,
    })
  })

  it('uses the same-origin HttpOnly cookie and JSON content type', async () => {
    useStore.getState().setSession(session)

    server.use(
      apiRoutes.put('/contests/contest-1/settings', async ({ request }) => {
        expect(request.headers.get('authorization')).toBeNull()
        expect(request.credentials).toBe('same-origin')
        expect(request.headers.get('content-type')).toBe('application/json')
        expect(await request.json()).toEqual({ rankingMode: 'acm' })
        return HttpResponse.json({
          id: 'contest-1',
          name: 'Forge Contest',
          description: '',
          status: 'running',
          rankingMode: 'acm',
        })
      })
    )

    const response = await api.updateContestSettings('contest-1', {
      rankingMode: 'acm',
    })

    expect(response.rankingMode).toBe('acm')
  })

  it('authenticates calls without exposing a bearer token to JavaScript', async () => {
    useStore.getState().setSession(session)
    let authorization: string | null = null
    let credentials: RequestCredentials | null = null

    server.use(
      apiRoutes.get('/auth/me', ({ request }) => {
        authorization = request.headers.get('authorization')
        credentials = request.credentials
        return HttpResponse.json({
          id: 'user-1',
          username: 'alice',
          name: 'Alice',
          role: 'student',
        })
      })
    )

    await expect(api.me()).resolves.toMatchObject({ username: 'alice' })
    expect(authorization).toBeNull()
    expect(credentials).toBe('same-origin')
  })

  it('sends an idempotency key when creating a submission', async () => {
    useStore.getState().setSession(session)
    let requestKey = ''

    server.use(
      apiRoutes.post('/contests/contest-1/submissions', ({ request }) => {
        requestKey = request.headers.get('idempotency-key') ?? ''
        return HttpResponse.json({
          id: 'submission-1',
          contestId: 'contest-1',
          problemID: 'problem-1',
          userId: 'user-1',
          language: 'cpp',
          code: 'int main() {}',
          verdict: 'queued',
          score: 0,
          submittedAt: '2026-10-22T00:00:00Z',
        })
      })
    )

    await api.submit('contest-1', {
      problemID: 'problem-1',
      language: 'cpp',
      code: 'int main() {}',
    })

    expect(requestKey).toMatch(/^[a-f0-9-]{32,}$/)
  })

  it('returns a structured ApiError and clears an invalid session on 401', async () => {
    useStore.getState().setSession(session)
    server.use(
      apiRoutes.get('/auth/me', () =>
        jsonError('Session expired', 401, 'session_expired')
      )
    )
    const unauthorized = vi.fn()
    window.addEventListener('forge:unauthorized', unauthorized)

    try {
      await expect(api.me()).rejects.toMatchObject({
        name: 'ApiError',
        message: 'Session expired',
        status: 401,
        code: 'session_expired',
      })
    } finally {
      window.removeEventListener('forge:unauthorized', unauthorized)
    }

    expect(useStore.getState().session).toBeNull()
    expect(useStore.getState().authStatus).toBe('anonymous')
    expect(localStorage.getItem('forge_session')).toBeNull()
    expect(unauthorized).toHaveBeenCalledTimes(1)
  })

  it('preserves the session and surfaces JSON errors for non-401 responses', async () => {
    useStore.getState().setSession(session)
    server.use(
      apiRoutes.get('/auth/me', () => jsonError('Forbidden', 403, 'forbidden'))
    )

    const error = await api.me().catch((reason: unknown) => reason)

    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ message: 'Forbidden', status: 403, code: 'forbidden' })
    expect(useStore.getState().session?.username).toBe('alice')
    expect(useStore.getState().authStatus).toBe('authenticated')
  })

  it('downloads an authenticated blob with its server filename', async () => {
    useStore.getState().setSession(session)
    let requestedCredentials: RequestCredentials | null = null
    let downloadedName = ''

    server.use(
      apiRoutes.get('/contests/contest-1/pdf', async ({ request }) => {
        requestedCredentials = request.credentials
        return new HttpResponse('pdf-bytes', {
          headers: {
            'Content-Type': 'application/pdf',
            'Content-Disposition':
              "attachment; filename*=UTF-8''contest-final%20v2.pdf",
          },
        })
      })
    )

    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(function (this: HTMLAnchorElement) {
        downloadedName = this.download
      })
    vi.mocked(URL.createObjectURL).mockReturnValue('blob:forge-download')

    await api.downloadContestPdf('contest-1')

    expect(requestedCredentials).toBe('same-origin')
    expect(click).toHaveBeenCalledTimes(1)
    expect(downloadedName).toBe('contest-final v2.pdf')
    expect(URL.createObjectURL).toHaveBeenCalledTimes(1)
    const blob = vi.mocked(URL.createObjectURL).mock.calls[0][0] as Blob
    expect(await blob.text()).toBe('pdf-bytes')
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:forge-download')
  })

  it('aborts and reports a retryable error when a request exceeds its deadline', async () => {
    vi.useFakeTimers()
    const originalFetch = globalThis.fetch
    vi.stubGlobal(
      'fetch',
      vi.fn(
        (_input: RequestInfo | URL, init?: RequestInit) =>
          new Promise<Response>((_resolve, reject) => {
            init?.signal?.addEventListener('abort', () => {
              reject(new DOMException('aborted', 'AbortError'))
            })
          })
      )
    )
    try {
      const pending = api.time()
      const rejection = expect(pending).rejects.toMatchObject({
        name: 'ApiError',
        status: 0,
        code: 'timeout',
      })
      await vi.advanceTimersByTimeAsync(REQUEST_TIMEOUT_MS + 1)
      await rejection
    } finally {
      vi.unstubAllGlobals()
      globalThis.fetch = originalFetch
      vi.useRealTimers()
    }
  })
})
