package judge

import (
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
func confineArgv(argv []string, _ Limits) []string { return argv }

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
