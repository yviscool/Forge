package httpapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/judge"
	"github.com/yviscool/forge/internal/testdata"
)

func parseCSV(t *testing.T, raw string) [][]string {
	t.Helper()
	rows, err := csv.NewReader(strings.NewReader(raw)).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v\n%s", err, raw)
	}
	return rows
}

func putRaw(t *testing.T, url, token, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest("PUT", url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r.Body)
	return r.StatusCode, buf.Bytes()
}

func deleteAuth(t *testing.T, url, token string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r.Body)
	return r.StatusCode, buf.Bytes()
}

func TestUserCSVExportQuotesFieldsAndNeutralizesFormulas(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	u, err := e.svc.CreateUser(`=HYPERLINK("https://example.invalid")`, "student")
	if err != nil {
		t.Fatal(err)
	}
	g, err := e.svc.CreateGroup("+danger,group")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.AddUserToGroup(u.ID, g.ID); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.CreateUser("Doe, Jane\nSecond line", "student")
	if err != nil {
		t.Fatal(err)
	}

	rows := parseCSV(t, e.srv.exportUsers())
	if len(rows) < 4 {
		t.Fatalf("unexpected export rows: %#v", rows)
	}
	seenFormula := false
	seenQuotedName := false
	for _, row := range rows[1:] {
		if len(row) != 3 {
			t.Fatalf("wrong column count: %#v", row)
		}
		if strings.HasPrefix(row[0], "'=") {
			seenFormula = true
		}
		if row[0] == "Doe, Jane\nSecond line" {
			seenQuotedName = true
		}
		for _, cell := range row {
			if strings.HasPrefix(cell, "=") || strings.HasPrefix(cell, "+") ||
				strings.HasPrefix(cell, "@") {
				t.Fatalf("dangerous spreadsheet cell was not neutralized: %q", cell)
			}
		}
	}
	if !seenFormula {
		t.Fatalf("formula prefix missing: %#v", rows)
	}
	if !seenQuotedName {
		t.Fatalf("comma/newline name was not preserved through CSV quoting: %#v", rows)
	}
}

func TestStatisticsCSVNeutralizesNamesAndProblemTitles(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	u, err := e.svc.CreateUser("@SUM(1,1)", "student")
	if err != nil {
		t.Fatal(err)
	}
	c, err := e.svc.CreateContest("Stats", "", "oi")
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: `=WEBSERVICE("https://example.invalid")`,
		Statement: "s", Input: "i", Output: "o", Constraints: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.StartContest(c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Submit(c.ID, p.ID, u.ID, "cpp", "int main() {}"); err != nil {
		t.Fatal(err)
	}
	c, err = e.svc.GetContest(c.ID)
	if err != nil {
		t.Fatal(err)
	}

	rows := parseCSV(t, e.srv.exportStatistics(c))
	if len(rows) < 2 {
		t.Fatalf("unexpected statistics rows: %#v", rows)
	}
	for _, row := range rows {
		for _, cell := range row {
			if strings.HasPrefix(cell, "=") || strings.HasPrefix(cell, "+") ||
				strings.HasPrefix(cell, "-") || strings.HasPrefix(cell, "@") {
				t.Fatalf("dangerous spreadsheet cell was not neutralized: %q", cell)
			}
		}
	}
	if !strings.HasPrefix(rows[1][0], "'@") {
		t.Fatalf("student formula was not neutralized: %#v", rows[1])
	}
}

func TestDeleteProblemRouteAndAuthorization(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Delete"}`)
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	pid := p["id"].(string)
	url := e.url + "/api/v1/contests/" + cid + "/problems/" + pid

	if code, _ := deleteAuth(t, url, e.student); code != http.StatusForbidden {
		t.Fatalf("student delete should 403, got %d", code)
	}
	if code, _ := deleteAuth(t, url, ""); code != http.StatusUnauthorized {
		t.Fatalf("anonymous delete should 401, got %d", code)
	}
	if code, body := deleteAuth(t, url, e.teacher); code != http.StatusOK {
		t.Fatalf("teacher delete %d %s", code, body)
	}
	if code, _ := getAuth(t, url, e.teacher); code != http.StatusNotFound {
		t.Fatalf("deleted problem should be gone, got %d", code)
	}
	if code, _ := deleteAuth(t, url, e.teacher); code != http.StatusNotFound {
		t.Fatalf("second delete should 404, got %d", code)
	}
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid, e.teacher); code != http.StatusOK {
		t.Fatalf("contest detail %d %s", code, body)
	} else {
		var contest map[string]any
		_ = json.Unmarshal(body, &contest)
		if ids, ok := contest["problemIds"].([]any); ok && len(ids) != 0 {
			t.Fatalf("contest still references deleted problem: %s", body)
		}
	}
}

