package app

import (
	"testing"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/realtime"
)

func TestContestBindingsAndFinish(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	u, _ := svc.CreateUser("A", "student")
	g, _ := svc.CreateGroup("G")
	_ = svc.AddUserToGroup(u.ID, g.ID)
	c, _ := svc.CreateContest("C", "")
	if err := svc.AddGroupToContest(c.ID, g.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddGroupToContest(c.ID, g.ID); err != nil {
		t.Fatal("idempotent re-bind should pass")
	}
	if _, err := svc.FinishContest(c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartContest(c.ID); err == nil {
		t.Fatal("finished contest must not restart")
	}
	if err := svc.RemoveUserFromGroup(u.ID, g.ID); err != nil {
		t.Fatal(err)
	}
}

func TestJudgeCasesSubtaskAggregation(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	u, _ := svc.CreateUser("A", "student")
	c, _ := svc.CreateContest("C", "")
	p, _ := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
	})
	_, _ = svc.StartContest(c.ID)
	sub, err := svc.Submit(c.ID, p.ID, u.ID, "cpp", "code")
	if err != nil {
		t.Fatal(err)
	}
	subtasks := map[int]int{0: 40, 1: 60}
	j, err := svc.JudgeCases(sub.ID,
		[]domain.CaseResult{
			{CaseIndex: 0, Verdict: domain.CaseAC},
			{CaseIndex: 1, Verdict: domain.CaseWA},
			{CaseIndex: 2, Verdict: domain.CaseAC},
		},
		func(i int) int {
			if i < 2 {
				return 0
			}
			return 1
		},
		func(st int) int { return subtasks[st] })
	if err != nil {
		t.Fatal(err)
	}
	if j.Score != 60 || j.Verdict != "wrong_answer" {
		t.Fatalf("cases aggregation wrong: %+v", j)
	}
	r := svc.Ranking(c.ID)
	if len(r) != 1 || r[0].Score != 60 {
		t.Fatalf("ranking wrong: %+v", r)
	}
}
