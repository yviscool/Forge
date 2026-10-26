package judge

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// jobLimiter Windows Job Object 内存墙 + 关闭即杀树。
type jobLimiter struct {
	job windows.Handle
}

func newJobLimiter(memoryMiB, maxProcs int) (*jobLimiter, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	if memoryMiB > 0 {
		var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		info.BasicLimitInformation.LimitFlags =
			windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
				windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY |
				windows.JOB_OBJECT_LIMIT_JOB_MEMORY
		if maxProcs > 0 {
			info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
			info.BasicLimitInformation.ActiveProcessLimit = uint32(maxProcs)
		}
		info.ProcessMemoryLimit = uintptr(int64(memoryMiB) << 20)
		info.JobMemoryLimit = uintptr(int64(memoryMiB) << 20)
		_, err = windows.SetInformationJobObject(
			job,
			windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)),
			uint32(unsafe.Sizeof(info)),
		)
		if err != nil {
			windows.CloseHandle(job)
			return nil, err
		}
	} else {
		// 无内存墙也要 KILL_ON_CLOSE；单进程墙只在 maxProcs>0 时加
		// （编译器会起 cc1plus 等子进程，绝不能套单进程墙）。
		var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if maxProcs > 0 {
			info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
			info.BasicLimitInformation.ActiveProcessLimit = uint32(maxProcs)
		}
		_, err = windows.SetInformationJobObject(
			job,
			windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)),
			uint32(unsafe.Sizeof(info)),
		)
		if err != nil {
			windows.CloseHandle(job)
			return nil, err
		}
	}
	return &jobLimiter{job: job}, nil
}

// confineArgv Windows 下直通（Job Object 已做内存墙+单进程墙）。
func confineArgv(argv []string, _ Limits, _ string) []string { return argv }

// processPeakBytes Windows 无需 rusage 回退：Job Object 已给峰值。
func processPeakBytes(_ *os.ProcessState) uint64 { return 0 }

// sysProcAttr Windows 通用进程属性:隐藏控制台窗(教师机上 GUI/弹窗类
// 提交不得闪窗,沙箱内外一致);杀树由 Job Object 覆盖。
func sysProcAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{HideWindow: true} }

// killTree Windows 无进程组信号；组杀统一走 jobLimiter.Terminate。
func killTree(int) {}

// crashedOnMemoryWall Windows 的 MLE 判定走 Job 内存墙峰值贴墙路径
// （runner.go），无需退出信号启发式。
func crashedOnMemoryWall(*os.ProcessState, string, int64) bool { return false }

// Attach 将已启动进程纳入 Job（调用方需在 Start 后立刻执行）。
// 通过 PID 二次打开句柄：os.Process 不暴露 Windows 句柄。
func (l *jobLimiter) Attach(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(l.job, h)
}

// Terminate 杀掉整个 Job 树（超时路径）。
func (l *jobLimiter) Terminate() {
	_ = windows.TerminateJobObject(l.job, 1)
}

// OutOfMemory Windows 的 OOM 判定走 Job 峰值贴墙路径(runner.go)。
func (l *jobLimiter) OutOfMemory() bool { return false }

// PeakBytes 进程峰值物理内存（用于用量上报与 MLE 贴墙判定）。
func (l *jobLimiter) PeakBytes() uint64 {
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	var ret uint32
	err := windows.QueryInformationJobObject(
		l.job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		&ret,
	)
	if err != nil {
		return 0
	}
	if info.PeakProcessMemoryUsed > info.PeakJobMemoryUsed {
		return uint64(info.PeakProcessMemoryUsed)
	}
	return uint64(info.PeakJobMemoryUsed)
}

func (l *jobLimiter) Close() {
	windows.CloseHandle(l.job)
}

// BwrapStatus Windows 无 bwrap 路径(观测矩阵用)。
func BwrapStatus() string { return "off:not linux" }

// CgroupStatus Windows 无 cgroup 路径(观测矩阵用)。
func CgroupStatus() string { return "off:not linux" }