func TestSyncRejectsIncompleteOrEmptyTestData(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	e.srv.Files = testdata.New(t.TempDir())

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Sync"}`)
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	pid := p["id"].(string)
	base := fmt.Sprintf("%s/api/v1/contests/%s/problems/%s/files", e.url, cid, pid)

	if code, _ := postAuth(t, base+"/sync", e.teacher, ``); code != http.StatusBadRequest {
		t.Fatalf("empty inventory sync should 400, got %d", code)
	}
	if code, _ := putRaw(t, base+"/only.in", e.teacher, "1"); code != http.StatusOK {
		t.Fatal("upload incomplete case failed")
	}
	if code, body := postAuth(t, base+"/sync", e.teacher, ``); code != http.StatusBadRequest {
		t.Fatalf("incomplete sync should 400, got %d %s", code, body)
	}
	if code, _ := putRaw(t, base+"/only.ans", e.teacher, "2"); code != http.StatusOK {
		t.Fatal("upload .ans failed")
	}
	if code, body := postAuth(t, base+"/sync", e.teacher, ``); code != http.StatusOK {
		t.Fatalf("complete .ans pair should sync, got %d %s", code, body)
	} else if cases, ok := body["testCases"].([]any); !ok || len(cases) != 1 {
		t.Fatalf(".ans pair should produce one case: %v", body)
	}
}

func TestAnswersOnlySyncAndSubmissionBypassesCodeLanguageGate(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	e.srv.Files = testdata.New(t.TempDir())

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher,
		`{"name":"Answers","allowedLanguages":["cpp"]}`)
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"Answer","statement":"s","input":"i","output":"o","constraints":"k","taskType":"answers_only"}`)
	pid := p["id"].(string)
	base := fmt.Sprintf("%s/api/v1/contests/%s/problems/%s/files", e.url, cid, pid)

	if code, _ := putRaw(t, base+"/key.ans", e.teacher, "42\n"); code != http.StatusOK {
		t.Fatalf("answer key upload: %d", code)
	}
	// A stale .in file from a task-type migration must not block answer sync.
	if code, _ := putRaw(t, base+"/stale.in", e.teacher, "ignored"); code != http.StatusOK {
		t.Fatalf("stale input upload: %d", code)
	}
	if code, body := getAuth(t, base, e.teacher); code != http.StatusOK ||
		!strings.Contains(string(body), `"answerOnly":true`) ||
		!strings.Contains(string(body), `"missingPairs":[]`) {
		t.Fatalf("answer inventory: %d %s", code, body)
	}
	if code, body := postAuth(t, base+"/sync", e.teacher, ``); code != http.StatusOK {
		t.Fatalf("answer sync: %d %s", code, body)
	} else {
		cases := body["testCases"].([]any)
		key := cases[0].(map[string]any)
		if len(cases) != 1 || key["inputFile"] != "" || key["score"].(float64) != 100 {
			t.Fatalf("answer test case: %s", body)
		}
	}

	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)
	submit := fmt.Sprintf(`{"problemID":%q,"language":"python","code":"42"}`, pid)
	if code, body := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, submit); code != http.StatusAccepted {
		t.Fatalf("answer-only submission should bypass code language gate: %d %s", code, body)
	} else if body["language"] != "answer" {
		t.Fatalf("answer submission language: %s", body)
	}

	// Multiple answer keys are ambiguous because one submission is compared
	// against every key. Refuse to synchronize such a paper before launch.
	_, c2 := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Ambiguous"}`)
	cid2 := c2["id"].(string)
	_, p2 := postAuth(t, e.url+"/api/v1/contests/"+cid2+"/problems", e.teacher,
		`{"code":"A","title":"Answer","statement":"s","input":"i","output":"o","constraints":"k","taskType":"answers_only"}`)
	pid2 := p2["id"].(string)
	base2 := fmt.Sprintf("%s/api/v1/contests/%s/problems/%s/files", e.url, cid2, pid2)
	putRaw(t, base2+"/a.ans", e.teacher, "1")
	putRaw(t, base2+"/b.out", e.teacher, "2")
	if code, body := postAuth(t, base2+"/sync", e.teacher, ``); code != http.StatusBadRequest ||
		!strings.Contains(fmt.Sprint(body), "exactly one") {
		t.Fatalf("ambiguous answer keys must fail closed: %d %s", code, body)
	}
}

