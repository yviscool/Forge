//go:build !windows

package judge

import "syscall"

// 非 Windows 平台无 LowBox 沙箱:Linux 走 bwrap+prlimit(见 job_other.go),
// 其他平台 ulimit 降级墙。此处仅提供 runner 挂钩的 no-op。

// ConfigureWinSandbox 非 Windows no-op(配置合法性由 config 层保证)。
func ConfigureWinSandbox(string) error { return nil }

// WinSandboxStatus 非 Windows 恒为 off。
func WinSandboxStatus() string { return "off" }

// sandboxSetup 非 Windows 无沙箱属性(Job/bwrap 墙照常)。
func sandboxSetup(string) (*syscall.SysProcAttr, []string, func(), error) {
	return nil, nil, func() {}, nil
}

// runtimeDirAclEnabled 非 Windows 无运行时目录 ACL。
func runtimeDirAclEnabled() bool { return false }

// prepareRuntimeDir 非 Windows no-op。
func prepareRuntimeDir(string, string) {}

// RevokeRuntimeAcls 非 Windows no-op(无运行时目录 ACL)。
func RevokeRuntimeAcls() {}
