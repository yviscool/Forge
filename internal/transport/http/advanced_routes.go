package httpapi

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/domain"
)

// clientIP 提取客户端物理机位 IP。
//
// 只有当服务被明确配置为位于可信反向代理之后（FORGE_TRUST_PROXY=1）时才
// 采用代理头；否则一律使用 TCP 对端地址。机位绑定是考务反作弊手段，若默认
// 信任 X-Forwarded-For，考生只需伪造该请求头即可冒充已绑定机位。
func (h *Server) clientIP(r *http.Request) string {
	if h.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			// Read from the trusted edge inward. The right-most address is the
			// one appended by the proxy closest to Forge; taking the left-most
			// would let a client inject a fake address before the proxy appends
			// the real one.
			for i := len(parts) - 1; i >= 0; i-- {
				if ip := canonicalIP(parts[i]); ip != "" {
					return ip
				}
			}
		}
		if xri := canonicalIP(r.Header.Get("X-Real-IP")); xri != "" {
			return xri
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		if ip := canonicalIP(host); ip != "" {
			return ip
		}
	}
	if ip := canonicalIP(r.RemoteAddr); ip != "" {
		return ip
	}
	return strings.TrimSpace(r.RemoteAddr)
}

// isLoopbackAddr 判断 clientIP 的规范化结果是否为本机回环地址
// （托盘面板等本机窗口用它换取「登录前也能唤出主窗」的例外）。
func isLoopbackAddr(addr string) bool {
	ip := net.ParseIP(strings.Trim(addr, "[]"))
	return ip != nil && ip.IsLoopback()
}

func canonicalIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	raw = strings.Trim(raw, "[]")
	ip := net.ParseIP(raw)
	if ip == nil {
		return ""
	}
	return ip.String()
}

