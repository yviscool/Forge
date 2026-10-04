//go:build !windows

package judge

import (
	"fmt"
	"os/exec"
	"sync"
)

var prlimitOnce sync.Once
var prlimitPath string

// Linux 资源墙：prlimit 包裹（AS 地址空间 + CPU 秒 + nproc 防 fork 炸弹）。
// 缺 prlimit 时降级为仅超时（文档注明），verdict 映射均为 best-effort。
func confineArgv(argv []string, lim Limits) []string {
	prlimitOnce.Do(func() {
		prlimitPath, _ = exec.LookPath("prlimit")
	})
	if prlimitPath == "" || len(argv) == 0 {
		return argv
	}
	wrapped := []string{prlimitPath}
	if lim.MemoryMiB > 0 {
		wrapped = append(wrapped, fmt.Sprintf("--as=%d", int64(lim.MemoryMiB)<<20))
	}
	if lim.TimeMs > 0 {
		wrapped = append(wrapped, fmt.Sprintf("--cpu=%d", lim.TimeMs/1000+2))
	}
	wrapped = append(wrapped, "--nproc=64", "--")
	return append(wrapped, argv...)
}

func (l *jobLimiter) Attach(_ int) error { return nil }

func (l *jobLimiter) Terminate() {}

func (l *jobLimiter) PeakBytes() uint64 { return 0 }

func (l *jobLimiter) Close() {}
