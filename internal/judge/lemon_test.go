package judge

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// ProbeToolchains 工具链探测：lang -> 可执行路径，缺失为 ""（调用方大声报告）。
func ProbeToolchains(cfg ToolchainConfig) map[string]string {
	out := map[string]string{}
	for lang, spec := range map[string]ToolSpec{
		"cpp": cfg.CPP, "c": cfg.C, "go": cfg.Go, "python": cfg.Python,
	} {
		if spec.Command == "" {
			continue
		}
		if p, err := exec.LookPath(spec.Command); err == nil {
			out[lang] = p
		}
	}
	return out
}

func TestProbeToolchains(t *testing.T) {
	got := ProbeToolchains(DefaultToolchainConfig())
	if len(got) == 0 {
		t.Fatal("no toolchain found at all")
	}
	//  bogus 配置必须报告缺失（不炸）。
	bogus := DefaultToolchainConfig()
	bogus.CPP.Command = "definitely-not-a-compiler-xyz"
	if m := ProbeToolchains(bogus); m["cpp"] != "" {
		t.Fatal("bogus toolchain should probe empty")
	}
	t.Logf("probed: %v", got)
}

// TestDispatchMatrix lemon-helloworld 式：单编译，输入分发全 verdict。
func TestDispatchMatrix(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	code := loadCode(t, "dispatch.cpp")
	mk := func(in string) CaseInput { return CaseInput{Input: in, Expected: "AC"} }
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: code,
		Cases:   []CaseInput{mk("1"), mk("2"), mk("4"), mk("5"), mk("6")},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.CaseVerdict{
		domain.CaseAC, domain.CaseWA, domain.CaseTLE, domain.CaseRE, domain.CaseOLE,
	}
	assertVerdicts(t, "dispatch", verdictsOf(got), want)
}

// TestDispatchMLE 内存项单独测（墙行为与平台相关，隔离断言）。
func TestDispatchMLE(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("job wall is windows-only in this phase")
	}
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "dispatch.cpp"),
		Cases:   []CaseInput{{Input: "3", Expected: "AC"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 10000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertVerdicts(t, "dispatch/mle", verdictsOf(got), []domain.CaseVerdict{domain.CaseMLE})
}

// TestSlowStartGuard 慢启动必须 AC（防启动开销误杀 TLE）。
func TestSlowStartGuard(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "slowstart.cpp"),
		Cases:   []CaseInput{{Input: "3 4", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 3000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertVerdicts(t, "slowstart", verdictsOf(got), []domain.CaseVerdict{domain.CaseAC})
}

// TestForkbombContained fork 炸弹必须被墙快速拦下（Windows），Linux 走 prlimit 行为。
func TestForkbombContained(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("active-process wall assertion is windows-only")
	}
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	start := time.Now()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "forkbomb.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: ""}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 10000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertVerdicts(t, "forkbomb", verdictsOf(got), []domain.CaseVerdict{domain.CaseRE})
	if time.Since(start) > 8*time.Second {
		t.Fatal("contained forkbomb must die fast, not by timeout")
	}
}

func verdictsOf(got []domain.CaseResult) []domain.CaseVerdict {
	v := make([]domain.CaseVerdict, len(got))
	for i, c := range got {
		v[i] = c.Verdict
	}
	return v
}
