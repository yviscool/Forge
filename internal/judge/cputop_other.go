//go:build !linux && !darwin && !windows

package judge

// physicalCores 其他平台无轻量探测路径，返回 0 由调用方回退逻辑核数。
func physicalCores() int { return 0 }
