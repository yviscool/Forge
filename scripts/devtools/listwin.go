//go:build ignore

// 枚举指定进程的所有顶层窗口：类名/标题/尺寸/可见性。
// 用法：go run scripts/devtools/listwin.go <pid>
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	pEnumWindows            = user32.NewProc("EnumWindows")
	pGetWindowThreadProcess = user32.NewProc("GetWindowThreadProcessId")
	pGetWindowTextW         = user32.NewProc("GetWindowTextW")
	pGetClassNameW          = user32.NewProc("GetClassNameW")
	pIsWindowVisible        = user32.NewProc("IsWindowVisible")
	pGetWindowRect          = user32.NewProc("GetWindowRect")
	pGetWindowLong          = user32.NewProc("GetWindowLongW")
	pShowWindow             = user32.NewProc("ShowWindow")
)

type rect struct{ Left, Top, Right, Bottom int32 }

func text(proc *syscall.LazyProc, hwnd syscall.Handle, max int) string {
	buf := make([]uint16, max)
	proc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(max))
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: listwin <pid>")
		os.Exit(1)
	}
	pid, _ := strconv.Atoi(os.Args[1])
	fmt.Printf("process: %d\n", pid)

	cb := syscall.NewCallback(func(hwnd uintptr, lp uintptr) uintptr {
		var wp uint32
		pGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&wp)))
		if wp != uint32(pid) {
			return 1
		}
		title := text(pGetWindowTextW, syscall.Handle(hwnd), 256)
		class := text(pGetClassNameW, syscall.Handle(hwnd), 256)
		var r rect
		pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		vis, _, _ := pIsWindowVisible.Call(hwnd)
		if title == "" && class == "" {
			return 1
		}
		fmt.Printf("hwnd=0x%x [%s] class=%s %dx%d vis=%v\n",
			hwnd, title, class, r.Right-r.Left, r.Bottom-r.Top, vis != 0)
		return 1
	})
	pEnumWindows.Call(cb, 0)
	_ = 0
}
