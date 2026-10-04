package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/judge"
	"github.com/yviscool/forge/internal/realtime"
)

func post(t *testing.T, url, body string) (int, map[string]any) {
	t.Helper()
	var r *http.Response
	var err error
	if body == "" {
		r, err = http.Post(url, "application/json", nil)
	} else {
		r, err = http.Post(url, "application/json", bytes.NewBufferString(body))
	}
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(r.Body).Decode(&out)
	return r.StatusCode, out
}

func postAuth(t *testing.T, url, token, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(r.Body).Decode(&out)
	return r.StatusCode, out
}

func getAuth(t *testing.T, url, token string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
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

// testEnv 自带教师 + 学生 token 的测试环境。
type testEnv struct {
	url     string
	svc     *app.Service
	srv     *Server
	teacher string
	student string
	stuID   string
	close   func()
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	svc := app.NewService(memory.New(), realtime.NewHub(), nil, nil)
	srv := NewServer(svc, nil)
	ts := httptest.NewServer(srv)
	e := &testEnv{url: ts.URL, svc: svc, srv: srv, close: ts.Close}
	e.teacher = e.mkUser(t, "Prof", "teacher", "pw-teacher")
	uid, tok := e.mkUserFull(t, "Alice", "student", "pw-alice")
	e.student, e.stuID = tok, uid
	return e
}

func (e *testEnv) mkUserFull(t *testing.T, name, role, pw string) (string, string) {
	t.Helper()
	u, err := e.svc.CreateUser(name, role)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.srv.auth.ResetPassword(u.ID, pw); err != nil {
		t.Fatal(err)
	}
	code, body := post(t, e.url+"/api/v1/auth/login",
		fmt.Sprintf(`{"login":%q,"password":%q}`, name, pw))
	if code != 200 {
		t.Fatalf("login %s: %d %v", name, code, body)
	}
	return u.ID, body["token"].(string)
}

func (e *testEnv) mkUser(t *testing.T, name, role, pw string) string {
	t.Helper()
	_, tok := e.mkUserFull(t, name, role, pw)
	return tok
}

func TestV1ContestLifecycle(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	if code, _ := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":""}`); code != 400 {
		t.Fatalf("empty name should 400, got %d", code)
	}
	if code, _ := post(t, e.url+"/api/v1/contests", `{"name":"x"}`); code != 401 {
		t.Fatalf("anonymous create should 401, got %d", code)
	}
	if code, _ := postAuth(t, e.url+"/api/v1/contests", e.student, `{"name":"x"}`); code != 403 {
		t.Fatalf("student create should 403, got %d", code)
	}
	code, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"CSP 模拟"}`)
	if code != 201 {
		t.Fatalf("create %d", code)
	}
	cid := c["id"].(string)

	code, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"和","statement":"a+b","input":"a b","output":"s","constraints":"k"}`)
	if code != 201 {
		t.Fatalf("create problem %d %v", code, p)
	}
	pid := p["id"].(string)

	submit := fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"x"}`, pid)
	if sc, _ := post(t, e.url+"/api/v1/contests/"+cid+"/submissions", submit); sc != 401 {
		t.Fatalf("anonymous submit should 401, got %d", sc)
	}
	if sc, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, submit); sc != 400 {
		t.Fatalf("draft submit should 400, got %d", sc)
	}
	if sc, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``); sc != 200 {
		t.Fatalf("start %d", sc)
	}
	sc, sub := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, submit)
	if sc != 202 {
		t.Fatalf("submit %d %v", sc, sub)
	}
	sid := sub["id"].(string)
	if sub["userId"] != e.stuID {
		t.Fatalf("submitter must be session user: %v", sub)
	}
	if sc, _ := postAuth(t, e.url+"/api/v1/submissions/"+sid+"/judge", e.student, `{"verdict":"accepted","score":100}`); sc != 403 {
		t.Fatalf("student judge should 403, got %d", sc)
	}
	if sc, _ := postAuth(t, e.url+"/api/v1/submissions/"+sid+"/judge", e.teacher, `{"verdict":"accepted","score":100}`); sc != 200 {
		t.Fatalf("judge %d", sc)
	}

	r, err := http.Get(e.url + "/api/v1/contests/" + cid + "/ranking")
	if err != nil || r.StatusCode != 200 {
		t.Fatal("ranking failed")
	}
	var ranks []map[string]any
	_ = json.NewDecoder(r.Body).Decode(&ranks)
	r.Body.Close()
	hit := false
	for _, rk := range ranks {
		if rk["userName"] == "Alice" && rk["score"].(float64) == 100 {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("ranking wrong: %v", ranks)
	}

	if sc, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/validate", e.teacher, ``); sc != 200 {
		t.Fatalf("validate %d", sc)
	}

	for _, p := range []string{"/healthz", "/readyz", "/api/v1/i18n"} {
		rr, err := http.Get(e.url + p)
		if err != nil || rr.StatusCode != 200 {
			t.Fatalf("%s -> %v %v", p, rr, err)
		}
		rr.Body.Close()
	}
	rr, err := http.Get(e.url + "/app")
	if err != nil {
		t.Fatal(err)
	}
	defer rr.Body.Close()
	if rr.StatusCode == 200 {
		return // built bundle served
	}
	if rr.StatusCode != 404 {
		t.Fatalf("/app -> %d, want 200 or 404", rr.StatusCode)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(rr.Body)
	if !bytes.Contains(buf.Bytes(), []byte("npm run build")) {
		t.Fatalf("/app 404 should hint rebuild, got %q", buf.String())
	}
}

func TestAuthFlows(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	// me / logout
	if code, _ := getAuth(t, e.url+"/api/v1/auth/me", ""); code != 401 {
		t.Fatalf("anonymous me should 401, got %d", code)
	}
	if code, body := getAuth(t, e.url+"/api/v1/auth/me", e.student); code != 200 {
		t.Fatalf("me %d %s", code, body)
	}
	if code, _ := post(t, e.url+"/api/v1/auth/logout", ``); code != 401 {
		t.Fatalf("anonymous logout should 401, got %d", code)
	}
	if code, _ := postAuth(t, e.url+"/api/v1/auth/logout", e.student, ``); code != 200 {
		t.Fatalf("logout %d", code)
	}
	if code, _ := getAuth(t, e.url+"/api/v1/auth/me", e.student); code != 401 {
		t.Fatalf("logged-out token should 401, got %d", code)
	}

	// 重新登录：改密（错旧密码/成功）与教师重置。
	_, e.student = e.mkUserFull(t, "Alice", "student", "pw-alice")
	if code, _ := postAuth(t, e.url+"/api/v1/auth/password", e.student,
		`{"oldPassword":"nope","newPassword":"newpw12"}`); code != 400 {
		t.Fatalf("wrong old password should 400, got %d", code)
	}
	if code, _ := postAuth(t, e.url+"/api/v1/auth/password", e.student,
		`{"oldPassword":"pw-alice","newPassword":"newpw12"}`); code != 200 {
		t.Fatalf("change password %d", code)
	}
	if code, _ := post(t, e.url+"/api/v1/auth/login",
		`{"login":"Alice","password":"newpw12"}`); code != 200 {
		t.Fatalf("login with new password %d", code)
	}
	// 改密后旧 token 失效。
	if code, _ := getAuth(t, e.url+"/api/v1/auth/me", e.student); code != 401 {
		t.Fatalf("pre-change token should 401, got %d", code)
	}
}

func TestSubmissionCodeVisibility(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	bobID, bobTok := e.mkUserFull(t, "Bob", "student", "pw-bob")

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Vis"}`)
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)
	submit := fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"secret-code"}`, p["id"])
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, submit)

	// 挂一条 CE 原文，校验 peer 连编译信息一起脱敏。
	subs := e.svc.ListSubmissions(cid)
	_, _ = e.svc.JudgeCases(subs[0].ID, domain.JudgeOutcome{
		Cases:          []domain.CaseResult{{CaseIndex: 0, Verdict: domain.CaseCE}},
		CompileMessage: "main.cpp:1: error",
	}, func(int) int { return 0 }, func(int) int { return 100 })

	// Bob 看不到 Alice 的代码，教师全见。
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", bobTok); code != 200 {
		t.Fatalf("list %d", code)
	} else {
		var subs []map[string]any
		_ = json.Unmarshal(body, &subs)
		if len(subs) != 1 || subs[0]["code"] != "" || subs[0]["compileMessage"] != nil {
			t.Fatalf("peer code+ce must be redacted: %s", body)
		}
		_ = bobID
	}
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.teacher); code != 200 {
		t.Fatalf("teacher list %d", code)
	} else {
		var subs []map[string]any
		_ = json.Unmarshal(body, &subs)
		if len(subs) != 1 || subs[0]["code"] != "secret-code" || subs[0]["compileMessage"] != "main.cpp:1: error" {
			t.Fatalf("teacher must see code+ce: %s", body)
		}
	}
}

const pipelineCode = `#include <bits/stdc++.h>
using namespace std;
int main(){long long a,b;if(!(cin>>a>>b))return 0;cout<<a+b;return 0;}
`

// TestAutoJudgePipeline 提交→worker→JudgeCases 全链路（需 g++）。
func TestAutoJudgePipeline(t *testing.T) {
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("g++ not found")
	}
	store := memory.New()
	svc := app.NewService(store, realtime.NewHub(), nil, nil)
	srv := NewServer(svc, nil)

	ctx, cancel := context.WithCancel(context.Background())
	pool := judge.AutoJudgePool(svc, judge.Toolchain{})
	defer func() { cancel(); pool.Wait() }()
	pool.Start(ctx, 1)
	srv.OnSubmit = func(x domain.Submission) {
		p, err := svc.GetProblem(x.ContestID, x.ProblemID)
		if err != nil {
			return
		}
		if req, ok := judge.BuildRequest(x, p); ok {
			pool.Submit(judge.Job{SubID: x.ID, Req: req})
		}
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	u, err := svc.CreateUser("Auto", "student")
	if err != nil {
		t.Fatal(err)
	}
	tu, err := svc.CreateUser("T", "teacher")
	if err != nil {
		t.Fatal(err)
	}
	_ = srv.auth.ResetPassword(u.ID, "pw-auto")
	_ = srv.auth.ResetPassword(tu.ID, "pw-t")
	login := func(name, pw string) string {
		code, body := post(t, ts.URL+"/api/v1/auth/login",
			fmt.Sprintf(`{"login":%q,"password":%q}`, name, pw))
		if code != 200 {
			t.Fatalf("login: %d %v", code, body)
		}
		return body["token"].(string)
	}
	tTeacher := login("T", "pw-t")
	tStudent := login("Auto", "pw-auto")

	_, c := postAuth(t, ts.URL+"/api/v1/contests", tTeacher, `{"name":"Pipe"}`)
	cid := c["id"].(string)
	probBody, _ := json.Marshal(map[string]any{
		"code": "A", "title": "plus", "statement": "s", "input": "i",
		"output": "o", "constraints": "k", "timeLimitMs": 2000, "memoryLimitMib": 256,
		"testCases": []map[string]any{
			{"id": "t1", "inputFile": "3 4", "outputFile": "7", "score": 50, "subtask": 0},
			{"id": "t2", "inputFile": "0 0", "outputFile": "0", "score": 50, "subtask": 1},
		},
	})
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/contests/"+cid+"/problems", bytes.NewReader(probBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tTeacher)
	r, err := http.DefaultClient.Do(req)
	if err != nil || r.StatusCode != 201 {
		t.Fatalf("create problem: %v %v", r, err)
	}
	var p map[string]any
	_ = json.NewDecoder(r.Body).Decode(&p)
	r.Body.Close()
	postAuth(t, ts.URL+"/api/v1/contests/"+cid+"/start", tTeacher, ``)

	codeBody, _ := json.Marshal(map[string]any{
		"problemID": p["id"], "language": "cpp", "code": pipelineCode,
	})
	rr, err := func() (*http.Response, error) {
		rq, _ := http.NewRequest("POST", ts.URL+"/api/v1/contests/"+cid+"/submissions", bytes.NewReader(codeBody))
		rq.Header.Set("Content-Type", "application/json")
		rq.Header.Set("Authorization", "Bearer "+tStudent)
		return http.DefaultClient.Do(rq)
	}()
	if err != nil || rr.StatusCode != 202 {
		t.Fatalf("submit: %v %v", rr, err)
	}
	var sub map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&sub)
	rr.Body.Close()
	sid := sub["id"].(string)

	deadline := time.Now().Add(60 * time.Second)
	for {
		x, err := svc.GetSubmission(sid)
		if err != nil {
			t.Fatal(err)
		}
		if x.Verdict == "accepted" && x.Score == 100 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("autojudge timed out: %+v", x)
		}
		time.Sleep(200 * time.Millisecond)
	}
	found := false
	for _, r := range svc.Ranking(cid) {
		if r.UserName == "Auto" && r.Score == 100 {
			found = true
		}
	}
	if !found {
		t.Fatalf("ranking: %+v", svc.Ranking(cid))
	}
}
