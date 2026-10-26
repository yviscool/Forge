package judge

// LemonLime 对齐项的单元/集成测试：
// 部分分特判（Lemon scoreFile / testlib stderr）、语言限额倍率、
// 硬墙+边界重测、静态内存预检、Unix 资源墙 argv 构造、请求组装。

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

// ---- 纯函数 ----

func TestParseLemonScore(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"100\n", 100, true},
		{"  40  ", 40, true},
		{"0", 0, true},
		{"", 0, false},
		{"abc", 0, false},
		{"-5", 0, false},
	}
	for _, c := range cases {
		got, ok := parseLemonScore(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseLemonScore(%q) = %d,%v want %d,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseTestlibResult(t *testing.T) {
	cases := []struct {
		in     string
		score  float64
		hasPts bool
		fail   bool
	}{
		{"ok 42", 1, true, false},
		{"partially correct (40) numbers", 0.4, true, false},
		{"points 0.7", 0.7, true, false},
		{"points 2", 2, true, false},
		{"FAIL bad checker", 0, false, true},
		{"", 0, false, false},
		{"random noise", 0, false, false},
	}
	for _, c := range cases {
		score, hasPts, fail := parseTestlibResult(c.in)
		if hasPts != c.hasPts || fail != c.fail || (hasPts && score != c.score) {
			t.Errorf("parseTestlibResult(%q) = %v,%v,%v want %v,%v,%v",
				c.in, score, hasPts, fail, c.score, c.hasPts, c.fail)
		}
	}
}

func TestExtraTimeAndBorderline(t *testing.T) {
	if got := extraTimeMs(1000); got != 500 { // max(2000, 2000) * 0.25
		t.Errorf("extraTimeMs(1000) = %d, want 500", got)
	}
	if got := extraTimeMs(10000); got != 5000 { // max(2000, 20000) * 0.25
		t.Errorf("extraTimeMs(10000) = %d, want 5000", got)
	}
	// 边界抖动：软硬墙之间完成 → 可重测。
	if !borderlineTLE(RunResult{TimeMs: 1050}, 1000) {
		t.Error("1050/1000 应视为边界抖动")
	}
	// 硬墙击杀 → 稳定 TLE，不重测。
	if borderlineTLE(RunResult{TimedOut: true, TimeMs: 1500}, 1000) {
		t.Error("TimedOut 不应重测")
	}
	// 超出边界 → 不重测。
	if borderlineTLE(RunResult{TimeMs: 1400}, 1000) {
		t.Error("1400/1000 超出边界（limit+250 / limit*1.25）不应重测")
	}
	// 未超限 → 不重测。
	if borderlineTLE(RunResult{TimeMs: 900}, 1000) {
		t.Error("未超限不应重测")
	}
	// CPU 口径的边界抖动。
	if !borderlineTLE(RunResult{TimeMs: 100, CpuMs: 1100}, 1000) {
		t.Error("CPU 105% 应视为边界抖动")
	}
}

func TestUnixWrapBuilders(t *testing.T) {
	sh := ShUlimitArgv([]string{"/usr/bin/prog", "-x"}, Limits{TimeMs: 1500, MemoryMiB: 256})
	if sh[0] != "/bin/sh" || sh[1] != "-c" {
		t.Fatalf("sh 前缀错误: %v", sh[:2])
	}
	if !strings.Contains(sh[2], "ulimit -v") || !strings.Contains(sh[2], "ulimit -t 3") {
		t.Errorf("ulimit 脚本缺内存/CPU 墙: %q", sh[2])
	}
	if sh[3] != "/usr/bin/prog" || sh[4] != "-x" {
		t.Errorf("argv 透传错误: %v", sh[3:])
	}

	bw := BwrapArgv("/usr/bin/bwrap", []string{"prog", "arg"}, "/tmp/w")
	joined := strings.Join(bw, " ")
	for _, want := range []string{"--unshare-all", "--die-with-parent", "--ro-bind /usr /usr", "--bind /tmp/w /tmp/w", "-- prog"} {
		if !strings.Contains(joined, want) {
			t.Errorf("bwrap 参数缺 %q: %v", want, bw)
		}
	}
	// 二进制必须在 argv 头部——缺失会让 exec 去找 "--dev" 这个"可执行文件"
	// （linux CI 上真实暴露过的缺陷）。
	if bw[0] != "/usr/bin/bwrap" {
		t.Errorf("bwrap 二进制必须在 argv[0]: %v", bw)
	}
	if stripped := BwrapArgv("", []string{"prog"}, "/tmp/w"); len(stripped) != 1 || stripped[0] != "prog" {
		t.Errorf("空 bwrap 路径必须原样返回: %v", stripped)
	}

	pr := PrlimitArgv([]string{"prog"}, Limits{TimeMs: 1000, MemoryMiB: 128}, "/usr/bin/prlimit")
	joined = strings.Join(pr, " ")
	for _, want := range []string{"--as=134217728", "--cpu=3", "--nproc=256"} {
		if !strings.Contains(joined, want) {
			t.Errorf("prlimit 参数缺 %q: %v", want, pr)
		}
	}
}

func TestStaticMemoryBytesSelf(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("无法定位测试二进制")
	}
	got, err := StaticMemoryBytes(exe)
	if err != nil {
		t.Skipf("解析测试二进制失败: %v", err)
	}
	if got == 0 {
		t.Error("自身静态内存足迹应为正数")
	}
}

// ---- 编译器可用性探测 ----

func hasGPP(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("g++ 不可用，跳过编译集成用例")
	}
}

