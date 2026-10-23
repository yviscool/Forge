package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

func decodeObject(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode object: %v (%s)", err, raw)
	}
	return out
}

func decodeArray(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode array: %v (%s)", err, raw)
	}
	return out
}

func unlockExamPassword(
	t *testing.T,
	client *http.Client,
	url, token, password string,
) (int, http.Header, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(
		http.MethodPost,
		url,
		strings.NewReader(fmt.Sprintf(`{"password":%q}`, password)),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, resp.Header.Clone(), body
}

func TestAnonymousCannotReadProtectedCollectionsOrContestDetail(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Private"}`)
	cid := c["id"].(string)

	for _, path := range []string{
		"/api/v1/contests",
		"/api/v1/contests/" + cid,
		"/api/v1/contests/" + cid + "/problems",
		"/api/v1/contests/" + cid + "/submissions",
		"/api/v1/contests/" + cid + "/messages",
		"/api/v1/contests/" + cid + "/pdf",
		"/api/v1/events?contestId=" + cid,
		"/api/v1/users",
		"/api/v1/groups",
	} {
		if code, body := getAuth(t, e.url+path, ""); code != http.StatusUnauthorized {
			t.Fatalf("%s: anonymous should 401, got %d %s", path, code, body)
		}
	}
}

func TestStudentCannotReadTeacherCollectionsOrEnqueuedEndpoints(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"T"}`)
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	pid := p["id"].(string)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)
	_, sub := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student,
		fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"x"}`, pid))
	sid := sub["id"].(string)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/v1/users", ""},
		{"GET", "/api/v1/groups", ""},
		{"GET", "/api/v1/users/export", ""},
		{"POST", "/api/v1/users/import", `{"csv":"name,role\nEve,student\n"}`},
		{"POST", "/api/v1/contests/" + cid + "/start", ""},
		{"POST", "/api/v1/contests/" + cid + "/finish", ""},
		{"GET", "/api/v1/contests/" + cid + "/statistics", ""},
		{"GET", "/api/v1/contests/" + cid + "/bundle", ""},
		{"POST", "/api/v1/contests/" + cid + "/problems/" + pid + "/validate", ""},
		{"POST", "/api/v1/submissions/" + sid + "/judge", `{"verdict":"accepted","score":100}`},
		{"POST", "/api/v1/contests/" + cid + "/problems/" + pid + "/rejudge", ""},
	} {
		if tc.method == "GET" {
			code, body := getAuth(t, e.url+tc.path, e.student)
			if code != http.StatusForbidden {
				t.Fatalf("%s %s: student should 403, got %d %s", tc.method, tc.path, code, body)
			}
			continue
		}
		code, body := postAuth(t, e.url+tc.path, e.student, tc.body)
		if code != http.StatusForbidden {
			t.Fatalf("%s %s: student should 403, got %d %s", tc.method, tc.path, code, body)
		}
	}
}

