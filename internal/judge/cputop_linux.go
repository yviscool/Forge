//go:build linux

package judge

import (
	"os"
	"strings"
)

// physicalCores 从 /proc/cpuinfo 数 (physical id, core id) 去重对；
// 缺 physical id 时退数唯一 core id；两者皆缺（部分虚拟化/s390x）返回 0，
// 由调用方回退逻辑核数。
func physicalCores() int {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	pairs := map[string]bool{}
	cores := map[string]bool{}
	phys, core := "", ""
	flush := func() {
		if phys != "" && core != "" {
			pairs[phys+"/"+core] = true
		}
		if core != "" {
			cores[core] = true
		}
		phys, core = "", ""
	}
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			flush()
			continue
		}
		k, v, ok := strings.Cut(ln, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "physical id":
			phys = strings.TrimSpace(v)
		case "core id":
			core = strings.TrimSpace(v)
		}
	}
	flush()
	if len(pairs) > 0 {
		return len(pairs)
	}
	return len(cores)
}
