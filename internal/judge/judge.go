// Package judge 本地评测 worker：编译 → 分点运行（时限/内存墙）→ 输出比较。
// 对标 LemonLime TaskJudger（编译）+ JudgingThread（运行）+ 比较模式。
//
// 安全边界：每单跑在独立临时目录、最小环境变量、stdout 上限截断；
// Windows 下 Job Object 强制内存上限 + 关闭即杀树；超时走 TerminateJob。
// 剩余硬化项（网络隔离、Linux cgroup）见 runner.go 注释，后续补。
package judge

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// Limits 单点资源限制（LemonLime TestCase.timeLimit/memoryLimit 的运行态）。
// IOMode=file 时程序通过文件交互（OI 文件 IO 题）。
// OutputLimitKiB=0 时取 DefaultOutputCap。
type Limits struct {
	TimeMs         int
	MemoryMiB      int
	IOMode         domain.IOMode
	InFile         string
	OutFile        string
	OutputLimitKiB int
}

// DefaultOutputCap 缺省 stdout 上限 16MB。
const DefaultOutputCap = 16 << 20

// CaseInput 单测试点输入输出。Input/Expected 既可是内联文本，
// 也可是磁盘路径（存在即读文件——兼容 LemonLime 数据文件式管理）。
// Limits 非零项覆盖题目级限额（分点限额，LemonLime TestCase 语义）。
type CaseInput struct {
	Input    string
	Expected string
	Limits   Limits
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
}

// TaskType 即 domain.TaskType（别名，调用方无需多导一包）。
type TaskType = domain.TaskType

// InterpretedTimeFactor 解释型语言时限倍率（对标 LemonLime extraTimeRatio
// 思想，教室场景取整为 2x：解释器启动 + 字节码开销不应计入算法时限）。
var InterpretedTimeFactor = 2.0

// CheckerSpec 特判程序源码（随题目下发，评测机编译执行）。
type CheckerSpec struct {
	Language string
	Code     string
}

// resolveIO 路径存在则读文件，否则按内联文本处理。
func resolveIO(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	if len(t) < 4096 {
		if fi, err := os.Stat(t); err == nil && !fi.IsDir() {
			if b, err := os.ReadFile(t); err == nil {
				return string(b)
			}
		}
	}
	return s
}

// SubtaskMaps 由题目 TestCases 生成子任务映射（与 JudgeCases 聚合语义配套）。
// 无测试点时返回 nil, nil，调用方应跳过自动评测、保留人工判题。
func SubtaskMaps(tcs []domain.TestCase) (subtaskOf func(int) int, fullOf func(int) int) {
	if len(tcs) == 0 {
		return nil, nil
	}
	subtaskOf = func(i int) int {
		if i < 0 || i >= len(tcs) {
			return 0
		}
		return tcs[i].Subtask
	}
	full := map[int]int{}
	for _, tc := range tcs {
		full[tc.Subtask] += tc.Score
	}
	fullOf = func(st int) int { return full[st] }
	return subtaskOf, fullOf
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
			if domain.CompareOutput(req.Code, resolveIO(c.Expected), req.Compare, req.RealEps) {
				out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseAC}
			} else {
				out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseWA}
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
	compile, err := o.Compiler.Compile(cctx, req.Language, []byte(req.Code), workdir)
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
			// 特判自身编译失败：判题事故，整单按 RE 处理并透出信息。
			out := make([]domain.CaseResult, len(req.Cases))
			for i := range out {
				out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseRE}
			}
			return domain.JudgeOutcome{Cases: out}, nil
		}
	}

	lim := req.Limits
	if lim.TimeMs <= 0 {
		lim.TimeMs = 1000
	}
	if lim.MemoryMiB <= 0 {
		lim.MemoryMiB = 512
	}
	// 解释型语言时限补偿（编译产物 compile.Interpreted 标识）。
	if compile.Interpreted && InterpretedTimeFactor > 1 {
		lim.TimeMs = int(float64(lim.TimeMs) * InterpretedTimeFactor)
	}
	out := make([]domain.CaseResult, len(req.Cases))
	for i, c := range req.Cases {
		cl := mergeLimits(lim, c.Limits)
		// 交互式：跳过普通运行，走管道对接。
		if req.TaskType == domain.TaskInteraction && req.Interactor != nil {
			ir, err := RunInteraction(ctx, o.Compiler, *req.Interactor, compile.Argv(), resolveIO(c.Input), cl, workdir)
			if err != nil {
				out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseRE}
				continue
			}
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: ir.Verdict, TimeMs: ir.TimeMs}
			continue
		}
		run, err := o.Runner.Run(ctx, compile.Argv(), resolveIO(c.Input), cl, workdir)
		if err != nil {
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseRE}
			continue
		}
		switch {
		case run.TimedOut || run.CpuMs > cl.TimeMs:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseTLE, TimeMs: run.TimeMs, CpuMs: run.CpuMs}
		case run.OutOfMemory:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseMLE, TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB, CpuMs: run.CpuMs}
		case run.Truncated:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseOLE, TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB, CpuMs: run.CpuMs}
		case run.NoOutput:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseWA, TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB, CpuMs: run.CpuMs}
		case run.ExitCode != 0:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: mapRuntimeError(req.Language, run), TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB, CpuMs: run.CpuMs}
		default:
			out[i] = judgeOutput(ctx, o.Runner, checker, compile, c, run, req, cl, workdir)
			out[i].CaseIndex = i
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

// judgeOutput 默认比较或特判二选一。
func judgeOutput(ctx context.Context, _ Runner, checker *compiledChecker, _ CompileResult, c CaseInput, run RunResult, req Request, _ Limits, workdir string) domain.CaseResult {
	res := domain.CaseResult{TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB, CpuMs: run.CpuMs}
	if checker != nil {
		ok, msg := checker.check(ctx, resolveIO(c.Input), run.Stdout, resolveIO(c.Expected), workdir)
		_ = msg
		if ok {
			res.Verdict = domain.CaseAC
		} else {
			res.Verdict = domain.CaseWA
		}
		return res
	}
	if domain.CompareOutput(run.Stdout, resolveIO(c.Expected), req.Compare, req.RealEps) {
		res.Verdict = domain.CaseAC
	} else {
		res.Verdict = domain.CaseWA
	}
	return res
}

// mapRuntimeError 非零退出映射：Python 语法错误归 CE，其余归 RE。
func mapRuntimeError(lang string, run RunResult) domain.CaseVerdict {
	if (lang == "python" || lang == "python3") && strings.Contains(run.Stderr, "SyntaxError") {
		return domain.CaseCE
	}
	return domain.CaseRE
}
