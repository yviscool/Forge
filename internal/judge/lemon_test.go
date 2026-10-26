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
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: code,
		Cases:   []CaseInput{mk("1"), mk("2"), mk("4"), mk("5"), mk("6")},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
	})
	got := oc.Cases
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
	requireProcessWalls(t)
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "dispatch.cpp"),
		Cases:   []CaseInput{{Input: "3", Expected: "AC"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 10000, MemoryMiB: 256},
	})
	got := oc.Cases
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
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "slowstart.cpp"),
		Cases:   []CaseInput{{Input: "3 4", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 3000, MemoryMiB: 256},
	})
	got := oc.Cases
	if err != nil {
		t.Fatal(err)
	}
	assertVerdicts(t, "slowstart", verdictsOf(got), []domain.CaseVerdict{domain.CaseAC})
}

// TestForkbombContained fork 炸弹必须被墙快速拦下（Windows），Linux 走 prlimit 行为。
func TestForkbombContained(t *testing.T) {
	requireProcessWalls(t)
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	start := time.Now()
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "forkbomb.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: ""}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 10000, MemoryMiB: 256},
	})
	got := oc.Cases
	if err != nil {
		t.Fatal(err)
	}
	if oc.CompileMessage != "" {
		t.Fatalf("forkbomb fixture must compile: %s", oc.CompileMessage)
	}
	assertVerdicts(t, "forkbomb", verdictsOf(got), []domain.CaseVerdict{domain.CaseRE})
	if time.Since(start) > 8*time.Second {
		t.Fatal("contained forkbomb must die fast, not by timeout")
	}
}

// TestPipeHoldingChildDoesNotStall 父退子留(子进程握 stdout 睡 30s)必须
// 在 WaitDelay 内返回,不得把判题拖到子进程睡醒(孤儿管道死锁回归)。
func TestPipeHoldingChildDoesNotStall(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	start := time.Now()
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "pipeholder.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: ""}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 256},
	})
	got := oc.Cases
	if err != nil {
		t.Fatal(err)
	}
	if oc.CompileMessage != "" {
		t.Fatalf("pipeholder fixture must compile: %s", oc.CompileMessage)
	}
	switch runtime.GOOS {
	case "windows":
		// Job ActiveProcessLimit=1 直接拦住子进程 spawn(RE)——遏制即正确;
		// "父退子留握管道"的状态在 Windows 单进程墙下不可达。
		assertVerdicts(t, "pipeholder", verdictsOf(got), []domain.CaseVerdict{domain.CaseRE})
	default:
		// Unix:父正常退出、子握管道睡 30s——WaitDelay 必须 5s 内放行,
		// 孤儿由组杀清算;不得把判题拖到子进程睡醒。
		assertVerdicts(t, "pipeholder", verdictsOf(got), []domain.CaseVerdict{domain.CaseAC})
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("orphan pipe must not stall judging (WaitDelay): %v", elapsed)
	}
}

func verdictsOf(got []domain.CaseResult) []domain.CaseVerdict {
	v := make([]domain.CaseVerdict, len(got))
	for i, c := range got {
		v[i] = c.Verdict
	}
	return v
}

// TestMemoryReported 内存必须真实上报（值为 0 的墙等于没墙）。
func TestMemoryReported(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: "#include<iostream>\nint main(){long long a,b;if(!(std::cin>>a>>b))return 0;std::cout<<a+b;return 0;}",
		Cases:   []CaseInput{{Input: "1 2", Expected: "3"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 512},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("verdict: %+v", got.Cases[0])
	}
	if got.Cases[0].MemoryKiB <= 0 {
		t.Fatalf("peak memory must be measured, got %+v", got.Cases[0])
	}
}

// TestInterpretedTimeFactor 解释型 2x 时限：sleep 1.2s 在 1s 限下必须 AC。
func TestInterpretedTimeFactor(t *testing.T) {
	requireTool(t, "python")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "python", Code: "import time\ntime.sleep(1.1)\nprint('ok')",
		Cases:   []CaseInput{{Input: "", Expected: "ok"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 1000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("2x factor must cover 1.2s sleep under 1s limit: %+v", got.Cases[0])
	}
}
