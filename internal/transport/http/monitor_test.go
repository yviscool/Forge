package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

func TestMonitorSnapshot(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	admin := e.mkAdmin(t)

	code, body := requestJSON(t, "POST", e.url+"/api/v1/contests", admin,
		`{"name":"监考赛","rankingMode":"oi","mode":"online"}`)
	if code != 201 {
		t.Fatal("create")
	}
	cid, _ := body["id"].(string)
	code, body = requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/participants", admin,
		fmt.Sprintf(`{"userId":%q}`, e.stuID))
	if code != 200 {
		t.Fatal("authorize")
	}
	code, body = requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/problems", admin,
		`{"code":"A","title":"监考题","statement":"作答。","input":"n","output":"x","constraints":"n<=9","examples":"1\n2"}`)
	if code != 201 {
		t.Fatal("add problem")
	}
	pid, _ := body["id"].(string)
	_ = pid
	code, body = requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/problems", admin,
		`{"code":"B","title":"判断题","statement":"判断。","input":"无","output":"无","taskType":"choice","choiceQuestions":[{"id":1,"type":"judge","stem":"2>1","answer":"T","score":10}]}`)
	if code != 201 {
		t.Fatal("add choice problem")
	}
	choicePid, _ := body["id"].(string)
	if code, _ := requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/start", admin, ""); code != 200 {
		t.Fatal("start")
	}

	// 学生提交一次（选择题交卷即判）。
	code, _ = requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/submissions", e.student,
		fmt.Sprintf(`{"problemId":%q,"language":"choice_json","code":"{\"1\":\"T\"}"}`, choicePid))
	if code != 202 {
		t.Fatal("submit")
	}

	// 在线注册表注入（模拟学生 SSE 在线）。
	now := time.Now()
	e.srv.SetPresenceSource(func(string) map[string]time.Time {
		return map[string]time.Time{e.stuID: now}
	})

	// 学生不可访问监考快照。
	if c, _ := requestJSON(t, "GET", e.url+"/api/v1/contests/"+cid+"/monitor", e.student, ""); c != 403 {
		t.Fatalf("student monitor = %d, want 403", c)
	}
	code, raw := getAuth(t, e.url+"/api/v1/contests/"+cid+"/monitor", admin)
	if code != 200 {
		t.Fatalf("teacher monitor = %d", code)
	}
	var snap struct {
		ContestID    string `json:"contestId"`
		Mode         string `json:"mode"`
		Participants []struct {
			UserID        string `json:"userId"`
			Online        bool   `json:"online"`
			IPConflict    bool   `json:"ipConflict"`
			SubmissionCnt int    `json:"submissionCount"`
			Problems      []struct {
				ProblemID   string `json:"problemId"`
				Attempts    int    `json:"attempts"`
				LastVerdict string `json:"lastVerdict"`
			} `json:"problems"`
			LastVersionProblemID string `json:"lastVersionProblemId"`
		} `json:"participants"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.ContestID != cid || snap.Mode != "online" {
		t.Fatalf("snapshot head = %+v", snap)
	}
	if len(snap.Participants) != 1 {
		t.Fatalf("participants = %d, want 1", len(snap.Participants))
	}
	p := snap.Participants[0]
	if p.UserID != e.stuID || !p.Online {
		t.Fatalf("participant = %+v", p)
	}
	if p.SubmissionCnt != 1 {
		t.Fatalf("submissionCount = %d, want 1", p.SubmissionCnt)
	}
	foundChoice := false
	for _, stat := range p.Problems {
		if stat.ProblemID == choicePid {
			foundChoice = true
			if stat.Attempts != 1 || stat.LastVerdict != domain.VerdictAccepted {
				t.Fatalf("choice stat = %+v", stat)
			}
		}
	}
	if !foundChoice {
		t.Fatal("choice problem missing from stats")
	}
}

func TestMonitorIPConflictFlag(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	admin := e.mkAdmin(t)

	code, body := requestJSON(t, "POST", e.url+"/api/v1/contests", admin, `{"name":"冲突赛"}`)
	if code != 201 {
		t.Fatal("create")
	}
	cid, _ := body["id"].(string)
	// 直接构造两个考生共用一个 IP 的绑定。
	c, err := e.svc.GetContest(cid)
	if err != nil {
		t.Fatal(err)
	}
	c.BindIPEnabled = true
	c.BoundIPs = map[string]string{e.stuID: "10.0.0.9", "usr-fake": "10.0.0.9"}
	if err := e.svc.Store().UpdateContest(c); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", e.url+"/api/v1/contests/"+cid+"/monitor", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var snap struct {
		Participants []struct {
			UserID     string `json:"userId"`
			IP         string `json:"ip"`
			IPConflict bool   `json:"ipConflict"`
		} `json:"participants"`
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&snap); err != nil {
		t.Fatal(err)
	}
	conflicts := 0
	for _, p := range snap.Participants {
		if p.IPConflict {
			conflicts++
		}
	}
	if conflicts == 0 {
		t.Fatalf("expected IP conflict flags, got %+v", snap.Participants)
	}
}
