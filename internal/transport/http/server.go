// Canonical HTTP 服务器：新栈直驱（app.Service + pdf + realtime），只暴露 /api/v1。
// Legacy /api 与旧手写页已砍掉（项目未上线，不做兼容）。
package httpapi

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/pdf"
	"github.com/yviscool/forge/internal/testdata"
	webdist "github.com/yviscool/forge/internal/web"
)

type Server struct {
	svc  *app.Service
	auth *auth.Service
	log  *slog.Logger
	// OnSubmit 提交成功后的钩子（自动评测入队；nil 则仅记录手动判题）。
	OnSubmit func(domain.Submission)
	// Files 测试数据文件仓（main 按 DataDir 装配，测试用临时目录）。
	Files *testdata.Store

	handler http.Handler
}

// ServeHTTP 让 *Server 本身就是 http.Handler（钩子可在启动后挂载）。
func (h *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

func NewServer(svc *app.Service, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	h := &Server{svc: svc, log: log, auth: auth.New(svc.Store(), nil)}
	h.Files = testdata.New("./data/testdata")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", Healthz)
	mux.HandleFunc("/readyz", Readyz)
	mux.HandleFunc("/api/v1/", h.dispatch)
	mux.HandleFunc("/app", h.spa)
	mux.HandleFunc("/app/", h.spa)
	mux.HandleFunc("/", h.root)
	h.handler = WithMiddleware(mux, log)
	return h
}

func (h *Server) root(w http.ResponseWriter, r *http.Request) {
	// 单页应用即首页：构建产物缺失时给明确指引。
	http.Redirect(w, r, "/app", http.StatusFound)
}

func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func writeOut(w http.ResponseWriter, v any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Server) err(w http.ResponseWriter, r *http.Request, code int, err error) {
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	WriteError(w, r, code, "request_failed", msg)
}

// dispatch /api/v1 下所有路由。
func (h *Server) dispatch(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/api/v1")
	parts := strings.Split(strings.Trim(p, "/"), "/")

	// /contests
	if len(parts) == 1 && parts[0] == "contests" {
		switch r.Method {
		case "GET":
			writeOut(w, h.svc.ListContests(), 200)
		case "POST":
			if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
				return
			}
			var v struct {
				Name             string   `json:"name"`
				Description      string   `json:"description"`
				RankingMode      string   `json:"rankingMode"`
				AllowedLanguages []string `json:"allowedLanguages"`
			}
			if decode(r, &v) != nil {
				h.err(w, r, 400, fmt.Errorf("invalid json"))
				return
			}
			c, err := h.svc.CreateContest(v.Name, v.Description, v.RankingMode)
			if err != nil {
				h.err(w, r, 400, err)
				return
			}
			if len(v.AllowedLanguages) > 0 {
				if c, err = h.svc.SetContestLanguages(c.ID, v.AllowedLanguages); err != nil {
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

	// /users
	if len(parts) == 1 && parts[0] == "users" {
		switch r.Method {
		case "GET":
			writeOut(w, h.svc.ListUsers(), 200)
		case "POST":
			if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
				return
			}
			var v struct {
				Name     string `json:"name"`
				Role     string `json:"role"`
				Password string `json:"password"`
			}
			if decode(r, &v) != nil {
				h.err(w, r, 400, fmt.Errorf("invalid json"))
				return
			}
			u, err := h.svc.CreateUser(v.Name, v.Role)
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
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		var v struct {
			CSV string `json:"csv"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		writeOut(w, h.importUsers(v.CSV), 200)
		return
	}

	// /users/export（CSV 导出，不含密码，教师）
	if len(parts) == 2 && parts[0] == "users" && parts[1] == "export" && r.Method == "GET" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		_, _ = w.Write([]byte(h.exportUsers()))
		return
	}

	// /groups
	if len(parts) == 1 && parts[0] == "groups" {
		switch r.Method {
		case "GET":
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
		j, err := h.svc.JudgeCases(parts[1], domain.JudgeOutcome{Cases: v.Cases},
			func(i int) int { return 0 },
			func(st int) int {
				if s, ok := v.Subtasks[st]; ok {
					return s
				}
				return 0
			})
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
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
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

	// /events?contestId=
	if len(parts) == 1 && parts[0] == "events" && r.Method == "GET" {
		h.events(w, r)
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

	// /contests/{cid}
	if len(rest) == 1 && r.Method == "GET" {
		c, err := h.svc.GetContest(cid)
		if err != nil {
			h.err(w, r, 404, err)
			return
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
		if c, err := h.svc.GetContest(cid); err == nil && c.RankingMode == "acm" {
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
			writeOut(w, h.svc.ListProblems(cid), 200)
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

	// /contests/{cid}/problems/{pid}
	if len(rest) == 3 && rest[1] == "problems" && r.Method == "GET" {
		p, err := h.svc.GetProblem(cid, rest[2])
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		writeOut(w, p, 200)
		return
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

	// /contests/{cid}/settings（教师：榜单模式/语言白名单）
	if len(rest) == 2 && rest[1] == "settings" && r.Method == "PUT" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return
		}
		var v struct {
			RankingMode      *string   `json:"rankingMode"`
			AllowedLanguages *[]string `json:"allowedLanguages"`
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
		if v.AllowedLanguages != nil {
			if c, err = h.svc.SetContestLanguages(cid, *v.AllowedLanguages); err != nil {
				h.err(w, r, 400, err)
				return
			}
		}
		writeOut(w, c, 200)
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
	if len(rest) == 4 && rest[1] == "problems" {
		pid, action := rest[2], rest[3]
		p, err := h.svc.GetProblem(cid, pid)
		if err != nil {
			h.err(w, r, 404, err)
			return
		}
		switch {
		case action == "validate" && r.Method == "POST":
			if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
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
			pdfBytes, err := pdf.GeneratePDF(c, p)
			if err != nil {
				writeOut(w, map[string]string{"error": err.Error()}, 503)
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

	// /contests/{cid}/submissions
	if len(rest) == 2 && rest[1] == "submissions" {
		switch r.Method {
		case "GET":
			r2, ok := h.requireLogin(w, r)
			if !ok {
				return
			}
			writeOut(w, h.redactSubmissions(r2, h.svc.ListSubmissions(cid)), 200)
		case "POST":
			r2, ok := h.requireLogin(w, r)
			if !ok {
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
			x, err := h.svc.Submit(cid, pid, uid, v.Language, v.Code)
			if err != nil {
				h.err(w, r, 400, err)
				return
			}
			if h.OnSubmit != nil {
				h.OnSubmit(x)
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
	cid := r.URL.Query().Get("contestId")
	ch, cancel := h.svc.Subscribe(cid)
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fl, ok := w.(http.Flusher)
	if !ok {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, b)
			fl.Flush()
		}
	}
}

// spa 构建产物即首页；/app 下静态资源直出 dist。
func (h *Server) spa(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/app")
	rel = strings.TrimPrefix(rel, "/")
	if rel != "" && !strings.HasSuffix(r.URL.Path, "/") {
		// 静态资源优先
		if b, err := fs.ReadFile(webdist.Dist, "dist/"+rel); err == nil {
			switch {
			case strings.HasSuffix(rel, ".js"):
				w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			case strings.HasSuffix(rel, ".css"):
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
			case strings.HasSuffix(rel, ".html"):
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
			}
			_, _ = w.Write(b)
			return
		}
	}
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
			Password string `json:"password"`
		}
		if decode(r, &v) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return
		}
		sess, err := h.auth.Login(v.Login, v.Password)
		if err != nil {
			WriteError(w, r, 401, "invalid_credentials", "invalid login or password")
			return
		}
		writeOut(w, map[string]any{"token": sess.Token, "userId": sess.UserID, "role": sess.Role}, 200)
		return
	}
	// POST /auth/logout（登录态，幂等）
	if len(rest) == 1 && rest[0] == "logout" && r.Method == "POST" {
		if _, ok := h.requireLogin(w, r); !ok {
			return
		}
		_ = h.auth.Logout(bearer(r))
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

// redactSubmissions 代码可见性：学生只能看自己的代码与编译信息，教师全见。
func (h *Server) redactSubmissions(r *http.Request, subs []domain.Submission) []domain.Submission {
	if isTeacher(r) {
		return subs
	}
	sess, ok := sessionOf(r)
	if !ok {
		return subs
	}
	out := make([]domain.Submission, len(subs))
	for i, s := range subs {
		if s.UserID != sess.UserID {
			s.Code = ""
			s.CompileMessage = ""
		}
		out[i] = s
	}
	return out
}
