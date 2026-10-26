//go:build windows

package judge

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// physicalCores 用 GetLogicalProcessorInformation 数 RelationProcessorCore
// 条目（x/sys/windows 未导出该 API，经 kernel32 lazy DLL 调用）。
// 结构布局镜像 win32 SYSTEM_LOGICAL_PROCESSOR_INFORMATION，由 unsafe.Sizeof
// 按平台自动对齐（win64 = 32B）。
func physicalCores() int {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetLogicalProcessorInformation")
	if err := proc.Find(); err != nil {
		return 0
	}
	entrySize := int(unsafe.Sizeof(slpiEntry{}))
	var needed uint32
	// 首次以空缓冲调用取所需字节数（必然失败并回填 needed）。
	_, _, _ = proc.Call(0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 || needed > 1<<20 {
		return 0
	}
	buf := make([]byte, needed)
	r1, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&needed)))
	if r1 == 0 {
		return 0
	}
	cores := 0
	for off := 0; off+entrySize <= int(needed); off += entrySize {
		if (*slpiEntry)(unsafe.Pointer(&buf[off])).Relationship == 0 { // RelationProcessorCore
			cores++
		}
	}
	return cores
}

type slpiEntry struct {
	ProcessorMask uintptr
	Relationship  uint32
	_             uint32
	// union{ ProcessorCore; NumaNode; Group; Cache; Reserved[2] } 至多 16B。
	union [16]byte
}
