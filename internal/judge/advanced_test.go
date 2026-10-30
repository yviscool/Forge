package judge

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	oc, err := o.JudgeOne(ctx, mkReq(loadCode(t, "spj_even_ok.cpp")))
	got := oc.Cases
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseAC || got[1].Verdict != domain.CaseAC {
		t.Fatalf("spj ok: %+v", got)
	}
	oc, err = o.JudgeOne(ctx, mkReq(loadCode(t, "spj_even_bad.cpp")))
	got = oc.Cases
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
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "fileio_ok.cpp"),
		Cases:   []CaseInput{{Input: "3 4", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace, Limits: lim,
	})
	got := oc.Cases
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseAC {
		t.Fatalf("fileio ok: %+v", got[0])
	}
	oc, err = o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "fileio_nowrite.cpp"),
		Cases:   []CaseInput{{Input: "3 4", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace, Limits: lim,
	})
	got = oc.Cases
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
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "case_lim.cpp"),
		Cases: []CaseInput{
			{Input: "5", Expected: "5"},
			{Input: "2", Expected: "2", Limits: Limits{TimeMs: 400}},
		},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 256},
	})
	got := oc.Cases
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
	r1, err := tc.Compile(ctx, "cpp", src, work, CompileOptions{})
	if err != nil || !r1.OK {
		t.Fatalf("compile: %v %+v", r1, err)
	}
	if hits, _ := cc.Stats(); hits != 0 {
		t.Fatal("first compile must miss")
	}
	work2 := t.TempDir()
	r2, err := tc.Compile(ctx, "cpp", src, work2, CompileOptions{})
	if err != nil || !r2.OK {
		t.Fatalf("cached compile: %v", err)
	}
	if hits, _ := cc.Stats(); hits != 1 {
		t.Fatalf("second compile must hit, hits=%d", hits)
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
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "ole_big.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: "x"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 512},
	})
	got := oc.Cases
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
	oc, err := o.JudgeOne(ctx, Request{
		TaskType: domain.TaskAnswersOnly, Code: "42",
		Cases:   []CaseInput{{Expected: "42"}, {Expected: "43"}},
		Compare: domain.CompareIgnoreSpace,
	})
	got := oc.Cases
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
	oc, err := o.JudgeOne(ctx, mkReq("inter_sol_ok.cpp"))
	got := oc.Cases
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseAC {
		t.Fatalf("interact ok: %+v", got[0])
	}
	oc, err = o.JudgeOne(ctx, mkReq("inter_sol_bad.cpp"))
	got = oc.Cases
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseWA {
		t.Fatalf("interact bad must be WA, got %+v", got[0])
	}
}

