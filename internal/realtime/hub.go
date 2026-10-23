package realtime

import (
	"sync"

	"github.com/yviscool/forge/internal/domain"
)

type subscription struct {
	ch        chan domain.Event
	contestID string
}

// Hub SSE 事件中心：非阻塞广播 + 按 contestId 过滤。
type Hub struct {
	mu   sync.RWMutex
	subs map[chan domain.Event]subscription
}

func NewHub() *Hub {
	return &Hub{subs: map[chan domain.Event]subscription{}}
}

func (h *Hub) Publish(e domain.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch, sub := range h.subs {
		if sub.contestID != "" && e.ContestID != "" && sub.contestID != e.ContestID {
			continue
		}
		select {
		case ch <- e:
		default:
			// A slow browser must not cause an undetectable gap in ranking or
			// submission state. Evict the oldest queued event and enqueue an
			// explicit resynchronization marker. The frontend handles this by
			// refetching authoritative REST state.
			select {
			case <-ch:
			default:
			}
			marker := domain.Event{
				Type: "stream.overflow", ContestID: e.ContestID,
				Data: map[string]any{"resync": true}, At: e.At,
			}
			select {
			case ch <- marker:
			default:
			}
		}
	}
}

func (h *Hub) Subscribe(contestID string) (<-chan domain.Event, func()) {
	ch := make(chan domain.Event, 32)
	h.mu.Lock()
	h.subs[ch] = subscription{ch: ch, contestID: contestID}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		close(ch)
		h.mu.Unlock()
	}
}

// SubscriberCount reports the active SSE stream count without exposing the
// subscription map.
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}

// Broadcaster 适配 ports.Broadcaster 接口（结构鸭子类型由 app 层直接使用）。
func (h *Hub) Broadcast(e domain.Event) { h.Publish(e) }
