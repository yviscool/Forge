package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/judge"
	"github.com/yviscool/forge/internal/ports"
	"github.com/yviscool/forge/internal/realtime"
)

type unhealthyStore struct{ ports.Store }

func (unhealthyStore) Ping(context.Context) error { return errors.New("database unavailable") }

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
	return requestJSON(t, http.MethodPost, url, token, body)
}

func requestJSONWithHeaders(
	t *testing.T,
	method, url, token, body string,
	headers map[string]string,
) (int, http.Header, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(r.Body).Decode(&out)
	return r.StatusCode, r.Header.Clone(), out
}

func requestJSON(t *testing.T, method, url, token, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
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

func loginForToken(t *testing.T, url, login, password string) (int, map[string]any, string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"login": login, "password": password})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, url+"/api/v1/auth/login",
		bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	for _, cookie := range resp.Cookies() {
		if cookie.Name == sessionCookieName {
			return resp.StatusCode, out, cookie.Value
		}
	}
	return resp.StatusCode, out, ""
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

func getAuthResponse(t *testing.T, url, token string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
	return r.StatusCode, r.Header.Clone(), buf.Bytes()
}

func newCookieClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

func postWithClient(t *testing.T, client *http.Client, url, token, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(r.Body).Decode(&out)
	return r.StatusCode, out
}

func getWithClient(t *testing.T, client *http.Client, url, token string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r, err := client.Do(req)
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
	e.teacher = e.mkUser(t, "Prof", "teacher", "pw-teacher-1")
	uid, tok := e.mkUserFull(t, "Alice", "student", "pw-alice-1")
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
	code, body, token := loginForToken(t, e.url, name, pw)
	if code != 200 {
		t.Fatalf("login %s: %d %v", name, code, body)
	}
	if token == "" {
		t.Fatalf("login %s did not issue a session cookie", name)
	}
	return u.ID, token
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
	// Draft contests are visible only to explicitly invited students. Invite
	// Alice so this test reaches the lifecycle rule (draft => submission 400)
	// rather than being rejected earlier by contest access control.
	if code, body := postAuth(t, e.url+"/api/v1/contests/"+cid+"/participants",
		e.teacher, fmt.Sprintf(`{"userId":%q}`, e.stuID)); code != 200 {
		t.Fatalf("invite student: %d %v", code, body)
	}

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
		t.Fatalf("draft submit by invited student should 400, got %d", sc)
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

	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/ranking", e.student); code != 200 {
		t.Fatalf("ranking failed: %d %s", code, body)
	}
	var ranks []map[string]any
	_, rankBody := getAuth(t, e.url+"/api/v1/contests/"+cid+"/ranking", e.student)
	_ = json.Unmarshal(rankBody, &ranks)
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

