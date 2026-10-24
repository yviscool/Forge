//go:build !desktop

package desktop

// DefaultDesktopMode 服务构建默认 headless；--desktop 显式开启桌面体验。
const DefaultDesktopMode = false

const wailsShell = false

// WailsShellEnabled 非 Wails 构建恒为 false（托盘菜单 + 浏览器外壳）。
func WailsShellEnabled() bool { return wailsShell }
