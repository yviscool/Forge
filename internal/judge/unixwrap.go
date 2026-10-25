package judge

import (
	"fmt"
	"strings"
)

// Unix 资源墙/沙箱的 argv 构造器（纯函数，三平台可单测）。
// 对标 LemonLime processrunner_unix：Linux 用 bwrap 命名空间 + prlimit，
// macOS 等无 prlimit 平台降级为 /bin/sh 内建 ulimit。

// ShUlimitArgv 用 /bin/sh 内建 ulimit 包一层（macOS 降级路径）：
// ulimit -v（地址空间 KB）+ ulimit -t（CPU 整秒）。
// ulimit 失败不阻塞执行（best-effort 墙），硬杀仍由外层 ctx 超时兜底。
// 形如 sh -c 'ulimit …; exec "$0" "$@"' cmd arg1 arg2。
func ShUlimitArgv(argv []string, lim Limits) []string {
	if len(argv) == 0 {
		return argv
	}
	var b strings.Builder
	b.WriteString("ulimit -n 256 2>/dev/null; ")
	if lim.MemoryMiB > 0 {
		fmt.Fprintf(&b, "ulimit -v %d 2>/dev/null; ", int64(lim.MemoryMiB)<<10)
	}
	if lim.TimeMs > 0 {
		fmt.Fprintf(&b, "ulimit -t %d 2>/dev/null; ", int64(lim.TimeMs)/1000+2)
	}
	b.WriteString(`exec "$0" "$@"`)
	out := []string{"/bin/sh", "-c", b.String()}
	return append(out, argv...)
}

// BwrapArgv 用 bubblewrap 包一层命名空间沙箱（Linux 强化路径，对标
// LemonLime processrunner_unix 的 bwrap 参数）：--unshare-all 断网/断 IPC、
// /usr 只读、/tmp 私有 tmpfs、工作目录可写。返回 bwrap + "--" + inner。
func BwrapArgv(inner []string, workdir string) []string {
	if len(inner) == 0 {
		return inner
	}
	args := []string{
		"--dev", "/dev",
		"--proc", "/proc",
		"--ro-bind", "/usr", "/usr",
		"--symlink", "/usr/lib", "/lib",
		"--symlink", "/usr/lib64", "/lib64",
		"--symlink", "/usr/bin", "/bin",
		"--symlink", "/usr/sbin", "/sbin",
		"--tmpfs", "/tmp",
		"--unshare-all",
		"--die-with-parent",
	}
	if workdir != "" {
		args = append(args, "--chdir", workdir, "--bind", workdir, workdir)
	}
	args = append(args, "--")
	return append(args, inner...)
}

// PrlimitArgv 用 util-linux prlimit 包一层（Linux 首选资源墙）：
// RLIMIT_AS（地址空间）+ RLIMIT_CPU（秒，+2 宽限）+ RLIMIT_NPROC（防 fork 炸弹）。
func PrlimitArgv(argv []string, lim Limits, prlimit string) []string {
	if prlimit == "" || len(argv) == 0 {
		return argv
	}
	wrapped := []string{prlimit}
	if lim.MemoryMiB > 0 {
		wrapped = append(wrapped, fmt.Sprintf("--as=%d", int64(lim.MemoryMiB)<<20))
	}
	if lim.TimeMs > 0 {
		wrapped = append(wrapped, fmt.Sprintf("--cpu=%d", lim.TimeMs/1000+2))
	}
	wrapped = append(wrapped, "--nproc=64", "--")
	return append(wrapped, argv...)
}
