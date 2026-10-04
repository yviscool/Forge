package judge

import (
	"context"
	"sync"

	"github.com/yviscool/forge/internal/domain"
)

// ReportFunc worker 完成一单后的回写（通常落到 app.Service.JudgeCases）。
type ReportFunc func(subID string, cases []domain.CaseResult)

// Job 一次评测任务。
type Job struct {
	SubID string
	Req   Request
}

// Pool 有界 worker 池：Submit 入队，N 个 worker 串行 JudgeOne 后 Report。
// 对标 LemonLime JudgingController 的任务分发。
type Pool struct {
	judge  func(context.Context, Request) ([]domain.CaseResult, error)
	report ReportFunc

	mu      sync.Mutex
	queue   chan Job
	wg      sync.WaitGroup
	started bool
}

// NewPool 创建评测池。workers<=0 时默认为 2。queueLen 为等待队列长度。
func NewPool(workers, queueLen int, judge func(context.Context, Request) ([]domain.CaseResult, error), report ReportFunc) *Pool {
	if workers <= 0 {
		workers = 2
	}
	if queueLen <= 0 {
		queueLen = 64
	}
	return &Pool{judge: judge, report: report, queue: make(chan Job, queueLen)}
}

// Start 启动 workers，ctx 取消时排空停机（已入队任务做完再退）。
func (p *Pool) Start(ctx context.Context, workers int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return
	}
	p.started = true
	if workers > 0 {
		for i := 0; i < workers; i++ {
			p.wg.Add(1)
			go p.loop(ctx)
		}
	}
}

// Submit 非阻塞入队：队列满返回 false（调用方转 429/稍后重试）。
func (p *Pool) Submit(j Job) bool {
	select {
	case p.queue <- j:
		return true
	default:
		return false
	}
}

// Wait 等待 workers 全部退出（配合 ctx 取消使用）。
func (p *Pool) Wait() { p.wg.Wait() }

func (p *Pool) loop(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			// 排空已入队任务再退出，保证提交不丢结果。
			for {
				select {
				case j := <-p.queue:
					p.do(context.Background(), j)
				default:
					return
				}
			}
		case j := <-p.queue:
			p.do(ctx, j)
		}
	}
}

func (p *Pool) do(ctx context.Context, j Job) {
	cases, err := p.judge(ctx, j.Req)
	if err != nil {
		cases = make([]domain.CaseResult, len(j.Req.Cases))
		for i := range cases {
			cases[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseRE}
		}
	}
	if p.report != nil {
		p.report(j.SubID, cases)
	}
}
