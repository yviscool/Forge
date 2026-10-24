package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestCollectEndpoint 全链路：发号 → 建赛（赛季+组别）→ 出题（slug）→
// 开赛提交 → 教师导出收题包（zip 目录结构与 manifest）。
func TestCollectEndpoint(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	// 第二名学生：不给他发号，收题时应退登录名目录并给出告警。
	bobID, _ := e.mkUserFull(t, "Bob", "student", "pw-bob-12345")

	// 发号：Alice 拿 2026/J 的 HA-J00001。
	code, body := postAuth(t, e.url+"/api/v1/users/csp-tickets", e.teacher, fmt.Sprintf(
		`{"season":"2026","province":"HA","group":"J","userIds":[%q]}`, e.stuID))
	if code != 201 {
		t.Fatalf("csp-tickets %d %v", code, body)
	}
	assignments, _ := body["assignments"].([]any)
	if len(assignments) != 1 {
		t.Fatalf("assignments = %v", body)
	}
	first := assignments[0].(map[string]any)
	if first["ticket"] != "HA-J00001" || first["season"] != "2026" || first["group"] != "J" {
		t.Fatalf("assignment = %v", first)
	}

	// 建赛即声明赛季与组别；响应必须回显。
	code, c := postAuth(t, e.url+"/api/v1/contests", e.teacher,
		`{"name":"CSP-J 模拟","season":"2026","ticketGroup":"J"}`)
	if code != 201 {
		t.Fatalf("create contest %d %v", code, c)
	}
	cid := c["id"].(string)
	if c["season"] != "2026" || c["ticketGroup"] != "J" {
		t.Fatalf("contest scope = %v/%v", c["season"], c["ticketGroup"])
	}

	// settings 也能改口径：清空再设回。
	if code, body := requestJSON(t, "PUT", e.url+"/api/v1/contests/"+cid+"/settings",
		e.teacher, `{"season":"","ticketGroup":""}`); code != 200 {
		t.Fatalf("clear settings %d %v", code, body)
	}
	if code, body := requestJSON(t, "PUT", e.url+"/api/v1/contests/"+cid+"/settings",
		e.teacher, `{"season":"2026","ticketGroup":"J"}`); code != 200 {
		t.Fatalf("set settings %d %v", code, body)
	}

	for _, uid := range []string{e.stuID, bobID} {
		if code, body := postAuth(t, e.url+"/api/v1/contests/"+cid+"/participants",
			e.teacher, fmt.Sprintf(`{"userId":%q}`, uid)); code != 200 {
			t.Fatalf("invite %s: %d %v", uid, code, body)
		}
	}

	// 出题带 slug；响应回显 slug。
	code, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"扑克牌","slug":"poker","statement":"题面","input":"输入","output":"输出","constraints":"约束"}`)
	if code != 201 {
		t.Fatalf("create problem %d %v", code, p)
	}
	if p["slug"] != "poker" {
		t.Fatalf("problem slug = %v", p["slug"])
	}
	pid := p["id"].(string)

	// 非法 slug 拒绝。
	if code, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"B","title":"坏","slug":"Poker!","statement":"题面","input":"输入","output":"输出","constraints":"约束"}`); code != 400 {
		t.Fatalf("bad slug should 400, got %d", code)
	}

	if code, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``); code != 200 {
		t.Fatalf("start %d", code)
	}

	submit := fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":%q}`, pid, "// v1")
	if code, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, submit); code != 202 {
		t.Fatalf("submit v1 %d", code)
	}
	submit = fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":%q}`, pid, "// v2 last")
	if code, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, submit); code != 202 {
		t.Fatalf("submit v2 %d", code)
	}

	// 学生无权收题。
	if code, _ := getAuth(t, e.url+"/api/v1/contests/"+cid+"/collect", e.student); code != 403 {
		t.Fatalf("student collect = %d, want 403", code)
	}

	// JSON 口径：清单与告警。
	code, raw := getAuth(t, e.url+"/api/v1/contests/"+cid+"/collect?format=json", e.teacher)
	if code != 200 {
		t.Fatalf("collect json %d", code)
	}
	var pkg struct {
		ContestID string `json:"contestId"`
		Season    string `json:"season"`
		Group     string `json:"group"`
		Entries   []struct {
			Folder   string `json:"folder"`
			FileName string `json:"fileName"`
			Code     string `json:"code"`
			Ticket   string `json:"ticket"`
		} `json:"entries"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatalf("decode collect json: %v", err)
	}
	if pkg.Season != "2026" || pkg.Group != "J" {
		t.Fatalf("package header = %s/%s", pkg.Season, pkg.Group)
	}
	if len(pkg.Entries) != 1 || pkg.Entries[0].Folder != "HA-J00001" ||
		pkg.Entries[0].FileName != "poker.cpp" || pkg.Entries[0].Code != "// v2 last" {
		t.Fatalf("entries = %+v", pkg.Entries)
	}
	warn := strings.Join(pkg.Warnings, "\n")
	if !strings.Contains(warn, "has no J ticket") {
		t.Fatalf("missing-ticket warning expected: %v", pkg.Warnings)
	}

	// zip 口径：目录结构 + manifest.csv。
	code, resp, raw := getAuthResponse(t, e.url+"/api/v1/contests/"+cid+"/collect", e.teacher)
	if code != 200 {
		t.Fatalf("collect zip %d", code)
	}
	if ct := resp.Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("content-type = %q", ct)
	}
	if cd := resp.Get("Content-Disposition"); !strings.Contains(cd, ".zip") {
		t.Fatalf("content-disposition = %q", cd)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(rc)
		_ = rc.Close()
		files[f.Name] = buf.String()
	}
	if got := files["HA-J00001/poker.cpp"]; got != "// v2 last" {
		t.Fatalf("HA-J00001/poker.cpp = %q", got)
	}
	manifest, ok := files["manifest.csv"]
	if !ok {
		t.Fatalf("manifest missing: %v", fileNames(zr))
	}
	for _, want := range []string{"HA-J00001", "poker.cpp", "folder,ticket", "Alice"} {
		if !strings.Contains(manifest, want) {
			t.Fatalf("manifest missing %q: %s", want, manifest)
		}
	}
	if _, ok := files["Bob/poker.cpp"]; ok {
		t.Fatal("Bob submitted nothing and must not have an entry")
	}
}

func fileNames(zr *zip.Reader) []string {
	out := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		out = append(out, f.Name)
	}
	return out
}
