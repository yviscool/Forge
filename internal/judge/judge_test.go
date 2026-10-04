package judge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// ---------- fakes: verdict 映射纯逻辑，不碰真实进程 ----------

type fakeCompiler struct {
	ok  bool
	msg string
}

func (f fakeCompiler) Compile(_ context.Context, _ string, _ []byte, _ string) (CompileResult, error) {
	if !f.ok {
		return CompileResult{Message: f.msg}, nil
	}
	return CompileResult{OK: true, Executable: "fake-exe"}, nil
}

type fakeRunner struct{ out RunResult }

func (f fakeRunner) Run(_ context.Context, _ []string, _ string, _ Limits, _ string) (RunResult, error) {
	return f.out, nil
}

func TestJudgeOneVerdictMapping(t *testing.T) {
	cases := []struct {
		name string
		out  RunResult
		want domain.CaseVerdict
	}{
		{"timeout->TLE", RunResult{TimedOut: true}, domain.CaseTLE},
		{"oom->MLE", RunResult{OutOfMemory: true, ExitCode: -1}, domain.CaseMLE},
		{"nonzero->RE", RunResult{ExitCode: 1, Stdout: "x"}, domain.CaseRE},
		{"mismatch->WA", RunResult{Stdout: "8"}, domain.CaseWA},
		{"match->AC", RunResult{Stdout: "7"}, domain.CaseAC},
		{"truncated->OLE", RunResult{Stdout: "777...TRUNC", Truncated: true}, domain.CaseOLE},
		{"no-output-file->WA", RunResult{NoOutput: true}, domain.CaseWA},
	}
	for _, tc := range cases {
		o := &Orchestrator{Compiler: fakeCompiler{ok: true}, Runner: fakeRunner{out: tc.out}}
		got, err := o.JudgeOne(context.Background(), Request{
			Language: "cpp",
			Cases:    []CaseInput{{Input: "3 4", Expected: "7"}},
			Compare:  domain.CompareIgnoreSpace,
			Limits:   Limits{TimeMs: 1000, MemoryMiB: 256},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Verdict != tc.want {
			t.Fatalf("%s: got %s want %s", tc.name, got[0].Verdict, tc.want)
		}
	}
}

func TestJudgeOneCompileError(t *testing.T) {
	o := &Orchestrator{Compiler: fakeCompiler{ok: false, msg: "boom"}, Runner: fakeRunner{}}
	got, _ := o.JudgeOne(context.Background(), Request{
		Cases: []CaseInput{{}, {}}, Compare: domain.CompareIgnoreSpace,
	})
	if len(got) != 2 || got[0].Verdict != domain.CaseCE || got[1].Verdict != domain.CaseCE {
		t.Fatalf("CE fan-out broken: %+v", got)
	}
}

func TestSubtaskMapsEmpty(t *testing.T) {
	so, fo := SubtaskMaps(nil)
	if so != nil || fo != nil {
		t.Fatal("empty testcases must yield nil maps (skip auto-judge)")
	}
}

// ---------- integration: 真实工具链（缺失则跳过） ----------

func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("toolchain %s not found", name)
	}
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func realOrchestrator() *Orchestrator {
	return &Orchestrator{Compiler: Toolchain{}, Runner: LocalRunner{}}
}

func TestIntegrationAC_CPP(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: fixture(t, "aplusb.cpp"),
		Cases:   []CaseInput{{Input: "3 4\n", Expected: "7"}, {Input: "0 0", Expected: "0"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range got {
		if c.Verdict != domain.CaseAC {
			t.Fatalf("case %d: %+v", i, c)
		}
	}
}

func TestIntegrationWA_CPP(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: fixture(t, "wrong.cpp"),
		Cases:   []CaseInput{{Input: "3 4", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseWA {
		t.Fatalf("want WA, got %+v", got[0])
	}
}

func TestIntegrationTLE_CPP(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	start := time.Now()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: fixture(t, "tle.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: ""}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 500, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseTLE {
		t.Fatalf("want TLE, got %+v", got[0])
	}
	if time.Since(start) > 30*time.Second {
		t.Fatal("TLE kill took too long")
	}
}

func TestIntegrationRE_CPP(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: fixture(t, "re.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: ""}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseRE {
		t.Fatalf("want RE, got %+v", got[0])
	}
}

func TestIntegrationCE_CPP(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: fixture(t, "ce.cpp"),
		Cases:   []CaseInput{{Input: "x", Expected: "x"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseCE {
		t.Fatalf("want CE, got %+v", got[0])
	}
}

func TestIntegrationMLE_CPP(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("job wall is windows-only in this phase")
	}
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: fixture(t, "mle.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: "alive"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 10000, MemoryMiB: 64},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseMLE {
		t.Fatalf("want MLE, got %+v", got[0])
	}
}

func TestIntegrationAC_Python(t *testing.T) {
	requireTool(t, "python")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "python", Code: fixture(t, "aplusb.py"),
		Cases:   []CaseInput{{Input: "3 4", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseAC {
		t.Fatalf("want AC, got %+v", got[0])
	}
}

func TestIntegrationCE_Python(t *testing.T) {
	requireTool(t, "python")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "python", Code: "def broken(:\n",
		Cases:   []CaseInput{{Input: "", Expected: ""}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseCE {
		t.Fatalf("want CE, got %+v", got[0])
	}
}

// ---------- pool ----------

func TestPoolReportsAll(t *testing.T) {
	var mu sync.Mutex
	reported := map[string]int{}
	pool := NewPool(16,
		func(_ context.Context, r Request) ([]domain.CaseResult, error) {
			return []domain.CaseResult{{CaseIndex: 0, Verdict: domain.CaseAC}}, nil
		},
		func(subID string, _ []domain.CaseResult) {
			mu.Lock()
			reported[subID]++
			mu.Unlock()
		},
	)
	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx, 2)
	for i := 0; i < 5; i++ {
		if !pool.Submit(Job{SubID: string(rune('a' + i)), Req: Request{Cases: []CaseInput{{}}}}) {
			t.Fatal("submit rejected")
		}
	}
	cancel()
	pool.Wait()
	mu.Lock()
	defer mu.Unlock()
	if len(reported) != 5 {
		t.Fatalf("want 5 reports, got %v", reported)
	}
}
