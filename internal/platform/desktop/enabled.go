//go:build desktop

package desktop

// DefaultDesktopMode 桌面构建（-tags desktop）双击启动即进入桌面模式，
// 与 magpie 的应用体验一致；--server 可强制回 headless。
const DefaultDesktopMode = true

// wailsShell Wails 原生外壳仅在 -tags "desktop wails" 下编译。
const wailsShell = true

// WailsShellEnabled 报告当前构建是否为 Wails 原生壳
// （原生壳的主窗即教师台，不再自动开外部浏览器）。
func WailsShellEnabled() bool { return wailsShell }