func TestContestRedactionAndStudentVisibility(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	bobID, bobTok := e.mkUserFull(t, "Bob", "student", "pw-bob-1234")
	_, outsiderTok := e.mkUserFull(t, "Outsider", "student", "pw-out-1234")

	code, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Locked"}`)
	if code != http.StatusCreated {
		t.Fatalf("create contest: %d %v", code, c)
	}
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k","testCases":[{"id":"t1","inputFile":"3 4","outputFile":"7","score":100,"subtask":0}]}`)
	pid := p["id"].(string)

	// Attach operational secrets and make Bob the sole participant.
	contest, err := e.svc.GetContest(cid)
	if err != nil {
		t.Fatal(err)
	}
	contest.GroupIDs = []string{}
	contest.ParticipantUserIDs = []string{bobID}
	contest.BoundIPs = map[string]string{bobID: "10.0.0.8", "usr-other": "10.0.0.9"}
	contest.BindIPEnabled = true
	contest.Extensions = map[string]int{bobID: 15, "usr-other": 30}
	contest.Broadcasts = []domain.BroadcastMessage{{ID: "m1", ContestID: cid, Title: "secret", Content: "secret"}}
	contest.WindowEnabled = true
	end := time.Now().Add(time.Hour)
	contest.EndTime = &end
	if err := e.svc.Store().UpdateContest(contest); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetZipPassword(cid, "exam-zip-secret"); err != nil {
		t.Fatal(err)
	}

	// Staff see the full operational object.
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid, e.teacher); code != http.StatusOK {
		t.Fatalf("teacher detail: %d %s", code, body)
	} else {
		got := decodeObject(t, body)
		if got["zipProtected"] != true || len(got["boundIps"].(map[string]any)) != 2 {
			t.Fatalf("teacher should see operational data: %s", body)
		}
		for _, key := range []string{"zipPassword", "zipPasswordHash"} {
			if _, ok := got[key]; ok {
				t.Fatalf("%s leaked to staff: %s", key, body)
			}
		}
	}

	// Student sees safe timing/navigation fields but no operational secrets.
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid, bobTok); code != http.StatusOK {
		t.Fatalf("student detail: %d %s", code, body)
	} else {
		got := decodeObject(t, body)
		for _, key := range []string{"zipPassword", "zipPasswordHash", "boundIps", "broadcasts"} {
			if _, ok := got[key]; ok {
				t.Fatalf("%s leaked to student: %s", key, body)
			}
		}
		if got["windowEnabled"] != true {
			t.Fatalf("student needs timing fields: %s", body)
		}
		if got["zipProtected"] != true || got["zipUnlocked"] == true {
			t.Fatalf("student should learn that the exam zip is locked: %s", body)
		}
		ext := got["extensions"].(map[string]any)
		if len(ext) != 1 || ext[bobID] != float64(15) {
			t.Fatalf("student should see only own extension: %s", body)
		}
	}

	// Lobby only lists contests the student can access.
	if code, body := getAuth(t, e.url+"/api/v1/contests", bobTok); code != http.StatusOK {
		t.Fatalf("bob lobby: %d %s", code, body)
	} else if list := decodeArray(t, body); len(list) != 1 || list[0]["id"] != cid {
		t.Fatalf("bob should see the contest: %s", body)
	}
	if code, body := getAuth(t, e.url+"/api/v1/contests", outsiderTok); code != http.StatusOK {
		t.Fatalf("outsider lobby: %d %s", code, body)
	} else if list := decodeArray(t, body); len(list) != 0 {
		t.Fatalf("outsider must not see a draft invitation: %s", body)
	}
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid, outsiderTok); code != http.StatusForbidden {
		t.Fatalf("outsider detail should 403, got %d %s", code, body)
	}

	// Students never receive answer keys or judge inputs.
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", bobTok); code != http.StatusForbidden {
		t.Fatalf("locked student problem list should 403, got %d %s", code, body)
	}
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid, bobTok); code != http.StatusForbidden {
		t.Fatalf("locked student problem detail should 403, got %d %s", code, body)
	}
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/pdf", bobTok); code != http.StatusForbidden {
		t.Fatalf("locked student PDF should 403, got %d %s", code, body)
	}
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher); code != http.StatusOK || !strings.Contains(string(body), `"testCases"`) {
		t.Fatalf("teacher needs test data: %d %s", code, body)
	}

	// A wrong password cannot mint a paper grant.
	bobClient := newCookieClient(t)
	if code, body := postWithClient(t, bobClient, e.url+"/api/v1/contests/"+cid+"/unlock-zip", bobTok,
		`{"password":"wrong-password"}`); code != http.StatusForbidden {
		t.Fatalf("wrong zip password should 403, got %d %s", code, body)
	}
	// The correct password sets an HttpOnly cookie and all paper reads work.
	if code, body := postWithClient(t, bobClient, e.url+"/api/v1/contests/"+cid+"/unlock-zip", bobTok,
		`{"password":"exam-zip-secret"}`); code != http.StatusOK {
		t.Fatalf("unlock: %d %s", code, body)
	}
	if code, body := getWithClient(t, bobClient, e.url+"/api/v1/contests/"+cid, bobTok); code != http.StatusOK ||
		!strings.Contains(string(body), `"zipUnlocked":true`) {
		t.Fatalf("detail should report the signed unlock state: %d %s", code, body)
	}
	if code, body := getWithClient(t, bobClient, e.url+"/api/v1/contests/"+cid+"/problems", bobTok); code != http.StatusOK {
		t.Fatalf("unlocked problem list: %d %s", code, body)
	} else if strings.Contains(string(body), `"testCases"`) || strings.Contains(string(body), `"choiceKeyJson"`) {
		t.Fatalf("student problem list leaked judge material: %s", body)
	}
	if code, body := getWithClient(t, bobClient, e.url+"/api/v1/contests/"+cid+"/export", bobTok); code != http.StatusOK {
		t.Fatalf("unlocked paper HTML: %d %s", code, body)
	}
}

