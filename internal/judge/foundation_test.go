package judge

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// ---------- 评测基石第二批:文件 IO OLE / 静态 MLE / 交互升级 / 外部 diff /
// 题目级覆盖 / 缓存 LRU+singleflight / 物理核 ----------

type captureRunner struct {
	lim   Limits
	argv  []string
	calls int32
}

func (r *captureRunner) Run(_ context.Context, argv []string, _ string, lim Limits, _ string) (RunResult, error) {
	atomic.AddInt32(&r.calls, 1)
	r.argv = argv
	r.lim = lim
	return RunResult{Stdout: "1"}, nil
}

// TestPerProblemRatioApplication 题目级倍率与工具链倍率叠加相乘，
// 题目级禁内存墙最后生效（对标 LemonLime 每题每编译器 ratio）。
func TestPerProblemRatioApplication(t *testing.T) {
	disable := true
	req := Request{
		Language: "cpp", Code: "int main(){}",
		Cases:            []CaseInput{{Input: "1", Expected: "1"}},
		Compare:          domain.CompareIgnoreSpace,
		Limits:           Limits{TimeMs: 1000, MemoryMiB: 256},
		TimeLimitRatio:   2,
		MemoryLimitRatio: 3,
	}
	o := &Orchestrator{Compiler: fakeCompiler{ok: true}, Runner: &captureRunner{}}
	oc, err := o.JudgeOne(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	runner := o.Runner.(*captureRunner)
	if runner.lim.TimeMs != 2000 || runner.lim.MemoryMiB != 768 {
		t.Fatalf("per-problem ratios must apply: %+v", runner.lim)
	}
	if oc.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("verdict: %+v", oc.Cases[0])
	}

	req2 := req
	req2.DisableMemoryLimitCheck = &disable
	o2 := &Orchestrator{Compiler: fakeCompiler{ok: true}, Runner: &captureRunner{}}
	if _, err := o2.JudgeOne(context.Background(), req2); err != nil {
		t.Fatal(err)
	}
	if lim := o2.Runner.(*captureRunner).lim; lim.MemoryMiB != 0 {
		t.Fatalf("disable memory limit must zero the wall: %+v", lim)
	}
}

// TestFileIOOutputLimitOLE 文件 IO 模式读输出文件必须有上限（此前
// os.ReadFile 无上限，OutputLimitKiB 在文件 IO 下不生效）。
func TestFileIOOutputLimitOLE(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	code := "#include <cstdio>\nint main(){FILE*f=fopen(\"p.out\",\"w\");for(int i=0;i<20000;i++)fputs(\"xxxxxxxxxx\",f);fclose(f);return 0;}"
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: code,
		Cases: []CaseInput{{Input: "", Expected: "x"}},
		Limits: Limits{TimeMs: 5000, MemoryMiB: 256, IOMode: domain.IOFile,
			InFile: "p.in", OutFile: "p.out", OutputLimitKiB: 64},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cases[0].Verdict != domain.CaseOLE {
		t.Fatalf("oversized output file must be OLE: %+v", got.Cases[0])
	}
}

// TestMLEStaticPrecheck 静态内存预检：BSS 大数组起跑前即 MLE（全平台确定性）。
func TestMLEStaticPrecheck(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: fixture(t, "mle_static.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: "alive"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 10000, MemoryMiB: 64},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := got.Cases[0]
	if c.Verdict != domain.CaseMLE {
		t.Fatalf("static memory over wall must MLE pre-run: %+v", c)
	}
	if c.MemoryKiB < 64*1024 {
		t.Fatalf("static usage must be reported: %+v", c)
	}
}

// TestInteractionPartialScore 交互器 stderr 打分协议：partially correct (50)
// → 满分一半（Forge 扩展，复用 testlib 标记解析）。
func TestInteractionPartialScore(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	inter := InteractorSpec{Language: "cpp", Code: loadCode(t, "inter_score.cpp")}
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "inter_sol_ok.cpp"),
		Cases:    []CaseInput{{Input: "3 4", Expected: "", Score: 40}},
		TaskType: domain.TaskInteraction, Interactor: &inter,
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 3000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := oc.Cases[0]
	if c.Verdict != domain.CasePartly || c.Score != 20 {
		t.Fatalf("interactor partial scoring: %+v", c)
	}
}

