package main

// 一次性验证台：只托管 /tray 面板（真正的 desktop.JoinPageHandler），
// 并用 stub 顶替 /ui/command（模拟 Wails 外壳的 200 应答）。
// 用途：在真浏览器里确认面板点「打开教师台」不再弹新窗口，而是发指令。
import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/yviscool/forge/internal/platform/desktop"
)

func main() {
	addr := "127.0.0.1:18099"
	var mu sync.Mutex
	var actions []string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ui/command":
			var v struct {
				Action string `json:"action"`
			}
			_ = json.NewDecoder(r.Body).Decode(&v)
			mu.Lock()
			actions = append(actions, v.Action)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/__actions":
			mu.Lock()
			defer mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(actions)
		case "/api/v1/tray-summary":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"online":3,"participants":3,"submissions":9,"queued":0,"conflicts":0,` +
				`"generatedAt":"2026-10-24T08:00:00Z",` +
				`"contest":{"name":"CSP-J 模拟认证","mode":"online","status":"running","windowEndsAt":"2026-10-24T10:00:00Z"},` +
				`"recent":[{"userName":"演示考生1","problemCode":"A","verdict":"accepted","submittedAt":"2026-10-24T07:59:00Z"}]}`))
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, "<h1>stub %s</h1>", r.URL.Path)
		}
	})

	log.Fatal(http.ListenAndServe(addr, desktop.JoinPageHandler(next, addr)))
}
