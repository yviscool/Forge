package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

func ticketResponse(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestEventsTicketRequiresContestAndContestAccess(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, created := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Exam"}`)
	cid := created["id"].(string)
	_, outsider := e.mkUserFull(t, "Outsider", "student", "pw-outsider-1")

	missing := ticketResponse(t, e.url+"/api/v1/events/ticket", e.teacher)
	missing.Body.Close()
	if missing.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing contestId should 400, got %d", missing.StatusCode)
	}

	notFound := ticketResponse(t, e.url+"/api/v1/events/ticket?contestId=missing", e.teacher)
	notFound.Body.Close()
	if notFound.StatusCode != http.StatusNotFound {
		t.Fatalf("missing contest should 404, got %d", notFound.StatusCode)
	}

	forbidden := ticketResponse(t, e.url+"/api/v1/events/ticket?contestId="+cid, outsider)
	forbidden.Body.Close()
	if forbidden.StatusCode != http.StatusForbidden {
		t.Fatalf("outsider should 403, got %d", forbidden.StatusCode)
	}

	anonymous := ticketResponse(t, e.url+"/api/v1/events/ticket?contestId="+cid, "")
	anonymous.Body.Close()
	if anonymous.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous should 401, got %d", anonymous.StatusCode)
	}
}

func TestEventsTicketCookieAndContestlessStreamBoundary(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, created := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Exam"}`)
	cid := created["id"].(string)
	if code, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/participants", e.teacher,
		`{"userId":"`+e.stuID+`"}`); code != http.StatusOK {
		t.Fatalf("attach participant: %d", code)
	}

	response := ticketResponse(t, e.url+"/api/v1/events/ticket?contestId="+cid, e.student)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("ticket status %d", response.StatusCode)
	}
	cookies := response.Cookies()
	if len(cookies) != 1 || cookies[0].Name != streamCookieName {
		t.Fatalf("expected one stream cookie, got %#v", cookies)
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("ticket cookie lacks secure browser attributes: %#v", cookie)
	}
	if cookie.Path != "/api/v1/events" {
		t.Fatalf("ticket cookie path = %q", cookie.Path)
	}
	if cookie.MaxAge < int(11*time.Hour/time.Second) {
		t.Fatalf("ticket cookie should cover a full exam day, got MaxAge=%d", cookie.MaxAge)
	}

	// Cookie transport is accepted without a bearer header on the stream.
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/events?contestId="+cid, nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	r2, ok := e.srv.authenticateEvents(recorder, request, cid)
	if !ok {
		t.Fatalf("cookie ticket rejected: %d %s", recorder.Code, recorder.Body.String())
	}
	sess, ok := sessionOf(r2)
	if !ok || sess.UserID != e.stuID || sess.Role != domain.RoleStudent {
		t.Fatalf("unexpected stream session: %#v %v", sess, ok)
	}

	// A mismatch in contest binding is rejected exactly like a bad cookie.
	request = httptest.NewRequest(http.MethodGet,
		"/api/v1/events?contestId=other", nil)
	request.AddCookie(cookie)
	recorder = httptest.NewRecorder()
	if _, ok := e.srv.authenticateEvents(recorder, request, "other"); ok {
		t.Fatal("cross-contest stream ticket was accepted")
	}

	global := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	global.Header.Set("Authorization", "Bearer "+e.teacher)
	recorder = httptest.NewRecorder()
	e.srv.events(recorder, global)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("contestless stream should 400, got %d", recorder.Code)
	}
}

func TestLogoutClearsStreamCookie(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	req, err := http.NewRequest(http.MethodPost, e.url+"/api/v1/auth/logout", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+e.student)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("logout status %d", response.StatusCode)
	}
	found := false
	for _, cookie := range response.Cookies() {
		if cookie.Name == streamCookieName {
			found = true
			if cookie.MaxAge >= 0 || cookie.Value != "" {
				t.Fatalf("stream cookie was not deleted: %#v", cookie)
			}
		}
	}
	if !found {
		t.Fatal("logout must clear the HttpOnly stream cookie")
	}
}

