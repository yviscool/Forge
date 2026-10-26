package app

import (
	"context"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/judge"
	"github.com/yviscool/forge/internal/realtime"
)

// 编排迁移自原 judge/wire.go（依赖方向归正 app→judge 后，集成测试随迁）。

func seedJudgeFixture(t *testing.T, contest string, problem domain.Problem) (*Service, string, string) {
	t.Helper()
	store := memory.New()
	svc := NewService(store, realtime.NewHub(), nil, nil)
	c, err := svc.CreateContest(contest, "", "oi")
	if err != nil {
		t.Fatal(err)
	}
	problem.ContestID = c.ID
	if problem.Code == "" {
		problem.Code = "A"
	}
	p, err := svc.CreateProblem(problem)
	if err != nil {
		t.Fatal(err)
	}
	return svc, c.ID, p.ID
}

func newReportPool(t *testing.T) (*judge.Pool, chan string) {
	t.Helper()
	reported := make(chan string, 1)
	pool := judge.NewPool(1,
		func(context.Context, judge.Request) (domain.JudgeOutcome, error) {
			return domain.JudgeOutcome{Cases: []domain.CaseResult{{CaseIndex: 0, Verdict: domain.CaseAC}}}, nil
		},
		func(subID string, _ domain.JudgeOutcome) { reported <- subID },
	)
	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx, 1)
	t.Cleanup(func() {
		cancel()
		pool.Wait()
	})
	return pool, reported
}

func TestEnqueueQueuedSubmissionsRecoversRestartWork(t *testing.T) {
	svc, cid, pid := seedJudgeFixture(t, "Recovery", domain.Problem{
		Title: "T", Statement: "s", Input: "i", Output: "o", Constraints: "k",
		TestCases: []domain.TestCase{{ID: "t1", InputFile: "1", OutputFile: "1", Score: 100}},
	})
	x, err := svc.Store().CreateSubmission(domain.Submission{
		ID: "sub-recover", ContestID: cid, ProblemID: pid, UserID: "u1",
		Language: "cpp", Code: "x", Verdict: domain.VerdictQueued, SubmittedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	pool, reported := newReportPool(t)

	n, err := EnqueueQueuedSubmissions(context.Background(), svc, pool)
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
	svc, cid, pid := seedJudgeFixture(t, "Periodic recovery", domain.Problem{
		Title: "T", Statement: "s", Input: "i", Output: "o", Constraints: "k",
		TestCases: []domain.TestCase{{ID: "t1", InputFile: "1", OutputFile: "1", Score: 100}},
	})
	pool, reported := newReportPool(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go RunQueueReconciler(ctx, svc, pool, 10*time.Millisecond, nil)

	x, err := svc.Store().CreateSubmission(domain.Submission{
		ID: "sub-periodic", ContestID: cid, ProblemID: pid, UserID: "u1",
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

// TestReportJudgeOutcomeAppliesDependencyZeroing 依赖清零语义必须在
// 自动评测回写链路上生效（此前 AggregateScore 无 deps 版是回写缺口）。
func TestReportJudgeOutcomeAppliesDependencyZeroing(t *testing.T) {
	svc, cid, pid := seedJudgeFixture(t, "Deps", domain.Problem{
		Title: "T", Statement: "s", Input: "i", Output: "o", Constraints: "k",
		// 子任务 1 依赖 2；1 满分 60、2 满分 40。点 0(子任务1) 特判半对、
		// 点 1(子任务2) AC：依赖未全 AC → 子任务1 清零，总分应为 40。
		TestCases: []domain.TestCase{
			{ID: "t0", InputFile: "0", OutputFile: "0", Score: 60, Subtask: 1, DependsOn: []int{2}},
			{ID: "t1", InputFile: "1", OutputFile: "1", Score: 40, Subtask: 2},
		},
	})
	if _, err := svc.Store().CreateSubmission(domain.Submission{
		ID: "sub-deps", ContestID: cid, ProblemID: pid, UserID: "u1",
		Language: "cpp", Code: "x", Verdict: domain.VerdictQueued, SubmittedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	svc.ReportJudgeOutcome("sub-deps", domain.JudgeOutcome{Cases: []domain.CaseResult{
		{CaseIndex: 0, Verdict: domain.CasePartly, Score: 30},
		{CaseIndex: 1, Verdict: domain.CaseAC},
	}})
	x, err := svc.GetSubmission("sub-deps")
	if err != nil {
		t.Fatal(err)
	}
	if x.Score != 40 {
		t.Fatalf("dependency zeroing: score = %d, want 40", x.Score)
	}
	if x.Verdict != domain.VerdictPartlyCorrect {
		t.Fatalf("verdict = %q, want %q", x.Verdict, domain.VerdictPartlyCorrect)
	}
}
