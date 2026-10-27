// Canonical HTTP 服务器：新栈直驱（app.Service + pdf + realtime），只暴露 /api/v1。
// Legacy /api 与旧手写页已砍掉（项目未上线，不做兼容）。
package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/judge"
	"github.com/yviscool/forge/internal/observability"
	"github.com/yviscool/forge/internal/pdf"
	"github.com/yviscool/forge/internal/platform/desktop"
	"github.com/yviscool/forge/internal/ports"
	"github.com/yviscool/forge/internal/testdata"
	webdist "github.com/yviscool/forge/internal/web"
)

type Server struct {
	svc  *app.Service
	auth *auth.Service
	log  *slog.Logger
	// Readiness checks are wired by the executable so transport stays free of
	// host-specific filesystem and toolchain policy.
	readinessMu     sync.RWMutex
	readinessChecks map[string]func(context.Context) error
	presenceSource  func(cid string) map[string]time.Time
	shutdownHook    func()
	uiCommander     func(action string)

	desktopTokenMu  sync.Mutex
	desktopToken    string
	desktopTokenExp time.Time
	// LoginLimiter protects the public credential endpoint. Tests may replace
	// it with a limiter driven by a fake clock.
	LoginLimiter *auth.LoginLimiter
	// ExamUnlockLimiter protects the exam password endpoint from online
	// guessing. It is separate from login attempts so a noisy classroom NAT
	// cannot consume the credential-login budget.
	ExamUnlockLimiter *auth.LoginLimiter
	// TrustProxy 是否采用 X-Forwarded-For / X-Real-IP 作为客户端机位 IP。
	// 默认 false：仅在服务确实位于可信反向代理之后时才由 main 打开。
	TrustProxy bool
	// OnSubmit best-effort hands a durable queued submission to the worker
	// pool. A transient enqueue failure must not turn persisted work into a
	// terminal system error; the reconciler retries it after capacity frees up.
	OnSubmit func(domain.Submission) error
	// Files 测试数据文件仓（main 按 DataDir 装配，测试用临时目录）。
	Files *testdata.Store

	metrics *observability.Metrics
	handler http.Handler
}