// ---- 特判协议（真实编译执行） ----

// dataDrivenLemonChecker 输入文件首个 token 是数字则作为分数写 _score，
// 否则原样写入（制造非法 _score 触发 CheckerError）。
const dataDrivenLemonChecker = `
#include <cstdio>
#include <cstdlib>
int main(int argc, char** argv) {
    FILE* in = fopen(argv[1], "r");
    char buf[64] = {0};
    if (in) { fscanf(in, "%63s", buf); fclose(in); }
    FILE* sf = fopen(argv[5], "w");
    fprintf(sf, "%s\n", buf);
    fclose(sf);
    FILE* mf = fopen(argv[6], "w");
    fprintf(mf, "lemon says %s", buf);
    fclose(mf);
    return 0;
}
`

// dataDrivenTestlibChecker 按输入 token 决定 stderr 打分标记。
const dataDrivenTestlibChecker = `
#include <cstdio>
#include <cstring>
int main(int argc, char** argv) {
    FILE* in = fopen(argv[1], "r");
    char buf[64] = {0};
    if (in) { fscanf(in, "%63s", buf); fclose(in); }
    if (!strcmp(buf, "fail")) { fprintf(stderr, "FAIL broken"); return 3; }
    if (!strcmp(buf, "wa")) { fprintf(stderr, "wrong answer x"); return 1; }
    if (!strcmp(buf, "partial")) { fprintf(stderr, "partially correct (40) xx"); return 0; }
    if (!strcmp(buf, "points")) { fprintf(stderr, "points 0.7"); return 0; }
    fprintf(stderr, "ok 42");
    return 0;
}
`

func compileFlavorChecker(t *testing.T, flavor, src string) *compiledChecker {
	t.Helper()
	dir := t.TempDir()
	checker, err := compileChecker(context.Background(), Toolchain{}, CheckerSpec{
		Language: "cpp", Code: src, Flavor: flavor,
	}, dir)
	if err != nil {
		t.Fatalf("compileChecker: %v", err)
	}
	if !checker.ok {
		t.Fatalf("checker 编译失败: %s", checker.msg)
	}
	return checker
}

func TestLemonCheckerFlavor(t *testing.T) {
	hasGPP(t)
	c := compileFlavorChecker(t, "lemon", dataDrivenLemonChecker)
	cases := []struct {
		input       string
		wantVerdict domain.CaseVerdict
		wantScore   int
	}{
		{"100", domain.CaseAC, 100},
		{"40", domain.CasePartly, 40},
		{"0", domain.CaseWA, 0},
	}
	for _, tc := range cases {
		v, score, msg := c.check(context.Background(), tc.input, "out", "ans", 100, t.TempDir())
		if v != tc.wantVerdict || score != tc.wantScore {
			t.Errorf("lemon(%q) = %v,%d want %v,%d", tc.input, v, score, tc.wantVerdict, tc.wantScore)
		}
		if msg != "" && !strings.Contains(msg, "lemon says") {
			t.Errorf("lemon(%q) message 异常: %q", tc.input, msg)
		}
	}
	// 非法 _score → CheckerError。
	v, _, _ := c.check(context.Background(), "oops", "out", "ans", 100, t.TempDir())
	if v != domain.CaseCheckerError {
		t.Errorf("lemon(非法 scorefile) = %v, want CheckerError", v)
	}
}

