//go:build !windows

package judge

import (
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
)

var toolOnce sync.Once
var prlimitPath, bwrapPath string

func toolPaths() (prlimit, bwrap string) {
	toolOnce.Do(func() {
		prlimitPath, _ = exec.LookPath("prlimit")
		bwrapPath, _ = exec.LookPath("bwrap")
	})
	return prlimitPath, bwrapPath
}

// Unix 资源墙层级（对标 LemonLime processrunner_unix）：
//  1. Linux 且有 bwrap：命名空间沙箱（断网/文件系统隔离）包外层，prlimit 内层；
//  2. 其余且有 prlimit：RLIMIT_AS / RLIMIT_CPU / RLIMIT_NPROC；
//  3. macOS（无 prlimit）：/bin/sh ulimit 降级，内存墙 best-effort，
//     用量测量走 wait4 rusage（见 processPeakBytes）。
type jobLimiter struct{}

func newJobLimiter(_, _ int) (*jobLimiter, error) { return &jobLimiter{}, nil }

func confineArgv(argv []string, lim Limits, workdir string) []string {
	if len(argv) == 0 {
		return argv
	}
	prlimit, bwrap := toolPaths()
	inner := argv
	switch {
	case prlimit != "":
		inner = PrlimitArgv(argv, lim, prlimit)
	case runtime.GOOS == "darwin":
		inner = ShUlimitArgv(argv, lim)
	}
	// bwrap 只包选手程序（workdir 非空）；编译器进程树不进沙箱。
	if runtime.GOOS == "linux" && bwrap != "" && workdir != "" {
		return BwrapArgv(inner, workdir)
	}
	return inner
}

func (l *jobLimiter) Attach(_ int) error { return nil }

func (l *jobLimiter) Terminate() {}

func (l *jobLimiter) PeakBytes() uint64 { return 0 }

func (l *jobLimiter) Close() {}

// processPeakBytes 从 wait4 rusage 取峰值内存（Linux ru_maxrss 单位 KiB，
// macOS 单位字节）。bwrap/prlimit 包裹下 wait4 语义仍覆盖被收割的整棵
// 子树，选手进程的峰值不会丢。
func processPeakBytes(ps *os.ProcessState) uint64 {
	ru, ok := ps.SysUsage().(*syscall.Rusage)
	if !ok || ru == nil {
		return 0
	}
	if runtime.GOOS == "darwin" {
		return uint64(ru.Maxrss)
	}
	return uint64(ru.Maxrss) << 10
}