func TestEventPayloadRedaction(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	student := domain.Session{UserID: e.stuID, Role: domain.RoleStudent}
	contest := domain.Contest{
		ID: "c1", BoundIPs: map[string]string{
			e.stuID: "10.0.0.8", "peer": "10.0.0.9",
		},
		Extensions: map[string]int{e.stuID: 5, "peer": 20},
		Broadcasts: []domain.BroadcastMessage{{ID: "m1", Content: "secret"}},
	}
	event, send := e.srv.eventForSession(domain.Event{
		Type: "contest.updated", Data: contest,
	}, student)
	if !send {
		t.Fatal("student should receive public contest update")
	}
	got, ok := event.Data.(domain.Contest)
	if !ok {
		t.Fatalf("unexpected contest payload: %#v", event.Data)
	}
	if got.BoundIPs != nil || got.Broadcasts != nil {
		t.Fatalf("contest secrets leaked in SSE: %#v", got)
	}
	if len(got.Extensions) != 1 || got.Extensions[e.stuID] != 5 {
		t.Fatalf("peer extension leaked in SSE: %#v", got.Extensions)
	}

	// A zip-protected paper must not leak metadata through the shared SSE feed;
	// the REST paper gate remains the only place that can reveal it.
	if _, send := e.srv.eventForSession(domain.Event{
		Type: "contest.updated",
		Data: domain.Contest{ID: "c1", ZipPasswordHash: "test-verifier"},
	}, student); send {
		t.Fatal("zip-protected contest metadata must not reach students via SSE")
	}

	event, send = e.srv.eventForSession(domain.Event{
		Type: "submission.judged",
		Data: domain.Submission{UserID: "peer", Code: "secret-code", CompileMessage: "secret-ce"},
	}, student)
	if send {
		t.Fatalf("peer submission event leaked in SSE: %#v", event)
	}

	event, send = e.srv.eventForSession(domain.Event{
		Type: "problem.updated",
		Data: domain.Problem{
			ChoiceKeyJSON: `{"1":"A"}`,
			TestCases:     []domain.TestCase{{InputFile: "secret.in"}},
			ChoiceQuestions: []domain.ChoiceQuestion{
				{ID: 1, Answer: "A", Explanation: "secret"},
			},
		},
	}, student)
	if !send {
		t.Fatal("problem update should remain visible")
	}
	problem := event.Data.(domain.Problem)
	if problem.ChoiceKeyJSON != "" || len(problem.TestCases) != 0 ||
		problem.ChoiceQuestions[0].Answer != nil || problem.ChoiceQuestions[0].Explanation != "" {
		t.Fatalf("problem judge material leaked in SSE: %#v", problem)
	}

	for _, eventType := range []string{"contest.ip_bound", "contest.ip_unbound", "contest.ip_unbound_all"} {
		if _, send := e.srv.eventForSession(domain.Event{Type: eventType}, student); send {
			t.Fatalf("%s must not reach students", eventType)
		}
	}
	if _, send := e.srv.eventForSession(domain.Event{
		Type: "contest.broadcast",
		Data: domain.BroadcastMessage{TargetUserID: "peer", Content: "secret"},
	}, student); send {
		t.Fatal("targeted broadcast leaked to another student")
	}

	// Staff still receive the operational payload unchanged.
	staff := domain.Session{UserID: "teacher", Role: domain.RoleTeacher}
	protected := domain.Contest{ID: "c1", ZipPasswordHash: "test-verifier"}
	event, send = e.srv.eventForSession(domain.Event{Type: "contest.updated", Data: protected}, staff)
	if !send || event.Data.(domain.Contest).ZipPasswordHash != "test-verifier" {
		t.Fatal("staff event was unexpectedly redacted")
	}
	encoded, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "test-verifier") ||
		strings.Contains(string(encoded), "zipPasswordHash") {
		t.Fatalf("staff SSE serialization leaked the ZIP verifier: %s", encoded)
	}
}

func TestStudentContestExportUsesRedactedPayload(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, created := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"Export"}`)
	cid := created["id"].(string)
	if code, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/participants", e.teacher,
		`{"userId":"`+e.stuID+`"}`); code != http.StatusOK {
		t.Fatalf("attach participant: %d", code)
	}
	_, problem := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"Title","statement":"Statement","input":"Input","output":"Output","constraints":"Constraints","testCases":[{"id":"t1","inputFile":"SECRET-IN","outputFile":"SECRET-OUT","score":100,"subtask":0}]}`)
	if problem["id"] == nil {
		t.Fatalf("problem creation failed: %#v", problem)
	}
	code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/export", e.student)
	if code != http.StatusOK {
		t.Fatalf("student export: %d %s", code, body)
	}
	if strings.Contains(string(body), "SECRET-IN") || strings.Contains(string(body), "SECRET-OUT") {
		t.Fatalf("student export leaked test data: %s", body)
	}
}

func TestClientIPIgnoresSpoofedProxyHeadersByDefault(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/contests/x/submissions", nil)
	request.RemoteAddr = "192.168.1.77:54321"
	request.Header.Set("X-Forwarded-For", "10.0.0.8")
	request.Header.Set("X-Real-IP", "10.0.0.9")
	if got := e.srv.clientIP(request); got != "192.168.1.77" {
		t.Fatalf("untrusted proxy header changed the exam seat IP: %q", got)
	}

	e.srv.TrustProxy = true
	if got := e.srv.clientIP(request); got != "10.0.0.8" {
		t.Fatalf("trusted proxy mode should use X-Forwarded-For: %q", got)
	}

	request.Header.Set("X-Forwarded-For", "10.0.0.8, 203.0.113.7")
	if got := e.srv.clientIP(request); got != "203.0.113.7" {
		t.Fatalf("trusted proxy must use the right-most appended address: %q", got)
	}

	request.Header.Set("X-Forwarded-For", "not-an-ip")
	request.Header.Set("X-Real-IP", "198.51.100.9")
	if got := e.srv.clientIP(request); got != "198.51.100.9" {
		t.Fatalf("invalid XFF should fall back to a valid X-Real-IP: %q", got)
	}

	request.RemoteAddr = "[2001:db8::9]:443"
	request.Header.Del("X-Forwarded-For")
	request.Header.Del("X-Real-IP")
	if got := e.srv.clientIP(request); got != "2001:db8::9" {
		t.Fatalf("IPv6 remote address was not canonicalized: %q", got)
	}
}
