package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/yviscool/forge/internal/domain"
)

const sessKey ctxKey = "session"

// bearer 从 Authorization 头提取 token。
func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func withSession(r *http.Request, s domain.Session) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), sessKey, s))
}

func sessionOf(r *http.Request) (domain.Session, bool) {
	s, ok := r.Context().Value(sessKey).(domain.Session)
	return s, ok
}

// currentSession 解析并校验 token（中间件式：失败返回 401 由调用方决定）。
func (h *Server) currentSession(r *http.Request) (domain.Session, bool) {
	tok := bearer(r)
	if tok == "" {
		return domain.Session{}, false
	}
	sess, err := h.auth.Authenticate(tok)
	if err != nil {
		return domain.Session{}, false
	}
	return sess, true
}

// requireLogin 登录门禁，同时把会话注入 context。
func (h *Server) requireLogin(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	sess, ok := h.currentSession(r)
	if !ok {
		WriteError(w, r, 401, "unauthorized", "login required")
		return r, false
	}
	return withSession(r, sess), true
}

// requireRole 角色门禁：teacher/admin 默认互通（传参限定）。
func (h *Server) requireRole(w http.ResponseWriter, r *http.Request, roles ...string) (*http.Request, bool) {
	r2, ok := h.requireLogin(w, r)
	if !ok {
		return r, false
	}
	sess, _ := sessionOf(r2)
	for _, role := range roles {
		if sess.Role == role {
			return r2, true
		}
	}
	WriteError(w, r, 403, "forbidden", "insufficient role")
	return r, false
}

// isTeacher 会话是否为教师侧（teacher/admin）。
func isTeacher(r *http.Request) bool {
	sess, ok := sessionOf(r)
	return ok && (sess.Role == domain.RoleTeacher || sess.Role == domain.RoleAdmin)
}
