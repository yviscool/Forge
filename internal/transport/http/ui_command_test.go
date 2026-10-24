package httpapi

import (
	"testing"
	"time"
)

// 托盘面板在教师尚未登录时也要能把主窗唤到前面去登录：本机回环来的
// show-main 免会话，其余动作（以及任何局域网来源）仍要求教师会话。
func TestUICommandLoopbackShowMainWithoutSession(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	seen := make(chan string, 8)
	e.srv.SetUICommander(func(action string) { seen <- action })

	// httptest 的客户端来自 127.0.0.1 → 回环。
	if code, body := post(t, e.url+"/api/v1/ui/command", `{"action":"show-main"}`); code != 200 {
		t.Fatalf("loopback show-main without session should 200, got %d %v", code, body)
	}
	select {
	case action := <-seen:
		if action != "show-main" {
			t.Fatalf("dispatched %q, want show-main", action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("show-main was not dispatched to the shell")
	}

	// 免会话只给 show-main，别的动作照旧 401。
	if code, _ := post(t, e.url+"/api/v1/ui/command", `{"action":"open-bigscreen"}`); code != 401 {
		t.Fatalf("anonymous open-bigscreen should 401, got %d", code)
	}
	// 已登录教师四个动作都能用。
	if code, body := postAuth(t, e.url+"/api/v1/ui/command", e.teacher, `{"action":"open-student"}`); code != 200 {
		t.Fatalf("teacher open-student should 200, got %d %v", code, body)
	}
	// 未知动作仍然是 400（不能靠免会话绕过白名单）。
	if code, _ := postAuth(t, e.url+"/api/v1/ui/command", e.teacher, `{"action":"rm -rf"}`); code != 400 {
		t.Fatalf("unknown action should 400, got %d", code)
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	for _, addr := range []string{"127.0.0.1", "127.0.0.5", "::1", "[::1]"} {
		if !isLoopbackAddr(addr) {
			t.Fatalf("%q should be loopback", addr)
		}
	}
	for _, addr := range []string{"", "10.1.2.3", "192.168.1.9", "::ffff:10.1.2.3"} {
		if isLoopbackAddr(addr) {
			t.Fatalf("%q must not be treated as loopback", addr)
		}
	}
}
