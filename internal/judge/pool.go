package judge

import (
	"context"
	"errors"
	"sync"

	"github.com/yviscool/forge/internal/domain"
)

// ErrQueueFull reports a deliberately non-blocking enqueue rejection.
var ErrQueueFull = errors.New("judge queue is full")

// ReportFunc worker 完成一单后的回写（通常落到 app.Service.JudgeCases）。
type ReportFunc func(subID string, oc domain.JudgeOutcome)

// Job 一次评测任务。
type Job struct {
	SubID string
	Req   Request
}

// Pool 有界 worker 池：Submit 入队，N 个 worker 串行 JudgeOne 后 Report。
// 对标 LemonLime JudgingController 的任务分发。
type Pool struct {
	judge     func(context.Context, Request) (domain.JudgeOutcome, error)
	report    ReportFunc
	inhibitor *sleepInhibitor

	mu      sync.Mutex
	queue   chan Job
	active  map[string]struct{}
	wg      sync.WaitGroup
	started bool
	workers int
}

// Stats is a cheap, concurrency-safe operational snapshot.
type Stats struct {
	Workers  int `json:"workers"`
	Queued   int `json:"queued"`
	Active   int `json:"active"`
	Capacity int `json:"capacity"`
}

// NewPool 创建评测池。queueLen 为等待队列长度；worker 数由 Start 指定。
func NewPool(queueLen int, judge func(context.Context, Request) (domain.JudgeOutcome, error), report ReportFunc) *Pool {
	if queueLen <= 0 {
		queueLen = 64
	}
	return &Pool{
		judge: judge, report: report, queue: make(chan Job, queueLen),
		active:    make(map[string]struct{}),
		inhibitor: newSleepInhibitor(),
	}
}

// Start 启动 workers，ctx 取消时排空停机（已入队任务做完再退）。
func (p *Pool) Start(ctx context.Context, workers int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return
	}
	p.started = true
	p.workers = workers
	if workers > 0 {
		for i := 0; i < workers; i++ {
			p.wg.Add(1)
			go p.loop(ctx)
		}
	}
}

// Healthy reports whether the worker pool has live consumers.
func (p *Pool) Healthy() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started || p.workers < 1 {
		return errors.New("judge worker pool is not running")
	}
	return nil
}

// Stats returns queue pressure without exposing internal channels.
func (p *Pool) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return Stats{
		Workers:  p.workers,
		Queued:   len(p.queue),
		Active:   len(p.active),
		Capacity: cap(p.queue),
	}
}

// Submit 非阻塞入队：队列满返回 false（调用方转 429/稍后重试）。
func (p *Pool) Submit(j Job) bool {
	if !p.reserve(j.SubID) {
		return true
	}
	select {
	case p.queue <- j:
		return true
	default:
		p.release(j.SubID)
		return false
	}
}

// Enqueue waits for queue capacity and is used by restart recovery, where
// dropping a recovered submission would leave it queued forever.
func (p *Pool) Enqueue(ctx context.Context, j Job) error {
	if !p.reserve(j.SubID) {
		return nil
	}
	select {
	case <-ctx.Done():
		p.release(j.SubID)
		return ctx.Err()
	case p.queue <- j:
		return nil
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
	defer p.release(j.SubID)
	// 评测期间阻止系统休眠（Windows 实装；其余平台 no-op）。
	p.inhibitor.acquire()
	defer p.inhibitor.release()
	oc, err := p.judge(ctx, j.Req)
	if err != nil {
		// 基础设施故障（临时目录/编译器崩溃等）不得误报学生 RE：
		// 全点 SystemError，教师可一键重判（此前是死枚举）。
		cases := make([]domain.CaseResult, len(j.Req.Cases))
		for i := range cases {
			cases[i] = domain.CaseResult{CaseIndex: i, Verdict: domain.CaseSystemError}
		}
		oc = domain.JudgeOutcome{Cases: cases}
	}
	if p.report != nil {
		p.report(j.SubID, oc)
	}
}

func (p *Pool) reserve(subID string) bool {
	if subID == "" {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.active[subID]; exists {
		return false
	}
	p.active[subID] = struct{}{}
	return true
}

func (p *Pool) release(subID string) {
	if subID == "" {
		return
	}
	p.mu.Lock()
	delete(p.active, subID)
	p.mu.Unlock()
}
