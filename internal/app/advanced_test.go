package app

import (
	"testing"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/realtime"
)

func TestUpdateProblemAndSubtasks(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	c, _ := svc.CreateContest("C", "")
	p, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
		TestCases: []domain.TestCase{
			{ID: "t1", Subtask: 0, Score: 30},
			{ID: "t2", Subtask: 1, Score: 70},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 合并语义：空标题不覆盖，合法改名生效。
	if _, err := svc.UpdateProblem(domain.Problem{ID: p.ID, Title: ""}); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.GetProblem(c.ID, p.ID); got.Title != "T" {
		t.Fatalf("empty title must not overwrite: %+v", got)
	}
	if _, err := svc.UpdateProblem(domain.Problem{ID: p.ID, Title: "T2"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.GetProblem(c.ID, p.ID); got.Title != "T2" {
		t.Fatal("rename lost")
	}

	cfg, err := svc.ConfigureSubtasks(p.ID, []SubtaskConfig{
		{Subtask: 0, Score: 40, TimeLimitMs: 500, MemoryMiB: 128},
		{Subtask: 1, Score: 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TestCases[0].Score != 40 || cfg.TestCases[0].TimeLimitMs != 500 || cfg.TestCases[0].MemoryMiB != 128 {
		t.Fatalf("subtask0 not applied: %+v", cfg.TestCases[0])
	}
	if cfg.TestCases[1].Score != 60 || cfg.TestCases[1].TimeLimitMs != 0 {
		t.Fatalf("subtask1 partial: %+v", cfg.TestCases[1])
	}
	if _, err := svc.ConfigureSubtasks("ghost", nil); err == nil {
		t.Fatal("ghost problem should error")
	}
	if _, err := svc.UpdateProblem(domain.Problem{ID: "ghost"}); err == nil {
		t.Fatal("ghost update should error")
	}
}

func TestRejudgeAndStatistics(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	u, _ := svc.CreateUser("A", "student")
	c, _ := svc.CreateContest("C", "")
	p, _ := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
	})
	_, _ = svc.StartContest(c.ID)
	s1, _ := svc.Submit(c.ID, p.ID, u.ID, "cpp", "code1")
	s2, _ := svc.Submit(c.ID, p.ID, u.ID, "cpp", "code2")
	_, _ = svc.Judge(s1.ID, "accepted", 100)
	_, _ = svc.Judge(s2.ID, "wrong_answer", 0)

	subs, err := svc.RejudgeProblem(c.ID, p.ID)
	if err != nil || len(subs) != 2 {
		t.Fatalf("rejudge list: %v %+v", err, subs)
	}
	if _, err := svc.RejudgeProblem(c.ID, "ghost"); err == nil {
		t.Fatal("ghost rejudge should error")
	}

	st, err := svc.Statistics(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Submissions != 2 || st.Users != 1 {
		t.Fatalf("stats head: %+v", st)
	}
	if len(st.Problems) != 1 {
		t.Fatalf("stats problems: %+v", st)
	}
	ps := st.Problems[0]
	if ps.Submissions != 2 || ps.Accepted != 1 || ps.BestScore != 100 || ps.FirstACUser != "A" {
		t.Fatalf("problem stats: %+v", ps)
	}
	if ps.AcceptRate != 0.5 {
		t.Fatalf("rate: %v", ps.AcceptRate)
	}
	if r := svc.RankingACM(c.ID); len(r) != 1 || r[0].Solved != 1 {
		t.Fatalf("acm: %+v", r)
	}
}

func TestJudgeCasesOLEAndDetail(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	u, _ := svc.CreateUser("A", "student")
	c, _ := svc.CreateContest("C", "")
	p, _ := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
	})
	_, _ = svc.StartContest(c.ID)
	sub, _ := svc.Submit(c.ID, p.ID, u.ID, "cpp", "code")
	so := func(i int) int { return 0 }
	fo := func(int) int { return 100 }
	j, err := svc.JudgeCases(sub.ID, domain.JudgeOutcome{Cases: []domain.CaseResult{{CaseIndex: 0, Verdict: domain.CaseOLE}}}, so, fo)
	if err != nil {
		t.Fatal(err)
	}
	if j.Verdict != "output_limit" {
		t.Fatalf("OLE mapping: %+v", j)
	}
	if len(j.Cases) != 1 || j.Cases[0].Verdict != domain.CaseOLE {
		t.Fatalf("detail persisted: %+v", j.Cases)
	}
	got, _ := svc.GetSubmission(sub.ID)
	if len(got.Cases) != 1 {
		t.Fatal("detail must survive store round-trip")
	}
}

func TestContestBundleRoundTrip(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	c, _ := svc.CreateContest("Bundle", "desc", "acm")
	p, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
		CompareMode: domain.CompareIgnoreSpace, TaskType: domain.TaskTraditional,
		TestCases: []domain.TestCase{{ID: "t1", InputFile: "1", OutputFile: "1", Score: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = p

	b, err := svc.ExportContest(c.ID)
	if err != nil || len(b.Problems) != 1 {
		t.Fatalf("export: %v %+v", err, b)
	}
	c2, err := svc.ImportBundle(b)
	if err != nil {
		t.Fatal(err)
	}
	if c2.ID == c.ID || c2.RankingMode != "acm" {
		t.Fatalf("import identity: %+v", c2)
	}
	ps := svc.ListProblems(c2.ID)
	if len(ps) != 1 || ps[0].Title != "T" || ps[0].ID == p.ID {
		t.Fatalf("import problems: %+v", ps)
	}
	if ps[0].TestCases[0].InputFile != "1" {
		t.Fatal("testcase payload lost")
	}
	if _, err := svc.ExportContest("ghost"); err == nil {
		t.Fatal("ghost export should error")
	}
}

func TestCompileMessagePersisted(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	u, _ := svc.CreateUser("A", "student")
	c, _ := svc.CreateContest("C", "")
	p, _ := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
	})
	_, _ = svc.StartContest(c.ID)
	sub, _ := svc.Submit(c.ID, p.ID, u.ID, "cpp", "broken code")
	oc := domain.JudgeOutcome{
		Cases:          []domain.CaseResult{{CaseIndex: 0, Verdict: domain.CaseCE}},
		CompileMessage: "main.cpp:1: error: expected ';'",
	}
	j, err := svc.JudgeCases(sub.ID, oc, func(int) int { return 0 }, func(int) int { return 100 })
	if err != nil {
		t.Fatal(err)
	}
	if j.Verdict != "compile_error" || j.CompileMessage == "" {
		t.Fatalf("CE detail: %+v", j)
	}
}