func TestContestSubmissionsAndMessagesEnforceStudentAccess(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	bobID, bobTok := e.mkUserFull(t, "Bob", "student", "pw-bob-1234")
	_, outsiderTok := e.mkUserFull(t, "Outsider", "student", "pw-out-1234")

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Restricted"}`)
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	pid := p["id"].(string)
	contest, _ := e.svc.GetContest(cid)
	contest.ParticipantUserIDs = []string{bobID}
	_ = e.svc.Store().UpdateContest(contest)
	if _, err := e.svc.SetZipPassword(cid, "zip-secret"); err != nil {
		t.Fatal(err)
	}
	bobClient := newCookieClient(t)
	if code, body := postWithClient(t, bobClient, e.url+"/api/v1/contests/"+cid+"/unlock-zip", bobTok, `{"password":"zip-secret"}`); code != http.StatusOK {
		t.Fatalf("unlock: %d %s", code, body)
	}
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)
	if code, body := postWithClient(t, bobClient, e.url+"/api/v1/contests/"+cid+"/submissions", bobTok,
		fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"secret"}`, pid)); code != http.StatusAccepted {
		t.Fatalf("unlocked submission: %d %s", code, body)
	}
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/messages", e.teacher,
		`{"title":"notice","content":"hidden from outsiders"}`)

	if code, _ := getAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", outsiderTok); code != http.StatusForbidden {
		t.Fatalf("outsider submissions should 403, got %d", code)
	}
	if code, _ := getAuth(t, e.url+"/api/v1/contests/"+cid+"/messages", outsiderTok); code != http.StatusForbidden {
		t.Fatalf("outsider messages should 403, got %d", code)
	}
	if code, body := getWithClient(t, bobClient, e.url+"/api/v1/contests/"+cid+"/submissions", bobTok); code != http.StatusOK {
		t.Fatalf("participant submissions: %d %s", code, body)
	}
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/messages", bobTok); code != http.StatusOK || !strings.Contains(string(body), "hidden from outsiders") {
		t.Fatalf("participant messages: %d %s", code, body)
	}
}

func TestZipGrantIsBoundToUserAndClearedOnLogout(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	bobID, bobTok := e.mkUserFull(t, "Bob", "student", "pw-bob-1234")
	_, outsiderTok := e.mkUserFull(t, "Outsider", "student", "pw-out-1234")
	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Protected"}`)
	cid := c["id"].(string)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	contest, _ := e.svc.GetContest(cid)
	contest.ParticipantUserIDs = []string{bobID}
	if err := e.svc.Store().UpdateContest(contest); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetZipPassword(cid, "zip-secret"); err != nil {
		t.Fatal(err)
	}

	resp, err := func() (*http.Response, error) {
		req, _ := http.NewRequest(http.MethodPost, e.url+"/api/v1/contests/"+cid+"/unlock-zip",
			strings.NewReader(`{"password":"zip-secret"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+bobTok)
		return http.DefaultClient.Do(req)
	}()
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var grant *http.Cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == zipCookieName {
			grant = cookie
		}
	}
	if resp.StatusCode != http.StatusOK || grant == nil {
		t.Fatalf("unlock did not issue a grant: %d %#v", resp.StatusCode, resp.Cookies())
	}
	if !grant.HttpOnly || grant.SameSite != http.SameSiteStrictMode || grant.Path != "/api/v1" {
		t.Fatalf("grant cookie is not scoped safely: %#v", grant)
	}

	// The same signed cookie must not work for another student.
	req, _ := http.NewRequest(http.MethodGet, e.url+"/api/v1/contests/"+cid+"/problems", nil)
	req.Header.Set("Authorization", "Bearer "+outsiderTok)
	req.AddCookie(grant)
	reused, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	reused.Body.Close()
	if reused.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-account grant must be rejected, got %d", reused.StatusCode)
	}

	// Rotating the password must revoke every grant minted against the old
	// verifier, even if the teacher chooses the same human-readable password.
	if _, err := e.svc.SetZipPassword(cid, "zip-secret"); err != nil {
		t.Fatal(err)
	}
	rotatedReq, _ := http.NewRequest(http.MethodGet, e.url+"/api/v1/contests/"+cid+"/problems", nil)
	rotatedReq.Header.Set("Authorization", "Bearer "+bobTok)
	rotatedReq.AddCookie(grant)
	rotatedResp, err := http.DefaultClient.Do(rotatedReq)
	if err != nil {
		t.Fatal(err)
	}
	rotatedResp.Body.Close()
	if rotatedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("grant survived password rotation, got %d", rotatedResp.StatusCode)
	}

	// Logout clears the browser grant in addition to the server session.
	logoutReq, _ := http.NewRequest(http.MethodPost, e.url+"/api/v1/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+bobTok)
	logoutReq.AddCookie(grant)
	logoutResp, err := http.DefaultClient.Do(logoutReq)
	if err != nil {
		t.Fatal(err)
	}
	defer logoutResp.Body.Close()
	cleared := false
	for _, cookie := range logoutResp.Cookies() {
		if cookie.Name == zipCookieName && cookie.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("logout must clear the exam unlock cookie")
	}
}

func TestExamUnlockRateLimitsFailedPasswords(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	bobID, bobTok := e.mkUserFull(t, "Bob", "student", "pw-bob-1234")
	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Protected"}`)
	cid := c["id"].(string)
	contest, err := e.svc.GetContest(cid)
	if err != nil {
		t.Fatal(err)
	}
	contest.ParticipantUserIDs = []string{bobID}
	if err := e.svc.Store().UpdateContest(contest); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetZipPassword(cid, "exam-secret"); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 10, 22, 8, 0, 0, 0, time.UTC)
	e.srv.ExamUnlockLimiter.SetClock(func() time.Time { return now })
	client := newCookieClient(t)
	url := e.url + "/api/v1/contests/" + cid + "/unlock-zip"

	for attempt := 1; attempt <= 6; attempt++ {
		if code, _, _ := unlockExamPassword(t, client, url, bobTok, "wrong"); code != http.StatusForbidden {
			t.Fatalf("attempt %d: got %d, want 403", attempt, code)
		}
	}
	code, headers, body := unlockExamPassword(t, client, url, bobTok, "still-wrong")
	if code != http.StatusTooManyRequests {
		t.Fatalf("blocked attempt: got %d %v, want 429", code, body)
	}
	if headers.Get("Retry-After") == "" {
		t.Fatal("rate-limited response must include Retry-After")
	}
	if body["code"] != "unlock_rate_limited" {
		t.Fatalf("unexpected error body: %v", body)
	}

	now = now.Add(5*time.Minute + time.Second)
	if code, _, body := unlockExamPassword(t, client, url, bobTok, "exam-secret"); code != http.StatusOK {
		t.Fatalf("attempt after window expiry: got %d %v", code, body)
	}
}