func TestTestlibCheckerFlavor(t *testing.T) {
	hasGPP(t)
	c := compileFlavorChecker(t, "testlib", dataDrivenTestlibChecker)
	cases := []struct {
		input       string
		wantVerdict domain.CaseVerdict
		wantScore   int
	}{
		{"ok", domain.CaseAC, 100},
		{"partial", domain.CasePartly, 40},
		{"points", domain.CasePartly, 70},
		{"wa", domain.CaseWA, 0},
		{"fail", domain.CaseCheckerError, 0},
	}
	for _, tc := range cases {
		v, score, _ := c.check(context.Background(), tc.input, "out", "ans", 100, t.TempDir())
		if v != tc.wantVerdict || score != tc.wantScore {
			t.Errorf("testlib(%q) = %v,%d want %v,%d", tc.input, v, score, tc.wantVerdict, tc.wantScore)
		}
	}
}

// ---- JudgeOne 编排：倍率 / 禁用内存墙 / 边界重测 / 静态内存 ----

type fakeCompileRunner struct {
	compile CompileResult
	runs    []RunResult
	gotLim  []Limits
}

func (f *fakeCompileRunner) Compile(_ context.Context, _ string, _ []byte, _ string, _ CompileOptions) (CompileResult, error) {
	return f.compile, nil
}

func (f *fakeCompileRunner) Run(_ context.Context, _ []string, _ string, lim Limits, _ string) (RunResult, error) {
	f.gotLim = append(f.gotLim, lim)
	if len(f.runs) > 0 {
		r := f.runs[0]
		f.runs = f.runs[1:]
		return r, nil
	}
	return RunResult{ExitCode: 0, Stdout: "42"}, nil
}

func okCompile() CompileResult {
	return CompileResult{OK: true, Executable: "no-such-exe"}
}