func TestFilesAndRejudge(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	e.srv.Files = testdata.New(t.TempDir())

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"F"}`)
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	pid := p["id"].(string)

	base := fmt.Sprintf("%s/api/v1/contests/%s/problems/%s/files", e.url, cid, pid)
	if code, _ := putRaw(t, base+"/a01.in", e.student, "3 4"); code != 403 {
		t.Fatalf("student upload should 403, got %d", code)
	}
	if code, _ := putRaw(t, base+"/a01.in", e.teacher, "3 4"); code != 200 {
		t.Fatalf("upload %d", code)
	}
	if code, _ := putRaw(t, base+"/../evil", e.teacher, "x"); code != 400 && code != 404 {
		t.Fatalf("traversal should fail, got %d", code)
	}
	if code, body := getAuth(t, base, e.teacher); code != 200 {
		t.Fatalf("list %d", code)
	} else {
		var v map[string]any
		_ = json.Unmarshal(body, &v)
		if len(v["files"].([]any)) != 1 || len(v["missingPairs"].([]any)) != 1 {
			t.Fatalf("pairs check: %s", body)
		}
	}
	if code, _ := putRaw(t, base+"/a01.out", e.teacher, "7"); code != 200 {
		t.Fatalf("upload out %d", code)
	}
	if code, body := getAuth(t, base, e.teacher); code == 200 {
		var v map[string]any
		_ = json.Unmarshal(body, &v)
		if len(v["missingPairs"].([]any)) != 0 {
			t.Fatalf("pairs complete: %s", body)
		}
	}
	if code, body := getAuth(t, base+"/a01.in", e.teacher); code != 200 || string(body) != "3 4" {
		t.Fatalf("download: %d %s", code, body)
	}
	// Materialize paired files as scored test cases for automatic judging.
	if code, body := postAuth(t, base+"/sync", e.teacher, ``); code != 200 {
		t.Fatalf("sync cases %d %s", code, body)
	} else if cases, ok := body["testCases"].([]any); !ok || len(cases) != 1 {
		t.Fatalf("synced test cases: %v", body)
	} else {
		tc := cases[0].(map[string]any)
		if tc["subtask"].(float64) != 0 || tc["score"].(float64) != 100 {
			t.Fatalf("synced case defaults: %v", tc)
		}
		if !strings.HasSuffix(tc["inputFile"].(string), "a01.in") ||
			!strings.HasSuffix(tc["outputFile"].(string), "a01.out") {
			t.Fatalf("synced paths: %v", tc)
		}
	}
	// Syncing is idempotent and preserves teacher-tuned scores/limits.
	if code, _ := putRaw(t, base+"/a02.in", e.teacher, "5 6"); code != 200 {
		t.Fatal("second input upload failed")
	}
	if code, _ := putRaw(t, base+"/a02.out", e.teacher, "11"); code != 200 {
		t.Fatal("second output upload failed")
	}
	if code, _ := putRaw(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid, e.teacher,
		fmt.Sprintf(`{"testCases":[{"id":"a01","inputFile":%q,"outputFile":%q,"score":37,"subtask":0},{"id":"a02","inputFile":%q,"outputFile":%q,"score":63,"subtask":0}]}`,
			filepath.Join(e.srv.Files.Root(), cid, pid, "a01.in"),
			filepath.Join(e.srv.Files.Root(), cid, pid, "a01.out"),
			filepath.Join(e.srv.Files.Root(), cid, pid, "a02.in"),
			filepath.Join(e.srv.Files.Root(), cid, pid, "a02.out"))); code != 200 {
		t.Fatal("manual test case setup failed")
	}
	if code, body := postAuth(t, base+"/sync", e.teacher, ``); code != 200 {
		t.Fatalf("resync %d %s", code, body)
	} else {
		cases := body["testCases"].([]any)
		if len(cases) != 2 || cases[0].(map[string]any)["score"].(float64) != 37 {
			t.Fatalf("resync must preserve tuned scores: %v", body)
		}
	}

	// 重判：空提交返回 0。
	if code, body := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/rejudge", e.teacher, ``); code != 202 {
		t.Fatalf("rejudge %d %v", code, body)
	} else if int(body["total"].(float64)) != 0 {
		t.Fatalf("empty rejudge: %v", body)
	}
	// Scoring/subtask policy must be finalized while the paper is a draft.
	if code, body := putRaw(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/subtasks", e.teacher,
		`{"subtasks":[{"subtask":0,"score":100,"timeLimitMs":500,"memoryMib":128}]}`); code != 200 {
		t.Fatalf("draft subtasks %d %s", code, body)
	}
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)
	if code, body := putRaw(t, base+"/a03.in", e.teacher, "7 8"); code != http.StatusConflict ||
		!strings.Contains(string(body), "frozen") {
		t.Fatalf("running test-data upload must be frozen: %d %s", code, body)
	}
	if code, body := postAuth(t, base+"/sync", e.teacher, ``); code != http.StatusConflict ||
		!strings.Contains(fmt.Sprint(body), "frozen") {
		t.Fatalf("running test-data sync must be frozen: %d %s", code, body)
	}
	if code, body := deleteAuth(t, base+"/a01.in", e.teacher); code != http.StatusConflict ||
		!strings.Contains(string(body), "frozen") {
		t.Fatalf("running test-data delete must be frozen: %d %s", code, body)
	}
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"x"}`, pid))
	if code, body := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/rejudge", e.teacher, ``); code != 202 || int(body["total"].(float64)) != 1 {
		t.Fatalf("rejudge count: %d %v", code, body)
	}
	if code, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/rejudge", e.student, ``); code != 403 {
		t.Fatalf("student rejudge should 403, got %d", code)
	}

	// 子任务配置 + 统计。
	subBody := `{"subtasks":[{"subtask":0,"score":100,"timeLimitMs":500,"memoryMib":128}]}`
	if code, _ := putRaw(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/subtasks", e.teacher, subBody); code != 400 {
		t.Fatalf("running subtask mutation should be frozen, got %d", code)
	}
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/statistics", e.teacher); code != 200 {
		t.Fatalf("statistics %d", code)
	} else {
		var v map[string]any
		_ = json.Unmarshal(body, &v)
		if int(v["submissions"].(float64)) != 1 {
			t.Fatalf("stats: %s", body)
		}
	}
	if code, _ := getAuth(t, e.url+"/api/v1/contests/"+cid+"/statistics", e.student); code != 403 {
		t.Fatalf("student statistics should 403, got %d", code)
	}
}

