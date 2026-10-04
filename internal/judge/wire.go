package judge

import (
	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/domain"
)

// AutoJudgePool 创建自动评测池：JudgeOne 评测，回写走 svc.JudgeCases。
// 无测试点的题目跳过自动评测（保留人工判题入口）。
func AutoJudgePool(svc *app.Service, workers int, tc Toolchain) *Pool {
	o := &Orchestrator{Compiler: tc, Runner: LocalRunner{}}
	return NewPool(workers, 128, o.JudgeOne, func(subID string, cases []domain.CaseResult) {
		x, err := svc.GetSubmission(subID)
		if err != nil {
			return
		}
		p, err := svc.GetProblem(x.ContestID, x.ProblemID)
		if err != nil {
			return
		}
		so, fo := SubtaskMaps(p.TestCases)
		if so == nil {
			return
		}
		_, _ = svc.JudgeCases(subID, cases, so, fo)
	})
}

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
			Limits: Limits{TimeMs: tc.TimeLimitMs, MemoryMiB: tc.MemoryMiB},
		}
	}
	req := Request{
		Language: x.Language, Code: x.Code, Cases: cases,
		Compare: cmpOf(p), RealEps: p.RealEps, TaskType: p.TaskType,
		Limits: Limits{
			TimeMs: p.TimeLimitMs, MemoryMiB: p.MemoryLimitMiB,
			IOMode: ioOf(p), InFile: p.InFile, OutFile: p.OutFile,
		},
	}
	if req.Limits.InFile == "" {
		req.Limits.InFile = p.Code + ".in"
	}
	if req.Limits.OutFile == "" {
		req.Limits.OutFile = p.Code + ".out"
	}
	if p.CheckerCode != "" {
		req.Checker = &CheckerSpec{Language: checkerLang(p.CheckerLang), Code: p.CheckerCode}
		req.Compare = domain.CompareSpecialJudge
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
