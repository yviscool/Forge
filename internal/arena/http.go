package arena

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

//go:embed web/index.html web/teacher.html web/app.js web/styles.css
var webFS embed.FS

type Server struct{ svc *Service }

func NewServer(s *Service) http.Handler {
	h := &Server{svc: s}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/contests", h.contests)
	mux.HandleFunc("/api/contests/", h.contestSubroutes)
	mux.HandleFunc("/api/users", h.users)
	mux.HandleFunc("/api/groups", h.groups)
	mux.HandleFunc("/api/events", h.events)
	mux.HandleFunc("/web/", h.assets)
	mux.HandleFunc("/", h.pages)
	return mux
}
func jsonOut(w http.ResponseWriter, v any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func decode(r *http.Request, v any) error { return json.NewDecoder(r.Body).Decode(v) }
func (h *Server) pages(w http.ResponseWriter, r *http.Request) {
	name := "web/index.html"
	if strings.HasPrefix(r.URL.Path, "/teacher") {
		name = "web/teacher.html"
	}
	b, e := webFS.ReadFile(name)
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
func (h *Server) assets(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	b, e := webFS.ReadFile(name)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(name, ".css") {
		w.Header().Set("Content-Type", "text/css")
	}
	if strings.HasSuffix(name, ".js") {
		w.Header().Set("Content-Type", "text/javascript")
	}
	_, _ = w.Write(b)
}
func (h *Server) contests(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		jsonOut(w, h.svc.Contests(), 200)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct{ Name, Description string }
	if decode(r, &v) != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	c, e := h.svc.CreateContest(v.Name, v.Description)
	if e != nil {
		jsonOut(w, map[string]string{"error": e.Error()}, 400)
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
	if len(parts) == 4 && parts[3] == "start" && r.Method == "POST" {
		c, e := h.svc.StartContest(cid)
		if e != nil {
			jsonOut(w, map[string]string{"error": e.Error()}, 400)
			return
		}
		jsonOut(w, c, 200)
		return
	}
	if len(parts) == 4 && parts[3] == "ranking" {
		jsonOut(w, h.svc.Ranking(cid), 200)
		return
	}
	if len(parts) == 4 && parts[3] == "problems" && r.Method == "GET" {
		jsonOut(w, h.svc.Problems(cid), 200)
		return
	}
	if len(parts) == 4 && parts[3] == "problems" && r.Method == "POST" {
		var p Problem
		if decode(r, &p) != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		p.ContestID = cid
		x, e := h.svc.AddProblem(p)
		if e != nil {
			jsonOut(w, map[string]string{"error": e.Error()}, 400)
			return
		}
		jsonOut(w, x, 201)
		return
	}
	if len(parts) == 4 && parts[3] == "submissions" && r.Method == "POST" {
		var x Submission
		if decode(r, &x) != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		x.ContestID = cid
		y, e := h.svc.Submit(x)
		if e != nil {
			jsonOut(w, map[string]string{"error": e.Error()}, 400)
			return
		}
		jsonOut(w, y, 202)
		return
	}
	if len(parts) == 6 && parts[3] == "problems" && parts[5] == "export" {
		ps := h.svc.Problems(cid)
		for _, p := range ps {
			if p.ID == parts[4] {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				t := template.Must(template.New("p").Parse(`<!doctype html><meta charset="utf-8"><style>@page{size:A4;margin:18mm}body{font-family:system-ui,sans-serif;line-height:1.6}pre{background:#f3f4f6;padding:12px}h1{font-size:24px}</style><h1>{{.Code}} {{.Title}}</h1><p>{{.Statement}}</p><h2>约束</h2><p>{{.Constraints}}</p><h2>输入</h2><p>{{.Input}}</p><h2>输出</h2><p>{{.Output}}</p><h2>样例</h2><pre>{{.Examples}}</pre>`))
				_ = t.Execute(w, p)
				return
			}
		}
	}
	http.NotFound(w, r)
}
func (h *Server) users(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct{ Name, Role string }
	if decode(r, &v) != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	u, e := h.svc.CreateUser(v.Name, v.Role)
	if e != nil {
		jsonOut(w, map[string]string{"error": e.Error()}, 400)
		return
	}
	jsonOut(w, u, 201)
}
func (h *Server) groups(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct{ Name string }
	if decode(r, &v) != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	g, e := h.svc.CreateGroup(v.Name)
	if e != nil {
		jsonOut(w, map[string]string{"error": e.Error()}, 400)
		return
	}
	jsonOut(w, g, 201)
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
