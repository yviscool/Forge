package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRootPathRedirectsPreserveRoute 历史入口必须映射回 SPA 并保留路径：
// /teacher 丢路径会让学生大厅顶替教师台（桌面主窗曾因此打开错误视图）。
func TestRootPathRedirectsPreserveRoute(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	cases := []struct{ in, want string }{
		{"/teacher", "/app/teacher"},
		{"/teacher?tab=monitor", "/app/teacher?tab=monitor"},
		{"/login", "/app/login"},
		{"/", "/app"},
		{"/anything-else", "/app"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.in, nil)
		rec := httptest.NewRecorder()
		e.srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Errorf("GET %s status = %d, want 302", c.in, rec.Code)
			continue
		}
		if got := rec.Header().Get("Location"); got != c.want {
			t.Errorf("GET %s redirect = %q, want %q", c.in, got, c.want)
		}
	}
}
