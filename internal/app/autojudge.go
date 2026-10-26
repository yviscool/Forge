package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/judge"
)

// DefaultReconcileInterval bounds how long a durably queued submission can be
// stranded after a transient in-memory queue rejection or process restart.
const DefaultReconcileInterval = 30 * time.Second

// AutoJudgePool 创建自动评测池：JudgeOne 评测，回写走 ReportJudgeOutcome。
// 无测试点的题目跳过自动评测（保留人工判题入口）。
// 依赖方向：app → judge（评测核心只依赖 domain，不反向 import 应用层）。
func AutoJudgePool(svc *Service, tc judge.Toolchain) *judge.Pool {
	o := &judge.Orchestrator{Compiler: tc, Runner: judge.LocalRunner{}}
	return judge.NewPool(128, o.JudgeOne, svc.ReportJudgeOutcome)
}

// ReportJudgeOutcome 自动评测回写：按子任务映射（含依赖 DAG 清零语义）
// 聚合落库并广播 SSE；无测试点的题目留待人工判题。
func (s *Service) ReportJudgeOutcome(subID string, oc domain.JudgeOutcome) {
	x, err := s.GetSubmission(subID)
	if err != nil {
		return
	}
	p, err := s.GetProblem(x.ContestID, x.ProblemID)
	if err != nil {
		return
	}
	so, fo, do := judge.SubtaskMaps(p.TestCases)
	if so == nil {
		return
	}
	_, _ = s.JudgeCases(subID, oc, so, fo, do)
}

// EnqueueQueuedSubmissions recovers durable queued work. Enqueue blocks under
// queue pressure, while Pool deduplication makes repeated reconciliation safe.
// Submissions without automatic test cases remain queued for manual judging.
func EnqueueQueuedSubmissions(ctx context.Context, svc *Service, pool *judge.Pool) (int, error) {
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
		req, ok := judge.BuildRequest(x, p)
		if !ok {
			continue
		}
		if err := pool.Enqueue(ctx, judge.Job{SubID: x.ID, Req: req}); err != nil {
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
	svc *Service,
	pool *judge.Pool,
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
