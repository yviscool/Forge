import { useEffect, useRef } from 'react'
import { api, eventStreamUrl } from '../api/client'

export const KNOWN_EVENT_TYPES = [
  'contest.created',
  'contest.started',
  'contest.updated',
  'contest.notice',
  'contest.window_updated',
  'contest.extension_updated',
  'contest.broadcast',
  'contest.ip_bound',
  'contest.ip_unbound',
  'contest.ip_unbound_all',
  'contest.finished',
  'problem.created',
  'problem.updated',
  'problem.deleted',
  'problems.reordered',
  'problem.rejudge',
  'submission.created',
  'submission.judged',
  'ranking.updated',
  'stream.overflow',
] as const

export interface StreamEvent<T = unknown> {
  type: string
  contestId?: string
  data: T
  at?: string
}

export type StreamHandler = (type: string, data: unknown, event: StreamEvent) => void

/**
 * Subscribes to the Forge SSE stream.
 *
 * The server emits named SSE events (`event: submission.judged`), so
 * EventSource.onmessage alone never sees them. We register listeners for the
 * known event names and keep `onmessage` as a fallback for unnamed events.
 */
export function useSSE(
  contestId: string | null | undefined,
  onEvent: StreamHandler,
  options: { enabled?: boolean } = {}
) {
  const handlerRef = useRef(onEvent)
  const lastEventRef = useRef<string>('')

  useEffect(() => {
    handlerRef.current = onEvent
  }, [onEvent])

  const enabled = options.enabled ?? true

  useEffect(() => {
    if (!enabled || typeof window === 'undefined' || typeof EventSource === 'undefined') {
      return
    }

    let closed = false
    let source: EventSource | null = null
    let retryTimer: number | null = null
    let retryAttempt = 0
    let generation = 0
    let openedAt = 0

    const clearRetryTimer = () => {
      if (retryTimer !== null) {
        window.clearTimeout(retryTimer)
        retryTimer = null
      }
    }

    const dispatch = (event: MessageEvent<string>, fallbackType?: string) => {
      if (closed || !event.data) return
      try {
        const parsed = JSON.parse(event.data) as StreamEvent
        const type = parsed.type || fallbackType || 'message'
        const scopedContest = parsed.contestId
        if (contestId && scopedContest && scopedContest !== contestId) return

        // A named event may also arrive through the default message channel;
        // de-duplicate identical payloads so callbacks run once per event.
        const fingerprint = `${type}:${event.data}`
        if (fingerprint === lastEventRef.current) return
        lastEventRef.current = fingerprint

        handlerRef.current(type, parsed.data, { ...parsed, type })
      } catch {
        // Ignore malformed frames; the stream stays alive for later events.
      }
    }

    let listeners: Array<readonly [string, (event: Event) => void]> = []
    const closeSource = () => {
      if (!source) return
      listeners.forEach(([type, listener]) => source!.removeEventListener(type, listener))
      source.onmessage = null
      source.onerror = null
      source.onopen = null
      source.close()
      source = null
      listeners = []
    }

    const connect = () => {
      if (closed || !contestId) return
      const currentGeneration = ++generation
      openedAt = 0
      handlerRef.current('stream.connecting', null, {
        type: 'stream.connecting',
        contestId: contestId || undefined,
        data: null,
      })
      source = new EventSource(eventStreamUrl(contestId))
      listeners = KNOWN_EVENT_TYPES.map((type) => {
        const listener = (event: Event) => dispatch(event as MessageEvent<string>, type)
        source!.addEventListener(type, listener)
        return [type, listener] as const
      })
      source.onmessage = (event) => dispatch(event)
      source.onopen = () => {
        openedAt = Date.now()
        handlerRef.current('stream.connected', null, {
          type: 'stream.connected',
          contestId: contestId || undefined,
          data: null,
        })
      }
      source.onerror = () => {
        if (closed || currentGeneration !== generation) return
        // EventSource retries the same URL forever, but the short-lived
        // stream ticket may have expired or the server may have restarted.
        // Close it and exchange the bearer session for a fresh ticket before
        // opening the stream again.
        closeSource()
        handlerRef.current('stream.offline', null, {
          type: 'stream.offline',
          contestId: contestId || undefined,
          data: null,
        })
        if (openedAt > 0 && Date.now() - openedAt >= 15_000) {
          retryAttempt = 0
        }
        const delay = Math.min(30_000, 500 * 2 ** retryAttempt)
        retryAttempt += 1
        retryTimer = window.setTimeout(() => {
          retryTimer = null
          void openWithFreshTicket()
        }, delay)
      }
    }

    // Exchange the bearer session for an HttpOnly stream ticket first.
    // EventSource cannot attach Authorization headers; keeping the long-lived
    // token in the URL would leak it into proxy and server access logs.
    const openWithFreshTicket = async () => {
      if (closed || !contestId) return
      try {
        await api.eventsTicket(contestId)
        connect()
      } catch (error) {
        if (closed) return
        const status = (error as { status?: number }).status
        if (status === 401 || status === 403) {
          handlerRef.current('stream.unauthorized', null, {
            type: 'stream.unauthorized',
            contestId: contestId || undefined,
            data: null,
          })
          return
        }
        // A transient network or server failure should not permanently kill
        // live updates during an exam. Keep retrying with bounded backoff.
        const delay = Math.min(30_000, 500 * 2 ** retryAttempt)
        retryAttempt += 1
        retryTimer = window.setTimeout(() => {
          retryTimer = null
          void openWithFreshTicket()
        }, delay)
      }
    }

    if (contestId) {
      void openWithFreshTicket()
    }

    return () => {
      closed = true
      clearRetryTimer()
      closeSource()
    }
  }, [contestId, enabled])
}
