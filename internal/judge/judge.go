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

	"github.com/yviscool/forge/internal/domain"
)

// Limits 单点资源限制（LemonLime TestCase.timeLimit/memoryLimit 的运行态）。
type Limits struct {
	TimeMs    int
	MemoryMiB int
}

// CaseInput 单测试点输入输出。Input/Expected 既可是内联文本，
// 也可是磁盘路径（存在即读文件——兼容 LemonLime 数据文件式管理）。
type CaseInput struct {
	Input    string
	Expected string
}

// Request 一次完整评测请求。
type Request struct {
	Language string // cpp | c | python | go
	Code     string
	Cases    []CaseInput
	Compare  domain.ComparisonMode
	RealEps  float64
	Limits   Limits
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

// JudgeOne 执行完整评测，返回与 Cases 一一对应的 CaseResult。
func (o *Orchestrator) JudgeOne(ctx context.Context, req Request) ([]domain.CaseResult, error) {
	workdir, err := os.MkdirTemp("", "forge_judge_*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workdir)

	compile, err := o.Compiler.Compile(ctx, req.Language, []byte(req.Code), workdir)
	if err != nil {
		return nil, err
	}
	if !compile.OK {
		out := make([]domain.CaseResult, len(req.Cases))
		for i := range out {
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseCE}
		}
		return out, nil
	}

	lim := req.Limits
	if lim.TimeMs <= 0 {
		lim.TimeMs = 1000
	}
	if lim.MemoryMiB <= 0 {
		lim.MemoryMiB = 512
	}
	out := make([]domain.CaseResult, len(req.Cases))
	for i, c := range req.Cases {
		run, err := o.Runner.Run(ctx, compile.Argv(), resolveIO(c.Input), lim, workdir)
		if err != nil {
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseRE}
			continue
		}
		switch {
		case run.TimedOut:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseTLE, TimeMs: run.TimeMs}
		case run.OutOfMemory:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseMLE, TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB}
		case run.ExitCode != 0:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: mapRuntimeError(req.Language, run), TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB}
		case domain.CompareOutput(run.Stdout, resolveIO(c.Expected), req.Compare, req.RealEps):
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseAC, Score: 0, TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB}
		default:
			out[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseWA, TimeMs: run.TimeMs, MemoryKiB: run.PeakKiB}
		}
	}
	return out, nil
}

// mapRuntimeError 非零退出映射：Python 语法错误归 CE，其余归 RE。
func mapRuntimeError(lang string, run RunResult) domain.CaseVerdict {
	if (lang == "python" || lang == "python3") && strings.Contains(run.Stderr, "SyntaxError") {
		return domain.CaseCE
	}
	return domain.CaseRE
}