func TestJudgeOneToolchainRatios(t *testing.T) {
	// 时限倍率 3x、内存倍率 2x。
	fr := &fakeCompileRunner{compile: CompileResult{
		OK: true, Executable: "no-such-exe",
		TimeLimitRatio: 3, MemoryLimitRatio: 2,
	}}
	req := Request{
		Language: "cpp", Code: "int main(){}",
		Cases:    []CaseInput{{Input: "1", Expected: "42"}},
		Limits:   Limits{TimeMs: 1000, MemoryMiB: 256},
		TaskType: domain.TaskTraditional,
	}
	if _, err := (&Orchestrator{Compiler: fr, Runner: fr}).JudgeOne(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(fr.gotLim) == 0 {
		t.Fatal("runner 未被调用")
	}
	if got := fr.gotLim[0].TimeMs; got != 3000 {
		t.Errorf("时限倍率未生效: got %d want 3000", got)
	}
	if got := fr.gotLim[0].MemoryMiB; got != 512 {
		t.Errorf("内存倍率未生效: got %d want 512", got)
	}
}

func TestJudgeOneDisableMemoryLimit(t *testing.T) {
	fr := &fakeCompileRunner{compile: CompileResult{
		OK: true, Executable: "no-such-exe", DisableMemoryLimitCheck: true,
	}}
	req := Request{
		Language: "cpp", Code: "int main(){}",
		Cases:    []CaseInput{{Input: "1", Expected: "42"}},
		Limits:   Limits{TimeMs: 1000, MemoryMiB: 256},
		TaskType: domain.TaskTraditional,
	}
	if _, err := (&Orchestrator{Compiler: fr, Runner: fr}).JudgeOne(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got := fr.gotLim[0].MemoryMiB; got != 0 {
		t.Errorf("DisableMemoryLimitCheck 未生效: got %d want 0", got)
	}
}

func TestJudgeOneBorderlineRejudge(t *testing.T) {
	// 第一次超软限 5%（边界抖动），第二次回到限内 → 最终 AC，共跑 2 次。
	fr := &fakeCompileRunner{
		compile: okCompile(),
		runs: []RunResult{
			{ExitCode: 0, TimeMs: 1050, Stdout: "42"},
			{ExitCode: 0, TimeMs: 900, Stdout: "42"},
		},
	}
	req := Request{
		Language: "cpp", Code: "int main(){}",
		Cases:    []CaseInput{{Input: "1", Expected: "42"}},
		Limits:   Limits{TimeMs: 1000, MemoryMiB: 256},
		TaskType: domain.TaskTraditional,
	}
	oc, err := (&Orchestrator{Compiler: fr, Runner: fr}).JudgeOne(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(fr.gotLim) != 2 {
		t.Fatalf("期望边界重测一次（共 2 次运行），实际 %d 次", len(fr.gotLim))
	}
	if oc.Cases[0].Verdict != domain.CaseAC {
		t.Errorf("重测后应为 AC，got %v", oc.Cases[0].Verdict)
	}
}

func TestJudgeOneStableTLENoRejudge(t *testing.T) {
	// 硬墙击杀：稳定 TLE，不重测。
	fr := &fakeCompileRunner{
		compile: okCompile(),
		runs:    []RunResult{{TimedOut: true, TimeMs: 1500}},
	}
	req := Request{
		Language: "cpp", Code: "int main(){}",
		Cases:    []CaseInput{{Input: "1", Expected: "42"}},
		Limits:   Limits{TimeMs: 1000, MemoryMiB: 256},
		TaskType: domain.TaskTraditional,
	}
	oc, err := (&Orchestrator{Compiler: fr, Runner: fr}).JudgeOne(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(fr.gotLim) != 1 {
		t.Errorf("TimedOut 不应重测，运行了 %d 次", len(fr.gotLim))
	}
	if oc.Cases[0].Verdict != domain.CaseTLE {
		t.Errorf("应为 TLE，got %v", oc.Cases[0].Verdict)
	}
	// 超软限且远超边界（不重测），仍按 wall 判 TLE。
	fr2 := &fakeCompileRunner{
		compile: okCompile(),
		runs:    []RunResult{{ExitCode: 0, TimeMs: 2000, Stdout: "42"}},
	}
	oc2, err := (&Orchestrator{Compiler: fr2, Runner: fr2}).JudgeOne(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(fr2.gotLim) != 1 || oc2.Cases[0].Verdict != domain.CaseTLE {
		t.Errorf("远超边界应直接 TLE 且不重测：runs=%d verdict=%v", len(fr2.gotLim), oc2.Cases[0].Verdict)
	}
}

func TestJudgeOneStaticMemoryMLE(t *testing.T) {
	hasGPP(t)
	// 1GiB 全局数组：静态内存预检直接 MLE，不进入运行。
	tc := Toolchain{}
	req := Request{
		Language: "cpp",
		Code:     "int big[1 << 28];\nint main() { return 0; }",
		Cases:    []CaseInput{{Input: "1", Expected: "42", Score: 100}},
		Limits:   Limits{TimeMs: 1000, MemoryMiB: 256},
		TaskType: domain.TaskTraditional,
	}
	oc, err := (&Orchestrator{Compiler: tc, Runner: LocalRunner{}}).JudgeOne(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseMLE {
		t.Fatalf("超大全局数组应预检 MLE，got %v", oc.Cases[0].Verdict)
	}
	if !strings.Contains(oc.Cases[0].Message, "静态内存") {
		t.Errorf("MLE 应带静态内存附注: %q", oc.Cases[0].Message)
	}
}

// ---- 请求组装 ----

func TestBuildRequestCaseScoreAndFlavor(t *testing.T) {
	p := domain.Problem{
		TaskType:    domain.TaskTraditional,
		CompareMode: domain.CompareLemonSpecialJudge,
		CheckerCode: "int main(){}",
		TestCases: []domain.TestCase{
			{ID: "t1", InputFile: "1", OutputFile: "42", Score: 30, TimeLimitMs: 1000, MemoryMiB: 256},
		},
		TimeLimitMs: 1000, MemoryLimitMiB: 256,
	}
	req, ok := BuildRequest(domain.Submission{Language: "cpp", Code: "int main(){}"}, p)
	if !ok {
		t.Fatal("BuildRequest 应成功")
	}
	if req.Cases[0].Score != 30 {
		t.Errorf("分点分值未透传: %d", req.Cases[0].Score)
	}
	if req.Checker == nil || req.Checker.Flavor != "lemon" {
		t.Errorf("lemon 模式未映射特判协议: %+v", req.Checker)
	}
	if req.Compare != domain.CompareLemonSpecialJudge {
		t.Errorf("Compare 应保持 lemon 模式: %v", req.Compare)
	}

	p.CompareMode = domain.CompareTestlibSpecialJudge
	req, _ = BuildRequest(domain.Submission{Language: "cpp", Code: "x"}, p)
	if req.Checker == nil || req.Checker.Flavor != "testlib" {
		t.Errorf("testlib 模式未映射: %+v", req.Checker)
	}

	p.CompareMode = domain.CompareSpecialJudge
	req, _ = BuildRequest(domain.Submission{Language: "cpp", Code: "x"}, p)
	if req.Checker == nil || req.Checker.Flavor != "" {
		t.Errorf("经典 special_judge 应为退出码协议: %+v", req.Checker)
	}
}
