package realtime

import (
	"sync"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

type subscription struct {
	ch        chan domain.Event
	contestID string
	userID    string
}

// Presence 单个 SSE 连接的在线快照：考生身份 + 连接建立时间。
type Presence struct {
	UserID    string    `json:"userId"`
	Since     time.Time `json:"since"`
	ContestID string    `json:"contestId"`
}

// Hub SSE 事件中心：非阻塞广播 + 按 contestId 过滤 + 考生在线注册表。
// 在线注册表服务监考大屏：SSE 连接建立即视为在线，断开即离线。
type Hub struct {
	mu       sync.RWMutex
	subs     map[chan domain.Event]subscription
	presence map[string]map[string]Presence // contestID -> userID -> Presence
}

func NewHub() *Hub {
	return &Hub{
		subs:     map[chan domain.Event]subscription{},
		presence: map[string]map[string]Presence{},
	}
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

// Subscribe 注册一个事件订阅；userID 非空时同步登记考生在线状态。
func (h *Hub) Subscribe(contestID, userID string) (<-chan domain.Event, func()) {
	ch := make(chan domain.Event, 32)
	h.mu.Lock()
	h.subs[ch] = subscription{ch: ch, contestID: contestID, userID: userID}
	if userID != "" && contestID != "" {
		if h.presence[contestID] == nil {
			h.presence[contestID] = map[string]Presence{}
		}
		h.presence[contestID][userID] = Presence{UserID: userID, ContestID: contestID, Since: time.Now()}
	}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		if userID != "" && contestID != "" {
			if bucket := h.presence[contestID]; bucket != nil {
				delete(bucket, userID)
				if len(bucket) == 0 {
					delete(h.presence, contestID)
				}
			}
		}
		close(ch)
		h.mu.Unlock()
	}
}

// PresenceMap 返回某场比赛的在线考生快照（userID → Presence 副本）。
func (h *Hub) PresenceMap(contestID string) map[string]Presence {
	h.mu.RLock()
	defer h.mu.RUnlock()
	bucket := h.presence[contestID]
	out := make(map[string]Presence, len(bucket))
	for uid, p := range bucket {
		out[uid] = p
	}
	return out
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