func TestRejudgeCountsOnlySuccessfullyEnqueuedSubmissions(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Rejudge queue"}`)
	cid := c["id"].(string)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/participants",
		e.teacher, fmt.Sprintf(`{"userId":%q}`, e.stuID))
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student,
		fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"x"}`, p["id"]))

	e.srv.OnSubmit = func(domain.Submission) error { return judge.ErrQueueFull }
	pid := p["id"].(string)
	code, body := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/rejudge",
		e.teacher, ``)
	if code != http.StatusAccepted || int(body["total"].(float64)) != 1 ||
		int(body["enqueued"].(float64)) != 0 {
		t.Fatalf("rejudge must report real enqueue count: %d %v", code, body)
	}
	subs := e.svc.ListSubmissions(cid)
	if len(subs) != 1 || subs[0].Verdict != domain.VerdictQueued {
		t.Fatalf("failed rejudge enqueue must leave durable queued work: %+v", subs)
	}
}

func TestUsersImportExport(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	csvBody := "name,role,group,password\nDave,student,ClassA,pw-dave-123\nEve,teacher,,pw-eve-1234\nZed,student,,ab\n"
	payload, _ := json.Marshal(map[string]string{"csv": csvBody})
	code, body := postAuth(t, e.url+"/api/v1/users/import", e.teacher, string(payload))
	if code != 200 {
		t.Fatalf("import %d %v", code, body)
	}
	// 教师不能导入 staff；短密码记录也会使该行失败。
	if int(body["created"].(float64)) != 1 || len(body["errors"].([]any)) != 2 {
		t.Fatalf("import result: %v", body)
	}
	if code, _ := postAuth(t, e.url+"/api/v1/users/import", e.student, string(payload)); code != 403 {
		t.Fatalf("student import should 403, got %d", code)
	}
	// Dave 能用导入的密码登录。
	if code, _ := post(t, e.url+"/api/v1/auth/login", `{"login":"Dave","password":"pw-dave-123"}`); code != 200 {
		t.Fatalf("imported login %d", code)
	}
	// 导出含 Dave/ClassA，不含密码。
	if code, data := getAuth(t, e.url+"/api/v1/users/export", e.teacher); code != 200 {
		t.Fatalf("export %d", code)
	} else if !strings.Contains(string(data), "Dave,student,ClassA") || strings.Contains(string(data), "pw-dave-123") {
		t.Fatalf("export content: %s", data)
	}
}

func TestACMRankingEndpoint(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"ACM","rankingMode":"acm"}`)
	if c["rankingMode"] != "acm" {
		t.Fatalf("rankingMode not stored: %v", c)
	}
	if code, _ := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"X","rankingMode":"elo"}`); code != 400 {
		t.Fatalf("bad mode should 400, got %d", code)
	}
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)
	submit := fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"x"}`, p["id"])
	_, sub := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, submit)
	postAuth(t, e.url+"/api/v1/submissions/"+sub["id"].(string)+"/judge", e.teacher, `{"verdict":"accepted","score":100}`)

	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/ranking", e.student); code != 200 {
		t.Fatalf("acm ranking %d", code)
	} else {
		var ranks []map[string]any
		_ = json.Unmarshal(body, &ranks)
		found := false
		for _, r := range ranks {
			if r["userName"] == "Alice" {
				if int(r["solved"].(float64)) != 1 {
					t.Fatalf("acm solved: %s", body)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("alice missing: %s", body)
		}
	}
}

func TestLanguagesAndExport(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	// 建赛即锁 C++。
	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"CSP","allowedLanguages":["cpp"]}`)
	if len(c["allowedLanguages"].([]any)) != 1 {
		t.Fatalf("langs: %v", c)
	}
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	pid := p["id"].(string)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)

	py := fmt.Sprintf(`{"problemID":%q,"language":"python","code":"x"}`, pid)
	if code, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, py); code != 400 {
		t.Fatalf("python should be rejected, got %d", code)
	}
	cpp := fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"int main(){}"}`, pid)
	if code, sub := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, cpp); code != 202 {
		t.Fatalf("cpp submit %d %v", code, sub)
	} else {
		// 教师判 CE 带原文。
		sid := sub["id"].(string)
		postAuth(t, e.url+"/api/v1/submissions/"+sid+"/judge", e.teacher, `{"verdict":"compile_error","score":0}`)
		if _, err := e.svc.Store().GetSubmission(sid); err != nil {
			t.Fatal(err)
		}
	}

	// Settings are frozen once the contest starts; changing them cannot reopen
	// a language gate mid-exam.
	if code, body := putRaw(t, e.url+"/api/v1/contests/"+cid+"/settings", e.teacher,
		`{"allowedLanguages":["cpp","python"]}`); code != 400 {
		t.Fatalf("running settings mutation should be frozen: %d %s", code, body)
	}
	if code, _ := putRaw(t, e.url+"/api/v1/contests/"+cid+"/settings", e.student, `{}`); code != 403 {
		t.Fatalf("student settings should 403, got %d", code)
	}
	if code, _ := putRaw(t, e.url+"/api/v1/contests/"+cid+"/settings", e.teacher, `{"rankingMode":"elo"}`); code != 400 {
		t.Fatalf("bad mode should 400, got %d", code)
	}

	// 成绩单导出。
	if code, data := getAuth(t, e.url+"/api/v1/contests/"+cid+"/statistics/export", e.teacher); code != 200 {
		t.Fatalf("export %d", code)
	} else if !strings.Contains(string(data), "Alice") || !strings.Contains(string(data), "name,total,accepted") {
		t.Fatalf("export content: %s", data)
	}
	if code, _ := getAuth(t, e.url+"/api/v1/contests/"+cid+"/statistics/export", e.student); code != 403 {
		t.Fatalf("student export should 403, got %d", code)
	}
}

