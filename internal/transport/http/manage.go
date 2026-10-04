package httpapi

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/domain"
)

// importUsers CSV 批量导入：name,role,group,password（首行可为表头）。
func (h *Server) importUsers(raw string) map[string]any {
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return map[string]any{"created": 0, "errors": []string{err.Error()}}
	}
	created := 0
	var errs []string
	for i, f := range rows {
		if len(f) == 0 {
			continue
		}
		if i == 0 && strings.EqualFold(strings.TrimSpace(f[0]), "name") {
			continue
		}
		name := strings.TrimSpace(f[0])
		if name == "" {
			continue
		}
		role, group, pw := "student", "", ""
		if len(f) > 1 && strings.TrimSpace(f[1]) != "" {
			role = strings.TrimSpace(f[1])
		}
		if len(f) > 2 {
			group = strings.TrimSpace(f[2])
		}
		if len(f) > 3 {
			pw = strings.TrimSpace(f[3])
		}
		u, err := h.svc.CreateUser(name, role)
		if err != nil {
			errs = append(errs, fmt.Sprintf("line %d: %v", i+1, err))
			continue
		}
		if pw != "" {
			if err := h.auth.ResetPassword(u.ID, pw); err != nil {
				errs = append(errs, fmt.Sprintf("line %d password: %v", i+1, err))
			}
		}
		if group != "" {
			gid := h.ensureGroup(group)
			if gid == "" {
				errs = append(errs, fmt.Sprintf("line %d: bad group", i+1))
			} else if err := h.svc.AddUserToGroup(u.ID, gid); err != nil {
				errs = append(errs, fmt.Sprintf("line %d group: %v", i+1, err))
			}
		}
		created++
	}
	if errs == nil {
		errs = []string{}
	}
	return map[string]any{"created": created, "errors": errs}
}

func (h *Server) ensureGroup(name string) string {
	for _, g := range h.svc.ListGroups() {
		if g.Name == name {
			return g.ID
		}
	}
	g, err := h.svc.CreateGroup(name)
	if err != nil {
		return ""
	}
	return g.ID
}

// exportUsers 导出 name,role,groups（不含密码）。
func (h *Server) exportUsers() string {
	var b strings.Builder
	b.WriteString("name,role,groups\n")
	users := h.svc.ListUsers()
	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })
	groups := map[string]string{}
	for _, g := range h.svc.ListGroups() {
		groups[g.ID] = g.Name
	}
	for _, u := range users {
		var gs []string
		for _, gid := range u.Groups {
			if n, ok := groups[gid]; ok {
				gs = append(gs, n)
			}
		}
		fmt.Fprintf(&b, "%s,%s,%s\n", u.Name, u.Role, strings.Join(gs, ";"))
	}
	return b.String()
}

// filesRoutes 测试数据文件管理（教师）：
// PUT .../files/{name} / GET .../files / GET .../files/{name} / DELETE .../files/{name}
func (h *Server) filesRoutes(w http.ResponseWriter, r *http.Request, cid, pid string, rest []string) bool {
	if len(rest) == 0 || rest[0] != "files" {
		return false
	}
	if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
		return true
	}
	if _, err := h.svc.GetProblem(cid, pid); err != nil {
		h.err(w, r, 404, err)
		return true
	}
	// /files
	if len(rest) == 1 && r.Method == "GET" {
		names, _ := h.Files.List(cid, pid)
		if names == nil {
			names = []string{}
		}
		missing, _ := h.Files.Pairs(cid, pid)
		if missing == nil {
			missing = []string{}
		}
		writeOut(w, map[string]any{"files": names, "missingPairs": missing}, 200)
		return true
	}
	// /files/{name}
	if len(rest) == 2 {
		name := rest[1]
		switch r.Method {
		case "PUT":
			body, err := io.ReadAll(io.LimitReader(r.Body, 11<<20))
			if err != nil {
				h.err(w, r, 400, err)
				return true
			}
			if err := h.Files.Put(cid, pid, name, body); err != nil {
				h.err(w, r, 400, err)
				return true
			}
			writeOut(w, map[string]any{"ok": true, "size": len(body)}, 200)
			return true
		case "GET":
			b, err := h.Files.Get(cid, pid, name)
			if err != nil {
				h.err(w, r, 404, err)
				return true
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write(b)
			return true
		case "DELETE":
			if err := h.Files.Delete(cid, pid, name); err != nil {
				h.err(w, r, 400, err)
				return true
			}
			writeOut(w, map[string]bool{"ok": true}, 200)
			return true
		}
	}
	http.NotFound(w, r)
	return true
}

// rejudgeRoute POST .../problems/{pid}/rejudge：全部提交重入队。
func (h *Server) rejudgeRoute(w http.ResponseWriter, r *http.Request, cid, pid string) {
	if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
		return
	}
	subs, err := h.svc.RejudgeProblem(cid, pid)
	if err != nil {
		h.err(w, r, 404, err)
		return
	}
	enqueued := 0
	for _, x := range subs {
		if h.OnSubmit != nil {
			// OnSubmit 闭包负责组装请求并入队（与提交路径一致）。
			h.OnSubmit(x)
			enqueued++
		}
	}
	writeOut(w, map[string]any{"total": len(subs), "enqueued": enqueued}, 202)
}

// subtasksRoute PUT .../problems/{pid}/subtasks：子任务分数与限额。
func (h *Server) subtasksRoute(w http.ResponseWriter, r *http.Request, cid, pid string) {
	if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
		return
	}
	if _, err := h.svc.GetProblem(cid, pid); err != nil {
		h.err(w, r, 404, err)
		return
	}
	var v struct {
		Subtasks []app.SubtaskConfig `json:"subtasks"`
	}
	if decode(r, &v) != nil {
		h.err(w, r, 400, fmt.Errorf("invalid json"))
		return
	}
	updated, err := h.svc.ConfigureSubtasks(pid, v.Subtasks)
	if err != nil {
		h.err(w, r, 400, err)
		return
	}
	writeOut(w, updated, 200)
}

// exportStatistics 成绩单 CSV：姓名 + 总分/罚时 + 每题最佳。
func (h *Server) exportStatistics(c domain.Contest) string {
	var b strings.Builder
	probs := h.svc.ListProblems(c.ID)
	subs := h.svc.ListSubmissions(c.ID)
	best := map[string]map[string]int{} // user -> problem -> best
	names := map[string]string{}
	for _, s := range subs {
		names[s.UserID] = s.UserName
		if best[s.UserID] == nil {
			best[s.UserID] = map[string]int{}
		}
		if s.Score > best[s.UserID][s.ProblemID] {
			best[s.UserID][s.ProblemID] = s.Score
		}
	}
	if c.RankingMode == "acm" {
		b.WriteString("name,solved,penalty\n")
		for _, r := range h.svc.RankingACM(c.ID) {
			fmt.Fprintf(&b, "%s,%d,%d\n", r.UserName, r.Solved, r.Penalty)
		}
		return b.String()
	}
	b.WriteString("name,total,accepted")
	for _, p := range probs {
		fmt.Fprintf(&b, ",%s %s", p.Code, p.Title)
	}
	b.WriteString("\n")
	for _, r := range h.svc.Ranking(c.ID) {
		fmt.Fprintf(&b, "%s,%d,%d", r.UserName, r.Score, r.Accepted)
		for _, p := range probs {
			fmt.Fprintf(&b, ",%d", best[r.UserID][p.ID])
		}
		b.WriteString("\n")
	}
	return b.String()
}
