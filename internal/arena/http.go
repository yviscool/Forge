package arena

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

//go:embed web/index.html web/teacher.html web/app.js web/styles.css
var webFS embed.FS

type Server struct {
	svc *Service
}

func NewServer(s *Service) http.Handler {
	h := &Server{svc: s}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/contests", h.contests)
	mux.HandleFunc("/api/contests/", h.contestSubroutes)
	mux.HandleFunc("/api/users", h.users)
	mux.HandleFunc("/api/groups", h.groups)
	mux.HandleFunc("/api/groups/", h.groupSubroutes)
	mux.HandleFunc("/api/submissions/", h.submissionSubroutes)
	mux.HandleFunc("/api/events", h.events)
	mux.HandleFunc("/api/i18n", h.i18n)

	mux.HandleFunc("/web/", h.assets)
	mux.HandleFunc("/", h.pages)
	return mux
}

func jsonOut(w http.ResponseWriter, v any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func (h *Server) pages(w http.ResponseWriter, r *http.Request) {
	name := "web/index.html"
	if strings.HasPrefix(r.URL.Path, "/teacher") {
		name = "web/teacher.html"
	}
	b, err := webFS.ReadFile(name)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (h *Server) assets(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	b, err := webFS.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(name, ".css") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	if strings.HasSuffix(name, ".js") {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	}
	_, _ = w.Write(b)
}

func (h *Server) i18n(w http.ResponseWriter, r *http.Request) {
	translations := map[string]map[string]string{
		"zh-CN": {
			"brand":           "Forge 竞赛工坊",
			"title":           "比赛大厅",
			"subtitle":        "实时竞赛与评测排名系统",
			"teacher":         "教师控制台",
			"manage":          "多比赛配置、独立用户体系与实时评测",
			"create":          "创建比赛",
			"switch_contest":  "选择比赛",
			"problems":        "试题列表",
			"submissions":     "提交流",
			"ranking":         "实时排名",
			"users":           "用户管理",
			"groups":          "编组管理",
			"add_problem":     "添加试题",
			"validate":        "校验题面",
			"export_pdf":      "导出 CCF 规范 PDF",
			"start_contest":   "启动比赛",
			"finish_contest":  "结束比赛",
			"submit_code":     "提交代码",
			"language":        "编程语言",
			"status":          "状态",
			"draft":           "草稿",
			"running":         "进行中",
			"finished":        "已结束",
		},
		"en-US": {
			"brand":           "Forge Arena",
			"title":           "Contest Lobby",
			"subtitle":        "Real-Time Competition & Evaluation Platform",
			"teacher":         "Teacher Console",
			"manage":          "Multi-Contest Hosting, User Grouping & Live Judging",
			"create":          "Create Contest",
			"switch_contest":  "Select Contest",
			"problems":        "Problems",
			"submissions":     "Submissions",
			"ranking":         "Leaderboard",
			"users":           "Users",
			"groups":          "Groups",
			"add_problem":     "Add Problem",
			"validate":        "Validate",
			"export_pdf":      "Export CCF PDF",
			"start_contest":   "Start Contest",
			"finish_contest":  "Finish Contest",
			"submit_code":     "Submit Code",
			"language":        "Language",
			"status":          "Status",
			"draft":           "Draft",
			"running":         "Running",
			"finished":        "Finished",
		},
	}
	jsonOut(w, translations, 200)
}

func (h *Server) contests(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		jsonOut(w, h.svc.ListContests(), 200)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if decode(r, &v) != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	c, err := h.svc.CreateContest(v.Name, v.Description)
	if err != nil {
		jsonOut(w, map[string]string{"error": err.Error()}, 400)
		return
	}
	jsonOut(w, c, 201)
}

func (h *Server) contestSubroutes(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}
	cid := parts[2]

	// GET /api/contests/{cid}
	if len(parts) == 3 && r.Method == "GET" {
		c, err := h.svc.GetContest(cid)
		if err != nil {
			jsonOut(w, map[string]string{"error": err.Error()}, 404)
			return
		}
		jsonOut(w, c, 200)
		return
	}

	// POST /api/contests/{cid}/start
	if len(parts) == 4 && parts[3] == "start" && r.Method == "POST" {
		c, err := h.svc.StartContest(cid)
		if err != nil {
			jsonOut(w, map[string]string{"error": err.Error()}, 400)
			return
		}
		jsonOut(w, c, 200)
		return
	}

	// POST /api/contests/{cid}/finish
	if len(parts) == 4 && parts[3] == "finish" && r.Method == "POST" {
		c, err := h.svc.FinishContest(cid)
		if err != nil {
			jsonOut(w, map[string]string{"error": err.Error()}, 400)
			return
		}
		jsonOut(w, c, 200)
		return
	}

	// GET /api/contests/{cid}/ranking
	if len(parts) == 4 && parts[3] == "ranking" && r.Method == "GET" {
		jsonOut(w, h.svc.Ranking(cid), 200)
		return
	}

	// Problems routes
	if len(parts) == 4 && parts[3] == "problems" {
		if r.Method == "GET" {
			jsonOut(w, h.svc.Problems(cid), 200)
			return
		}
		if r.Method == "POST" {
			var p Problem
			if decode(r, &p) != nil {
				http.Error(w, "invalid json", 400)
				return
			}
			p.ContestID = cid
			created, err := h.svc.AddProblem(p)
			if err != nil {
				jsonOut(w, map[string]string{"error": err.Error()}, 400)
				return
			}
			jsonOut(w, created, 201)
			return
		}
	}

	// GET /api/contests/{cid}/problems/{pid} & PUT
	if len(parts) == 5 && parts[3] == "problems" {
		pid := parts[4]
		if r.Method == "GET" {
			p, err := h.svc.GetProblem(cid, pid)
			if err != nil {
				jsonOut(w, map[string]string{"error": err.Error()}, 404)
				return
			}
			jsonOut(w, p, 200)
			return
		}
		if r.Method == "PUT" {
			var p Problem
			if decode(r, &p) != nil {
				http.Error(w, "invalid json", 400)
				return
			}
			p.ID = pid
			p.ContestID = cid
			updated, err := h.svc.UpdateProblem(p)
			if err != nil {
				jsonOut(w, map[string]string{"error": err.Error()}, 400)
				return
			}
			jsonOut(w, updated, 200)
			return
		}
	}

	// POST /api/contests/{cid}/problems/{pid}/validate
	if len(parts) == 6 && parts[3] == "problems" && parts[5] == "validate" && r.Method == "POST" {
		pid := parts[4]
		p, err := h.svc.GetProblem(cid, pid)
		if err != nil {
			jsonOut(w, map[string]string{"error": "problem not found"}, 404)
			return
		}
		if valErr := h.svc.ValidateProblem(p); valErr != nil {
			jsonOut(w, map[string]any{"valid": false, "error": valErr.Error()}, 200)
			return
		}
		jsonOut(w, map[string]any{"valid": true, "message": "Problem statement passes all structural requirements"}, 200)
		return
	}

	// GET /api/contests/{cid}/problems/{pid}/export (CCF CSP A4 Print HTML)
	if len(parts) == 6 && parts[3] == "problems" && parts[5] == "export" && r.Method == "GET" {
		pid := parts[4]
		p, err := h.svc.GetProblem(cid, pid)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		c, _ := h.svc.GetContest(cid)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		htmlBytes, err := RenderProblemHTML(c, p)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_, _ = w.Write(htmlBytes)
		return
	}

	// GET /api/contests/{cid}/problems/{pid}/pdf (Direct A4 PDF via headless browser)
	if len(parts) == 6 && parts[3] == "problems" && parts[5] == "pdf" && r.Method == "GET" {
		pid := parts[4]
		p, err := h.svc.GetProblem(cid, pid)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		c, _ := h.svc.GetContest(cid)
		pdfBytes, err := GenerateProblemPDF(c, p)
		if err != nil {
			jsonOut(w, map[string]string{
				"error":    err.Error(),
				"fallback": fmt.Sprintf("/api/contests/%s/problems/%s/export", cid, pid),
			}, 503)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pdf", p.Code))
		_, _ = w.Write(pdfBytes)
		return
	}

	// Submissions route
	if len(parts) == 4 && parts[3] == "submissions" {
		if r.Method == "GET" {
			jsonOut(w, h.svc.Submissions(cid), 200)
			return
		}
		if r.Method == "POST" {
			var x Submission
			if decode(r, &x) != nil {
				http.Error(w, "invalid json", 400)
				return
			}
			x.ContestID = cid
			y, err := h.svc.Submit(x)
			if err != nil {
				jsonOut(w, map[string]string{"error": err.Error()}, 400)
				return
			}
			jsonOut(w, y, 202)
			return
		}
	}

	// Group binding to contest: POST /api/contests/{cid}/groups
	if len(parts) == 4 && parts[3] == "groups" {
		if r.Method == "POST" {
			var req struct {
				GroupID string `json:"groupId"`
			}
			if decode(r, &req) != nil {
				http.Error(w, "invalid json", 400)
				return
			}
			if err := h.svc.AddGroupToContest(cid, req.GroupID); err != nil {
				jsonOut(w, map[string]string{"error": err.Error()}, 400)
				return
			}
			jsonOut(w, map[string]bool{"ok": true}, 200)
			return
		}
	}

	// Participant binding to contest: POST /api/contests/{cid}/participants
	if len(parts) == 4 && parts[3] == "participants" {
		if r.Method == "POST" {
			var req struct {
				UserID string `json:"userId"`
			}
			if decode(r, &req) != nil {
				http.Error(w, "invalid json", 400)
				return
			}
			if err := h.svc.AddParticipantToContest(cid, req.UserID); err != nil {
				jsonOut(w, map[string]string{"error": err.Error()}, 400)
				return
			}
			jsonOut(w, map[string]bool{"ok": true}, 200)
			return
		}
	}

	http.NotFound(w, r)
}

func (h *Server) users(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		jsonOut(w, h.svc.ListUsers(), 200)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if decode(r, &v) != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	u, err := h.svc.CreateUser(v.Name, v.Role)
	if err != nil {
		jsonOut(w, map[string]string{"error": err.Error()}, 400)
		return
	}
	jsonOut(w, u, 201)
}

func (h *Server) groups(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		jsonOut(w, h.svc.ListGroups(), 200)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct {
		Name string `json:"name"`
	}
	if decode(r, &v) != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	g, err := h.svc.CreateGroup(v.Name)
	if err != nil {
		jsonOut(w, map[string]string{"error": err.Error()}, 400)
		return
	}
	jsonOut(w, g, 201)
}

func (h *Server) groupSubroutes(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 4 && parts[3] == "members" {
		gid := parts[2]
		if r.Method == "POST" {
			var v struct {
				UserID string `json:"userId"`
			}
			if decode(r, &v) != nil {
				http.Error(w, "invalid json", 400)
				return
			}
			if err := h.svc.AddUserToGroup(v.UserID, gid); err != nil {
				jsonOut(w, map[string]string{"error": err.Error()}, 400)
				return
			}
			jsonOut(w, map[string]bool{"ok": true}, 200)
			return
		}
		if r.Method == "DELETE" {
			var v struct {
				UserID string `json:"userId"`
			}
			if decode(r, &v) != nil {
				http.Error(w, "invalid json", 400)
				return
			}
			if err := h.svc.RemoveUserFromGroup(v.UserID, gid); err != nil {
				jsonOut(w, map[string]string{"error": err.Error()}, 400)
				return
			}
			jsonOut(w, map[string]bool{"ok": true}, 200)
			return
		}
	}
	http.NotFound(w, r)
}

func (h *Server) submissionSubroutes(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// POST /api/submissions/{id}/judge
	if len(parts) == 4 && parts[3] == "judge" && r.Method == "POST" {
		subID := parts[2]
		var v struct {
			Verdict string `json:"verdict"`
			Score   int    `json:"score"`
		}
		if decode(r, &v) != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		judged, err := h.svc.Judge(subID, v.Verdict, v.Score)
		if err != nil {
			jsonOut(w, map[string]string{"error": err.Error()}, 400)
			return
		}
		jsonOut(w, judged, 200)
		return
	}
	http.NotFound(w, r)
}

func (h *Server) events(w http.ResponseWriter, r *http.Request) {
	ch, closeFn := h.svc.Subscribe()
	defer closeFn()

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
