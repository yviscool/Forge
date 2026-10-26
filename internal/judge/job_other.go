//go:build !windows

package judge

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
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
//  3. 无 prlimit 的 Unix（macOS/其他）：/bin/sh ulimit 降级墙（best-effort），
//     用量测量走 wait4 rusage（见 processPeakBytes）。
//     解释型运行时（Interpreted）跳过地址空间墙（ulimit -v / RLIMIT_AS 会误杀
//     自带内存管理的运行时），保留 CPU 与进程数墙。
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
		inner = PrlimitArgv(argv, effectiveLim(lim), prlimit)
	default:
		inner = ShUlimitArgv(argv, effectiveLim(lim))
	}
	// bwrap 只包选手程序（workdir 非空）；编译器与特判等辅助进程不进沙箱。
	if runtime.GOOS == "linux" && bwrap != "" && workdir != "" {
		return BwrapArgv(inner, workdir)
	}
	return inner
}

// effectiveLim 解释型运行时剥掉地址空间墙，其余墙保留。
func effectiveLim(lim Limits) Limits {
	if lim.Interpreted {
		lim.MemoryMiB = 0
	}
	return lim
}

// sysProcAttr Unix 下把子进程放进独立进程组：超时组杀不会漏掉 prlimit /
// bwrap exec 链或 fork 出的孙进程（Windows 由 Job Object 覆盖同等语义）。
func sysProcAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }

// killTree 组杀：对独立进程组发 SIGKILL（组已消亡时报 ESRCH，无害）。
func killTree(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
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

// crashedOnMemoryWall MLE 信号启发式（对标 LemonLime watcher_unix 信号映射
// SIGKILL/SIGABRT→MLE）：进程死于致命信号且（峰值贴墙 或 stderr 报
// bad_alloc —— RLIMIT_AS 拒绝分配的典型路径）时判 MLE 而非 RE。
// SIGKILL 可能来自外部 OOM killer；本方看门狗的组杀走 TLE 分支优先。
func crashedOnMemoryWall(ps *os.ProcessState, stderr string, limitBytes int64) bool {
	if limitBytes <= 0 || ps == nil {
		return false
	}
	status, ok := ps.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return false
	}
	switch status.Signal() {
	case syscall.SIGABRT, syscall.SIGSEGV, syscall.SIGBUS, syscall.SIGKILL:
	default:
		return false
	}
	if processPeakBytes(ps) >= uint64(limitBytes)*95/100 {
		return true
	}
	return strings.Contains(stderr, "bad_alloc") ||
		strings.Contains(stderr, "Cannot allocate memory")
}
