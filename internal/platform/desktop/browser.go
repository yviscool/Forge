package desktop

import (
	"errors"
	"os/exec"
	"runtime"
)

// BrowserOpener 打开系统默认浏览器的可注入入口（测试用假实现替代）。
type BrowserOpener interface {
	Open(url string) error
}

// SystemBrowser 使用系统默认浏览器。命令构造与 upstream/magpie 的
// internal/proc.OpenBrowser 保持一致：Windows 用 rundll32 全路径，
// macOS 用 open，Linux 用 xdg-open。
type SystemBrowser struct {
	// LookPath 可注入（测试断言命令而不真正拉起进程）。
	LookPath func(file string) (string, error)
	// Start 执行命令（测试注入记录器）。
	Start func(cmd *exec.Cmd) error
}

func (s SystemBrowser) browserCommand(target string) (*exec.Cmd, error) {
	if target == "" {
		return nil, errors.New("empty url")
	}
	look := s.LookPath
	if look == nil {
		look = exec.LookPath
	}
	switch runtime.GOOS {
	case "windows":
		// rundll32 位于 System32；用全路径避免 PATH 劫持。
		if _, err := look("rundll32.exe"); err != nil {
			return nil, err
		}
		return exec.Command(`C:\Windows\System32\rundll32.exe`, "url.dll,FileProtocolHandler", target), nil
	case "darwin":
		if _, err := look("open"); err != nil {
			return nil, err
		}
		return exec.Command("open", target), nil
	default:
		if _, err := look("xdg-open"); err != nil {
			return nil, err
		}
		return exec.Command("xdg-open", target), nil
	}
}

// Open 在系统默认浏览器中打开 target。
func (s SystemBrowser) Open(target string) error {
	cmd, err := s.browserCommand(target)
	if err != nil {
		return err
	}
	start := s.Start
	if start == nil {
		start = func(c *exec.Cmd) error { return c.Start() }
	}
	return start(cmd)
}

// Supported 报告当前平台是否具备打开浏览器的先决条件（不做真实调用）。
func (s SystemBrowser) Supported() bool {
	cmd, err := s.browserCommand("http://forge.invalid/")
	return err == nil && cmd != nil
}
