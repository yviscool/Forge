import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useSSE } from '../realtime/useSSE'
import { apiRoutes, jsonOk, server } from './msw'

type EventListenerCallback = (event: Event) => void

class FakeEventSource {
  static instances: FakeEventSource[] = []

  readonly url: string
  readonly readyState = 0
  readonly withCredentials = false
  readonly CONNECTING = 0
  readonly OPEN = 1
  readonly CLOSED = 2
  onerror: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent<string>) => void) | null = null
  onopen: ((event: Event) => void) | null = null
  closed = false

  private readonly listeners = new Map<string, Set<EventListenerCallback>>()

  constructor(url: string | URL) {
    this.url = String(url)
    FakeEventSource.instances.push(this)
  }

  addEventListener(type: string, callback: EventListenerCallback | null): void {
    if (!callback) return
    const listeners = this.listeners.get(type) ?? new Set<EventListenerCallback>()
    listeners.add(callback)
    this.listeners.set(type, listeners)
  }

  removeEventListener(type: string, callback: EventListenerCallback | null): void {
    if (!callback) return
    this.listeners.get(type)?.delete(callback)
  }

  close(): void {
    this.closed = true
  }

  emitNamed(type: string, data: unknown): void {
    const event = new MessageEvent(type, {
      data: typeof data === 'string' ? data : JSON.stringify(data),
    })
    this.listeners.get(type)?.forEach((listener) => listener(event))
  }

  emitMessage(data: unknown): void {
    const event = new MessageEvent('message', {
      data: typeof data === 'string' ? data : JSON.stringify(data),
    })
    this.onmessage?.(event)
  }

  listenerCount(type: string): number {
    return this.listeners.get(type)?.size ?? 0
  }
}

