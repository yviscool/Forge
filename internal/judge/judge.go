// Package judge 本地评测 worker：编译 → 分点运行（时限/内存墙）→ 输出比较。
// 对标 LemonLime TaskJudger（编译）+ JudgingThread（运行）+ 比较模式。
//
// 安全边界：每单跑在独立临时目录、最小环境变量、stdout 上限截断；
// Windows 下 Job Object 强制内存上限 + 关闭即杀树；Linux 有 bwrap 时
// 叠加命名空间沙箱（断网/文件系统隔离，对标 LemonLime processrunner），
// prlimit 兜底 RLIMIT 墙；macOS 降级 sh ulimit + wait4 rusage 测量。
package judge

import (
	"context"
	"math"
	"os"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// Limits 单点资源限制（LemonLime TestCase.timeLimit/memoryLimit 的运行态）。
// IOMode=file 时程序通过文件交互（OI 文件 IO 题）。
// OutputLimitKiB=0 时取 DefaultOutputCap。
// Interpreted 标记解释型运行时：Unix 侧跳过地址空间墙（ulimit -v /
// RLIMIT_AS 会误杀自带内存管理的运行时），保留 CPU/进程数墙。
type Limits struct {
	TimeMs         int
	MemoryMiB      int
	IOMode         domain.IOMode
	InFile         string
	OutFile        string
	OutputLimitKiB int
	Interpreted    bool
}

// DefaultOutputCap 缺省 stdout 上限 16MB。
const DefaultOutputCap = 16 << 20

// CaseInput 单测试点输入输出。Input/Expected 既可是内联文本，
// 也可是磁盘路径（存在即读文件——兼容 LemonLime 数据文件式管理）。
// Limits 非零项覆盖题目级限额（分点限额，LemonLime TestCase 语义）。
// Score 为该点分值（特判部分分与聚合用；0 表示未配置）。
type CaseInput struct {
	Input     string
	Expected  string
	Limits    Limits
	Subtask   int
	DependsOn []int
	Score     int
}

// Request 一次完整评测请求。
type Request struct {
	Language string // cpp | c | python | go
	Code     string
	Cases    []CaseInput
	Compare  domain.ComparisonMode
	RealEps  float64
	Limits   Limits
	// TaskType answers_only 时 Code 即答案文本，不编译直接比较。
	TaskType TaskType
	// Checker 非空时启用特判（input/output/answer 三参协议）。
	Checker *CheckerSpec
	// CompileTimeoutSec 本次编译上限（秒），0 则用工具链默认。
	CompileTimeoutSec int
	// Interactor 非空且 TaskType=interaction 时启用交互式评测。
	Interactor *InteractorSpec
	// Diff 非空且 Compare=external_tool 时用外部 diff 工具比较
	// （对标 LemonLime ExternalToolMode）。
	Diff *DiffSpec
	// 题目级工具链覆盖（对标 LemonLime 每题编译配置；与工具链倍率叠加）。
	CompileStd              string
	TimeLimitRatio          float64
	MemoryLimitRatio        float64
	DisableMemoryLimitCheck *bool
}

// DiffSpec 外部 diff 工具（对标 LemonLime Settings diffPath + diffArguments）。
type DiffSpec struct {
	Command string
	Args    []string
}

// DefaultDiffArgs LemonLime 缺省 diff 参数（对标 task.h ExternalToolMode）。
func DefaultDiffArgs() []string {
	return []string{"--ignore-space-change", "--text", "--brief"}
}

// TaskType 即 domain.TaskType（别名，调用方无需多导一包）。
type TaskType = domain.TaskType

// InterpretedTimeFactor 解释型语言时限倍率（对标 LemonLime extraTimeRatio
// 思想，教室场景取整为 2x：解释器启动 + 字节码开销不应计入算法时限）。
// 工具链配置了 timeLimitRatio 时以配置为准（见 CompileResult.TimeLimitRatio）。
var InterpretedTimeFactor = 2.0

// ExtraTimeRatio 硬杀墙相对软时限的额外时间比例（对标 LemonLime
// settings defaultExtraTimeRatio）：硬墙 = 时限 + max(2000ms, 2×时限)×比例。
// 落在软硬墙之间的程序视为边界抖动，有机会重测（RejudgeTimes）。
var ExtraTimeRatio = 0.25

// RejudgeTimes 边界 TLE 重测次数（对标 LemonLime settings rejudgeTimes；
// 程序在软硬墙之间完成时重跑，取重跑结果，消除抖动误杀）。
var RejudgeTimes = 1

// extraTimeMs 计算硬墙相对软时限的额外毫秒数。
func extraTimeMs(limitMs int) int {
	base := math.Max(2000, float64(limitMs)*2)
	return int(math.Ceil(base * ExtraTimeRatio))
}

// borderlineTLE 判定是否为边界抖动：程序在硬墙内完成但超出软时限，
// 且超量落在 LemonLime 口径（时限×(1+比例) 或 时限+1000×比例）内。
func borderlineTLE(run RunResult, limitMs int) bool {
	if run.TimedOut {
		return false
	}
	used := run.TimeMs
	if run.CpuMs > used {
		used = run.CpuMs
	}
	if used <= limitMs {
		return false
	}
	return used <= limitMs+int(1000*ExtraTimeRatio) ||
		used <= int(float64(limitMs)*(1+ExtraTimeRatio))
}

// resolveIO 路径存在则读文件，否则按内联文本处理。
// SubtaskMaps 由题目 TestCases 生成子任务映射（与 JudgeCases 聚合语义配套）。
// depsOf 返回各子任务声明的依赖（由用例级 DependsOn 去重聚合），供
// AggregateScoreWithDeps 的依赖清零语义使用。
// 无测试点时返回 nil, nil, nil，调用方应跳过自动评测、保留人工判题。
func SubtaskMaps(tcs []domain.TestCase) (subtaskOf func(int) int, fullOf func(int) int, depsOf func(int) []int) {
	if len(tcs) == 0 {
		return nil, nil, nil
	}
	subtaskOf = func(i int) int {
		if i < 0 || i >= len(tcs) {
			return 0
		}
		return tcs[i].Subtask
	}
	full := map[int]int{}
	deps := map[int][]int{}
	seen := map[int]map[int]bool{}
	for _, tc := range tcs {
		full[tc.Subtask] += tc.Score
		if tc.Subtask <= 0 {
			continue
		}
		for _, d := range tc.DependsOn {
			if seen[tc.Subtask] == nil {
				seen[tc.Subtask] = map[int]bool{}
			}
			if !seen[tc.Subtask][d] {
				seen[tc.Subtask][d] = true
				deps[tc.Subtask] = append(deps[tc.Subtask], d)
			}
		}
	}
	fullOf = func(st int) int { return full[st] }
	depsOf = func(st int) []int { return deps[st] }
	return subtaskOf, fullOf, depsOf
}

// Orchestrator 编排一次评测：编译一次，分点运行比较。
type Orchestrator struct {
	Compiler Compiler
	Runner   Runner
}

// JudgeOne 执行完整评测，返回分点明细 + 编译信息。
func (o *Orchestrator) JudgeOne(ctx context.Context, req Request) (domain.JudgeOutcome, error) {
	// AnswersOnly：答案文本直比，不编译不运行。
	if req.TaskType == domain.TaskAnswersOnly {
		out := make([]domain.CaseResult, len(req.Cases))
		for i, c := range req.Cases {
			out[i] = domain.CaseResult{
				CaseIndex: i,
				Verdict:   domain.CompareVerdict(req.Code, resolveIO(c.Expected), req.Compare, req.RealEps),
			}
		}
		return domain.JudgeOutcome{Cases: out}, nil
	}

	workdir, err := os.MkdirTemp("", "forge_judge_*")
	if err != nil {
		return domain.JudgeOutcome{}, err
	}
	defer os.RemoveAll(workdir)

	// 编译超时独立于运行 ctx：调用方传短 ctx 也不得掐编译（交互题修过的坑）。
	cctx := ctx
	if req.CompileTimeoutSec > 0 {
		var cancel context.CancelFunc
		cctx, cancel = context.WithTimeout(ctx, time.Duration(req.CompileTimeoutSec)*time.Second)
		defer cancel()
	}
	compile, err := o.Compiler.Compile(cctx, req.Language, []byte(req.Code), workdir, CompileOptions{Std: req.CompileStd})
	if err != nil {
		return domain.JudgeOutcome{}, err
	}
	if !compile.OK {
		out := make([]domain.CaseResult, len(req.Cases))
		for i := range out {
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseCE}
		}
		return domain.JudgeOutcome{Cases: out, CompileMessage: compile.Message}, nil
	}

	// 特判程序随单编译一次。
	var checker *compiledChecker
	if req.Checker != nil {
		checker, err = compileChecker(ctx, o.Compiler, *req.Checker, workdir)
		if err != nil {
			return domain.JudgeOutcome{}, err
		}
		if !checker.ok {
			// 特判自身编译失败：评测机特判故障，整单按 CheckerError 处理并透出信息。
			out := make([]domain.CaseResult, len(req.Cases))
			for i := range out {
				out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseCheckerError}
			}
			return domain.JudgeOutcome{Cases: out, CompileMessage: checker.msg}, nil
		}
	}

	lim := req.Limits
	if lim.TimeMs <= 0 {
		lim.TimeMs = 1000
	}
	if lim.MemoryMiB <= 0 {
		lim.MemoryMiB = 512
	}
	// 工具链声明的限额倍率优先（对标 LemonLime Compiler ratio）；
	// 未配置时解释型语言退回全局 InterpretedTimeFactor。
	if compile.TimeLimitRatio > 0 && compile.TimeLimitRatio != 1 {
		lim.TimeMs = int(math.Ceil(float64(lim.TimeMs) * compile.TimeLimitRatio))
	} else if compile.Interpreted && InterpretedTimeFactor > 1 {
		lim.TimeMs = int(float64(lim.TimeMs) * InterpretedTimeFactor)
	}
	if compile.MemoryLimitRatio > 0 && compile.MemoryLimitRatio != 1 {
		lim.MemoryMiB = int(math.Ceil(float64(lim.MemoryMiB) * compile.MemoryLimitRatio))
	}
	if compile.DisableMemoryLimitCheck {
		lim.MemoryMiB = 0
	}
	// 题目级倍率与工具链倍率叠加相乘（对标 LemonLime 每题每编译器 ratio）；
	// 题目级禁内存墙最后生效。
	if req.TimeLimitRatio > 0 && req.TimeLimitRatio != 1 {
		lim.TimeMs = int(math.Ceil(float64(lim.TimeMs) * req.TimeLimitRatio))
	}
	if req.MemoryLimitRatio > 0 && req.MemoryLimitRatio != 1 {
		lim.MemoryMiB = int(math.Ceil(float64(lim.MemoryMiB) * req.MemoryLimitRatio))
	}
	if req.DisableMemoryLimitCheck != nil && *req.DisableMemoryLimitCheck {
		lim.MemoryMiB = 0
	}
	// 解释型运行时：Unix 侧跳过地址空间墙（避免误杀），保留 CPU/进程数墙。
	lim.Interpreted = compile.Interpreted

	// 静态内存预检（对标 LemonLime watcher calculateStaticMemoryUsage）：
	// 选手在 BSS 里开超大全局数组时，起跑前即判 MLE，省一次必然失败的运行。
	if !compile.Interpreted && lim.MemoryMiB > 0 {
		if static, err := StaticMemoryBytes(compile.Executable); err == nil &&
			static > uint64(int64(lim.MemoryMiB)<<20) {
			out := make([]domain.CaseResult, len(req.Cases))
			for i := range out {
				out[i] = domain.CaseResult{
					CaseIndex: i, Verdict: domain.CaseMLE,
					MemoryKiB: int(static >> 10),
					Message:   "静态内存（全局数组）超过内存限制",
				}
			}
			return domain.JudgeOutcome{Cases: out}, nil
		}
	}
	out := make([]domain.CaseResult, len(req.Cases))
	failedSubtasks := map[int]bool{}
	for i, c := range req.Cases {
		if c.Subtask > 0 && domain.ShouldSkipCase(c.Subtask, c.DependsOn, failedSubtasks) {
			out[i] = domain.CaseResult{
				CaseIndex: i,
				Verdict:   domain.CaseSkipped,
			}
			continue
		}
		cl := mergeLimits(lim, c.Limits)
		// 交互式：跳过普通运行，走管道对接。
		if req.TaskType == domain.TaskInteraction && req.Interactor != nil {
			caseFull := c.Score
			if caseFull <= 0 {
				caseFull = 100
			}
			ir, err := RunInteraction(ctx, o.Compiler, *req.Interactor, compile.Argv(), resolveIO(c.Input), cl, workdir, caseFull)
			if err != nil {
				out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseRE}
				if c.Subtask > 0 {
					failedSubtasks[c.Subtask] = true
				}
				continue
			}
			out[i] = domain.CaseResult{
				CaseIndex: i, Verdict: ir.Verdict, TimeMs: ir.TimeMs,
				MemoryKiB: ir.MemoryKiB, CpuMs: ir.CpuMs, Score: ir.Score, Message: ir.Message,
			}
			if ir.Verdict != domain.CaseAC && c.Subtask > 0 {
				failedSubtasks[c.Subtask] = true
			}
			continue
		}
		run, err := o.Runner.Run(ctx, compile.Argv(), resolveIO(c.Input), cl, workdir)
		if err != nil {
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseRE}
			if c.Subtask > 0 {
				failedSubtasks[c.Subtask] = true
			}
			continue
		}
		// 边界抖动重测（对标 LemonLime needRejudge）：程序在软硬墙之间完成
		// 时重跑一次，取重跑结果；命中硬墙（TimedOut）的稳定 TLE 不重测。
		for attempt := 0; RejudgeTimes > 0 && attempt < RejudgeTimes; attempt++ {
			if !isOverSoftLimit(run, cl.TimeMs) || !borderlineTLE(run, cl.TimeMs) {
				break
			}
			run, err = o.Runner.Run(ctx, compile.Argv(), resolveIO(c.Input), cl, workdir)
			if err != nil {
				break
			}
		}
		if err != nil {
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseRE}
			if c.Subtask > 0 {
				failedSubtasks[c.Subtask] = true
			}
			continue
		}
		switch {
		case isOverSoftLimit(run, cl.TimeMs):
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseTLE, TimeMs: run.TimeMs, CpuMs: run.CpuMs}
		case run.OutOfMemory:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseMLE, TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB, CpuMs: run.CpuMs}
		case run.Truncated:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseOLE, TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB, CpuMs: run.CpuMs}
		case run.NoOutput:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseWA, TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB, CpuMs: run.CpuMs}
		case run.ExitCode != 0:
			// RE 附 stderr 尾部摘要（对标 LemonLime _tmperr 附带），方便学生定位。
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: mapRuntimeError(req.Language, run), TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB, CpuMs: run.CpuMs, Message: stderrTail(run.Stderr)}
		default:
			out[i] = judgeOutput(ctx, checker, c, run, req, workdir)
			out[i].CaseIndex = i
		}
		if out[i].Verdict != domain.CaseAC && c.Subtask > 0 {
			failedSubtasks[c.Subtask] = true
		}
	}
	return domain.JudgeOutcome{Cases: out}, nil
}