// advancedContestRoutes 处理机位绑定、时间窗口/加时、考场公告广播与 Zip 解锁。
func (h *Server) advancedContestRoutes(w http.ResponseWriter, r *http.Request, cid string, rest []string) bool {
	if len(rest) == 0 {
		return false
	}

	// /contests/{cid}/ip-bindings
	if len(rest) == 1 && rest[0] == "ip-bindings" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return true
		}
		switch r.Method {
		case "GET":
			bindings, enabled, err := h.svc.GetBoundIPs(cid)
			if err != nil {
				h.err(w, r, 404, err)
				return true
			}
			writeOut(w, map[string]any{"bindIpEnabled": enabled, "boundIps": bindings}, 200)
			return true
		case "PUT":
			var v struct {
				Enabled bool `json:"enabled"`
			}
			if decode(r, &v) != nil {
				h.err(w, r, 400, fmt.Errorf("invalid json"))
				return true
			}
			c, err := h.svc.SetIPBindingEnabled(cid, v.Enabled)
			if err != nil {
				h.err(w, r, 400, err)
				return true
			}
			writeOut(w, c, 200)
			return true
		default:
			h.err(w, r, 405, fmt.Errorf("method not allowed"))
			return true
		}
	}

	// /contests/{cid}/participants/{uid}/unbind-ip
	if len(rest) == 3 && rest[0] == "participants" && rest[2] == "unbind-ip" && r.Method == "POST" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return true
		}
		uid := rest[1]
		if err := h.svc.UnbindUserIP(cid, uid); err != nil {
			h.err(w, r, 400, err)
			return true
		}
		writeOut(w, map[string]bool{"ok": true}, 200)
		return true
	}

	// /contests/{cid}/unbind-all-ips
	if len(rest) == 1 && rest[0] == "unbind-all-ips" && r.Method == "POST" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return true
		}
		if err := h.svc.UnbindAllIPs(cid); err != nil {
			h.err(w, r, 400, err)
			return true
		}
		writeOut(w, map[string]bool{"ok": true}, 200)
		return true
	}

	// /contests/{cid}/window
	if len(rest) == 1 && rest[0] == "window" && r.Method == "PUT" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return true
		}
		var v struct {
			WindowEnabled   bool       `json:"windowEnabled"`
			StartTime       *time.Time `json:"startTime"`
			EndTime         *time.Time `json:"endTime"`
			DurationMinutes int        `json:"durationMinutes"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return true
		}
		c, err := h.svc.SetContestWindow(cid, v.WindowEnabled, v.StartTime, v.EndTime, v.DurationMinutes)
		if err != nil {
			h.err(w, r, 400, err)
			return true
		}
		writeOut(w, c, 200)
		return true
	}

	// /contests/{cid}/extensions
	if len(rest) == 1 && rest[0] == "extensions" && r.Method == "POST" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return true
		}
		var v struct {
			UserID  string `json:"userId"`
			Minutes int    `json:"minutes"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return true
		}
		c, err := h.svc.SetUserExtension(cid, v.UserID, v.Minutes)
		if err != nil {
			h.err(w, r, 400, err)
			return true
		}
		writeOut(w, c, 200)
		return true
	}

	// /contests/{cid}/extensions/{uid}
	if len(rest) == 2 && rest[0] == "extensions" && r.Method == "DELETE" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return true
		}
		uid := rest[1]
		c, err := h.svc.RemoveUserExtension(cid, uid)
		if err != nil {
			h.err(w, r, 400, err)
			return true
		}
		writeOut(w, c, 200)
		return true
	}

	// /contests/{cid}/notice
	if len(rest) == 1 && rest[0] == "notice" && r.Method == "PUT" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return true
		}
		var v struct {
			Notice string `json:"notice"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return true
		}
		c, err := h.svc.UpdateNotice(cid, v.Notice)
		if err != nil {
			h.err(w, r, 400, err)
			return true
		}
		writeOut(w, c, 200)
		return true
	}

	// /contests/{cid}/messages
	if len(rest) == 1 && rest[0] == "messages" {
		switch r.Method {
		case "GET":
			r2, _, ok := h.requireContestAccess(w, r, cid)
			if !ok {
				return true
			}
			uid := ""
			if sess, ok := sessionOf(r2); ok {
				uid = sess.UserID
			}
			msgs, err := h.svc.ListBroadcastMessages(cid, uid)
			if err != nil {
				h.err(w, r, 400, err)
				return true
			}
			writeOut(w, msgs, 200)
			return true
		case "POST":
			r2, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin)
			if !ok {
				return true
			}
			var v struct {
				Title        string `json:"title"`
				Content      string `json:"content"`
				TargetUserID string `json:"targetUserId"`
				Level        string `json:"level"`
			}
			if decode(r, &v) != nil {
				h.err(w, r, 400, fmt.Errorf("invalid json"))
				return true
			}
			senderName := "教师"
			if sess, ok := sessionOf(r2); ok {
				if u, err := h.svc.Store().GetUser(sess.UserID); err == nil && u.Name != "" {
					senderName = u.Name
				}
			}
			msg, err := h.svc.SendBroadcastMessage(cid, senderName, v.TargetUserID, v.Title, v.Content, v.Level)
			if err != nil {
				h.err(w, r, 400, err)
				return true
			}
			writeOut(w, msg, 201)
			return true
		default:
			h.err(w, r, 405, fmt.Errorf("method not allowed"))
			return true
		}
	}

	// /contests/{cid}/zip-password
	if len(rest) == 1 && rest[0] == "zip-password" && r.Method == "PUT" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return true
		}
		var v struct {
			Password string `json:"password"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return true
		}
		c, err := h.svc.SetZipPassword(cid, v.Password)
		if err != nil {
			h.err(w, r, 400, err)
			return true
		}
		writeOut(w, c, 200)
		return true
	}

	// /contests/{cid}/unlock-zip
	if len(rest) == 1 && rest[0] == "unlock-zip" && r.Method == "POST" {
		r2, c, ok := h.requireContestAccess(w, r, cid)
		if !ok {
			return true
		}
		sess, ok := sessionOf(r2)
		if !ok {
			WriteError(w, r, http.StatusUnauthorized, "unauthorized", "login required")
			return true
		}
		ip := h.clientIP(r)
		unlockKey := "zip:" + cid + ":" + sess.UserID
		if h.ExamUnlockLimiter != nil {
			allowed, retryAfter := h.ExamUnlockLimiter.Allow(ip, unlockKey)
			if !allowed {
				seconds := int(retryAfter / time.Second)
				if seconds < 1 {
					seconds = 1
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
				WriteError(w, r, http.StatusTooManyRequests, "unlock_rate_limited",
					"too many failed exam password attempts; try again later")
				return true
			}
		}
		var v struct {
			Password string `json:"password"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return true
		}
		valid, err := h.svc.VerifyZipPassword(cid, v.Password)
		if err != nil {
			if errors.Is(err, auth.ErrZipVerifyBusy) {
				w.Header().Set("Retry-After", "1")
				WriteError(w, r, http.StatusServiceUnavailable, "unlock_verifier_busy",
					"exam password verification is temporarily busy; retry shortly")
				return true
			}
			h.err(w, r, http.StatusInternalServerError, err)
			return true
		}
		if !valid {
			if h.ExamUnlockLimiter != nil {
				h.ExamUnlockLimiter.Fail(ip, unlockKey)
			}
			h.err(w, r, 403, fmt.Errorf("密码错误，解密失败"))
			return true
		}
		if h.ExamUnlockLimiter != nil {
			h.ExamUnlockLimiter.Reset(unlockKey)
		}
		expiresAt, err := h.issueZipGrant(w, r2, c)
		if err != nil {
			h.err(w, r, 500, err)
			return true
		}
		writeOut(w, map[string]any{"unlocked": true, "expiresAt": expiresAt}, 200)
		return true
	}

	return false
}