func TestQueuedSubmissionSurvivesEnqueueFailureAndReplayRetries(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	enqueueCalls := 0
	e.srv.OnSubmit = func(domain.Submission) error {
		enqueueCalls++
		if enqueueCalls == 1 {
			return judge.ErrQueueFull
		}
		return nil
	}

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Queue"}`)
	cid := c["id"].(string)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/participants",
		e.teacher, fmt.Sprintf(`{"userId":%q}`, e.stuID))
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)

	body := fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"x"}`, p["id"])
	headers := map[string]string{"Idempotency-Key": "queue-retry-request"}
	code, firstHeaders, first := requestJSONWithHeaders(
		t, http.MethodPost, e.url+"/api/v1/contests/"+cid+"/submissions",
		e.student, body, headers,
	)
	if code != http.StatusAccepted || first["verdict"] != domain.VerdictQueued {
		t.Fatalf("persisted submission must stay accepted while queued: %d headers=%v body=%v",
			code, firstHeaders, first)
	}
	code, replayHeaders, replay := requestJSONWithHeaders(
		t, http.MethodPost, e.url+"/api/v1/contests/"+cid+"/submissions",
		e.student, body, headers,
	)
	if code != http.StatusAccepted || replayHeaders.Get("Idempotent-Replay") != "true" {
		t.Fatalf("queued replay must retry enqueue: %d headers=%v body=%v",
			code, replayHeaders, replay)
	}
	if enqueueCalls != 2 {
		t.Fatalf("enqueue attempts = %d, want 2", enqueueCalls)
	}
	subs, err := e.svc.Store().ListSubmissions(cid)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || subs[0].Verdict != domain.VerdictQueued ||
		subs[0].Score != 0 || !subs[0].JudgedAt.IsZero() {
		t.Fatalf("enqueue failure must leave durable queued work: %+v", subs)
	}
	resp, err := http.Get(e.url + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var metrics bytes.Buffer
	_, _ = metrics.ReadFrom(resp.Body)
	if !strings.Contains(metrics.String(), "forge_judge_enqueue_failures_total 1") {
		t.Fatalf("enqueue failure metric missing:\n%s", metrics.String())
	}
}

func TestProblemIDsAreServerGeneratedAndContestScoped(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	createContest := func(name string) string {
		t.Helper()
		code, body := postAuth(t, e.url+"/api/v1/contests", e.teacher,
			fmt.Sprintf(`{"name":%q}`, name))
		if code != http.StatusCreated {
			t.Fatalf("create contest %s = %d %v", name, code, body)
		}
		return body["id"].(string)
	}
	c1 := createContest("Contest One")
	c2 := createContest("Contest Two")

	code, victim := postAuth(t, e.url+"/api/v1/contests/"+c2+"/problems", e.teacher,
		`{"code":"A","title":"Victim","statement":"statement","input":"in","output":"out","constraints":"none"}`)
	if code != http.StatusCreated {
		t.Fatalf("create victim = %d %v", code, victim)
	}
	victimID := victim["id"].(string)

	hostileID := `..\\..\\forge`
	code, created := postAuth(t, e.url+"/api/v1/contests/"+c1+"/problems", e.teacher,
		fmt.Sprintf(`{"id":%q,"code":"A","title":"Safe","statement":"statement","input":"in","output":"out","constraints":"none"}`, hostileID))
	if code != http.StatusCreated {
		t.Fatalf("create safe problem = %d %v", code, created)
	}
	if created["id"] == hostileID || created["id"] == "" {
		t.Fatalf("server must generate a safe problem id, got %v", created["id"])
	}
	if _, err := e.svc.GetProblem(c2, victimID); err != nil {
		t.Fatal("creating in another contest overwrote the victim")
	}

	code, body := requestJSON(t, http.MethodPut,
		e.url+"/api/v1/contests/"+c1+"/problems/reorder", e.teacher,
		fmt.Sprintf(`{"problemIds":[%q]}`, victimID))
	if code != http.StatusBadRequest {
		t.Fatalf("foreign problem reorder = %d %v, want 400", code, body)
	}
	if got, err := e.svc.GetProblem(c2, victimID); err != nil || got.Code != "A" {
		t.Fatalf("foreign reorder mutated victim: %+v %v", got, err)
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
	// 注：用户名必须全局唯一（登录按 username 精确匹配），这里用独立用户名
	// 避免与环境预置的 Alice 冲突（map 遍历顺序随机，曾导致 CI 偶发 401）。
	_, e.student = e.mkUserFull(t, "Alicia", "student", "pw-alicia-1")
	if code, _ := postAuth(t, e.url+"/api/v1/auth/password", e.student,
		`{"oldPassword":"nope","newPassword":"new-password-12"}`); code != 400 {
		t.Fatalf("wrong old password should 400, got %d", code)
	}
	if code, _ := postAuth(t, e.url+"/api/v1/auth/password", e.student,
		`{"oldPassword":"pw-alicia-1","newPassword":"new-password-12"}`); code != 200 {
		t.Fatalf("change password %d", code)
	}
	if code, _ := post(t, e.url+"/api/v1/auth/login",
		`{"login":"Alicia","password":"new-password-12"}`); code != 200 {
		t.Fatalf("login with new password %d", code)
	}
	// 改密后旧 token 失效。
	if code, _ := getAuth(t, e.url+"/api/v1/auth/me", e.student); code != 401 {
		t.Fatalf("pre-change token should 401, got %d", code)
	}
}

func TestTeacherCannotManageStaffAccounts(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	admin, err := e.svc.CreateUser("Root Admin", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.srv.auth.ResetPassword(admin.ID, "admin-password-123"); err != nil {
		t.Fatal(err)
	}
	_, adminToken := e.mkUserFull(t, "Second Admin", "admin", "second-admin-password")

	if code, body := postAuth(t, e.url+"/api/v1/users", e.teacher,
		`{"name":"Escalated Admin","username":"esc_admin","role":"admin","password":"long-password-1"}`); code != http.StatusForbidden {
		t.Fatalf("teacher create admin = %d %v, want 403", code, body)
	}
	if code, body := postAuth(t, e.url+"/api/v1/users", e.teacher,
		`{"name":"Escalated Teacher","username":"esc_teacher","role":"teacher","password":"long-password-2"}`); code != http.StatusForbidden {
		t.Fatalf("teacher create teacher = %d %v, want 403", code, body)
	}
	if code, body := postAuth(t, e.url+"/api/v1/users", e.teacher,
		`{"name":"New Student","username":"new_student","role":"student","password":"long-password-3"}`); code != http.StatusCreated {
		t.Fatalf("teacher create student = %d %v, want 201", code, body)
	}
	if code, body := postAuth(t, e.url+"/api/v1/users/"+admin.ID+"/password", e.teacher,
		`{"password":"takeover-password"}`); code != http.StatusForbidden {
		t.Fatalf("teacher reset admin password = %d %v, want 403", code, body)
	}
	if code, body := postAuth(t, e.url+"/api/v1/users/import", e.teacher,
		`{"csv":"name,role,password\nCSV Admin,admin,csv-admin-password\nCSV Student,student,csv-student-pw\n"}`); code != http.StatusOK {
		t.Fatalf("teacher CSV import = %d %v, want 200", code, body)
	} else if created := body["created"].(float64); created != 1 {
		t.Fatalf("teacher CSV created = %v, want 1 (student only)", created)
	}

	if code, body := postAuth(t, e.url+"/api/v1/users/"+admin.ID+"/password", adminToken,
		`{"password":"rotated-admin-password"}`); code != http.StatusOK {
		t.Fatalf("admin reset admin password = %d %v, want 200", code, body)
	}
}

func TestExistingSessionUsesCurrentRole(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	uid, token := e.mkUserFull(t, "Former Teacher", "teacher", "teacher-password-1")
	u, err := e.svc.Store().GetUser(uid)
	if err != nil {
		t.Fatal(err)
	}
	u.Role = domain.RoleStudent
	if err := e.svc.Store().UpdateUser(u); err != nil {
		t.Fatal(err)
	}

	if code, body := postAuth(t, e.url+"/api/v1/contests", token,
		`{"name":"stale privilege"}`); code != http.StatusForbidden {
		t.Fatalf("demoted session created contest: %d %v", code, body)
	}
	if code, body := getAuth(t, e.url+"/api/v1/auth/me", token); code != http.StatusOK {
		t.Fatalf("me after demotion = %d %s", code, body)
	}
}

func TestBrowserLoginResponseDoesNotExposeBearerToken(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	code, body, token := loginForToken(t, e.url, "Prof", "pw-teacher-1")
	if code != http.StatusOK || token == "" {
		t.Fatalf("login = %d token=%q body=%v", code, token, body)
	}
	if _, ok := body["token"]; ok {
		t.Fatalf("browser login must not expose a bearer token: %v", body)
	}
}

func TestHttpOnlySessionCookieAuthAndLogout(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	client := newCookieClient(t)
	payload := bytes.NewBufferString(`{"login":"Prof","password":"pw-teacher-1"}`)
	req, err := http.NewRequest(http.MethodPost, e.url+"/api/v1/auth/login", payload)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cookie login: %d", resp.StatusCode)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == sessionCookieName {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("login did not issue an HttpOnly session cookie")
	}
	if !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteStrictMode ||
		sessionCookie.Path != "/api/v1" {
		t.Fatalf("insecure session cookie: %+v", sessionCookie)
	}
	if sessionCookie.MaxAge != int(auth.SessionTTL/time.Second) {
		t.Fatalf("session cookie MaxAge = %d", sessionCookie.MaxAge)
	}

	if code, body := getWithClient(t, client, e.url+"/api/v1/auth/me", ""); code != 200 {
		t.Fatalf("cookie-only /auth/me: %d %s", code, body)
	}
	if code, body := postWithClient(t, client, e.url+"/api/v1/contests", "", `{"name":"Cookie"}`); code != 201 {
		t.Fatalf("cookie-authenticated create: %d %v", code, body)
	}
	if code, body := postWithClient(t, client, e.url+"/api/v1/auth/logout", "", ""); code != 200 {
		t.Fatalf("cookie logout: %d %v", code, body)
	}
	if code, _ := getWithClient(t, client, e.url+"/api/v1/auth/me", ""); code != 401 {
		t.Fatalf("logged-out cookie should 401, got %d", code)
	}
}

func TestSessionCookieSecureBehindTLSProxy(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	e.srv.TrustProxy = true

	req, err := http.NewRequest(http.MethodPost, e.url+"/api/v1/auth/login",
		bytes.NewBufferString(`{"login":"Prof","password":"pw-teacher-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Strict-Transport-Security"); got == "" {
		t.Fatal("trusted HTTPS proxy request must enable HSTS")
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == sessionCookieName {
			if !cookie.Secure {
				t.Fatal("TLS-terminated deployment must issue a Secure session cookie")
			}
			return
		}
	}
	t.Fatal("session cookie missing")
}

