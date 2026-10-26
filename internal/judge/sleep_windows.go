//go:build windows

package judge

import (
	"runtime"
	"sync"

	"golang.org/x/sys/windows"
)

// 防休眠：评测期间 SetThreadExecutionState(ES_CONTINUOUS|ES_SYSTEM_REQUIRED)
// 阻止系统睡眠（对标 LemonLime sleepinhibitor.cpp 的 Windows 路径）。
// 该 API 语义是 per-thread + continuous：状态切换固定在专用锁定线程上执行，
// 引用计数归零后在同一线程清除，跨线程调用不可靠。
const (
	esContinuous     = 0x80000000
	esSystemRequired = 0x00000001
)

var sleepProc = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")

type sleepInhibitor struct {
	mu   sync.Mutex
	refs int
	cmds chan sleepCmd
}

type sleepCmd int

const (
	sleepAcquire sleepCmd = iota
	sleepRelease
)

func newSleepInhibitor() *sleepInhibitor {
	s := &sleepInhibitor{cmds: make(chan sleepCmd, 16)}
	go s.loop()
	return s
}

func (s *sleepInhibitor) loop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	enabled := false
	for cmd := range s.cmds {
		if sleepProc.Find() != nil {
			continue
		}
		switch cmd {
		case sleepAcquire:
			if !enabled {
				_, _, _ = sleepProc.Call(esContinuous | esSystemRequired)
				enabled = true
			}
		case sleepRelease:
			if enabled {
				_, _, _ = sleepProc.Call(esContinuous)
				enabled = false
			}
		}
	}
}

// acquire 引用计数 +1；0→1 时请求开启（队列满则丢弃，最坏只是不禁睡）。
func (s *sleepInhibitor) acquire() {
	s.mu.Lock()
	s.refs++
	first := s.refs == 1
	s.mu.Unlock()
	if first {
		select {
		case s.cmds <- sleepAcquire:
		default:
		}
	}
}

// release 引用计数 -1；1→0 时请求关闭。
func (s *sleepInhibitor) release() {
	s.mu.Lock()
	if s.refs > 0 {
		s.refs--
	}
	last := s.refs == 0
	s.mu.Unlock()
	if last {
		select {
		case s.cmds <- sleepRelease:
		default:
		}
	}
}
