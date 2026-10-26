//go:build linux

package judge

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// cgroup v2 资源墙(探测降级):memory.max + pids.max + cgroup.kill。
// 相比 RLIMIT_AS 的优势:虚拟地址空间保留不再误判(解释型运行时友好),
// OOM 有确定性判定(memory.events),峰值有 memory.peak 可读。
//
// 前提是本进程的 cgroup 已被 systemd 委任(delegate)且启用 memory+pids
// 控制器——用户会话常见,CI/容器则不一定。任一探测失败即退回
// prlimit-only(零风险降级,绝不把基础设施失败变成选手程序的 RE)。

const cgroupRoot = "/sys/fs/cgroup"

var (
	cgroupProbeOnce sync.Once
	cgroupUsable    bool
	cgroupBase      string // 本进程 cgroup 目录(子组创建点)
	cgroupWhy       string // 不可用原因(观测用)
)

func cgroupFallback(reason string) {
	cgroupWhy = reason
}

// probeCgroup 探测 v2 挂载、控制器委任与子组可写性(一次性)。
func probeCgroup() bool {
	cgroupProbeOnce.Do(func() {
		controllersB, err := os.ReadFile(filepath.Join(cgroupRoot, "cgroup.controllers"))
		if err != nil {
			cgroupFallback("cgroup v2 not mounted")
			return
		}
		has := func(list, want string) bool {
			for _, c := range strings.Fields(list) {
				if c == want {
					return true
				}
			}
			return false
		}
		controllers := string(controllersB)
		if !has(controllers, "memory") || !has(controllers, "pids") {
			cgroupFallback("memory/pids controllers not available")
			return
		}
		self, err := os.ReadFile("/proc/self/cgroup")
		if err != nil {
			cgroupFallback("read /proc/self/cgroup: " + err.Error())
			return
		}
		base := ""
		for _, line := range strings.Split(string(self), "\n") {
			if strings.HasPrefix(line, "0::") {
				base = strings.TrimSpace(strings.TrimPrefix(line, "0::"))
			}
		}
		if base == "" {
			cgroupFallback("no v2 entry in /proc/self/cgroup")
			return
		}
		baseDir := filepath.Join(cgroupRoot, base)
		// 实际创建一个探针子组并写 memory.max——验证委任与控制器启用
		// (子组必须有 memory.max 文件,等价于父的 subtree_control 已启用)。
		probe := filepath.Join(baseDir, fmt.Sprintf("forge-probe-%d", os.Getpid()))
		if err := os.Mkdir(probe, 0o755); err != nil {
			cgroupFallback("mkdir probe: " + err.Error())
			return
		}
		defer os.RemoveAll(probe)
		if err := os.WriteFile(filepath.Join(probe, "memory.max"), []byte("max"), 0o644); err != nil {
			cgroupFallback("write memory.max: " + err.Error())
			return
		}
		cgroupUsable = true
		cgroupBase = baseDir
	})
	return cgroupUsable
}

// CgroupStatus 返回 cgroup 资源墙状态(观测用):
// off:<原因> / available:<路径>。
func CgroupStatus() string {
	if probeCgroup() {
		return "available:" + cgroupBase
	}
	return "off:" + cgroupWhy
}

// cgroupLimiter 单次评测的资源墙(子组)。
type cgroupLimiter struct {
	dir string
}

// tryCgroupWall 探测通过时返回 cgroup 资源墙,否则 nil(prlimit-only)。
// maxProcs<=0(编译器路径)不进 cgroup,维持 prlimit。
func tryCgroupWall(memMiB, maxProcs int) *cgroupLimiter {
	if maxProcs <= 0 || !probeCgroup() {
		return nil
	}
	dir := filepath.Join(cgroupBase, fmt.Sprintf("forge-%d", os.Getpid()))
	if err := os.Mkdir(dir, 0o755); err != nil {
		return nil
	}
	memLimit := "max"
	if memMiB > 0 {
		memLimit = strconv.FormatUint(uint64(memMiB)<<20, 10)
	}
	if err := os.WriteFile(filepath.Join(dir, "memory.max"), []byte(memLimit), 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return nil
	}
	if err := os.WriteFile(filepath.Join(dir, "pids.max"), []byte("64"), 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return nil
	}
	// memory.peak 从挂载起累计:写入触发复位(新内核支持,失败忽略——
	// 峰值缺失时 runner 落回 rusage 回填)。
	_ = os.WriteFile(filepath.Join(dir, "memory.peak"), []byte("0"), 0o644)
	return &cgroupLimiter{dir: dir}
}

// Attach 把已启动的选手进程移入子组(后代进程天然继承,fork 无逃逸)。
func (c *cgroupLimiter) Attach(pid int) error {
	return os.WriteFile(filepath.Join(c.dir, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0o644)
}

// PeakBytes 子树峰值内存(memory.peak;内核过旧时 0,runner 落回 rusage)。
func (c *cgroupLimiter) PeakBytes() uint64 {
	b, err := os.ReadFile(filepath.Join(c.dir, "memory.peak"))
	if err != nil {
		return 0
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// OutOfMemory memory.events 的 oom_kill 计数非零 → 确定性 MLE。
func (c *cgroupLimiter) OutOfMemory() bool {
	b, err := os.ReadFile(filepath.Join(c.dir, "memory.events"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "oom_kill ") {
			n, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "oom_kill")), 10, 64)
			return err == nil && n > 0
		}
	}
	return false
}

// Terminate cgroup.kill=1 整组收割(内核 5.14+;旧内核忽略,组杀兜底)。
func (c *cgroupLimiter) Terminate() {
	_ = os.WriteFile(filepath.Join(c.dir, "cgroup.kill"), []byte("1"), 0o644)
}

// Close 杀组并删除子组(尽力;EBUSY 残留空组无碍)。
func (c *cgroupLimiter) Close() {
	c.Terminate()
	_ = os.RemoveAll(c.dir)
}