describe('useSSE', () => {
  const businessEvents = (onEvent: ReturnType<typeof vi.fn>) =>
    onEvent.mock.calls.filter(([type]) => !String(type).startsWith('stream.'))

  beforeEach(() => {
    FakeEventSource.instances = []
    vi.stubGlobal('EventSource', FakeEventSource)
    server.use(
      apiRoutes.post('/events/ticket', () =>
        jsonOk({ expiresAt: '2026-10-21T09:00:00Z' })
      )
    )
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('subscribes to named events and forwards the parsed payload', async () => {
    const onEvent = vi.fn()
    renderHook(() => useSSE('contest-1', onEvent))

    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    const source = FakeEventSource.instances[0]
    expect(source).toBeDefined()
    expect(source.url).toBe('/api/v1/events?contestId=contest-1')
    expect(source.listenerCount('submission.judged')).toBe(1)

    act(() => {
      source.emitNamed('submission.judged', {
        type: 'submission.judged',
        contestId: 'contest-1',
        data: { submissionId: 'submission-9', verdict: 'Accepted' },
        at: '2026-10-21T08:00:00Z',
      })
    })

    expect(businessEvents(onEvent)).toHaveLength(1)
    expect(onEvent).toHaveBeenCalledWith(
      'submission.judged',
      { submissionId: 'submission-9', verdict: 'Accepted' },
      expect.objectContaining({
        type: 'submission.judged',
        contestId: 'contest-1',
        at: '2026-10-21T08:00:00Z',
      })
    )
  })

  it('reports connection lifecycle events separately from business payloads', async () => {
    const onEvent = vi.fn()
    renderHook(() => useSSE('contest-1', onEvent))

    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    expect(onEvent).toHaveBeenCalledWith(
      'stream.connecting',
      null,
      expect.objectContaining({ type: 'stream.connecting', contestId: 'contest-1' })
    )

    act(() => {
      FakeEventSource.instances[0].onopen?.(new Event('open'))
    })

    expect(onEvent).toHaveBeenCalledWith(
      'stream.connected',
      null,
      expect.objectContaining({ type: 'stream.connected', contestId: 'contest-1' })
    )
    expect(businessEvents(onEvent)).toHaveLength(0)
  })

  it('forwards server overflow markers so views can resynchronize', async () => {
    const onEvent = vi.fn()
    renderHook(() => useSSE('contest-1', onEvent))
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    const source = FakeEventSource.instances[0]

    act(() => {
      source.emitNamed('stream.overflow', {
        type: 'stream.overflow',
        contestId: 'contest-1',
        data: { resync: true },
      })
    })

    expect(onEvent).toHaveBeenCalledWith(
      'stream.overflow',
      { resync: true },
      expect.objectContaining({ type: 'stream.overflow', contestId: 'contest-1' })
    )
  })

  it('deduplicates the same frame delivered through named and default channels', async () => {
    const onEvent = vi.fn()
    renderHook(() => useSSE('contest-1', onEvent))
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    const source = FakeEventSource.instances[0]
    const payload = {
      type: 'ranking.updated',
      contestId: 'contest-1',
      data: { revision: 7 },
    }

    act(() => {
      source.emitNamed('ranking.updated', payload)
      source.emitMessage(payload)
      source.emitNamed('ranking.updated', payload)
    })

    expect(businessEvents(onEvent)).toHaveLength(1)
  })

  it('ignores foreign-contest and malformed frames, then closes the stream', async () => {
    const onEvent = vi.fn()
    const { unmount } = renderHook(() => useSSE('contest-1', onEvent))
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    const source = FakeEventSource.instances[0]

    act(() => {
      source.emitNamed('contest.notice', {
        type: 'contest.notice',
        contestId: 'contest-2',
        data: { notice: 'wrong contest' },
      })
      source.emitNamed('contest.notice', '{not-json')
    })

    expect(businessEvents(onEvent)).toHaveLength(0)

    unmount()
    expect(source.closed).toBe(true)
    expect(source.listenerCount('submission.judged')).toBe(0)
  })

  it('reports authorization failures without opening a stream', async () => {
    server.use(
      apiRoutes.post('/events/ticket', () =>
        jsonOk({ message: 'login required' }, { status: 401 })
      )
    )
    const onEvent = vi.fn()
    renderHook(() => useSSE('contest-1', onEvent))

    await waitFor(() =>
      expect(onEvent).toHaveBeenCalledWith(
        'stream.unauthorized',
        null,
        expect.objectContaining({ type: 'stream.unauthorized', contestId: 'contest-1' })
      )
    )
    expect(FakeEventSource.instances).toHaveLength(0)
  })

  it('renews the ticket and reconnects after a server restart', async () => {
    let ticketCalls = 0
    server.use(
      apiRoutes.post('/events/ticket', () => {
        ticketCalls += 1
        return jsonOk({ expiresAt: '2026-10-21T20:00:00Z' })
      })
    )
    const onEvent = vi.fn()
    renderHook(() => useSSE('contest-1', onEvent))

    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    const first = FakeEventSource.instances[0]
    act(() => {
      first.onerror?.(new Event('error'))
    })

    expect(first.closed).toBe(true)
    expect(FakeEventSource.instances).toHaveLength(1)

    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(2), {
      timeout: 2000,
    })
    expect(ticketCalls).toBe(2)
    expect(FakeEventSource.instances[1].url).toBe('/api/v1/events?contestId=contest-1')
  })

  it('keeps retrying transient ticket failures instead of silently going offline', async () => {
    let ticketCalls = 0
    server.use(
      apiRoutes.post('/events/ticket', () => {
        ticketCalls += 1
        if (ticketCalls === 1) {
          return jsonOk({ message: 'bad gateway' }, { status: 502 })
        }
        return jsonOk({ expiresAt: '2026-10-21T20:00:00Z' })
      })
    )
    const onEvent = vi.fn()
    renderHook(() => useSSE('contest-1', onEvent))

    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1), {
      timeout: 2500,
    })
    expect(ticketCalls).toBe(2)
    expect(onEvent).not.toHaveBeenCalledWith(
      'stream.unauthorized',
      null,
      expect.anything()
    )
  })
})
