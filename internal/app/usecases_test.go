package app

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/realtime"
)

func TestConcurrentUserCreationKeepsUsernamesUnique(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	const workers = 32
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.CreateUserWithUsername("Concurrent User", "student", "student")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	users := svc.ListUsers()
	if len(users) != workers {
		t.Fatalf("created %d users, want %d", len(users), workers)
	}
	seen := make(map[string]bool, len(users))
	for _, u := range users {
		key := strings.ToLower(u.Username)
		if seen[key] {
			t.Fatalf("duplicate username %q", u.Username)
		}
		seen[key] = true
	}
}

func TestContestBindingsAndFinish(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	u, _ := svc.CreateUser("A", "student")
	g, _ := svc.CreateGroup("G")
	_ = svc.AddUserToGroup(u.ID, g.ID)
	c, _ := svc.CreateContest("C", "")
	if _, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddGroupToContest(c.ID, g.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddGroupToContest(c.ID, g.ID); err != nil {
		t.Fatal("idempotent re-bind should pass")
	}
	if _, err := svc.StartContest(c.ID); err != nil {
		t.Fatal(err)
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
		domain.JudgeOutcome{Cases: []domain.CaseResult{
			{CaseIndex: 0, Verdict: domain.CaseAC},
			{CaseIndex: 1, Verdict: domain.CaseWA},
			{CaseIndex: 2, Verdict: domain.CaseAC},
		}},
		func(i int) int {
			if i < 2 {
				return 0
			}
			return 1
		},
		func(st int) int { return subtasks[st] }, nil)
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

func TestSubmitIdempotencyReplaysWithoutDuplicateSubmission(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	u, _ := svc.CreateUser("A", "student")
	c, _ := svc.CreateContest("C", "")
	p, _ := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
	})
	if _, err := svc.StartContest(c.ID); err != nil {
		t.Fatal(err)
	}

	first, replayed, err := svc.SubmitIdempotent(
		c.ID, p.ID, u.ID, "c++", "int main(){}", "", "request-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || replayed {
		t.Fatalf("first submission = %+v, replayed=%v", first, replayed)
	}
	if first.Language != "cpp" {
		t.Fatalf("stored language = %q, want canonical cpp", first.Language)
	}

	replay, replayed, err := svc.SubmitIdempotent(
		c.ID, p.ID, u.ID, "c++", "int main(){}", "", "request-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed || replay.ID != first.ID {
		t.Fatalf("replay = %+v, replayed=%v", replay, replayed)
	}
	if subs := svc.ListSubmissions(c.ID); len(subs) != 1 {
		t.Fatalf("idempotent submission count = %d, want 1", len(subs))
	}

	if _, _, err := svc.SubmitIdempotent(
		c.ID, p.ID, u.ID, "c++", "int main(){return 0;}", "", "request-1",
	); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("payload mismatch error = %v, want ErrIdempotencyConflict", err)
	}

	stored, err := svc.GetContest(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored.AllowedLanguages = []string{"python"}
	if err := svc.Store().UpdateContest(stored); err != nil {
		t.Fatal(err)
	}
	if _, replayed, err := svc.SubmitIdempotent(
		c.ID, p.ID, u.ID, "c++", "int main(){}", "", "request-1",
	); err != nil || !replayed {
		t.Fatalf("replay after policy change = replayed=%v err=%v", replayed, err)
	}
	if _, _, err := svc.SubmitIdempotent(
		c.ID, p.ID, u.ID, "c++", "int main(){return 1;}", "", "request-2",
	); err == nil || !strings.Contains(err.Error(), "language not allowed") {
		t.Fatalf("new submission after policy change error = %v", err)
	}

	if _, err := svc.FinishContest(c.ID); err != nil {
		t.Fatal(err)
	}
	if _, replayed, err := svc.SubmitIdempotent(
		c.ID, p.ID, u.ID, "c++", "int main(){}", "", "request-1",
	); err != nil || !replayed {
		t.Fatalf("replay after contest finished = replayed=%v err=%v", replayed, err)
	}
}

func TestJudgeCasesDoesNotOverwriteManualDecision(t *testing.T) {
	svc := NewService(memory.New(), realtime.NewHub(), nil, nil)
	u, err := svc.CreateUser("Late worker", "student")
	if err != nil {
		t.Fatal(err)
	}
	c, err := svc.CreateContest("Race", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "T", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartContest(c.ID); err != nil {
		t.Fatal(err)
	}
	sub, err := svc.Submit(c.ID, p.ID, u.ID, "cpp", "source")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Judge(sub.ID, domain.VerdictAccepted, 100); err != nil {
		t.Fatal(err)
	}

	late, err := svc.JudgeCases(sub.ID, domain.JudgeOutcome{
		Cases: []domain.CaseResult{{CaseIndex: 0, Verdict: domain.CaseWA}},
	}, func(int) int { return 0 }, func(int) int { return 100 }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if late.Verdict != domain.VerdictAccepted || late.Score != 100 {
		t.Fatalf("late worker overwrote manual decision: %+v", late)
	}
	stored, err := svc.GetSubmission(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Verdict != domain.VerdictAccepted || stored.Score != 100 {
		t.Fatalf("stored manual decision was overwritten: %+v", stored)
	}
}
