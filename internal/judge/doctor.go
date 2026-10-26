package judge

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"
)

// DoctorRow 一行诊断:检查项 / 状态 / 细节。
type DoctorRow struct {
	Check  string
	Status string
	Detail string
}

// DoctorChecks 运行评测基石探针矩阵(工具链/资源墙/沙箱/编译缓存)。
// 纯读+临时写,不改任何持久状态;readiness 与 `forge doctor` 共用。
func DoctorChecks(tc ToolchainConfig) []DoctorRow {
	rows := make([]DoctorRow, 0, 10)

	// 工具链逐语言解析。
	specs := []struct {
		lang string
		spec ToolSpec
	}{
		{"cpp", tc.CPP}, {"c", tc.C}, {"go", tc.Go}, {"python", tc.Python},
	}
	for _, sp := range specs {
		if sp.spec.Command == "" {
			rows = append(rows, DoctorRow{"toolchain." + sp.lang, "skip", "no command configured"})
			continue
		}
		resolved, err := exec.LookPath(sp.spec.Command)
		if err != nil {
			rows = append(rows, DoctorRow{"toolchain." + sp.lang, "missing", err.Error()})
			continue
		}
		rows = append(rows, DoctorRow{"toolchain." + sp.lang, "ok", resolved})
	}

	// 资源墙与沙箱(按平台)。
	rows = append(rows, resourceWallRows()...)

	// 编译缓存可写性。
	cacheDir := os.TempDir() + "/forge-ccache"
	if cc, err := NewCompileCache(cacheDir); err != nil {
		rows = append(rows, DoctorRow{"compile.cache", "fail", err.Error()})
	} else {
		_ = cc
		rows = append(rows, DoctorRow{"compile.cache", "ok", cacheDir})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Check < rows[j].Check })
	return rows
}

// WaitForProbes 等待所有异步探针出结论(最多 d)。
func WaitForProbes(d time.Duration) {
	waitForProbes(d)
}

// WallStatuses 各平台资源墙/沙箱探测状态(/metrics info gauge 用):
// windows: lowbox/job;linux: bwrap/cgroup;其他: ulimit。
func WallStatuses() map[string]string {
	return wallStatuses()
}

// statusWord 把状态串归一为 ok/off/warn 展示词。
func statusWord(status string) string {
	switch {
	case status == "available" || status == "appcontainer":
		return "ok"
	case len(status) >= 4 && status[:4] == "off:":
		return "off"
	default:
		return "warn"
	}
}

// FormatDoctorRows 输出对齐的纯文本报表。
func FormatDoctorRows(rows []DoctorRow) string {
	w := 0
	for _, r := range rows {
		if len(r.Check) > w {
			w = len(r.Check)
		}
	}
	out := ""
	for _, r := range rows {
		out += fmt.Sprintf("%-*s  %-10s %s\n", w, r.Check, r.Status, r.Detail)
	}
	return out
}
