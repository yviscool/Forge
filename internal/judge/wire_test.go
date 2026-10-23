package judge

import (
	"context"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/realtime"
)

func TestBuildRequestModes(t *testing.T) {
	sub := domain.Submission{ID: "s1", Language: "cpp", Code: "ans"}
	// 无测试点：拒绝自动评测。
	if _, ok := BuildRequest(sub, domain.Problem{}); ok {
		t.Fatal("empty testcases must refuse")
	}
	base := domain.Problem{
		Code: "A", TimeLimitMs: 2000, MemoryLimitMiB: 256,
		TestCases: []domain.TestCase{
			{ID: "t1", InputFile: "1", OutputFile: "1", Score: 50, Subtask: 0, TimeLimitMs: 400},
			{ID: "t2", InputFile: "2", OutputFile: "2", Score: 50, Subtask: 1},
		},
	}
	req, ok := BuildRequest(sub, base)
	if !ok {
		t.Fatal("should build")
	}
	if req.Cases[0].Limits.TimeMs != 400 || req.Cases[1].Limits.TimeMs != 0 {
		t.Fatalf("per-case limits: %+v", req.Cases)
	}
	if req.Cases[0].Subtask != 0 || req.Cases[1].Subtask != 1 {
		t.Fatalf("subtask indices must reach the judge: %+v", req.Cases)
	}
	if req.Limits.TimeMs != 2000 || req.Compare != domain.CompareIgnoreSpace {
		t.Fatalf("defaults: %+v", req.Limits)
	}
	// 特判。
	sp := base
	sp.CheckerCode = "int main(){}"
	req, _ = BuildRequest(sub, sp)
	if req.Checker == nil || req.Compare != domain.CompareSpecialJudge {
		t.Fatal("checker wiring")
	}
	// 答案题。
	ap := base
	ap.TaskType = domain.TaskAnswersOnly
	req, _ = BuildRequest(sub, ap)
	if req.TaskType != domain.TaskAnswersOnly || len(req.Cases) != 2 {
		t.Fatal("answers-only wiring")
	}
	// 文件 IO 默认文件名。
	fp := base
	fp.IOMode = domain.IOFile
	req, _ = BuildRequest(sub, fp)
	if req.Limits.InFile != "A.in" || req.Limits.OutFile != "A.out" {
		t.Fatalf("file defaults: %+v", req.Limits)
	}
	// 交互。
	ip := base
	ip.TaskType = domain.TaskInteraction
	ip.InteractorCode = "x"
	req, _ = BuildRequest(sub, ip)
	if req.Interactor == nil {
		t.Fatal("interactor wiring")
	}

	// Dependencies and per-case subtasks drive skip/aggregation semantics.
	dep := base
	dep.TestCases = []domain.TestCase{
		{ID: "d1", Subtask: 1, DependsOn: []int{2}},
		{ID: "d2", Subtask: 2},
	}
	req, _ = BuildRequest(sub, dep)
	if req.Cases[0].Subtask != 1 || len(req.Cases[0].DependsOn) != 1 || req.Cases[0].DependsOn[0] != 2 {
		t.Fatalf("dependency wiring: %+v", req.Cases)
	}
}

func TestEnqueueQueuedSubmissionsRecoversRestartWork(t *testing.T) {
	store := memory.New()
	svc := app.NewService(store, realtime.NewHub(), nil, nil)
	c, err := svc.CreateContest("Recovery", "", "oi")
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
		TestCases: []domain.TestCase{{ID: "t1", InputFile: "1", OutputFile: "1", Score: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}
	x, err := store.CreateSubmission(domain.Submission{
		ID: "sub-recover", ContestID: c.ID, ProblemID: p.ID, UserID: "u1",
		Language: "cpp", Code: "x", Verdict: domain.VerdictQueued, SubmittedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}

	reported := make(chan string, 1)
	pool := NewPool(1,
		func(context.Context, Request) (domain.JudgeOutcome, error) {
			return domain.JudgeOutcome{Cases: []domain.CaseResult{{CaseIndex: 0, Verdict: domain.CaseAC}}}, nil
		},
		func(subID string, _ domain.JudgeOutcome) { reported <- subID },
	)
	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx, 1)
	defer func() {
		cancel()
		pool.Wait()
	}()

	n, err := EnqueueQueuedSubmissions(ctx, svc, pool)
	if err != nil || n != 1 {
		t.Fatalf("recovery enqueue: n=%d err=%v", n, err)
	}
	select {
	case got := <-reported:
		if got != x.ID {
			t.Fatalf("reported %q, want %q", got, x.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("recovered submission was not processed")
	}
}

func TestRunQueueReconcilerRepairsWorkQueuedAfterStartup(t *testing.T) {
	store := memory.New()
	svc := app.NewService(store, realtime.NewHub(), nil, nil)
	c, err := svc.CreateContest("Periodic recovery", "", "oi")
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
		TestCases: []domain.TestCase{{ID: "t1", InputFile: "1", OutputFile: "1", Score: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}

	reported := make(chan string, 1)
	pool := NewPool(1,
		func(context.Context, Request) (domain.JudgeOutcome, error) {
			return domain.JudgeOutcome{Cases: []domain.CaseResult{{CaseIndex: 0, Verdict: domain.CaseAC}}}, nil
		},
		func(subID string, _ domain.JudgeOutcome) { reported <- subID },
	)
	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx, 1)
	go RunQueueReconciler(ctx, svc, pool, 10*time.Millisecond, nil)
	defer func() {
		cancel()
		pool.Wait()
	}()

	x, err := store.CreateSubmission(domain.Submission{
		ID: "sub-periodic", ContestID: c.ID, ProblemID: p.ID, UserID: "u1",
		Language: "cpp", Code: "x", Verdict: domain.VerdictQueued, SubmittedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-reported:
		if got != x.ID {
			t.Fatalf("reported %q, want %q", got, x.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("periodic reconciler did not process queued submission")
	}
}
