//go:build !windows

package judge

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
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

var (
	bwrapProbeOnce sync.Once
	bwrapUsable    atomic.Bool
	bwrapProbeDone = make(chan struct{})
)

// bwrapAvailable 探测 bwrap 是否真的可用于判题：两个条件都必须成立——
//  1. --unshare-all 能建命名空间（Ubuntu 23.10+ 默认限制非特权 user
//     namespaces，AppArmor 会拒绝）；
//  2. CPU 计量能穿透包装层：判题的软时限按 wall 或 user-CPU 双口径判定
//     （CCF 口径），沙箱内烧 CPU 而 wait4 的 utime 不涨的内核上，包装层
//     会让所有 CPU 判定失明（ubuntu-26.04 实测 CpuMs 恒为 1ms）。
//
// 任一不满足都退回 prlimit-only 资源墙——沙箱封装本身决不能变成选手
// 程序的 RE 或误判（对标 LemonLime "bwrap 缺失 → 不强化" 的降级）。
// 探测在后台异步执行：同步等它烧完 CPU 会吃光调用方紧巴巴的运行 ctx
// （首单全部 context deadline exceeded 的根因）。结果未就绪时先按
// prlimit-only 跑——正确性无损，probe 完成后后续评测自动启用沙箱。
func bwrapAvailable() bool {
	toolPaths()
	bwrapProbeOnce.Do(func() {
		go func() {
			defer close(bwrapProbeDone)
			bwrapProbeBody()
		}()
	})
	return bwrapUsable.Load()
}

func bwrapProbeBody() {
	if bwrapPath == "" {
		return
	}
	probe := exec.Command(
		bwrapPath, "--unshare-all", "--ro-bind", "/", "/", "--",
		"/bin/sh", "-c", "i=0; while [ $i -lt 10000000 ]; do i=$((i+1)); done",
	)
	probe.Stdout, probe.Stderr = nil, nil
	if err := probe.Run(); err != nil {
		return
	}
	if ps := probe.ProcessState; ps != nil &&
		ps.UserTime()+ps.SystemTime() >= 10*time.Millisecond {
		bwrapUsable.Store(true)
	}
}

// bwrapSandboxActive 报告选手程序当前是否真的在 bwrap 沙箱内运行
// （测试用：PID namespace 内的进程无法从外部按 PID 探测）。
func bwrapSandboxActive() bool {
	return bwrapAvailable()
}

// waitForBwrapProbe 等待后台探测完成（测试用，最多 d）。
func waitForBwrapProbe(d time.Duration) {
	toolPaths()
	select {
	case <-bwrapProbeDone:
	case <-time.After(d):
	}
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
	if runtime.GOOS == "linux" && bwrap != "" && workdir != "" && bwrapAvailable() {
		return BwrapArgv(bwrap, inner, workdir)
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
// bwrap 包裹下致命信号以 128+信号码 的退出码上报，一并识别。
func crashedOnMemoryWall(ps *os.ProcessState, stderr string, limitBytes int64) bool {
	if limitBytes <= 0 || ps == nil {
		return false
	}
	abnormal := false
	if status, ok := ps.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		switch status.Signal() {
		case syscall.SIGABRT, syscall.SIGSEGV, syscall.SIGBUS, syscall.SIGKILL:
			abnormal = true
		}
	} else if code := ps.ExitCode(); code >= 128+int(syscall.SIGABRT) {
		switch code {
		case 128 + int(syscall.SIGABRT), 128 + int(syscall.SIGBUS),
			128 + int(syscall.SIGKILL), 128 + int(syscall.SIGSEGV):
			abnormal = true
		}
	}
	if !abnormal {
		return false
	}
	if processPeakBytes(ps) >= uint64(limitBytes)*95/100 {
		return true
	}
	return strings.Contains(stderr, "bad_alloc") ||
		strings.Contains(stderr, "Cannot allocate memory")
}
