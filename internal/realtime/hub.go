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
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch, sub := range h.subs {
		if sub.contestID != "" && e.ContestID != "" && sub.contestID != e.ContestID {
			continue
		}
		select {
		case ch <- e:
		default:
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

// Broadcaster 适配 ports.Broadcaster 接口（结构鸭子类型由 app 层直接使用）。
func (h *Hub) Broadcast(e domain.Event) { h.Publish(e) }
