package app

import (
	"testing"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/realtime"
)

func struct2prob(contestID string) domain.Problem {
	return domain.Problem{
		ID: contestID + "-p1", ContestID: contestID, Code: "A", Title: "Sum",
		Statement: "a+b", Input: "a b", Output: "sum", Constraints: "k",
		Locale: "zh-CN", TimeLimitMs: 1000, MemoryLimitMiB: 512, UpdatedAt: time.Now().UTC(),
	}
}

func TestServiceSubmitJudgeRanking(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	u, _ := svc.CreateUser("Ada", "student")
	c, _ := svc.CreateContest("Spring", "")
	st := memory.New()
	_ = st
	// 直接经 store 建题（地基版 service 尚未收口 problem 用例，保持 store 直写能力）
	p, err := svc.store.CreateProblem(struct2prob(c.ID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartContest(c.ID); err != nil {
		t.Fatal(err)
	}
	sub, err := svc.Submit(c.ID, p.ID, u.ID, "cpp", "code")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Judge(sub.ID, "accepted", 100); err != nil {
		t.Fatal(err)
	}
	r := svc.Ranking(c.ID)
	if len(r) != 1 || r[0].Score != 100 {
		t.Fatalf("ranking broken: %#v", r)
	}
}
