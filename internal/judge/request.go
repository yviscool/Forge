package judge

import (
	"github.com/yviscool/forge/internal/domain"
)

// BuildRequest 由提交 + 题目组装评测请求。
// TestCase.InputFile/OutputFile 支持内联文本或数据文件路径（存在即读）。
// CompareMode 为空时默认 ignore_space（与 LemonLime 缺省一致）。
func BuildRequest(x domain.Submission, p domain.Problem) (Request, bool) {
	if p.TaskType == domain.TaskAnswersOnly {
		if len(p.TestCases) == 0 {
			return Request{}, false
		}
		cases := make([]CaseInput, len(p.TestCases))
		for i, tc := range p.TestCases {
			cases[i] = CaseInput{Expected: tc.OutputFile}
		}
		return Request{
			Language: x.Language, Code: x.Code, Cases: cases,
			Compare: cmpOf(p), RealEps: p.RealEps, TaskType: domain.TaskAnswersOnly,
		}, true
	}
	if len(p.TestCases) == 0 {
		return Request{}, false
	}
	cases := make([]CaseInput, len(p.TestCases))
	for i, tc := range p.TestCases {
		cases[i] = CaseInput{
			Input: tc.InputFile, Expected: tc.OutputFile,
			Limits:    Limits{TimeMs: tc.TimeLimitMs, MemoryMiB: tc.MemoryMiB},
			Subtask:   tc.Subtask,
			DependsOn: append([]int(nil), tc.DependsOn...),
			Score:     tc.Score,
		}
	}
	req := Request{
		Language: x.Language, Code: x.Code, Cases: cases,
		Compare: cmpOf(p), RealEps: p.RealEps, TaskType: p.TaskType,
		CompileTimeoutSec: p.CompileTimeoutSec,
		// 题目级工具链覆盖（对标 LemonLime 每题每编译器配置）。
		CompileStd:              p.CompileStd,
		CompileOpt:              p.CompileOpt,
		CompileWarn:             p.CompileWarn,
		CompileFlags:            p.CompileFlags,
		TimeLimitRatio:          p.TimeLimitRatio,
		MemoryLimitRatio:        p.MemoryLimitRatio,
		DisableMemoryLimitCheck: p.DisableMemoryLimitCheck,
		Limits: Limits{
			TimeMs: p.TimeLimitMs, MemoryMiB: p.MemoryLimitMiB,
			IOMode: ioOf(p), InFile: p.InFile, OutFile: p.OutFile,
			OutputLimitKiB: p.OutputLimitKiB,
		},
	}
	if req.Limits.InFile == "" {
		req.Limits.InFile = p.Code + ".in"
	}
	if req.Limits.OutFile == "" {
		req.Limits.OutFile = p.Code + ".out"
	}
	if p.CheckerCode != "" {
		flavor := checkerFlavor(p.CompareMode)
		req.Checker = &CheckerSpec{Language: checkerLang(p.CheckerLang), Code: p.CheckerCode, Flavor: flavor}
		switch flavor {
		case "lemon":
			req.Compare = domain.CompareLemonSpecialJudge
		case "testlib":
			req.Compare = domain.CompareTestlibSpecialJudge
		default:
			req.Compare = domain.CompareSpecialJudge
		}
	}
	if req.Compare == domain.CompareExternalTool {
		req.Diff = &DiffSpec{Command: p.DiffCommand, Args: p.DiffArgs}
	}
	if p.TaskType == domain.TaskInteraction && p.InteractorCode != "" {
		req.Interactor = &InteractorSpec{Language: checkerLang(p.InteractorLang), Code: p.InteractorCode}
	}
	return req, true
}

func cmpOf(p domain.Problem) domain.ComparisonMode {
	if p.CompareMode != "" {
		return p.CompareMode
	}
	return domain.CompareIgnoreSpace
}

func ioOf(p domain.Problem) domain.IOMode {
	if p.IOMode != "" {
		return p.IOMode
	}
	return domain.IOStdio
}

func checkerLang(s string) string {
	if s == "" {
		return "cpp"
	}
	return s
}

// checkerFlavor 由题目的比较模式推导特判协议：
// lemon_special_judge → Lemon scoreFile 协议；testlib_special_judge →
// testlib stderr 协议；其余（special_judge/custom_checker）→ 退出码协议。
func checkerFlavor(mode domain.ComparisonMode) string {
	switch domain.NormalizeComparisonMode(mode) {
	case domain.CompareLemonSpecialJudge:
		return "lemon"
	case domain.CompareTestlibSpecialJudge:
		return "testlib"
	default:
		return ""
	}
}