func TestForwardedHTTPSIgnoredWithoutTrustedProxy(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	req, err := http.NewRequest(http.MethodPost, e.url+"/api/v1/auth/login",
		bytes.NewBufferString(`{"login":"Prof","password":"pw-teacher-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Strict-Transport-Security") != "" {
		t.Fatal("untrusted forwarded proto must not enable HSTS")
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == sessionCookieName && cookie.Secure {
			t.Fatal("untrusted forwarded proto must not mark the cookie Secure")
		}
	}
}

func TestCrossOriginMutationIsRejected(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	req, err := http.NewRequest(http.MethodPost, e.url+"/api/v1/auth/login",
		bytes.NewBufferString(`{"login":"Prof","password":"pw-teacher-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://attacker.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin POST = %d, want 403", resp.StatusCode)
	}

	sameOrigin, err := http.NewRequest(http.MethodPost, e.url+"/api/v1/auth/login",
		bytes.NewBufferString(`{"login":"Prof","password":"pw-teacher-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	sameOrigin.Header.Set("Content-Type", "application/json")
	sameOrigin.Header.Set("Origin", e.url)
	resp, err = http.DefaultClient.Do(sameOrigin)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("same-origin POST = %d, want 200", resp.StatusCode)
	}
}

func TestLoginRateLimitReturnsRetryAfter(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	payload := fmt.Sprintf(`{"login":"Alice","password":%q}`, "definitely-wrong")
	var lastStatus int
	var retryAfter string
	for i := 0; i < 7; i++ {
		req, err := http.NewRequest(http.MethodPost, e.url+"/api/v1/auth/login",
			bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		lastStatus = resp.StatusCode
		retryAfter = resp.Header.Get("Retry-After")
	}
	if lastStatus != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after repeated bad passwords, got %d", lastStatus)
	}
	if retryAfter == "" {
		t.Fatal("rate-limited response must include Retry-After")
	}

	// A successful login for a different account from the same test client
	// must still work until the per-IP budget is exhausted.
	if code, body := post(t, e.url+"/api/v1/auth/login",
		fmt.Sprintf(`{"login":"Prof","password":%q}`, "pw-teacher-1")); code != 200 {
		t.Fatalf("other account should remain usable: %d %v", code, body)
	}
}

func TestSubmissionFeedLimitAndStudentIsolation(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	_, bobTok := e.mkUserFull(t, "Bob", "student", "pw-bob-1234")

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Feed"}`)
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	pid := p["id"].(string)
	if code, body := postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``); code != http.StatusOK {
		t.Fatalf("start contest: %d %v", code, body)
	}

	submit := func(token, code string) map[string]any {
		t.Helper()
		payload := fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":%q}`, pid, code)
		status, body := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", token, payload)
		if status != http.StatusAccepted {
			t.Fatalf("submit %q: %d %v", code, status, body)
		}
		return body
	}

	first := submit(e.student, "alice-older")
	time.Sleep(time.Millisecond)
	second := submit(e.student, "alice-newer")
	time.Sleep(time.Millisecond)
	bob := submit(bobTok, "bob-secret")

	decodeFeed := func(body []byte) []map[string]any {
		t.Helper()
		var feed []map[string]any
		if err := json.Unmarshal(body, &feed); err != nil {
			t.Fatalf("decode submission feed: %v: %s", err, body)
		}
		return feed
	}

	status, headers, body := getAuthResponse(
		t, e.url+"/api/v1/contests/"+cid+"/submissions?limit=1", e.student,
	)
	if status != http.StatusOK || headers.Get("X-Result-Limit") != "1" {
		t.Fatalf("student limited feed: %d headers=%v body=%s", status, headers, body)
	}
	feed := decodeFeed(body)
	if len(feed) != 1 || feed[0]["id"] != second["id"] || feed[0]["code"] != "alice-newer" {
		t.Fatalf("student limited feed = %s, want latest Alice submission %v", body, second["id"])
	}

	status, _, body = getAuthResponse(
		t, e.url+"/api/v1/contests/"+cid+"/submissions?limit=10", e.student,
	)
	if status != http.StatusOK {
		t.Fatalf("student feed: %d %s", status, body)
	}
	feed = decodeFeed(body)
	if len(feed) != 2 || feed[0]["id"] != second["id"] || feed[1]["id"] != first["id"] {
		t.Fatalf("student feed leaked peer submissions or lost order: %s", body)
	}
	for _, item := range feed {
		if item["id"] == bob["id"] || item["code"] == "bob-secret" {
			t.Fatalf("student feed leaked Bob submission: %s", body)
		}
	}

	status, headers, body = getAuthResponse(
		t, e.url+"/api/v1/contests/"+cid+"/submissions?limit=999", e.teacher,
	)
	if status != http.StatusOK || headers.Get("X-Result-Limit") != "500" {
		t.Fatalf("teacher capped feed: %d headers=%v body=%s", status, headers, body)
	}
	feed = decodeFeed(body)
	if len(feed) != 3 || feed[0]["id"] != bob["id"] {
		t.Fatalf("teacher feed = %s, want newest submission first", body)
	}

	status, _, body = getAuthResponse(
		t, e.url+"/api/v1/contests/"+cid+"/submissions?limit=0", e.student,
	)
	if status != http.StatusBadRequest {
		t.Fatalf("zero limit: %d %s, want 400", status, body)
	}
}

func TestSubmissionCodeVisibility(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	bobID, bobTok := e.mkUserFull(t, "Bob", "student", "pw-bob-1234")

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
	}, func(int) int { return 0 }, func(int) int { return 100 }, nil)

	// Students receive their own history only; peers do not leak timestamps,
	// verdicts, scores, source, or compile output. Teachers see the full feed.
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", bobTok); code != 200 {
		t.Fatalf("list %d", code)
	} else {
		var subs []map[string]any
		_ = json.Unmarshal(body, &subs)
		if len(subs) != 0 {
			t.Fatalf("peer submissions must not leak to another student: %s", body)
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

const pipelineCode = `#include <iostream>
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
	pool := app.AutoJudgePool(svc, judge.Toolchain{})
	defer func() { cancel(); pool.Wait() }()
	pool.Start(ctx, 1)
	srv.OnSubmit = func(x domain.Submission) error {
		p, err := svc.GetProblem(x.ContestID, x.ProblemID)
		if err != nil {
			return err
		}
		if req, ok := judge.BuildRequest(x, p); ok {
			if !pool.Submit(judge.Job{SubID: x.ID, Req: req}) {
				return judge.ErrQueueFull
			}
		}
		return nil
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
	_ = srv.auth.ResetPassword(u.ID, "pw-auto-1234")
	_ = srv.auth.ResetPassword(tu.ID, "pw-teacher-2")
	login := func(name, pw string) string {
		code, body, token := loginForToken(t, ts.URL, name, pw)
		if code != 200 {
			t.Fatalf("login: %d %v", code, body)
		}
		if token == "" {
			t.Fatal("login did not issue a session cookie")
		}
		return token
	}
	tTeacher := login("T", "pw-teacher-2")
	tStudent := login("Auto", "pw-auto-1234")

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

func TestReadinessReflectsStoreHealth(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		e := newTestEnv(t)
		defer e.close()
		code, body := getAuth(t, e.url+"/readyz", "")
		if code != http.StatusOK || !strings.Contains(string(body), `"status":"ready"`) {
			t.Fatalf("healthy readiness: %d %s", code, body)
		}
	})

	t.Run("unhealthy", func(t *testing.T) {
		svc := app.NewService(unhealthyStore{Store: memory.New()}, realtime.NewHub(), nil, nil)
		httpSrv := httptest.NewServer(NewServer(svc, nil))
		defer httpSrv.Close()
		code, body := getAuth(t, httpSrv.URL+"/readyz", "")
		if code != http.StatusServiceUnavailable || !strings.Contains(string(body), `"status":"unavailable"`) {
			t.Fatalf("unhealthy readiness: %d %s", code, body)
		}
	})
}

func TestReadinessRunsInjectedOperationalChecks(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	e.srv.AddReadinessCheck("disk", func(context.Context) error {
		return errors.New("disk full")
	})
	code, body := getAuth(t, e.url+"/readyz", "")
	if code != http.StatusServiceUnavailable ||
		!strings.Contains(string(body), `"disk":"unavailable"`) ||
		!strings.Contains(string(body), `"store":"ok"`) {
		t.Fatalf("failing operational check was not reported: %d %s", code, body)
	}

	e.srv.AddReadinessCheck("disk", func(context.Context) error { return nil })
	code, body = getAuth(t, e.url+"/readyz", "")
	if code != http.StatusOK || !strings.Contains(string(body), `"disk":"ok"`) {
		t.Fatalf("recovered operational check: %d %s", code, body)
	}
}

func TestSecurityHeadersAndSPACachePolicy(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	req, err := http.NewRequest(http.MethodGet, e.url+"/app", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'none'") ||
		!strings.Contains(got, "object-src 'none'") {
		t.Fatalf("missing hardened CSP: %q", got)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := resp.Header.Get("Cross-Origin-Resource-Policy"); got != "same-origin" {
		t.Fatalf("Cross-Origin-Resource-Policy = %q", got)
	}
	if resp.StatusCode == http.StatusOK {
		if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("SPA index cache policy = %q", got)
		}
		if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
			t.Fatalf("SPA index content type = %q", got)
		}
	}
}

func TestSubmissionIdempotencyHeaderReplaysWithoutDuplicate(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Idempotent"}`)
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"Sum","statement":"s","input":"i","output":"o","constraints":"k"}`)
	pid := p["id"].(string)
	if code, body := postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``); code != 200 {
		t.Fatalf("start contest: %d %v", code, body)
	}

	if code, _, _ := requestJSONWithHeaders(
		t, http.MethodPost, e.url+"/api/v1/contests/"+cid+"/submissions",
		e.student, `{}`, map[string]string{"Idempotency-Key": "short"},
	); code != http.StatusBadRequest {
		t.Fatalf("invalid idempotency key: %d, want 400", code)
	}

	body := fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"int main(){}"}`, pid)
	headers := map[string]string{"Idempotency-Key": "browser-request-1"}
	code, firstHeaders, first := requestJSONWithHeaders(
		t, http.MethodPost, e.url+"/api/v1/contests/"+cid+"/submissions",
		e.student, body, headers,
	)
	if code != http.StatusAccepted || firstHeaders.Get("Idempotent-Replay") != "" {
		t.Fatalf("first submit: %d headers=%v body=%v", code, firstHeaders, first)
	}

	code, replayHeaders, replay := requestJSONWithHeaders(
		t, http.MethodPost, e.url+"/api/v1/contests/"+cid+"/submissions",
		e.student, body, headers,
	)
	if code != http.StatusAccepted || replayHeaders.Get("Idempotent-Replay") != "true" {
		t.Fatalf("replay submit: %d headers=%v body=%v", code, replayHeaders, replay)
	}
	if replay["id"] != first["id"] {
		t.Fatalf("replay id = %v, want %v", replay["id"], first["id"])
	}
	subs, err := e.svc.Store().ListSubmissions(cid)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("stored submissions = %d, want 1", len(subs))
	}

	conflicting := fmt.Sprintf(
		`{"problemID":%q,"language":"cpp","code":"int main(){return 0;}"}`, pid,
	)
	code, _, conflict := requestJSONWithHeaders(
		t, http.MethodPost, e.url+"/api/v1/contests/"+cid+"/submissions",
		e.student, conflicting, headers,
	)
	if code != http.StatusConflict {
		t.Fatalf("conflicting idempotency key: %d %v", code, conflict)
	}
}

func TestMetricsEndpointReportsBoundedHTTPRoutes(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Metrics"}`)
	cid := c["id"].(string)
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid, e.teacher); code != http.StatusOK {
		t.Fatalf("load contest: %d %s", code, body)
	}

	req, err := http.NewRequest(http.MethodGet, e.url+"/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body bytes.Buffer
	_, _ = body.ReadFrom(resp.Body)
	text := body.String()

	if resp.StatusCode != http.StatusOK ||
		!strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("metrics response: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if !strings.Contains(text, `forge_http_requests_total{method="GET",route="/api/v1/contests/:cid",status="200"} 1`) {
		t.Fatalf("normalized contest metric missing:\n%s", text)
	}
	if strings.Contains(text, cid) {
		t.Fatalf("metrics leaked high-cardinality contest id %q:\n%s", cid, text)
	}
	if !strings.Contains(text, "forge_http_requests_in_flight 1") {
		t.Fatalf("current metrics scrape should be in flight:\n%s", text)
	}
}