// TestInteractionGrandchildHoldsStderr 交互正解 spawn 握 stderr 的子进程
// 后立即退出:WaitDelay 必须 5s 内放行(AC),不得拖到子进程 30s 睡醒
// (孤儿管道死锁回归;Windows 下 spawn 被 Job 单进程墙拦住,退化为普通正解)。
func TestInteractionGrandchildHoldsStderr(t *testing.T) {
	requireTool(t, "g++")
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	start := time.Now()
	inter := InteractorSpec{Language: "cpp", Code: loadCode(t, "inter_add.cpp")}
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "inter_sol_spawn.cpp"),
		Cases:    []CaseInput{{Input: "3 4", Expected: ""}},
		TaskType: domain.TaskInteraction, Interactor: &inter,
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 3000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("grandchild stderr must not stall: %+v msg=%q compile=%q",
			oc.Cases[0], oc.Cases[0].Message, oc.CompileMessage)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("orphan stderr must not stall interaction (WaitDelay): %v", elapsed)
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
	if _, err := ParseToolchainConfig(filepath.Join("testdata", "nope.json")); err == nil {
		t.Fatal("missing configured file must fail closed")
	}
	f := filepath.Join(t.TempDir(), "tc.json")
	if err := os.WriteFile(f, []byte(`{"cpp":{"command":"g++","std":"c++20"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ParseToolchainConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if got.CPP.Std != "c++20" || got.Python.Command != "python" {
		t.Fatalf("merge broken: %+v", got)
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{"cpp":{"command":"g++","surprise":true}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseToolchainConfig(bad); err == nil {
		t.Fatal("unknown toolchain fields must fail closed")
	}
	if err := os.WriteFile(bad, []byte(`{"go":{"compileTimeoutSec":-1}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseToolchainConfig(bad); err == nil {
		t.Fatal("negative compile limits must fail closed")
	}
	// spec() 方法必须透传新字段（曾漏合并导致墙没生效）。
	tc := Toolchain{Config: ToolchainConfig{CPP: ToolSpec{CompileTimeoutSec: 9, CompileMemoryMiB: 111}}}
	if sp := tc.spec("cpp"); sp.CompileTimeoutSec != 9 || sp.CompileMemoryMiB != 111 {
		t.Fatalf("spec merge: %+v", sp)
	}
	if sp := (Toolchain{}).spec("cpp"); sp.CompileTimeoutSec != 0 || sp.Command != "g++" {
		t.Fatalf("spec defaults: %+v", sp)
	}
}

func TestOutputLimitConfigurable(t *testing.T) {
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// 20MB 输出在 64KB 上限下必 OLE（默认 16MB 反而要跑完才截断）。
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "ole_big.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: "x"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 10000, MemoryMiB: 512, OutputLimitKiB: 64},
	})
	got := oc.Cases
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != domain.CaseOLE {
		t.Fatalf("64KB limit must OLE, got %+v", got[0])
	}
}

type sleepCompiler struct{ d time.Duration }