// TestInteractionInteractorCrash 交互器协议外异常退出 → CheckerError
// （评测器侧故障），不得误报学生 RE。
func TestInteractionInteractorCrash(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	inter := InteractorSpec{Language: "cpp", Code: loadCode(t, "inter_crash.cpp")}
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "inter_sol_ok.cpp"),
		Cases:    []CaseInput{{Input: "3 4", Expected: ""}},
		TaskType: domain.TaskInteraction, Interactor: &inter,
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 3000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseCheckerError {
		t.Fatalf("crashed interactor must be CheckerError: %+v", oc.Cases[0])
	}
}

// TestExternalToolCompare 外部 diff 工具比较（对标 LemonLime ExternalToolMode）。
func TestExternalToolCompare(t *testing.T) {
	requireTool(t, "g++")
	requireTool(t, "diff")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	mk := func(expr string) Request {
		return Request{
			Language: "cpp", Code: "#include <iostream>\nint main(){std::cout<<" + expr + ";}",
			Cases:   []CaseInput{{Input: "", Expected: "same"}},
			Compare: domain.CompareExternalTool,
			Diff:    &DiffSpec{Command: "diff"},
			Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
		}
	}
	oc, err := o.JudgeOne(ctx, mk("\"same\""))
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("external diff equal: %+v", oc.Cases[0])
	}
	oc, err = o.JudgeOne(ctx, mk("\"other\""))
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseWA {
		t.Fatalf("external diff differ: %+v", oc.Cases[0])
	}
}

// TestCompileCacheLRUEviction 容量上限 + 最久未用淘汰（此前只增不减）。
func TestCompileCacheLRUEviction(t *testing.T) {
	cc, err := NewCompileCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cc.maxEntries = 4
	key := func(i int) string { return fmt.Sprintf("key-%02d", i) }
	for i := 0; i < 6; i++ {
		f := t.TempDir() + "/e"
		if err := os.WriteFile(f, []byte("fake-exe-bytes"), 0o755); err != nil {
			t.Fatal(err)
		}
		cc.store(key(i), f)
	}
	cc.mu.Lock()
	n := len(cc.entries)
	ev := cc.evictions
	cc.mu.Unlock()
	if n != 4 || ev != 2 {
		t.Fatalf("LRU must cap entries: n=%d evictions=%d", n, ev)
	}
	// 最旧的 key-00 应已被淘汰。
	cc.mu.Lock()
	_, hasOldest := cc.entries[key(0)]
	cc.mu.Unlock()
	if hasOldest {
		t.Fatal("oldest entry must be evicted first")
	}
}

// TestCompileCacheSingleflight 同键并发只编译一次，其余等待复制。
func TestCompileCacheSingleflight(t *testing.T) {
	cc, err := NewCompileCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var builds int32
	var key = "flight-key-1"
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := cc.execute(key, t.TempDir(), func() (string, error) {
				atomic.AddInt32(&builds, 1)
				time.Sleep(50 * time.Millisecond) // 放大竞态窗口
				f := t.TempDir() + "/e"
				if err := os.WriteFile(f, []byte("fake-exe-bytes"), 0o755); err != nil {
					return "", err
				}
				return f, nil
			})
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		t.Fatal(firstErr)
	}
	if got := atomic.LoadInt32(&builds); got != 1 {
		t.Fatalf("same-key concurrent compile must build once, got %d", got)
	}
	hits, _ := cc.Stats()
	if hits == 0 {
		t.Fatal("waiters must count as hits")
	}
}

// TestDefaultWorkersSanity 缺省 worker 数在 [1, NumCPU] 且非零。
func TestDefaultWorkersSanity(t *testing.T) {
	n := DefaultWorkers()
	if n < 1 || n > runtime.NumCPU() {
		t.Fatalf("default workers out of range: %d", n)
	}
}