func TestExamUnlockResetsAccountAndScopesContestAndUserBuckets(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	aliceID, aliceTok := e.mkUserFull(t, "Alice2", "student", "pw-alice-1234")
	bobID, bobTok := e.mkUserFull(t, "Bob2", "student", "pw-bob-1234")
	newContest := func(name string) string {
		t.Helper()
		_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher,
			fmt.Sprintf(`{"name":%q}`, name))
		cid := c["id"].(string)
		contest, err := e.svc.GetContest(cid)
		if err != nil {
			t.Fatal(err)
		}
		contest.ParticipantUserIDs = []string{aliceID, bobID}
		if err := e.svc.Store().UpdateContest(contest); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.SetZipPassword(cid, "exam-secret"); err != nil {
			t.Fatal(err)
		}
		return cid
	}
	cid1 := newContest("Protected One")
	cid2 := newContest("Protected Two")

	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	e.srv.ExamUnlockLimiter.SetClock(func() time.Time { return now })
	alice := newCookieClient(t)
	bob := newCookieClient(t)
	url1 := e.url + "/api/v1/contests/" + cid1 + "/unlock-zip"
	url2 := e.url + "/api/v1/contests/" + cid2 + "/unlock-zip"

	for attempt := 1; attempt <= 5; attempt++ {
		if code, _, _ := unlockExamPassword(t, alice, url1, aliceTok, "wrong"); code != http.StatusForbidden {
			t.Fatalf("attempt %d: got %d, want 403", attempt, code)
		}
	}
	if code, _, body := unlockExamPassword(t, alice, url1, aliceTok, "exam-secret"); code != http.StatusOK {
		t.Fatalf("successful unlock: got %d %v", code, body)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if code, _, _ := unlockExamPassword(t, alice, url1, aliceTok, "wrong"); code != http.StatusForbidden {
			t.Fatalf("post-success attempt %d: got %d, want 403", attempt, code)
		}
	}
	if code, _, body := unlockExamPassword(t, bob, url1, bobTok, "exam-secret"); code != http.StatusOK {
		t.Fatalf("another user must have an independent bucket: %d %v", code, body)
	}

	for attempt := 1; attempt <= 4; attempt++ {
		if code, _, _ := unlockExamPassword(t, alice, url1, aliceTok, "wrong"); code != http.StatusForbidden {
			t.Fatalf("fill attempt %d: got %d, want 403", attempt, code)
		}
	}
	if code, _, body := unlockExamPassword(t, alice, url1, aliceTok, "exam-secret"); code != http.StatusTooManyRequests {
		t.Fatalf("same contest should be blocked for this user: %d %v", code, body)
	}
	if code, _, body := unlockExamPassword(t, alice, url2, aliceTok, "exam-secret"); code != http.StatusOK {
		t.Fatalf("another contest must have an independent bucket: %d %v", code, body)
	}
}
