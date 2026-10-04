// Package storetest 是 Store 端口的一致性套件：所有实现（memory/sqlite/…）
// 必须通过同一组契约，保证 app 层无感切换。
package storetest

import (
	"strings"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
)

// Exercise 在给定 Store 上跑全实体 CRUD + 排序 + 隔离 + ID 契约。
func Exercise(t *testing.T, s ports.Store) {
	t.Helper()

	// ---- users ----
	if _, err := s.GetUser("nope"); err == nil {
		t.Fatal("missing user should error")
	}
	u1, err := s.CreateUser(domain.User{Name: "Zed", Role: "student"})
	if err != nil {
		t.Fatal(err)
	}
	u2, err := s.CreateUser(domain.User{ID: "custom-u2", Name: "Ada", Role: "teacher"})
	if err != nil {
		t.Fatal(err)
	}
	if u2.ID != "custom-u2" {
		t.Fatal("preset ID must be kept")
	}
	if !strings.HasPrefix(u1.ID, "usr-") {
		t.Fatalf("user id prefix broken: %s", u1.ID)
	}
	users := s.ListUsers()
	if len(users) != 2 || users[0].Name != "Ada" {
		t.Fatalf("users sorted by name: %+v", users)
	}
	u1.Groups = []string{"g1"}
	if err := s.UpdateUser(u1); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetUser(u1.ID)
	if len(got.Groups) != 1 {
		t.Fatal("user update lost")
	}
	if err := s.UpdateUser(domain.User{ID: "ghost"}); err == nil {
		t.Fatal("update ghost user should error")
	}

	// ---- groups ----
	g1, _ := s.CreateGroup(domain.Group{Name: "Camp"})
	if _, err := s.GetGroup("ghost"); err == nil {
		t.Fatal("missing group should error")
	}
	g1.UserIDs = []string{u1.ID}
	if err := s.UpdateGroup(g1); err != nil {
		t.Fatal(err)
	}
	if gs := s.ListGroups(); len(gs) != 1 {
		t.Fatalf("groups: %+v", gs)
	}

	// ---- contests ----
	now := time.Now().UTC()
	c1, _ := s.CreateContest(domain.Contest{Name: "Morning", Status: "draft", CreatedAt: now})
	c2, _ := s.CreateContest(domain.Contest{Name: "Afternoon", Status: "draft", CreatedAt: now.Add(time.Second)})
	cs := s.ListContests()
	if len(cs) != 2 || cs[0].Name != "Morning" {
		t.Fatalf("contests sorted by creation: %+v", cs)
	}
	c1.Status = string(domain.Running)
	if err := s.UpdateContest(c1); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.GetContest(c1.ID); c.Status != string(domain.Running) {
		t.Fatal("contest update lost")
	}
	_ = c2

	// ---- problems（比赛隔离）----
	p1, err := s.CreateProblem(domain.Problem{ContestID: c1.ID, Code: "B", Title: "T2"})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.CreateProblem(domain.Problem{ContestID: c1.ID, Code: "A", Title: "T1"})
	if err != nil {
		t.Fatal(err)
	}
	_ = p1
	ps := s.ListProblems(c1.ID)
	if len(ps) != 2 || ps[0].Code != "A" || ps[1].Code != "B" {
		t.Fatalf("problems sorted by code: %+v", ps)
	}
	if other := s.ListProblems(c2.ID); len(other) != 0 {
		t.Fatalf("contest isolation broken: %+v", other)
	}
	p2.Title = "T1-renamed"
	if _, err := s.UpdateProblem(p2); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.GetProblem(p2.ID); p.Title != "T1-renamed" {
		t.Fatal("problem update lost")
	}
	c, _ := s.GetContest(c1.ID)
	if len(c.ProblemIDs) != 2 {
		t.Fatalf("contest.ProblemIDs not synced: %+v", c.ProblemIDs)
	}
	if _, err := s.GetProblem("ghost"); err == nil {
		t.Fatal("missing problem should error")
	}

	// ---- submissions（倒序）----
	base := time.Now().UTC()
	x1, _ := s.CreateSubmission(domain.Submission{ContestID: c1.ID, ProblemID: p2.ID, UserID: u1.ID, Score: 10, SubmittedAt: base})
	x2, _ := s.CreateSubmission(domain.Submission{ContestID: c1.ID, ProblemID: p2.ID, UserID: u1.ID, Score: 20, SubmittedAt: base.Add(time.Second)})
	subs := s.ListSubmissions(c1.ID)
	if len(subs) != 2 || subs[0].ID != x2.ID || subs[1].ID != x1.ID {
		t.Fatal("submissions must be newest-first")
	}
	x2.Score = 100
	if err := s.UpdateSubmission(x2); err != nil {
		t.Fatal(err)
	}
	if x, _ := s.GetSubmission(x2.ID); x.Score != 100 {
		t.Fatal("submission update lost")
	}
	if _, err := s.GetSubmission("ghost"); err == nil {
		t.Fatal("missing submission should error")
	}

	// ---- NextID 唯一递增 ----
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		id := s.NextID("tst")
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
		if !strings.HasPrefix(id, "tst-") {
			t.Fatalf("id prefix broken: %s", id)
		}
	}
}
