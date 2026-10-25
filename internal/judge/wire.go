package judge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/domain"
)

// DefaultReconcileInterval bounds how long a durably queued submission can be
// stranded after a transient in-memory queue rejection or process restart.
const DefaultReconcileInterval = 30 * time.Second

// AutoJudgePool 创建自动评测池：JudgeOne 评测，回写走 svc.JudgeCases。
// 无测试点的题目跳过自动评测（保留人工判题入口）。
func AutoJudgePool(svc *app.Service, tc Toolchain) *Pool {
	o := &Orchestrator{Compiler: tc, Runner: LocalRunner{}}
	return NewPool(128, o.JudgeOne, func(subID string, oc domain.JudgeOutcome) {
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
		_, _ = svc.JudgeCases(subID, oc, so, fo)
	})
}

// EnqueueQueuedSubmissions recovers durable queued work. Enqueue blocks under
// queue pressure, while Pool deduplication makes repeated reconciliation safe.
// Submissions without automatic test cases remain queued for manual judging.
func EnqueueQueuedSubmissions(ctx context.Context, svc *app.Service, pool *Pool) (int, error) {
	subs, err := svc.Store().ListQueuedSubmissions()
	if err != nil {
		return 0, err
	}
	enqueued := 0
	var reconcileErrs []error
	for _, x := range subs {
		p, err := svc.GetProblem(x.ContestID, x.ProblemID)
		if err != nil {
			reconcileErrs = append(reconcileErrs, fmt.Errorf(
				"submission %s problem %s/%s: %w",
				x.ID, x.ContestID, x.ProblemID, err,
			))
			continue
		}
		req, ok := BuildRequest(x, p)
		if !ok {
			continue
		}
		if err := pool.Enqueue(ctx, Job{SubID: x.ID, Req: req}); err != nil {
			reconcileErrs = append(reconcileErrs, fmt.Errorf(
				"enqueue submission %s: %w", x.ID, err,
			))
			break
		}
		enqueued++
	}
	return enqueued, errors.Join(reconcileErrs...)
}

// RunQueueReconciler performs an immediate recovery pass, then periodically
// repairs queued submissions that could not enter the bounded worker queue.
func RunQueueReconciler(
	ctx context.Context,
	svc *app.Service,
	pool *Pool,
	interval time.Duration,
	logger *slog.Logger,
) {
	if interval <= 0 {
		interval = DefaultReconcileInterval
	}
	if logger == nil {
		logger = slog.Default()
	}
	reconcile := func() {
		n, err := EnqueueQueuedSubmissions(ctx, svc, pool)
		if err != nil {
			logger.Error("reconcile queued submissions", "enqueued", n, "err", err)
			return
		}
		if n > 0 {
			logger.Debug("reconciled queued submissions", "enqueued", n)
		}
	}
	reconcile()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile()
		}
	}
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
