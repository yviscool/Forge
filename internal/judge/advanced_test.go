package judge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

func TestSpecialJudge(t *testing.T) {
	checker := loadCode(t, "spj_even_checker.cpp")
	mkReq := func(code string) Request {
		return Request{
			Language: "cpp", Code: code,
			Cases: []CaseInput{
				{Input: "9", Expected: "8"},
				{Input: "10", Expected: "10"},
			},
			Compare: domain.CompareSpecialJudge,
			Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
			Checker: &CheckerSpec{Language: "cpp", Code: checker},
		}
	}
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	got, err := o.JudgeOne(ctx, mkReq(loadCode(t, "spj_even_ok.cpp")))
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseAC || got[1].Verdict != domain.CaseAC {
		t.Fatalf("spj ok: %+v", got)
	}
	got, err = o.JudgeOne(ctx, mkReq(loadCode(t, "spj_even_bad.cpp")))
	if err != nil {
		t.Fatal(err)
	}
	// n=9 输出 9（奇数）-> WA；n=10 输出 10（偶数）-> AC。
	if got[0].Verdict != domain.CaseWA || got[1].Verdict != domain.CaseAC {
		t.Fatalf("spj bad: %+v", got)
	}
}

func TestFileIOMode(t *testing.T) {
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	lim := Limits{TimeMs: 2000, MemoryMiB: 256, IOMode: domain.IOFile, InFile: "p.in", OutFile: "p.out"}
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "fileio_ok.cpp"),
		Cases:   []CaseInput{{Input: "3 4", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace, Limits: lim,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseAC {
		t.Fatalf("fileio ok: %+v", got[0])
	}
	got, err = o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "fileio_nowrite.cpp"),
		Cases:   []CaseInput{{Input: "3 4", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace, Limits: lim,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseWA {
		t.Fatalf("missing output file must be WA, got %+v", got[0])
	}
}

func TestPerCaseLimits(t *testing.T) {
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "case_lim.cpp"),
		Cases: []CaseInput{
			{Input: "5", Expected: "5"},
			{Input: "2", Expected: "2", Limits: Limits{TimeMs: 400}},
		},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseAC || got[1].Verdict != domain.CaseTLE {
		t.Fatalf("per-case limits: %+v", got)
	}
}

func TestCompileCache(t *testing.T) {
	dir := t.TempDir()
	cc, err := NewCompileCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	tc := Toolchain{Cache: cc}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	work := t.TempDir()
	src := []byte("int main(){return 0;}")
	r1, err := tc.Compile(ctx, "cpp", src, work)
	if err != nil || !r1.OK {
		t.Fatalf("compile: %v %+v", r1, err)
	}
	if cc.Hits() != 0 {
		t.Fatal("first compile must miss")
	}
	work2 := t.TempDir()
	r2, err := tc.Compile(ctx, "cpp", src, work2)
	if err != nil || !r2.OK {
		t.Fatalf("cached compile: %v", err)
	}
	if cc.Hits() != 1 {
		t.Fatalf("second compile must hit, hits=%d", cc.Hits())
	}
	if _, err := os.Stat(r2.Executable); err != nil {
		t.Fatal("cached exe must exist in workdir")
	}
	_ = filepath.Join
}

func TestOutputLimit(t *testing.T) {
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "ole_big.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: "x"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 512},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseOLE {
		t.Fatalf("20MB output must be OLE, got %+v", got[0])
	}
}

func TestAnswersOnly(t *testing.T) {
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		TaskType: domain.TaskAnswersOnly, Code: "42",
		Cases:   []CaseInput{{Expected: "42"}, {Expected: "43"}},
		Compare: domain.CompareIgnoreSpace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseAC || got[1].Verdict != domain.CaseWA {
		t.Fatalf("answers-only: %+v", got)
	}
}

func TestInteraction(t *testing.T) {
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	inter := InteractorSpec{Language: "cpp", Code: loadCode(t, "inter_add.cpp")}
	mkReq := func(sol string) Request {
		return Request{
			Language: "cpp", Code: loadCode(t, sol),
			Cases:    []CaseInput{{Input: "3 4", Expected: ""}},
			TaskType: domain.TaskInteraction, Interactor: &inter,
			Limits: Limits{TimeMs: 3000, MemoryMiB: 256},
		}
	}
	got, err := o.JudgeOne(ctx, mkReq("inter_sol_ok.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseAC {
		t.Fatalf("interact ok: %+v", got[0])
	}
	got, err = o.JudgeOne(ctx, mkReq("inter_sol_bad.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseWA {
		t.Fatalf("interact bad must be WA, got %+v", got[0])
	}
}

func TestToolchainConfig(t *testing.T) {
	def := DefaultToolchainConfig()
	if def.CPP.Command != "g++" || def.CPP.Std == "" {
		t.Fatalf("defaults broken: %+v", def)
	}
	if got := LoadToolchainConfig(""); got.CPP.Command != "g++" {
		t.Fatal("empty path must yield defaults")
	}
	if got := LoadToolchainConfig(filepath.Join("testdata", "nope.json")); got.Go.Command != "go" {
		t.Fatal("missing file must yield defaults")
	}
	f := filepath.Join(t.TempDir(), "tc.json")
	if err := os.WriteFile(f, []byte(`{"cpp":{"command":"g++","std":"c++20"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	got := LoadToolchainConfig(f)
	if got.CPP.Std != "c++20" || got.Python.Command != "python" {
		t.Fatalf("merge broken: %+v", got)
	}
}

func TestOutputLimitConfigurable(t *testing.T) {
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// 20MB 输出在 64KB 上限下必 OLE（默认 16MB 反而要跑完才截断）。
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "ole_big.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: "x"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 10000, MemoryMiB: 512, OutputLimitKiB: 64},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseOLE {
		t.Fatalf("64KB limit must OLE, got %+v", got[0])
	}
}

type sleepCompiler struct{ d time.Duration }

func (s sleepCompiler) Compile(ctx context.Context, _ string, _ []byte, _ string) (CompileResult, error) {
	select {
	case <-time.After(s.d):
		return CompileResult{OK: true, Executable: "x"}, nil
	case <-ctx.Done():
		return CompileResult{}, ctx.Err()
	}
}

func TestCompileTimeoutConfigurable(t *testing.T) {
	// 3s 编译被 1s 上限掐断（ wiring 本身，与 g++ 速度无关）。
	o := &Orchestrator{Compiler: sleepCompiler{d: 3 * time.Second}, Runner: LocalRunner{}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: "x",
		Cases:             []CaseInput{{Input: "1", Expected: "1"}},
		Compare:           domain.CompareIgnoreSpace,
		Limits:            Limits{TimeMs: 5000, MemoryMiB: 256},
		CompileTimeoutSec: 1,
	})
	if err == nil {
		t.Fatal("1s compile timeout must interrupt 3s compile")
	}
	// 不设上限时不断（用例 ctx 足够长）。
	o2 := &Orchestrator{
		Compiler: sleepCompiler{d: 10 * time.Millisecond},
		Runner:   fakeRunner{out: RunResult{Stdout: "1"}},
	}
	got, err := o2.JudgeOne(ctx, Request{
		Language: "cpp", Code: "x",
		Cases:   []CaseInput{{Input: "1", Expected: "1"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 256},
	})
	if err != nil || got[0].Verdict != domain.CaseAC {
		t.Fatalf("default must not interrupt: %v %+v", err, got)
	}
}
