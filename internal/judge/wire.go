package judge

import (
	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/domain"
)

// AutoJudgePool 创建自动评测池：JudgeOne 评测，回写走 svc.JudgeCases。
// 无测试点的题目跳过自动评测（保留人工判题入口）。
func AutoJudgePool(svc *app.Service, workers int) *Pool {
	o := &Orchestrator{Compiler: Toolchain{}, Runner: LocalRunner{}}
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
	if len(p.TestCases) == 0 {
		return Request{}, false
	}
	cases := make([]CaseInput, len(p.TestCases))
	for i, tc := range p.TestCases {
		cases[i] = CaseInput{Input: tc.InputFile, Expected: tc.OutputFile}
	}
	cmp := p.CompareMode
	if cmp == "" {
		cmp = domain.CompareIgnoreSpace
	}
	return Request{
		Language: x.Language,
		Code:     x.Code,
		Cases:    cases,
		Compare:  cmp,
		RealEps:  p.RealEps,
		Limits:   Limits{TimeMs: p.TimeLimitMs, MemoryMiB: p.MemoryLimitMiB},
	}, true
}