func (s sleepCompiler) Compile(ctx context.Context, _ string, _ []byte, _ string, _ CompileOptions) (CompileResult, error) {
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
	oc, err := o2.JudgeOne(ctx, Request{
		Language: "cpp", Code: "x",
		Cases:   []CaseInput{{Input: "1", Expected: "1"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 256},
	})
	got := oc.Cases
	if err != nil || got[0].Verdict != domain.CaseAC {
		t.Fatalf("default must not interrupt: %v %+v", err, got)
	}
}

func TestParallelCPUTLE(t *testing.T) {
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// 4 线程烧固定量 CPU：调度充裕时 CPU 超限判 TLE，调度受限时撞墙钟判 TLE，
	// 两种路径都不允许出现 AC。
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: loadCode(t, "parallel.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: "done"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 1000, MemoryMiB: 512},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cases[0].Verdict != domain.CaseTLE {
		t.Fatalf("parallel overuse must be TLE: %+v", got.Cases[0])
	}
	// 宿主高负载时 4 线程分不到一个整核，墙钟先于 CPU 限额触顶也判 TLE，
	// 此时 CPU/wall 对比不构成并行证据；仅在墙钟未触顶时要求 CPU 证据。
	if got.Cases[0].CpuMs <= got.Cases[0].TimeMs && got.Cases[0].TimeMs < 1000 {
		t.Fatalf("cpu must exceed wall (parallelism evidence): %+v", got.Cases[0])
	}
}

func TestCompilerPerLangLimits(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tc := Toolchain{}
	if d := tc.timeoutFor(ToolSpec{CompileTimeoutSec: 5}); d != 5*time.Second {
		t.Fatalf("per-lang timeout: %v", d)
	}
	if d := tc.timeoutFor(ToolSpec{}); d != 30*time.Second {
		t.Fatalf("default timeout: %v", d)
	}
	tc2 := Toolchain{CompileTimeout: 7 * time.Second}
	if d := tc2.timeoutFor(ToolSpec{}); d != 7*time.Second {
		t.Fatalf("global timeout: %v", d)
	}
	// 1ms 全局超时必杀真编译（表现为 OK=false；kill 无输出）。
	dir := t.TempDir()
	r, err := Toolchain{CompileTimeout: time.Millisecond}.Compile(
		ctx, "cpp", []byte("#include<iostream>\nint main(){std::cout<<1;return 0;}"), dir, CompileOptions{})
	if err != nil || r.OK {
		t.Fatalf("1ms compile timeout must kill: ok=%v err=%v", r.OK, err)
	}
	// 64MB 编译内存墙装不下 bits 头编译（正常需 300MB+）。
	// 这里刻意用 bits/stdc++.h：断言的是「重型 TU 必然撞墙」，
	// g++ 恒有该头；darwin 的 CI 过滤器不收录本测试，无 clang 兼容问题。
	dir2 := t.TempDir()
	r2, err := Toolchain{Config: ToolchainConfig{CPP: ToolSpec{CompileMemoryMiB: 64}}}.Compile(
		ctx, "cpp", []byte("#include<bits/stdc++.h>\nint main(){return 0;}"), dir2, CompileOptions{})
	if err != nil {
		t.Fatalf("walled compile should fail cleanly, not error: %v", err)
	}
	if r2.OK {
		t.Fatal("64MB wall must fail bits-header compile")
	}
}

func TestSafeFileName(t *testing.T) {
	for _, tc := range []struct{ in, fb, want string }{
		{"", "dflt", "dflt"},
		{"p.in", "dflt", "p.in"},
		{"../evil", "dflt", "evil"},
		{"../../etc/x", "dflt", "x"},
		{"/abs/path", "dflt", "path"},
		{".", "dflt", "dflt"},
		{"..", "dflt", "dflt"},
	} {
		if got := SafeFileName(tc.in, tc.fb); got != tc.want {
			t.Fatalf("SafeFileName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// 反斜杠只在 Windows 是分隔符；无论哪种解释都不得逃逸。
	if runtime.GOOS == "windows" {
		if got := SafeFileName(`..\win`, "dflt"); got != "win" {
			t.Fatalf("backslash: %q", got)
		}
	} else if got := SafeFileName(`..\win`, "dflt"); strings.Contains(got, "/") {
		t.Fatalf("backslash must stay contained: %q", got)
	}
}

func TestFileIOTraversalContained(t *testing.T) {
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// 恶意文件名必须被压平：程序读写的是工作目录内文件，照常 AC。
	code := "#include <cstdio>\nusing namespace std;\nint main(){long long a,b;FILE*f=fopen(\"p.in\",\"r\");if(!f)return 1;fscanf(f,\"%lld%lld\",&a,&b);fclose(f);f=fopen(\"p.out\",\"w\");if(!f)return 1;fprintf(f,\"%lld\",a+b);fclose(f);return 0;}"
	lim := Limits{TimeMs: 3000, MemoryMiB: 256, IOMode: domain.IOFile, InFile: "../p.in", OutFile: "../../p.out"}
	got, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: code,
		Cases:   []CaseInput{{Input: "3 4", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace, Limits: lim,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("contained traversal must still judge: %+v", got.Cases[0])
	}
}

func TestCompileOptionsAdvanced(t *testing.T) {
	cache, err := NewCompileCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tc := Toolchain{Cache: cache}
	src := []byte("#include <iostream>\nint main() { return 0; }\n")
	sp1 := tc.spec("cpp")
	sp1.Args = append(append([]string{"-O2"}, sp1.Args...), "-Wall")
	k1 := cache.key("cpp", src, sp1)

	sp2 := tc.spec("cpp")
	sp2.Args = append(append([]string{"-O3"}, sp2.Args...), "-Wall", "-Wextra")
	k2 := cache.key("cpp", src, sp2)

	if k1 == k2 {
		t.Fatal("cache key must differ for different optimization and warning options")
	}

	sp3 := sp1
	sp3.Std = "c++17"
	k3 := cache.key("cpp", src, sp3)
	if k1 == k3 {
		t.Fatal("cache key must differ for different -std standards")
	}
}
