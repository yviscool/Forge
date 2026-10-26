//go:build !linux && !windows

package judge

// 非 Linux 的 Unix(macOS 等)无 cgroup v2 资源墙:ulimit 降级墙承担。
func tryCgroupWall(int, int) unixLimiter { return nil }
