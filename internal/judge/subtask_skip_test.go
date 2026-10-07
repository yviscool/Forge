package judge

import (
	"context"
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

type countingRunner struct {
	calls int
	out   RunResult
}

func (r *countingRunner) Run(_ context.Context, _ []string, _ string, _ Limits, _ string) (RunResult, error) {
	r.calls++
	return r.out, nil
}

func TestOrchestratorSubtaskShortCircuitSkip(t *testing.T) {
	runner := &countingRunner{
		out: RunResult{Stdout: "wrong_answer_output"},
	}
	o := &Orchestrator{
		Compiler: fakeCompiler{ok: true},
		Runner:   runner,
	}

	req := Request{
		Language: "cpp",
		Code:     "int main(){}",
		Cases: []CaseInput{
			// Case 0: Subtask 1 (will WA)
			{Input: "1", Expected: "correct", Subtask: 1},
			// Case 1: Subtask 1 (belongs to same subtask 1, should be skipped because subtask 1 already failed)
			{Input: "2", Expected: "correct", Subtask: 1},
			// Case 2: Subtask 2, depends on Subtask 1 (should be skipped because dep 1 failed)
			{Input: "3", Expected: "correct", Subtask: 2, DependsOn: []int{1}},
		},
		Compare: domain.CompareLineByLine,
	}

	outcome, err := o.JudgeOne(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(outcome.Cases) != 3 {
		t.Fatalf("expected 3 cases, got %d", len(outcome.Cases))
	}

	// Case 0 should be executed and judged WA
	if outcome.Cases[0].Verdict != domain.CaseWA {
		t.Fatalf("case 0 expected WA, got %s", outcome.Cases[0].Verdict)
	}

	// Case 1 should be Skipped (subtask 1 already failed on case 0)
	if outcome.Cases[1].Verdict != domain.CaseSkipped {
		t.Fatalf("case 1 expected Skipped, got %s", outcome.Cases[1].Verdict)
	}

	// Case 2 should be Skipped (depends on subtask 1 which failed)
	if outcome.Cases[2].Verdict != domain.CaseSkipped {
		t.Fatalf("case 2 expected Skipped, got %s", outcome.Cases[2].Verdict)
	}

	// Crucially, runner should only have been called ONCE (for Case 0)!
	if runner.calls != 1 {
		t.Fatalf("runner should only be called 1 time due to short-circuit skip, but was called %d times", runner.calls)
	}
}

func TestOrchestratorCheckerCompileError(t *testing.T) {
	o := &Orchestrator{
		Compiler: fakeCompiler{ok: false, msg: "checker syntax error"},
		Runner:   fakeRunner{},
	}

	// When checker fails to compile, orchestrator must report CaseCheckerError, NOT CaseRE
	req := Request{
		Language: "cpp",
		Code:     "int main(){}",
		Cases: []CaseInput{
			{Input: "1", Expected: "1"},
		},
		Checker: &CheckerSpec{Language: "cpp", Code: "broken checker"},
	}

	// Fake compiler will fail on code compile if we don't distinguish:
	// But let's test compiler ok for user code, fail on checker compile:
	checkerFailComp := &customCompiler{
		userOK:    true,
		checkerOK: false,
	}
	o.Compiler = checkerFailComp

	outcome, err := o.JudgeOne(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcome.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(outcome.Cases))
	}
	if outcome.Cases[0].Verdict != domain.CaseCheckerError {
		t.Fatalf("expected CaseCheckerError when checker fails to compile, got %s", outcome.Cases[0].Verdict)
	}
}

type customCompiler struct {
	userOK    bool
	checkerOK bool
}

func (c *customCompiler) Compile(_ context.Context, _ string, code []byte, _ string) (CompileResult, error) {
	if string(code) == "broken checker" {
		return CompileResult{OK: c.checkerOK, Message: "checker compile failed"}, nil
	}
	return CompileResult{OK: c.userOK, Executable: "fake-exe"}, nil
}
