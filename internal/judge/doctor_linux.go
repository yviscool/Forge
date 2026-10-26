//go:build linux

package judge

import "time"

// resourceWallRows Linux 的资源墙/沙箱诊断行。
// 探测(bwrap 计量穿透、cgroup 委任)同步等待出结论。
func resourceWallRows() []DoctorRow {
	waitForBwrapProbe(20 * time.Second)
	_ = probeCgroup()

	rows := []DoctorRow{}
	if prlimit, _ := toolPaths(); prlimit != "" {
		rows = append(rows, DoctorRow{"wall.prlimit", "ok", prlimit})
	} else {
		rows = append(rows, DoctorRow{"wall.prlimit", "missing", "RLIMIT walls unavailable; readiness will refuse autojudge"})
	}
	rows = append(rows, DoctorRow{"sandbox.bwrap", statusWord(BwrapStatus()), BwrapStatus()})
	rows = append(rows, DoctorRow{"wall.cgroup", statusWord(CgroupStatus()), CgroupStatus()})
	return rows
}

// waitForProbes Linux:等待 bwrap 探测。
func waitForProbes(d time.Duration) { waitForBwrapProbe(d) }
