//go:build !linux && !windows

package judge

import "time"

// resourceWallRows 其他 Unix:ulimit 降级墙(best-effort)。
func resourceWallRows() []DoctorRow {
	return []DoctorRow{
		{Check: "wall.ulimit", Status: "ok", Detail: "sh ulimit fallback walls (best-effort), peak via wait4 rusage"},
	}
}

// waitForProbes 其他平台无异步探针。
func waitForProbes(time.Duration) {}

// wallStatuses 其他平台的资源墙状态集。
func wallStatuses() map[string]string {
	return map[string]string{"ulimit": "always"}
}
