import { onUnmounted } from 'vue'

// 按 contestId 过滤的 SSE 订阅 hook。
export function useSSE(contestId: () => string | null, onEvent: (type: string) => void) {
  const es = new EventSource('/api/v1/events')
  es.onmessage = (e) => {
    try {
      const evt = JSON.parse(e.data)
      const cid = contestId()
      if (cid && evt.contestId && evt.contestId !== cid) return
      onEvent(evt.type)
    } catch {}
  }
  onUnmounted(() => es.close())
}