// mergeLimits 分点非零项覆盖题目级。
func mergeLimits(base, over Limits) Limits {
	if over.TimeMs > 0 {
		base.TimeMs = over.TimeMs
	}
	if over.MemoryMiB > 0 {
		base.MemoryMiB = over.MemoryMiB
	}
	if over.OutputLimitKiB > 0 {
		base.OutputLimitKiB = over.OutputLimitKiB
	}
	if over.IOMode != "" {
		base.IOMode = over.IOMode
	}
	if over.InFile != "" {
		base.InFile = over.InFile
	}
	if over.OutFile != "" {
		base.OutFile = over.OutFile
	}
	return base
}

// isOverSoftLimit 软时限判定：wall 或 CPU 任一超限即 TLE 口径
// （硬墙已放宽到 时限+额外时间，见 runner.go）。
func isOverSoftLimit(run RunResult, limitMs int) bool {
	return run.TimedOut || run.TimeMs > limitMs || run.CpuMs > limitMs
}

// stderrTail 取 stderr 尾部摘要（1KB，对标 LemonLime _tmperr 附加）。
func stderrTail(s string) string {
	if len(s) <= 1024 {
		return s
	}
	return s[len(s)-1024:]
}

// judgeOutput 默认比较或特判二选一；特判协议给出部分分（CasePartly 带 Score）。
func judgeOutput(ctx context.Context, checker *compiledChecker, c CaseInput, run RunResult, req Request, workdir string) domain.CaseResult {
	res := domain.CaseResult{TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB, CpuMs: run.CpuMs}
	caseFull := c.Score
	if caseFull <= 0 {
		caseFull = 100
	}
	if checker != nil {
		verdict, score, message := checker.check(ctx, resolveIO(c.Input), run.Stdout, resolveIO(c.Expected), caseFull, workdir)
		res.Verdict = verdict
		res.Score = score
		res.Message = message
		return res
	}
	if req.Compare == domain.CompareExternalTool {
		return compareExternal(ctx, req.Diff, run.Stdout, resolveIO(c.Expected), workdir, res)
	}
	res.Verdict = domain.CompareVerdict(run.Stdout, resolveIO(c.Expected), req.Compare, req.RealEps)
	return res
}

// mapRuntimeError 非零退出映射：Python 语法错误归 CE，其余归 RE。
func mapRuntimeError(lang string, run RunResult) domain.CaseVerdict {
	if (lang == "python" || lang == "python3") && strings.Contains(run.Stderr, "SyntaxError") {
		return domain.CaseCE
	}
	return domain.CaseRE
}
