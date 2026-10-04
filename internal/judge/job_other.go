//go:build !windows

package judge

// jobLimiter 非 Windows 占位：仅 Rab timeout 生效，内存墙待 cgroup 接入。
// Linux 接入点：clone3 进入 memory cgroup 或 prlimit(RLIMIT_AS) 子进程包装.
type jobLimiter struct{}

func newJobLimiter(_ int) (*jobLimiter, error) { return &jobLimiter{}, nil }

func (l *jobLimiter) Attach(_ int) error { return nil }

func (l *jobLimiter) Terminate() {}

func (l *jobLimiter) PeakBytes() uint64 { return 0 }

func (l *jobLimiter) Close() {}
