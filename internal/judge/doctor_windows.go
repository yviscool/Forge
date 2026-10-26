//go:build windows

package judge

import "time"

// resourceWallRows Windows 的资源墙/沙箱诊断行。
// Job Object 墙恒可用;LowBox 沙箱状态来自启动探测。
func resourceWallRows() []DoctorRow {
	ensureWinSandboxProbed(30 * time.Second)
	rows := []DoctorRow{
		{Check: "wall.job", Status: "ok", Detail: "memory/process walls + kill-on-close"},
		{Check: "sandbox.lowbox", Status: statusWord(WinSandboxStatus()), Detail: WinSandboxStatus()},
	}
	return rows
}

// waitForProbes Windows:等待 LowBox 探测。
func waitForProbes(d time.Duration) { ensureWinSandboxProbed(d) }

// wallStatuses Windows 的资源墙状态集。
func wallStatuses() map[string]string {
	return map[string]string{"lowbox": WinSandboxStatus(), "job": "always"}
}