// ServeHTTP 让 *Server 本身就是 http.Handler（钩子可在启动后挂载）。
func (h *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

// favicon 站点图标：原生窗口与浏览器标签共用，消除默认球形图标。
func (h *Server) favicon(w http.ResponseWriter, _ *http.Request) {
	ico, err := desktop.FaviconICO()
	if err != nil {
		http.Error(w, "icon unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(ico)
}

func NewServer(svc *app.Service, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	metrics := observability.NewMetrics()
	h := &Server{
		svc: svc, log: log, auth: auth.New(svc.Store(), nil),
		LoginLimiter:      auth.NewLoginLimiter(),
		ExamUnlockLimiter: auth.NewLoginLimiter(),
		readinessChecks:   make(map[string]func(context.Context) error),
		metrics:           metrics,
	}
	h.Files = testdata.New("./data/testdata")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", Healthz)
	mux.HandleFunc("/readyz", h.readyz)
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/favicon.ico", h.favicon)
	mux.HandleFunc("/api/v1/", h.dispatch)
	mux.HandleFunc("/app", h.spa)
	mux.HandleFunc("/app/", h.spa)
	mux.HandleFunc("/", h.root)
	h.handler = WithMiddleware(mux, log, func() bool { return h.TrustProxy }, metrics)
	return h
}

// SetPresenceSource 注入 SSE 在线注册表（监考大屏用）：比赛 → userID → 连接建立时间。
func (h *Server) SetPresenceSource(source func(cid string) map[string]time.Time) {
	h.presenceSource = source
}

// SetShutdownHook 注入优雅停机钩子（托盘面板的「退出服务」按钮调用）。
// 只允许注册一次；未注册时关机端点返回 503。
func (h *Server) SetShutdownHook(hook func()) {
	h.shutdownHook = hook
}

// MintDesktopLoginToken 铸造一次性桌面登录令牌（60 秒、单次有效）。
// 仅桌面壳在启动时调用；令牌经本机 webview URL 传递给 /auth/desktop-exchange
// 换取管理员会话——教师无需在本地应用里反复输密码。
func (h *Server) MintDesktopLoginToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	h.desktopTokenMu.Lock()
	h.desktopToken = token
	h.desktopTokenExp = time.Now().Add(60 * time.Second)
	h.desktopTokenMu.Unlock()
	return token, nil
}

// SetUICommander 注入桌面 UI 指令通道（仅 Wails 外壳注册）：
// 托盘面板 / 教师台的按钮可直接唤起对应原生窗口，不经过浏览器。
// 非 Wails 构建不注册，端点返回 503，前端静默降级。
func (h *Server) SetUICommander(commander func(action string)) {
	h.uiCommander = commander
}

// AddReadinessCheck registers a startup-injected operational dependency.
// Registering the same name replaces the previous probe.
func (h *Server) AddReadinessCheck(name string, check func(context.Context) error) {
	name = strings.TrimSpace(name)
	if name == "" || check == nil {
		return
	}
	h.readinessMu.Lock()
	h.readinessChecks[name] = check
	h.readinessMu.Unlock()
}

// SetRuntimeStatsProvider lets the executable expose judge and SSE pressure
// without coupling transport to concrete worker or hub implementations.
func (h *Server) SetRuntimeStatsProvider(provider func() observability.RuntimeStats) {
	h.metrics.SetRuntimeStatsProvider(provider)
}

func (h *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	checks := map[string]string{}
	if checker, ok := h.svc.Store().(ports.HealthChecker); ok {
		if err := runReadinessCheck(ctx, checker.Ping); err != nil {
			h.log.Error("readiness check failed", "check", "store", "err", err)
			checks["store"] = "unavailable"
		} else {
			checks["store"] = "ok"
		}
	} else {
		checks["store"] = "ok"
	}

	h.readinessMu.RLock()
	pending := make(map[string]func(context.Context) error, len(h.readinessChecks))
	for name, check := range h.readinessChecks {
		pending[name] = check
	}
	h.readinessMu.RUnlock()
	for name, check := range pending {
		if err := runReadinessCheck(ctx, check); err != nil {
			h.log.Error("readiness check failed", "check", name, "err", err)
			checks[name] = "unavailable"
			continue
		}
		checks[name] = "ok"
	}
	for _, status := range checks {
		if status != "ok" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "unavailable",
				"checks": checks,
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "checks": checks})
}

func runReadinessCheck(ctx context.Context, check func(context.Context) error) error {
	result := make(chan error, 1)
	go func() {
		result <- check(ctx)
	}()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Server) root(w http.ResponseWriter, r *http.Request) {
	// 单页应用即首页；历史入口路径（/teacher、/login）映射回 SPA 路由，
	// 保留路径与查询串——否则 /teacher 会被重定向到 /app 而丢失教师台视图
	// （托盘「监考大屏」深链 ?tab=monitor 也依赖查询串存活）。
	target := "/app"
	switch {
	case r.URL.Path == "/teacher" || strings.HasPrefix(r.URL.Path, "/teacher/"):
		target = "/app" + r.URL.Path
	case r.URL.Path == "/login":
		target = "/app/login"
	}
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func decode(r *http.Request, v any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	// Reject trailing JSON values so a request cannot smuggle a second object
	// past validation.
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}

func writeOut(w http.ResponseWriter, v any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func submissionFeedLimit(raw string) (int, error) {
	const (
		defaultLimit = 200
		maxLimit     = 500
	)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 {
		return 0, fmt.Errorf("limit must be a positive integer")
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return limit, nil
}

func (h *Server) err(w http.ResponseWriter, r *http.Request, code int, err error) {
	if code >= http.StatusInternalServerError {
		traceID, _ := r.Context().Value(traceKey).(string)
		h.log.Error("request failed", "traceId", traceID, "status", code, "err", err)
		WriteError(w, r, code, "internal_error", "internal server error")
		return
	}
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	WriteError(w, r, code, "request_failed", msg)
}

// writeAppError 把 app 层错误映射为 HTTP 状态码：角色矩阵拒绝统一 403，
// 其余按请求错误 400 处理（app 层错误文本以 "forbidden: " 前缀标识越权）。
func writeAppError(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil && strings.HasPrefix(err.Error(), "forbidden:") {
		WriteError(w, r, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	code := 400
	if err != nil && strings.Contains(err.Error(), "not found") {
		code = 404
	}
	WriteError(w, r, code, "request_failed", err.Error())
}

// dispatch /api/v1 下所有路由。
func (h *Server) dispatch(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/api/v1")
	parts := strings.Split(strings.Trim(p, "/"), "/")

	// /contests/import 与 /contests/{cid}/bundle（教师）
	if len(parts) >= 2 && parts[0] == "contests" {
		if h.bundleRoutes(w, r, parts) {
			return
		}
	}

	// /contests
	if len(parts) == 1 && parts[0] == "contests" {
		switch r.Method {
		case "GET":
			r2, ok := h.requireLogin(w, r)
			if !ok {
				return
			}
			contests, ok := h.visibleContests(r2)
			if !ok {
				WriteError(w, r, 401, "unauthorized", "session user no longer exists")
				return
			}
			writeOut(w, contests, 200)
		case "POST":
			if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
				return
			}
			var v struct {
				Name             string   `json:"name"`
				Description      string   `json:"description"`
				RankingMode      string   `json:"rankingMode"`
				Mode             string   `json:"mode"`
				AnswerMode       string   `json:"answerMode"`
				AllowedLanguages []string `json:"allowedLanguages"`
				Season           string   `json:"season"`
				TicketGroup      string   `json:"ticketGroup"`
			}
			if decode(r, &v) != nil {
				h.err(w, r, 400, fmt.Errorf("invalid json"))
				return
			}
			c, err := h.svc.CreateContestWithMode(v.Name, v.Description, v.RankingMode, v.Mode)
			if err != nil {
				h.err(w, r, 400, err)
				return
			}
			if v.AnswerMode != "" {
				if c, err = h.svc.SetContestAnswerMode(c.ID, v.AnswerMode); err != nil {
					h.err(w, r, 400, err)
					return
				}
			}
			if len(v.AllowedLanguages) > 0 {
				if c, err = h.svc.SetContestLanguages(c.ID, v.AllowedLanguages); err != nil {
					h.err(w, r, 400, err)
					return
				}
			}
			if v.Season != "" || v.TicketGroup != "" {
				if c, err = h.svc.SetContestTicketScope(c.ID, v.Season, v.TicketGroup); err != nil {
					h.err(w, r, 400, err)
					return
				}
			}
			writeOut(w, c, 201)
		default:
			h.err(w, r, 405, fmt.Errorf("method not allowed"))
		}
		return
	}

	// /contests/{cid}[...]
	if len(parts) >= 2 && parts[0] == "contests" {
		h.contestRoutes(w, r, parts[1:])
		return
	}

	// /tray-summary（托盘速览面板数据，教师专用）
	if len(parts) == 1 && parts[0] == "tray-summary" && r.Method == "GET" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		writeOut(w, h.traySummary(), 200)
		return
	}

	// /ui/command（教师 → 原生窗口指令；仅 Wails 外壳注册了执行器才有效）
	if len(parts) == 2 && parts[0] == "ui" && parts[1] == "command" && r.Method == "POST" {
		var v struct {
			Action string `json:"action"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		switch v.Action {
		case "show-main", "open-join", "open-bigscreen", "open-student":
		default:
			h.err(w, r, 400, fmt.Errorf("unknown ui action %q", v.Action))
			return
		}
		// 例外：本机回环来的 show-main 免会话。托盘面板在教师尚未登录时
		// 也要能把主窗唤到前面去登录（否则面板里「打开教师台登录」是死路）；
		// 主窗自身仍由登录页把关，且局域网来源不享受这个例外。
		loopbackShowMain := v.Action == "show-main" && isLoopbackAddr(h.clientIP(r))
		if !loopbackShowMain {
			if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
				return
			}
		}
		if h.uiCommander == nil {
			WriteError(w, r, http.StatusServiceUnavailable, "no_ui_commander",
				"native windows are only available in the desktop build")
			return
		}
		writeOut(w, map[string]any{"ok": true}, 200)
		go h.uiCommander(v.Action)
		return
	}

	// /shutdown（管理员优雅停机；托盘面板退出按钮）
	if len(parts) == 1 && parts[0] == "shutdown" && r.Method == "POST" {
		if _, ok := h.requireRole(w, r, domain.RoleAdmin); !ok {
			return
		}
		if h.shutdownHook == nil {
			WriteError(w, r, http.StatusServiceUnavailable, "no_shutdown_hook",
				"shutdown is only available in desktop mode")
			return
		}
		writeOut(w, map[string]any{"stopping": true}, 200)
		go func() {
			time.Sleep(300 * time.Millisecond) // 让响应先落回浏览器
			h.shutdownHook()
		}()
		return
	}

	// /users
	if len(parts) == 1 && parts[0] == "users" {
		switch r.Method {
		case "GET":
			if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
				return
			}
			users := h.svc.ListUsers()
			out := make([]userView, 0, len(users))
			for _, u := range users {
				out = append(out, userView{
					User:        u,
					HasPassword: h.svc.HasPassword(u.ID),
					Password:    h.svc.PasswordPlain(u.ID),
				})
			}
			writeOut(w, out, 200)
		case "POST":
			r2, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin)
			if !ok {
				return
			}
			var v struct {
				Name     string `json:"name"`
				Username string `json:"username"`
				Role     string `json:"role"`
				Password string `json:"password"`
				Province string `json:"province"`
			}
			if decode(r, &v) != nil {
				h.err(w, r, 400, fmt.Errorf("invalid json"))
				return
			}
			v.Role = strings.ToLower(strings.TrimSpace(v.Role))
			if v.Role == "" {
				v.Role = domain.RoleStudent
			}
			sess, _ := sessionOf(r2)
			if !domain.ValidRole(v.Role) {
				h.err(w, r, 400, fmt.Errorf("unknown role"))
				return
			}
			if !domain.CanAssignRole(sess.Role, v.Role) {
				WriteError(w, r, http.StatusForbidden, "forbidden",
					"only administrators can create or promote staff accounts")
				return
			}
			u, err := h.svc.CreateAccount(v.Name, v.Username, v.Role, v.Province)
			if err != nil {
				h.err(w, r, 400, err)
				return
			}
			if v.Password != "" {
				if err := h.auth.ResetPassword(u.ID, v.Password); err != nil {
					h.err(w, r, 400, err)
					return
				}
			}
			writeOut(w, u, 201)
		default:
			h.err(w, r, 405, fmt.Errorf("method not allowed"))
		}
		return
	}

	// /users/import（CSV 批量导入，教师）
	if len(parts) == 2 && parts[0] == "users" && parts[1] == "import" && r.Method == "POST" {
		r2, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin)
		if !ok {
			return
		}
		var v struct {
			CSV string `json:"csv"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		sess, _ := sessionOf(r2)
		writeOut(w, h.importUsers(v.CSV, sess.Role), 200)
		return
	}

	// /users/export（CSV 导出，不含密码，教师）
	if len(parts) == 2 && parts[0] == "users" && parts[1] == "export" && r.Method == "GET" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=forge-users.csv")
		_, _ = w.Write([]byte(h.exportUsers()))
		return
	}

	// /users/batch-province（批量设置省份，教师改学生、管理员任意）
	if len(parts) == 2 && parts[0] == "users" && parts[1] == "batch-province" && r.Method == "POST" {
		r2, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin)
		if !ok {
			return
		}
		var v struct {
			UserIDs  []string `json:"userIds"`
			Province string   `json:"province"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		sess, _ := sessionOf(r2)
		updated, err := h.svc.BatchSetProvince(sess.Role, v.UserIDs, v.Province)
		if err != nil {
			writeAppError(w, r, err)
			return
		}
		writeOut(w, map[string]any{"updated": updated}, 200)
		return
	}

	// /users/csp-tickets（CSP 准考证号分配：批量建考生账号或给既有账号发号）
	if len(parts) == 2 && parts[0] == "users" && parts[1] == "csp-tickets" && r.Method == "POST" {
		r2, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin)
		if !ok {
			return
		}
		var v struct {
			Province     string   `json:"province"`
			Group        string   `json:"group"`
			Season       string   `json:"season"`
			NumberLength int      `json:"numberLength"`
			CreateCount  int      `json:"createCount"`
			UserIDs      []string `json:"userIds"`
			Password     string   `json:"password"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		sess, _ := sessionOf(r2)
		assignments, err := h.svc.AssignCSPTickets(sess.Role, app.CSPTicketRequest{
			Province:     v.Province,
			Group:        v.Group,
			Season:       v.Season,
			NumberLength: v.NumberLength,
			CreateCount:  v.CreateCount,
			UserIDs:      v.UserIDs,
		})
		if err != nil {
			writeAppError(w, r, err)
			return
		}
		// 统一初始密码仅作用于本次新建的考生账号；给既有账号发号不动密码。
		if v.Password != "" {
			for _, a := range assignments {
				if !a.Created {
					continue
				}
				if err := h.auth.ResetPassword(a.User.ID, v.Password); err != nil {
					h.err(w, r, 400, fmt.Errorf("password for %s: %w", a.User.Username, err))
					return
				}
			}
		}
		writeOut(w, map[string]any{"assignments": assignments}, 201)
		return
	}

	// /users/{id}（PUT 编辑账号；DELETE 删除账号）
	if len(parts) == 2 && parts[0] == "users" &&
		parts[1] != "import" && parts[1] != "export" &&
		parts[1] != "batch-province" && parts[1] != "csp-tickets" {
		if r.Method == "DELETE" {
			r2, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin)
			if !ok {
				return
			}
			sess, _ := sessionOf(r2)
			if err := h.svc.DeleteUser(sess.Role, sess.UserID, parts[1]); err != nil {
				h.err(w, r, 400, err)
				return
			}
			writeOut(w, map[string]bool{"ok": true}, 200)
			return
		}
		if r.Method != "PUT" {
			h.err(w, r, 405, fmt.Errorf("method not allowed"))
			return
		}
		r2, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin)
		if !ok {
			return
		}
		var v struct {
			Name     string `json:"name"`
			Username string `json:"username"`
			Role     string `json:"role"`
			Province string `json:"province"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		sess, _ := sessionOf(r2)
		u, err := h.svc.UpdateUser(sess.Role, parts[1], app.UpdateUserInput{
			Name: v.Name, Username: v.Username, Role: v.Role, Province: v.Province,
		})
		if err != nil {
			writeAppError(w, r, err)
			return
		}
		writeOut(w, u, 200)
		return
	}

	// /provinces（内置省份表 + 用户数 + 启用状态）
	if len(parts) == 1 && parts[0] == "provinces" {
		switch r.Method {
		case "GET":
			if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
				return
			}
			views, err := h.svc.ListProvinces()
			if err != nil {
				h.err(w, r, 500, err)
				return
			}
			writeOut(w, views, 200)
		default:
			h.err(w, r, 405, fmt.Errorf("method not allowed"))
		}
		return
	}

	// /provinces/{code}（启用/停用，仅管理员）
	if len(parts) == 2 && parts[0] == "provinces" {
		if r.Method != "PUT" {
			h.err(w, r, 405, fmt.Errorf("method not allowed"))
			return
		}
		r2, ok := h.requireRole(w, r, domain.RoleAdmin)
		if !ok {
			return
		}
		var v struct {
			Enabled *bool `json:"enabled"`
		}
		if decode(r, &v) != nil || v.Enabled == nil {
			h.err(w, r, 400, fmt.Errorf("enabled (boolean) is required"))
			return
		}
		sess, _ := sessionOf(r2)
		if err := h.svc.SetProvinceEnabled(sess.Role, parts[1], *v.Enabled); err != nil {
			writeAppError(w, r, err)
			return
		}
		views, err := h.svc.ListProvinces()
		if err != nil {
			h.err(w, r, 500, err)
			return
		}
		for _, view := range views {
			if strings.EqualFold(view.Code, parts[1]) {
				writeOut(w, view, 200)
				return
			}
		}
		h.err(w, r, 404, fmt.Errorf("province not found"))
		return
	}

	// /groups
	if len(parts) == 1 && parts[0] == "groups" {
		switch r.Method {
		case "GET":
			if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
				return
			}
			writeOut(w, h.svc.ListGroups(), 200)
		case "POST":
			if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
				return
			}
			var v struct {
				Name string `json:"name"`
			}
			if decode(r, &v) != nil {
				h.err(w, r, 400, fmt.Errorf("invalid json"))
				return
			}
			g, err := h.svc.CreateGroup(v.Name)
			if err != nil {
				h.err(w, r, 400, err)
				return
			}
			writeOut(w, g, 201)
		default:
			h.err(w, r, 405, fmt.Errorf("method not allowed"))
		}
		return
	}

	// /groups/{gid}/members
	if len(parts) == 3 && parts[0] == "groups" && parts[2] == "members" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		gid := parts[1]
		var v struct {
			UserID string `json:"userId"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		switch r.Method {
		case "POST":
			if err := h.svc.AddUserToGroup(v.UserID, gid); err != nil {
				h.err(w, r, 400, err)
				return
			}
			writeOut(w, map[string]bool{"ok": true}, 200)
		case "DELETE":
			if err := h.svc.RemoveUserFromGroup(v.UserID, gid); err != nil {
				h.err(w, r, 400, err)
				return
			}
			writeOut(w, map[string]bool{"ok": true}, 200)
		default:
			h.err(w, r, 405, fmt.Errorf("method not allowed"))
		}
		return
	}

	// /submissions/{id}/judge
	if len(parts) == 3 && parts[0] == "submissions" && parts[2] == "judge" && r.Method == "POST" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		var v struct {
			Verdict string `json:"verdict"`
			Score   int    `json:"score"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		j, err := h.svc.Judge(parts[1], v.Verdict, v.Score)
		if err != nil {
			h.err(w, r, 400, err)
			return
		}
		writeOut(w, j, 200)
		return
	}

	// /submissions/{id}/cases（按点回写，LemonLime 子任务语义聚合）
	if len(parts) == 3 && parts[0] == "submissions" && parts[2] == "cases" && r.Method == "POST" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		var v struct {
			Cases    []domain.CaseResult `json:"cases"`
			Subtasks map[int]int         `json:"subtasks"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		sub, err := h.svc.GetSubmission(parts[1])
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		problem, err := h.svc.GetProblem(sub.ContestID, sub.ProblemID)
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		if len(problem.TestCases) > 0 && len(v.Cases) != len(problem.TestCases) {
			h.err(w, r, 400, fmt.Errorf("case count must equal the problem testcase count"))
			return
		}
		seen := make(map[int]bool, len(v.Cases))
		for _, result := range v.Cases {
			if result.CaseIndex < 0 ||
				(len(problem.TestCases) > 0 && result.CaseIndex >= len(problem.TestCases)) ||
				seen[result.CaseIndex] || !domain.ValidCaseVerdict(result.Verdict) {
				h.err(w, r, 400, fmt.Errorf("invalid judge case result"))
				return
			}
			seen[result.CaseIndex] = true
		}
		subtaskOf, fullOf, depsOf := judge.SubtaskMaps(problem.TestCases)
		if subtaskOf == nil {
			subtaskOf = func(int) int { return 0 }
			fullOf = func(int) int { return domain.ProblemTotalScore(problem) }
			depsOf = func(int) []int { return nil }
		}
		j, err := h.svc.JudgeCases(parts[1], domain.JudgeOutcome{
			Cases: v.Cases, CompileMessage: "",
		}, subtaskOf, fullOf, depsOf)
		if err != nil {
			h.err(w, r, 400, err)
			return
		}
		writeOut(w, j, 200)
		return
	}

	// /auth/*（身份端点）
	if len(parts) >= 2 && parts[0] == "auth" {
		h.authRoutes(w, r, parts[1:])
		return
	}

	// /users/{id}/password（教师重置）
	if len(parts) == 3 && parts[0] == "users" && parts[2] == "password" && r.Method == "POST" {
		r2, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin)
		if !ok {
			return
		}
		target, err := h.svc.Store().GetUser(parts[1])
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		sess, _ := sessionOf(r2)
		if !domain.CanManageUser(sess.Role, target.Role) {
			WriteError(w, r, http.StatusForbidden, "forbidden",
				"teachers can only reset student passwords")
			return
		}
		var v struct {
			Password string `json:"password"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		if err := h.auth.ResetPassword(parts[1], v.Password); err != nil {
			h.err(w, r, 400, err)
			return
		}
		writeOut(w, map[string]bool{"ok": true}, 200)
		return
	}

	// /events/ticket（浏览器 SSE 短时票据，写入 HttpOnly Cookie）
	if len(parts) == 2 && parts[0] == "events" && parts[1] == "ticket" && r.Method == "POST" {
		h.eventsTicket(w, r)
		return
	}

	// /events?contestId=
	if len(parts) == 1 && parts[0] == "events" && r.Method == "GET" {
		h.events(w, r)
		return
	}

	// /time (服务端时钟同步)
	if len(parts) == 1 && parts[0] == "time" && r.Method == "GET" {
		writeOut(w, map[string]any{"serverTime": h.svc.Health()["time"]}, 200)
		return
	}

	// /i18n
	if len(parts) == 1 && parts[0] == "i18n" && r.Method == "GET" {
		writeOut(w, i18nDict(), 200)
		return
	}

	http.NotFound(w, r)
}

func (h *Server) contestRoutes(w http.ResponseWriter, r *http.Request, rest []string) {
	cid := rest[0]

	// 考务安全与进阶高级路由（机位IP、时间窗口、广播公告、Zip解锁）
	if h.advancedContestRoutes(w, r, cid, rest[1:]) {
		return
	}

	// /contests/{cid}
	if len(rest) == 1 && r.Method == "GET" {
		r2, c, ok := h.requireContestAccess(w, r, cid)
		if !ok {
			return
		}
		if !isTeacher(r2) {
			unlocked := h.paperUnlocked(r2, c)
			sess, _ := sessionOf(r2)
			c = redactContest(c, sess.UserID)
			c.ZipUnlocked = unlocked
		}
		writeOut(w, c, 200)
		return
	}
	if len(rest) == 2 && rest[1] == "start" && r.Method == "POST" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		c, err := h.svc.StartContest(cid)
		if err != nil {
			h.err(w, r, 400, err)
			return
		}
		writeOut(w, c, 200)
		return
	}
	if len(rest) == 2 && rest[1] == "finish" && r.Method == "POST" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		c, err := h.svc.FinishContest(cid)
		if err != nil {
			h.err(w, r, 400, err)
			return
		}
		writeOut(w, c, 200)
		return
	}
	if len(rest) == 2 && rest[1] == "ranking" && r.Method == "GET" {
		r2, _, ok := h.requirePaperAccess(w, r, cid)
		if !ok {
			return
		}
		c, err := h.svc.GetContest(cid)
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		// NOI 全真模拟：榜单赛后公布，赛中对选手隐藏（教师台不受限）。
		if !isTeacher(r2) && !domain.StudentResultsVisible(c.Mode, domain.Status(c.Status)) {
			WriteError(w, r, http.StatusForbidden, "ranking_hidden",
				"ranking is published after the NOI contest finishes")
			return
		}
		if c.RankingMode == "acm" {
			writeOut(w, h.svc.RankingACM(cid), 200)
			return
		}
		writeOut(w, h.svc.Ranking(cid), 200)
		return
	}

	// /contests/{cid}/problems
	if len(rest) == 2 && rest[1] == "problems" {
		switch r.Method {
		case "GET":
			r2, _, ok := h.requirePaperAccess(w, r, cid)
			if !ok {
				return
			}
			problems := h.svc.ListProblems(cid)
			if !isTeacher(r2) {
				for i := range problems {
					problems[i] = redactProblem(problems[i])
				}
			}
			writeOut(w, problems, 200)
		case "POST":
			if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
				return
			}
			var p domain.Problem
			if decode(r, &p) != nil {
				h.err(w, r, 400, fmt.Errorf("invalid json"))
				return
			}
			p.ContestID = cid
			created, err := h.svc.CreateProblem(p)
			if err != nil {
				h.err(w, r, 400, err)
				return
			}
			writeOut(w, created, 201)
		default:
			h.err(w, r, 405, fmt.Errorf("method not allowed"))
		}
		return
	}

	if len(rest) == 2 && rest[1] == "export" && r.Method == "GET" {
		r2, c, ok := h.requirePaperAccess(w, r, cid)
		if !ok {
			return
		}
		probs := h.svc.ListProblems(cid)
		if !isTeacher(r2) {
			sess, _ := sessionOf(r2)
			c = redactContest(c, sess.UserID)
			for i := range probs {
				probs[i] = redactProblem(probs[i])
			}
		}
		html, err := pdf.RenderContestHTML(c, probs)
		if err != nil {
			h.err(w, r, 500, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(html)
		return
	}
	if len(rest) == 2 && rest[1] == "pdf" && r.Method == "GET" {
		r2, c, ok := h.requirePaperAccess(w, r, cid)
		if !ok {
			return
		}
		probs := h.svc.ListProblems(cid)
		if !isTeacher(r2) {
			sess, _ := sessionOf(r2)
			c = redactContest(c, sess.UserID)
			for i := range probs {
				probs[i] = redactProblem(probs[i])
			}
		}
		pdfBytes, err := pdf.GenerateContestPDFContext(r.Context(), c, probs)
		if err != nil {
			status := http.StatusServiceUnavailable
			code := "pdf_unavailable"
			if errors.Is(err, context.DeadlineExceeded) {
				status = http.StatusGatewayTimeout
				code = "pdf_timeout"
			}
			h.log.Error("contest PDF generation failed", "contestId", cid, "err", err)
			WriteError(w, r, status, code, "PDF renderer is temporarily unavailable")
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-problems.pdf", cid))
		// X-Pdf-Pages：文档查看器（Evince 复刻）显示 N / M 与翻页钳制用。
		w.Header().Set("X-Pdf-Pages", strconv.Itoa(countPdfPages(pdfBytes)))
		_, _ = w.Write(pdfBytes)
		return
	}

	// /contests/{cid}/problems/reorder
	if len(rest) == 3 && rest[1] == "problems" && rest[2] == "reorder" && r.Method == "PUT" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		var v struct {
			ProblemIDs []string `json:"problemIds"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		reordered, err := h.svc.ReorderProblems(cid, v.ProblemIDs)
		if err != nil {
			h.err(w, r, 400, err)
			return
		}
		writeOut(w, reordered, 200)
		return
	}

	// /contests/{cid}/problems/{pid}
	if len(rest) == 3 && rest[1] == "problems" {
		pid := rest[2]
		switch r.Method {
		case "GET":
			r2, _, ok := h.requirePaperAccess(w, r, cid)
			if !ok {
				return
			}
			p, err := h.svc.GetProblem(cid, pid)
			if err != nil {
				h.err(w, r, 404, err)
				return
			}
			if !isTeacher(r2) {
				p = redactProblem(p)
			}
			writeOut(w, p, 200)
			return
		case "PUT":
			if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
				return
			}
			var p domain.Problem
			if decode(r, &p) != nil {
				h.err(w, r, 400, fmt.Errorf("invalid json"))
				return
			}
			p.ID = pid
			p.ContestID = cid
			updated, err := h.svc.UpdateProblem(p)
			if err != nil {
				h.err(w, r, 400, err)
				return
			}
			writeOut(w, updated, 200)
			return
		}
	}

	if len(rest) == 2 && rest[1] == "statistics" && r.Method == "GET" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		st, err := h.svc.Statistics(cid)
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		writeOut(w, st, 200)
		return
	}

	// /contests/{cid}/settings（教师：榜单模式/语言白名单/赛季与组别）
	if len(rest) == 2 && rest[1] == "settings" && r.Method == "PUT" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		var v struct {
			RankingMode      *string   `json:"rankingMode"`
			Mode             *string   `json:"mode"`
			AnswerMode       *string   `json:"answerMode"`
			AllowedLanguages *[]string `json:"allowedLanguages"`
			Season           *string   `json:"season"`
			TicketGroup      *string   `json:"ticketGroup"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		c, err := h.svc.GetContest(cid)
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		if v.RankingMode != nil {
			if c, err = h.svc.SetRankingMode(cid, *v.RankingMode); err != nil {
				h.err(w, r, 400, err)
				return
			}
		}
		if v.Mode != nil {
			if c, err = h.svc.SetContestMode(cid, *v.Mode); err != nil {
				h.err(w, r, 400, err)
				return
			}
		}
		if v.AnswerMode != nil {
			if c, err = h.svc.SetContestAnswerMode(cid, *v.AnswerMode); err != nil {
				h.err(w, r, 400, err)
				return
			}
		}
		if v.AllowedLanguages != nil {
			if c, err = h.svc.SetContestLanguages(cid, *v.AllowedLanguages); err != nil {
				h.err(w, r, 400, err)
				return
			}
		}
		if v.Season != nil || v.TicketGroup != nil {
			season, group := c.Season, c.TicketGroup
			if v.Season != nil {
				season = *v.Season
			}
			if v.TicketGroup != nil {
				group = *v.TicketGroup
			}
			if c, err = h.svc.SetContestTicketScope(cid, season, group); err != nil {
				h.err(w, r, 400, err)
				return
			}
		}
		writeOut(w, c, 200)
		return
	}

	// /contests/{cid}/collect（收题包导出：准考证号/英文名/英文名.cpp 的 zip）
	if len(rest) == 2 && rest[1] == "collect" && r.Method == "GET" {
		h.collectRoute(w, r, cid)
		return
	}

	if len(rest) == 3 && rest[1] == "statistics" && rest[2] == "export" && r.Method == "GET" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		c, err := h.svc.GetContest(cid)
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=statistics-%s.csv", cid))
		_, _ = w.Write([]byte(h.exportStatistics(c)))
		return
	}

	// /contests/{cid}/problems/{pid}/versions（代码版本：保存/提交即版本）
	if len(rest) == 4 && rest[1] == "problems" && rest[3] == "versions" {
		h.codeVersionsRoute(w, r, cid, rest[2])
		return
	}

	// /contests/{cid}/problems/{pid}/files...（测试数据文件仓）
	if len(rest) >= 4 && rest[1] == "problems" && rest[3] == "files" {
		h.filesRoutes(w, r, cid, rest[2], rest[3:])
		return
	}

	// /contests/{cid}/problems/{pid}/rejudge
	if len(rest) == 4 && rest[1] == "problems" && rest[3] == "rejudge" && r.Method == "POST" {
		h.rejudgeRoute(w, r, cid, rest[2])
		return
	}

	// /contests/{cid}/problems/{pid}/subtasks
	if len(rest) == 4 && rest[1] == "problems" && rest[3] == "subtasks" && r.Method == "PUT" {
		h.subtasksRoute(w, r, cid, rest[2])
		return
	}
	// /contests/{cid}/monitor（监考大屏快照，教师专用）
	if len(rest) == 2 && rest[1] == "monitor" && r.Method == "GET" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		presence := map[string]time.Time{}
		if h.presenceSource != nil {
			presence = h.presenceSource(cid)
		}
		snapshot, err := h.svc.ContestMonitor(cid, presence)
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		writeOut(w, snapshot, 200)
		return
	}

	if len(rest) == 4 && rest[1] == "problems" {
		pid, action := rest[2], rest[3]
		r2, _, ok := h.requirePaperAccess(w, r, cid)
		if !ok {
			return
		}
		p, err := h.svc.GetProblem(cid, pid)
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		switch {
		case action == "validate" && r.Method == "POST":
			if !isTeacher(r2) {
				WriteError(w, r, 403, "forbidden", "insufficient role")
				return
			}
			if verr := domain.ValidateProblem(p); verr != nil {
				writeOut(w, map[string]any{"valid": false, "error": verr.Error()}, 200)
				return
			}
			writeOut(w, map[string]any{"valid": true}, 200)
			return
		case action == "export" && r.Method == "GET":
			c, _ := h.svc.GetContest(cid)
			if !isTeacher(r2) {
				sess, _ := sessionOf(r2)
				c = redactContest(c, sess.UserID)
				p = redactProblem(p)
			}
			html, err := pdf.RenderHTML(c, p)
			if err != nil {
				h.err(w, r, 500, err)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(html)
			return
		case action == "pdf" && r.Method == "GET":
			c, _ := h.svc.GetContest(cid)
			if !isTeacher(r2) {
				sess, _ := sessionOf(r2)
				c = redactContest(c, sess.UserID)
				p = redactProblem(p)
			}
			pdfBytes, err := pdf.GeneratePDFContext(r.Context(), c, p)
			if err != nil {
				status := http.StatusServiceUnavailable
				code := "pdf_unavailable"
				if errors.Is(err, context.DeadlineExceeded) {
					status = http.StatusGatewayTimeout
					code = "pdf_timeout"
				}
				h.log.Error("problem PDF generation failed",
					"contestId", cid, "problemId", pid, "err", err)
				WriteError(w, r, status, code, "PDF renderer is temporarily unavailable")
				return
			}
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pdf", p.Code))
			_, _ = w.Write(pdfBytes)
			return
		}
		http.NotFound(w, r)
		return
	}

	// DELETE /contests/{cid}/problems/{pid}
	if len(rest) == 3 && rest[1] == "problems" && r.Method == "DELETE" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		if err := h.svc.DeleteProblem(cid, rest[2]); err != nil {
			status := 400
			if err.Error() == "problem not found" || err.Error() == "contest not found" {
				status = 404
			}
			h.err(w, r, status, err)
			return
		}
		writeOut(w, map[string]bool{"ok": true}, 200)
		return
	}

	// /contests/{cid}/submissions
	if len(rest) == 2 && rest[1] == "submissions" {
		switch r.Method {
		case "GET":
			r2, _, ok := h.requirePaperAccess(w, r, cid)
			if !ok {
				return
			}
			limit, err := submissionFeedLimit(r2.URL.Query().Get("limit"))
			if err != nil {
				h.err(w, r, http.StatusBadRequest, err)
				return
			}
			userID := ""
			if !isTeacher(r2) {
				if sess, ok := sessionOf(r2); ok {
					userID = sess.UserID
				}
				if userID == "" {
					h.err(w, r, http.StatusForbidden, fmt.Errorf("participant login required"))
					return
				}
			}
			feed := h.svc.ListSubmissionFeed(cid, userID, limit)
			w.Header().Set("X-Result-Limit", strconv.Itoa(limit))
			writeOut(w, h.redactSubmissions(r2, cid, feed), 200)
		case "POST":
			r2, _, ok := h.requirePaperAccess(w, r, cid)
			if !ok {
				return
			}
			requestKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
			if err := domain.ValidateIdempotencyKey(requestKey); err != nil {
				h.err(w, r, http.StatusBadRequest, err)
				return
			}
			var v struct {
				ProblemID string `json:"problemID"`
				ProblemId string `json:"problemId"`
				UserID    string `json:"userId"`
				Language  string `json:"language"`
				Code      string `json:"code"`
			}
			if decode(r, &v) != nil {
				h.err(w, r, 400, fmt.Errorf("invalid json"))
				return
			}
			pid := v.ProblemID
			if pid == "" {
				pid = v.ProblemId
			}
			// 身份即归属：学生只能以自己名义提交，教师可代指定。
			uid := v.UserID
			if sess, ok := sessionOf(r2); ok && !isTeacher(r2) {
				uid = sess.UserID
			} else if uid == "" {
				if sess, ok := sessionOf(r2); ok {
					uid = sess.UserID
				}
			}
			x, replayed, err := h.svc.SubmitIdempotent(
				cid, pid, uid, v.Language, v.Code, h.clientIP(r), requestKey,
			)
			if err != nil {
				if errors.Is(err, app.ErrIdempotencyConflict) {
					WriteError(w, r, http.StatusConflict, "idempotency_conflict",
						"Idempotency-Key was already used for a different submission")
					return
				}
				h.err(w, r, 400, err)
				return
			}
			if replayed {
				w.Header().Set("Idempotent-Replay", "true")
			}
			// New submissions and idempotent replays both retry a still-queued
			// handoff. The pool deduplicates by submission ID, so a replay
			// cannot cause the same source to run twice concurrently.
			if h.OnSubmit != nil && x.Verdict == domain.VerdictQueued {
				if err := h.OnSubmit(x); err != nil {
					h.log.Error("judge enqueue failed", "submissionId", x.ID, "err", err)
					h.metrics.IncJudgeEnqueueFailure()
				}
			}
			if !isTeacher(r2) {
				if c, gerr := h.svc.GetContest(cid); gerr == nil &&
					!domain.StudentResultsVisible(c.Mode, domain.Status(c.Status)) {
					x = domain.MaskSubmissionForStudent(x)
				}
			}
			writeOut(w, x, 202)
		default:
			h.err(w, r, 405, fmt.Errorf("method not allowed"))
		}
		return
	}

	// /contests/{cid}/groups | /participants
	if len(rest) == 2 && rest[1] == "groups" && r.Method == "POST" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		var v struct {
			GroupID string `json:"groupId"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		if err := h.svc.AddGroupToContest(cid, v.GroupID); err != nil {
			h.err(w, r, 400, err)
			return
		}
		writeOut(w, map[string]bool{"ok": true}, 200)
		return
	}
	if len(rest) == 2 && rest[1] == "participants" && r.Method == "POST" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		var v struct {
			UserID string `json:"userId"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		if err := h.svc.AddParticipantToContest(cid, v.UserID); err != nil {
			h.err(w, r, 400, err)
			return
		}
		writeOut(w, map[string]bool{"ok": true}, 200)
		return
	}

	http.NotFound(w, r)
}

func (h *Server) events(w http.ResponseWriter, r *http.Request) {
	cid := strings.TrimSpace(r.URL.Query().Get("contestId"))
	if cid == "" {
		WriteError(w, r, http.StatusBadRequest, "contest_required", "contestId is required")
		return
	}
	r2, ok := h.authenticateEvents(w, r, cid)
	if !ok {
		return
	}
	r = r2
	sess, _ := sessionOf(r)
	streamUser := ""
	if sess, ok := sessionOf(r); ok {
		streamUser = sess.UserID
	}
	ch, cancel := h.svc.Subscribe(cid, streamUser)
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fl, ok := w.(http.Flusher)
	if !ok {
		return
	}
	_, _ = fmt.Fprint(w, "retry: 5000\n\n")
	fl.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			// SSE comments keep proxies and NAT devices from silently closing
			// long-running exam streams.
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			fl.Flush()
		case e, ok := <-ch:
			if !ok {
				return
			}
			e, send := h.eventForSession(e, sess)
			if !send {
				continue
			}
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, b)
			fl.Flush()
		}
	}
}

const streamCookieName = "forge_stream"

// cookieSecure mirrors the deployment's HTTPS state for auth cookies. A
// forwarding header is trusted only when the operator has explicitly opted in.
func (h *Server) cookieSecure(r *http.Request) bool {
	return isHTTPS(r, h.TrustProxy)
}

func (h *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/api/v1",
		MaxAge:   int(auth.SessionTTL / time.Second),
		Expires:  time.Now().Add(auth.SessionTTL).UTC(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   h.cookieSecure(r),
	})
}

func (h *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/api/v1",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   h.cookieSecure(r),
	})
}

// eventsTicket exchanges a bearer session for a short-lived, HttpOnly stream
// grant. EventSource then authenticates via the same-origin cookie without
// exposing the long-lived login token in its URL.
func (h *Server) eventsTicket(w http.ResponseWriter, r *http.Request) {
	cid := strings.TrimSpace(r.URL.Query().Get("contestId"))
	if cid == "" {
		WriteError(w, r, 400, "contest_required", "contestId is required")
		return
	}
	r2, _, ok := h.requirePaperAccess(w, r, cid)
	if !ok {
		return
	}
	sess, _ := sessionOf(r2)
	ticket, expires := h.auth.IssueStreamTicket(
		sess.UserID, cid, sess.Token, auth.StreamTicketTTL,
	)
	http.SetCookie(w, &http.Cookie{
		Name:     streamCookieName,
		Value:    ticket,
		Path:     "/api/v1/events",
		MaxAge:   int(auth.StreamTicketTTL / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   h.cookieSecure(r),
	})
	writeOut(w, map[string]any{"expiresAt": expires}, 200)
}

func (h *Server) authenticateEvents(w http.ResponseWriter, r *http.Request, cid string) (*http.Request, bool) {
	if strings.TrimSpace(cid) == "" {
		WriteError(w, r, http.StatusBadRequest, "contest_required", "contestId is required")
		return r, false
	}
	// Non-browser clients may keep using the normal bearer header.
	if bearer(r) != "" {
		r2, _, ok := h.requirePaperAccess(w, r, cid)
		return r2, ok
	}
	cookie, err := r.Cookie(streamCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "login or stream ticket required")
		return r, false
	}
	claims, err := h.auth.ValidateStreamTicket(cookie.Value, cid)
	if err != nil {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "stream ticket expired or invalid")
		return r, false
	}
	u, err := h.svc.Store().GetUser(claims.UserID)
	if err != nil {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "session user no longer exists")
		return r, false
	}
	if cid != "" {
		c, err := h.svc.GetContest(cid)
		if err != nil {
			h.err(w, r, 404, err)
			return r, false
		}
		staff := u.Role == domain.RoleTeacher || u.Role == domain.RoleAdmin
		if !domain.CanAccessContest(c, u, staff) {
			WriteError(w, r, http.StatusForbidden, "forbidden", "contest access denied")
			return r, false
		}
		if !staff {
			rWithSession := withSession(r, domain.Session{UserID: u.ID, Role: u.Role})
			if !h.paperUnlocked(rWithSession, c) {
				WriteError(w, r, http.StatusForbidden, "exam_locked", "exam package is locked")
				return r, false
			}
		}
	}
	return withSession(r, domain.Session{UserID: u.ID, Role: u.Role}), true
}

// clearStreamCookie invalidates the browser-side SSE grant on logout.
func (h *Server) clearStreamCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     streamCookieName,
		Value:    "",
		Path:     "/api/v1/events",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   h.cookieSecure(r),
	})
}

// spa 构建产物即首页；/app 下静态资源直出 dist。
func (h *Server) spa(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/app")
	rel = strings.TrimPrefix(rel, "/")
	if rel != "" && !strings.HasSuffix(r.URL.Path, "/") {
		// 静态资源优先
		if b, err := fs.ReadFile(webdist.Dist, "dist/"+rel); err == nil {
			if contentType := mime.TypeByExtension(path.Ext(rel)); contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			if strings.HasPrefix(rel, "assets/") {
				// Vite emits content-hashed asset names, so they can be cached
				// aggressively without risking a stale deployment.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			_, _ = w.Write(b)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	b, err := fs.ReadFile(webdist.Dist, "dist/index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(404)
		_, _ = w.Write([]byte("frontend not built yet: run `npm run build` in frontend/"))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func i18nDict() map[string]map[string]string {
	return map[string]map[string]string{
		"zh-CN": {
			"brand": "Forge 竞赛工坊", "title": "比赛大厅", "subtitle": "实时竞赛与评测排名系统",
			"teacher": "教师控制台", "problems": "试题列表", "submissions": "提交流",
			"ranking": "实时排名", "draft": "草稿", "running": "进行中", "finished": "已结束",
		},
		"en-US": {
			"brand": "Forge Arena", "title": "Contest Lobby", "subtitle": "Real-Time Competition & Evaluation Platform",
			"teacher": "Teacher Console", "problems": "Problems", "submissions": "Submissions",
			"ranking": "Leaderboard", "draft": "Draft", "running": "Running", "finished": "Finished",
		},
	}
}

// authRoutes /auth/* 身份端点。
func (h *Server) authRoutes(w http.ResponseWriter, r *http.Request, rest []string) {
	// POST /auth/login（公开）
	if len(rest) == 1 && rest[0] == "login" && r.Method == "POST" {
		var v struct {
			Login    string `json:"login"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		ident := v.Login
		if ident == "" {
			ident = v.Username
		}
		if h.LoginLimiter != nil {
			allowed, retryAfter := h.LoginLimiter.Allow(h.clientIP(r), ident)
			if !allowed {
				seconds := int(retryAfter / time.Second)
				if seconds < 1 {
					seconds = 1
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
				WriteError(w, r, http.StatusTooManyRequests, "rate_limited",
					"too many failed login attempts; try again later")
				return
			}
		}
		sess, err := h.auth.Login(ident, v.Password)
		if err != nil {
			if h.LoginLimiter != nil {
				h.LoginLimiter.Fail(h.clientIP(r), ident)
			}
			WriteError(w, r, 401, "invalid_credentials", "invalid login or password")
			return
		}
		if h.LoginLimiter != nil {
			h.LoginLimiter.Reset(ident)
		}
		u, err := h.svc.Store().GetUser(sess.UserID)
		if err != nil {
			_ = h.auth.Logout(sess.Token)
			WriteError(w, r, 401, "invalid_credentials", "account no longer exists")
			return
		}
		h.setSessionCookie(w, r, sess.Token)
		writeOut(w, map[string]any{
			"userId": sess.UserID, "role": domain.NormalizeRole(u.Role),
			"username": u.Username, "name": u.Name,
		}, 200)
		return
	}
	// POST /auth/desktop-exchange（一次性桌面令牌换会话；令牌只在
	// 本机 webview URL 中出现，持有即证明是桌面壳启动的主窗口）。
	if len(rest) == 1 && rest[0] == "desktop-exchange" && r.Method == "POST" {
		var v struct {
			Token string `json:"token"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		h.desktopTokenMu.Lock()
		want, exp := h.desktopToken, h.desktopTokenExp
		h.desktopToken = "" // 单次消费
		h.desktopTokenMu.Unlock()
		if want == "" || v.Token == "" ||
			subtle.ConstantTimeCompare([]byte(v.Token), []byte(want)) != 1 ||
			time.Now().After(exp) {
			WriteError(w, r, 401, "invalid_token", "desktop login token is invalid or expired")
			return
		}
		users, err := h.svc.Store().ListUsers()
		if err != nil {
			h.err(w, r, 500, err)
			return
		}
		var admin domain.User
		for _, u := range users {
			if u.Username == "admin" && u.Role == domain.RoleAdmin {
				admin = u
				break
			}
		}
		if admin.ID == "" {
			WriteError(w, r, 404, "no_admin", "administrator account not found")
			return
		}
		sess, err := h.auth.SessionForUser(admin.ID)
		if err != nil {
			h.err(w, r, 500, err)
			return
		}
		h.setSessionCookie(w, r, sess.Token)
		writeOut(w, map[string]any{
			"userId": admin.ID, "role": domain.RoleAdmin,
			"username": admin.Username, "name": admin.Name,
		}, 200)
		return
	}
	// POST /auth/logout（登录态，幂等）
	if len(rest) == 1 && rest[0] == "logout" && r.Method == "POST" {
		r2, ok := h.requireLogin(w, r)
		if !ok {
			return
		}
		sess, _ := sessionOf(r2)
		_ = h.auth.Logout(sess.Token)
		h.clearSessionCookie(w, r)
		h.clearStreamCookie(w, r)
		h.clearZipCookie(w, r)
		writeOut(w, map[string]bool{"ok": true}, 200)
		return
	}
	// GET /auth/me（登录态）
	if len(rest) == 1 && rest[0] == "me" && r.Method == "GET" {
		r2, ok := h.requireLogin(w, r)
		if !ok {
			return
		}
		sess, _ := sessionOf(r2)
		u, err := h.svc.Store().GetUser(sess.UserID)
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		writeOut(w, u, 200)
		return
	}
	// POST /auth/password（本人改密）
	if len(rest) == 1 && rest[0] == "password" && r.Method == "POST" {
		r2, ok := h.requireLogin(w, r)
		if !ok {
			return
		}
		var v struct {
			OldPassword string `json:"oldPassword"`
			NewPassword string `json:"newPassword"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		sess, _ := sessionOf(r2)
		if err := h.auth.ChangePassword(sess.UserID, v.OldPassword, v.NewPassword); err != nil {
			h.err(w, r, 400, err)
			return
		}
		writeOut(w, map[string]bool{"ok": true}, 200)
		return
	}
	http.NotFound(w, r)
}

// codeVersionsRoute 代码版本读写：POST 保存/提交版本，GET 查看历史（含代码，用于对比）。
// 学生只能写/读自己的；教师可代写与查看任意考生；zip 锁定的试卷同样要过纸面访问闸。
func (h *Server) codeVersionsRoute(w http.ResponseWriter, r *http.Request, cid, pid string) {
	r2, _, ok := h.requirePaperAccess(w, r, cid)
	if !ok {
		return
	}
	sess, sessOK := sessionOf(r2)
	if !sessOK {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "login required")
		return
	}
	switch r.Method {
	case "POST":
		var v struct {
			Code     string `json:"code"`
			Language string `json:"language"`
			Source   string `json:"source"`
			UserID   string `json:"userId"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		uid := sess.UserID
		if isTeacher(r2) && v.UserID != "" {
			uid = v.UserID
		}
		if uid == "" {
			WriteError(w, r, http.StatusForbidden, "forbidden", "participant login required")
			return
		}
		version, err := h.svc.SaveCodeVersion(cid, pid, uid, v.Language, v.Code, v.Source)
		if err != nil {
			h.err(w, r, 400, err)
			return
		}
		writeOut(w, version, 201)
	case "GET":
		uid := sess.UserID
		if q := r.URL.Query().Get("userId"); q != "" {
			if !isTeacher(r2) && q != uid {
				WriteError(w, r, 403, "forbidden", "students may only list their own versions")
				return
			}
			uid = q
		}
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil {
			limit = 50
		}
		versions, err := h.svc.ListCodeVersions(sess.Role, uid, cid, pid, uid, limit)
		if err != nil {
			writeAppError(w, r, err)
			return
		}
		writeOut(w, versions, 200)
	default:
		h.err(w, r, 405, fmt.Errorf("method not allowed"))
	}
}

// redactSubmissions is the student submission-history boundary. Students only
// receive their own attempts: exposing peers' timestamps, scores, and verdicts
// during a running contest enables live answer inference even when source code
// itself is redacted. Teachers retain the full operational feed.
func (h *Server) redactSubmissions(r *http.Request, cid string, subs []domain.Submission) []domain.Submission {
	if isTeacher(r) {
		return subs
	}
	currentUID := ""
	if sess, ok := sessionOf(r); ok {
		currentUID = sess.UserID
	}
	out := make([]domain.Submission, 0, len(subs))
	for _, s := range subs {
		if currentUID == "" || s.UserID != currentUID {
			continue
		}
		out = append(out, s)
	}
	// NOI 全真模拟：比赛进行中学生只能看到「已提交」，判定随成绩一并公布。
	if c, err := h.svc.GetContest(cid); err == nil && !domain.StudentResultsVisible(c.Mode, domain.Status(c.Status)) {
		for i := range out {
			out[i] = domain.MaskSubmissionForStudent(out[i])
		}
	}
	return out
}

// userView 是账号列表的对外形态：在领域模型之上补口令状态与明文口令。
// 明文只服务教师端"当面念给考生"的场景，鉴权仍然只比对 bcrypt 哈希；
// 该结构只出现在教师/管理员可访问的 /users 列表里。
type userView struct {
	domain.User
	HasPassword bool   `json:"hasPassword"`
	Password    string `json:"password,omitempty"`
}

// countPdfPages 统计 PDF 的 /Type /Page 对象数（减去 /Pages 目录页对象）。
func countPdfPages(data []byte) int {
	return bytes.Count(data, []byte("/Type /Page")) - bytes.Count(data, []byte("/Type /Pages"))
}
