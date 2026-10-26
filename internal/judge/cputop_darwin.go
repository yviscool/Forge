//go:build darwin

package judge

import "syscall"

func physicalCores() int {
	if n, err := syscall.SysctlUint32("hw.physicalcpu"); err == nil && n > 0 {
		return int(n)
	}
	return 0
}
