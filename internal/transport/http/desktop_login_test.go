package httpapi

import (
	"fmt"
	"testing"
)

// TestDesktopLoginTokenExchange 桌面壳自动登录链路：
// 铸造一次性令牌 → 交换会话 Cookie → /auth/me 通过 → 令牌不可重放。
func TestDesktopLoginTokenExchange(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	// 交换端点绑定 username=admin 的管理员账号。
	admin, err := e.svc.CreateUserWithUsername("Admin", "admin", "admin")
	if err != nil {
		t.Fatal(err)
	}
	token, err := e.srv.MintDesktopLoginToken()
	if err != nil {
		t.Fatal(err)
	}

	// 未登录状态基线。
	if code, _ := requestJSON(t, "GET", e.url+"/api/v1/auth/me", "", ""); code != 401 {
		t.Fatalf("anonymous /auth/me = %d, want 401", code)
	}

	// 用 Cookie 客户端交换（需要拿到 Set-Cookie）。
	client := newCookieClient(t)
	code, body := postWithClient(t, client, e.url+"/api/v1/auth/desktop-exchange", "",
		fmt.Sprintf(`{"token":%q}`, token))
	if code != 200 {
		t.Fatalf("exchange = %d %v", code, body)
	}
	if body["role"] != "admin" || body["userId"] != admin.ID {
		t.Fatalf("exchange identity = %v", body)
	}

	// 会话生效。
	code, raw := getWithClient(t, client, e.url+"/api/v1/auth/me", "")
	if code != 200 {
		t.Fatalf("post-exchange /auth/me = %d %s", code, raw)
	}

	// 令牌单次有效：重放必须失败。
	code, _ = postWithClient(t, newCookieClient(t), e.url+"/api/v1/auth/desktop-exchange", "",
		fmt.Sprintf(`{"token":%q}`, token))
	if code != 401 {
		t.Fatalf("replay = %d, want 401", code)
	}
	// 伪造令牌失败。
	if code, _ := postWithClient(t, newCookieClient(t), e.url+"/api/v1/auth/desktop-exchange", "",
		`{"token":"forged"}`); code != 401 {
		t.Fatalf("forged = %d, want 401", code)
	}
}