func TestBundleExportImport(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"B1"}`)
	cid := c["id"].(string)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)

	if code, _ := getAuth(t, e.url+"/api/v1/contests/"+cid+"/bundle", e.student); code != 403 {
		t.Fatalf("student bundle should 403, got %d", code)
	}
	code, raw := getAuth(t, e.url+"/api/v1/contests/"+cid+"/bundle", e.teacher)
	if code != 200 {
		t.Fatalf("export %d", code)
	}
	var b map[string]any
	_ = json.Unmarshal(raw, &b)
	if b["contest"].(map[string]any)["name"] != "B1" {
		t.Fatalf("bundle: %s", raw)
	}
	if code, body := postAuth(t, e.url+"/api/v1/contests/import", e.teacher, string(raw)); code != 201 {
		t.Fatalf("import %d %v", code, body)
	} else if body["id"] == cid {
		t.Fatal("import must mint new contest id")
	}
}

func TestSpaTraversalSafe(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	// 直接调 handler（绕过 mux 清洗）：任何越界读取不得 200 透出。
	for _, p := range []string{"/app/dist/../../go.mod", "/app/dist/%2e%2e/go.mod"} {
		req := httptest.NewRequest("GET", p, nil)
		rec := httptest.NewRecorder()
		e.srv.ServeHTTP(rec, req)
		if rec.Code == 200 && strings.Contains(rec.Body.String(), "module github") {
			t.Fatalf("traversal leaked: %s", p)
		}
	}
}
