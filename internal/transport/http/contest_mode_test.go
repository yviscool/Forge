package httpapi

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

func TestNOIContestMasksResultsUntilFinished(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	admin := e.mkAdmin(t)

	// 建比赛 + 选择题。
	code, body := requestJSON(t, "POST", e.url+"/api/v1/contests", admin,
		`{"name":"NOI 全真模拟","rankingMode":"oi","mode":"noi"}`)
	if code != 201 {
		t.Fatalf("create = %d %v", code, body)
	}
	cid, _ := body["id"].(string)
	if body["mode"] != "noi" {
		t.Fatalf("created contest mode = %v, want noi", body["mode"])
	}
	choiceProblem := `{"code":"A","title":"判断题","statement":"判断下列命题。","input":"无","output":"无","taskType":"choice","choiceQuestions":[
		{"id":1,"type":"judge","stem":"1+1=2","answer":"T","score":10}
	]}`
	code, body = requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/problems", admin, choiceProblem)
	if code != 201 {
		t.Fatalf("add problem = %d %v", code, body)
	}
	pid, _ := body["id"].(string)

	// 授权 + 启动。
	code, _ = requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/participants", admin,
		fmt.Sprintf(`{"userId":%q}`, e.stuID))
	if code != 200 {
		t.Fatalf("authorize = %d", code)
	}
	if code, _ := requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/start", admin, ""); code != 200 {
		t.Fatal("start contest")
	}

	// 学生交卷：POST 响应即时脱敏（verdict=submitted）。
	submissionBody := fmt.Sprintf(`{"problemId":%q,"language":"choice_json","code":%q}`,
		pid, `{"1":true}`)
	code, body = requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/submissions", e.student, submissionBody)
	if code != 202 {
		t.Fatalf("submit = %d %v", code, body)
	}
	if body["verdict"] != "submitted" || body["score"].(float64) != 0 {
		t.Fatalf("student submit response must be masked, got %v", body)
	}

	// 学生提交列表脱敏；教师看到真实判定。
	code, raw := getAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student)
	var studentFeed []map[string]any
	_ = json.Unmarshal(raw, &studentFeed)
	if len(studentFeed) != 1 || studentFeed[0]["verdict"] != "submitted" {
		t.Fatalf("student feed must be masked: %v", studentFeed)
	}
	code, raw = getAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", admin)
	var teacherFeed []map[string]any
	_ = json.Unmarshal(raw, &teacherFeed)
	if len(teacherFeed) != 1 || teacherFeed[0]["verdict"] != "accepted" {
		t.Fatalf("teacher feed must show real verdict: %v", teacherFeed)
	}

	// 赛中学生拿不到榜单；教师可以。
	if code, _ := requestJSON(t, "GET", e.url+"/api/v1/contests/"+cid+"/ranking", e.student, ""); code != 403 {
		t.Fatalf("student ranking during noi = %d, want 403", code)
	}
	if code, _ := requestJSON(t, "GET", e.url+"/api/v1/contests/"+cid+"/ranking", admin, ""); code != 200 {
		t.Fatalf("teacher ranking during noi = %d, want 200", code)
	}

	// 结束后：学生看到真实判定与榜单。
	if code, _ := requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/finish", admin, ""); code != 200 {
		t.Fatal("finish contest")
	}
	code, raw = getAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student)
	_ = json.Unmarshal(raw, &studentFeed)
	if len(studentFeed) != 1 || studentFeed[0]["verdict"] != "accepted" {
		t.Fatalf("student feed after finish must be real: %v", studentFeed)
	}
	if code, body := requestJSON(t, "GET", e.url+"/api/v1/contests/"+cid+"/ranking", e.student, ""); code != 200 {
		t.Fatalf("student ranking after finish = %d %v", code, body)
	}
}

func TestNOISSEEventsMaskedForStudents(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	admin := e.mkAdmin(t)

	code, body := requestJSON(t, "POST", e.url+"/api/v1/contests", admin,
		`{"name":"NOI SSE","rankingMode":"oi","mode":"noi"}`)
	if code != 201 {
		t.Fatal("create")
	}
	cid, _ := body["id"].(string)

	c, err := e.svc.GetContest(cid)
	if err != nil {
		t.Fatal(err)
	}
	c.Status = "running"
	_ = e.svc.Store().UpdateContest(c)

	student := domain.Session{UserID: e.stuID, Role: domain.RoleStudent}
	event, send := e.srv.eventForSession(domain.Event{
		Type: "submission.judged", ContestID: cid,
		Data: domain.Submission{ID: "s1", ContestID: cid, UserID: e.stuID,
			Verdict: "accepted", Score: 100},
	}, student)
	if !send {
		t.Fatal("own submission event must still be delivered")
	}
	if s, ok := event.Data.(domain.Submission); !ok || s.Verdict != "submitted" || s.Score != 0 {
		t.Fatalf("own judged event must be masked, got %+v", event.Data)
	}
	if _, send := e.srv.eventForSession(domain.Event{
		Type: "ranking.updated", ContestID: cid, Data: map[string]any{},
	}, student); send {
		t.Fatal("ranking events must be withheld from students during NOI")
	}

	// 教师不受影响：他人与本人的判定事件都保持原样。
	teacher := domain.Session{UserID: "u-admin", Role: domain.RoleAdmin}
	event, send = e.srv.eventForSession(domain.Event{
		Type: "submission.judged", ContestID: cid,
		Data: domain.Submission{ID: "s1", ContestID: cid, UserID: e.stuID,
			Verdict: "accepted", Score: 100},
	}, teacher)
	if !send {
		t.Fatal("teacher must receive events")
	}
	if s, ok := event.Data.(domain.Submission); !ok || s.Verdict != "accepted" {
		t.Fatalf("teacher event must stay unmasked, got %+v", event.Data)
	}
}

func TestContestModeSettingsFrozenAfterStart(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	admin := e.mkAdmin(t)

	code, body := requestJSON(t, "POST", e.url+"/api/v1/contests", admin,
		`{"name":"Mode freeze","rankingMode":"oi","mode":"online"}`)
	if code != 201 {
		t.Fatal("create")
	}
	cid, _ := body["id"].(string)

	// draft 阶段可切。
	if code, _ := requestJSON(t, "PUT", e.url+"/api/v1/contests/"+cid+"/settings", admin,
		`{"mode":"gesp"}`); code != 200 {
		t.Fatal("mode switch in draft must succeed")
	}
	// 非法取值 400。
	if code, _ := requestJSON(t, "PUT", e.url+"/api/v1/contests/"+cid+"/settings", admin,
		`{"mode":"demo"}`); code != 400 {
		t.Fatal("invalid mode must 400")
	}
	// 启动需要至少一道题。
	if code, _ := requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/problems", admin,
		`{"code":"A","title":"判断题","statement":"判断下列命题。","input":"无","output":"无","taskType":"choice","choiceQuestions":[
		{"id":1,"type":"judge","stem":"1+1=2","answer":"T","score":10}
	]}`); code != 201 {
		t.Fatal("add problem")
	}
	// 启动后冻结。
	if code, _ := requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/start", admin, ""); code != 200 {
		t.Fatal("start")
	}
	if code, _ := requestJSON(t, "PUT", e.url+"/api/v1/contests/"+cid+"/settings", admin,
		`{"mode":"noi"}`); code != 400 {
		t.Fatal("mode switch after start must be frozen")
	}
}
