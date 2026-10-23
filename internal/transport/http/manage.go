package httpapi

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/testdata"
)

const (
	maxImportCSVBytes = 4 << 20
	maxImportRows     = 5000
)

// importUsers CSV 批量导入：name,role,group,password（首行可为表头）。
func (h *Server) importUsers(raw, actorRole string) map[string]any {
	if len(raw) > maxImportCSVBytes {
		return map[string]any{
			"created": 0,
			"errors":  []string{fmt.Sprintf("CSV exceeds %d bytes", maxImportCSVBytes)},
		}
	}
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return map[string]any{"created": 0, "errors": []string{err.Error()}}
	}
	if len(rows) > maxImportRows+1 {
		return map[string]any{
			"created": 0,
			"errors":  []string{fmt.Sprintf("CSV exceeds %d data rows", maxImportRows)},
		}
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
			role = strings.ToLower(strings.TrimSpace(f[1]))
		}
		if len(f) > 2 {
			group = strings.TrimSpace(f[2])
		}
		if len(f) > 3 {
			pw = strings.TrimSpace(f[3])
		}
		if !domain.ValidRole(role) {
			errs = append(errs, fmt.Sprintf("line %d: unknown role %q", i+1, role))
			continue
		}
		if !domain.CanAssignRole(actorRole, role) {
			errs = append(errs, fmt.Sprintf(
				"line %d: only administrators can import staff accounts", i+1,
			))
			continue
		}
		if pw != "" {
			if err := auth.ValidatePassword(pw); err != nil {
				errs = append(errs, fmt.Sprintf("line %d password: %v", i+1, err))
				continue
			}
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

// csvText neutralizes spreadsheet formulas while leaving normal UTF-8 names
// untouched. encoding/csv handles commas, quotes, and newlines.
func csvText(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func writeCSV(rows [][]string) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()
	return b.String()
}

// exportUsers 导出 name,role,groups（不含密码）。
func (h *Server) exportUsers() string {
	users := h.svc.ListUsers()
	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })
	groups := map[string]string{}
	for _, g := range h.svc.ListGroups() {
		groups[g.ID] = g.Name
	}
	rows := make([][]string, 0, len(users)+1)
	rows = append(rows, []string{"name", "role", "groups"})
	for _, u := range users {
		var gs []string
		for _, gid := range u.Groups {
			if n, ok := groups[gid]; ok {
				gs = append(gs, n)
			}
		}
		rows = append(rows, []string{
			csvText(u.Name), csvText(u.Role), csvText(strings.Join(gs, ";")),
		})
	}
	return writeCSV(rows)
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
	problem, err := h.svc.GetProblem(cid, pid)
	if err != nil {
		h.err(w, r, 404, err)
		return true
	}
	if len(rest) > 1 && r.Method != "GET" {
		c, err := h.svc.GetContest(cid)
		if err != nil {
			h.err(w, r, 404, err)
			return true
		}
		if !c.CanModifySettings() {
			h.err(w, r, http.StatusConflict, fmt.Errorf("test data is frozen once the contest starts"))
			return true
		}
	}
	// /files
	if len(rest) == 1 && r.Method == "GET" {
		names, _ := h.Files.List(cid, pid)
		if names == nil {
			names = []string{}
		}
		var missing []string
		if problem.TaskType == domain.TaskAnswersOnly {
			// Answer-only keys are standalone .out/.ans files. Pairing does not
			// apply, and stale .in files must not block publication.
			missing = []string{}
		} else {
			missing, _ = h.Files.Pairs(cid, pid)
		}
		if missing == nil {
			missing = []string{}
		}
		writeOut(w, map[string]any{
			"files": names, "missingPairs": missing,
			"answerOnly": problem.TaskType == domain.TaskAnswersOnly,
		}, 200)
		return true
	}
	// /files/{name}
	if len(rest) == 2 {
		name := rest[1]
		switch r.Method {
		case "POST":
			if name != "sync" {
				break
			}
			var matched []testdata.MatchedCase
			if problem.TaskType == domain.TaskAnswersOnly {
				matched, err = h.Files.AnswerKeys(cid, pid)
			} else {
				missing, pairErr := h.Files.Pairs(cid, pid)
				if pairErr != nil {
					h.err(w, r, 500, pairErr)
					return true
				}
				if len(missing) > 0 {
					h.err(w, r, 400, fmt.Errorf(
						"test data is incomplete; resolve missing pairs first: %s",
						strings.Join(missing, ", "),
					))
					return true
				}
				matched, err = h.Files.MatchCases(cid, pid)
			}
			if err != nil {
				h.err(w, r, 500, err)
				return true
			}
			if len(matched) == 0 {
				if problem.TaskType == domain.TaskAnswersOnly {
					h.err(w, r, 400, fmt.Errorf("no .out/.ans answer key found"))
				} else {
					h.err(w, r, 400, fmt.Errorf("no complete .in/.out or .in/.ans pairs found"))
				}
				return true
			}
			if problem.TaskType == domain.TaskAnswersOnly && len(matched) != 1 {
				h.err(w, r, 400, fmt.Errorf(
					"answer-only tasks require exactly one .out/.ans answer key; found %d",
					len(matched),
				))
				return true
			}
			current, err := h.svc.GetProblem(cid, pid)
			if err != nil {
				h.err(w, r, 404, err)
				return true
			}
			existing := make(map[string]domain.TestCase, len(current.TestCases))
			for _, tc := range current.TestCases {
				key := tc.ID
				if key == "" {
					key = pairBaseName(tc.InputFile)
				}
				if key != "" {
					existing[key] = tc
				}
			}
			baseScore, remainder := 100, 0
			if problem.TaskType != domain.TaskAnswersOnly {
				baseScore = 100 / len(matched)
				if baseScore < 1 {
					baseScore = 1
				}
				remainder = 100 - baseScore*len(matched)
				if remainder < 0 {
					remainder = 0
				}
			}
			cases := make([]domain.TestCase, 0, len(matched))
			for i, pair := range matched {
				score := baseScore
				if i < remainder {
					score++
				}
				subtask := i
				var timeLimit, memoryMiB int
				var dependsOn []int
				if prev, ok := existing[pair.BaseName]; ok {
					score = prev.Score
					if prev.Subtask > 0 {
						subtask = prev.Subtask
					} else {
						subtask = 0
					}
					timeLimit = prev.TimeLimitMs
					memoryMiB = prev.MemoryMiB
					dependsOn = append([]int(nil), prev.DependsOn...)
				}
				cases = append(cases, domain.TestCase{
					ID:          pair.BaseName,
					InputFile:   pair.InputFile,
					OutputFile:  pair.OutputFile,
					Score:       score,
					Subtask:     subtask,
					TimeLimitMs: timeLimit,
					MemoryMiB:   memoryMiB,
					DependsOn:   dependsOn,
				})
			}
			updated, err := h.svc.UpdateProblem(domain.Problem{
				ID: pid, ContestID: cid, TestCases: cases,
			})
			if err != nil {
				h.err(w, r, 400, err)
				return true
			}
			writeOut(w, updated, 200)
			return true
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

func pairBaseName(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	for _, ext := range []string{".in", ".out", ".ans"} {
		if strings.HasSuffix(path, ext) {
			return strings.TrimSuffix(path, ext)
		}
	}
	return path
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
		if h.OnSubmit == nil {
			continue
		}
		// OnSubmit 闭包负责组装请求并入队（与提交路径一致）。
		if err := h.OnSubmit(x); err != nil {
			h.log.Warn("rejudge enqueue failed", "submissionId", x.ID, "err", err)
			h.metrics.IncJudgeEnqueueFailure()
			continue
		}
		enqueued++
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
		rows := [][]string{{"name", "solved", "penalty"}}
		for _, r := range h.svc.RankingACM(c.ID) {
			rows = append(rows, []string{
				csvText(r.UserName), fmt.Sprint(r.Solved), fmt.Sprint(r.Penalty),
			})
		}
		return writeCSV(rows)
	}
	header := []string{"name", "total", "accepted"}
	for _, p := range probs {
		header = append(header, csvText(p.Code+" "+p.Title))
	}
	rows := [][]string{header}
	for _, r := range h.svc.Ranking(c.ID) {
		row := []string{
			csvText(r.UserName), fmt.Sprint(r.Score), fmt.Sprint(r.Accepted),
		}
		for _, p := range probs {
			row = append(row, fmt.Sprint(best[r.UserID][p.ID]))
		}
		rows = append(rows, row)
	}
	return writeCSV(rows)
}

// bundleRoutes 比赛 Bundle 导入导出（教师；对标 LemonLime .cdf 往返，JSON 格式）。
func (h *Server) bundleRoutes(w http.ResponseWriter, r *http.Request, parts []string) bool {
	// GET /contests/{cid}/bundle
	if len(parts) == 3 && parts[0] == "contests" && parts[2] == "bundle" && r.Method == "GET" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return true
		}
		b, err := h.svc.ExportContest(parts[1])
		if err != nil {
			h.err(w, r, 404, err)
			return true
		}
		w.Header().Set("Content-Disposition", "attachment; filename=bundle-"+parts[1]+".json")
		writeOut(w, b, 200)
		return true
	}
	// POST /contests/import
	if len(parts) == 2 && parts[0] == "contests" && parts[1] == "import" && r.Method == "POST" {
		if _, ok := h.requireRole(w, r, domain.RoleTeacher, domain.RoleAdmin); !ok {
			return true
		}
		var b app.ContestBundle
		if decode(r, &b) != nil {
			h.err(w, r, 400, fmt.Errorf("invalid json"))
			return true
		}
		c, err := h.svc.ImportBundle(b)
		if err != nil {
			h.err(w, r, 400, err)
			return true
		}
		writeOut(w, c, 201)
		return true
	}
	return false
}
