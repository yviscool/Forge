//go:build !windows

package judge

// 非 Windows 平台防休眠暂不实现：Linux 需 DBus logind、macOS 需 IOKit，
// 引绑定依赖过重（upstream 三平台都做了，Forge 先覆盖主力部署面
// Windows 教师机）。占位类型保持 pool 代码三平台统一。
type sleepInhibitor struct{}

func newSleepInhibitor() *sleepInhibitor { return &sleepInhibitor{} }

func (s *sleepInhibitor) acquire() {}

func (s *sleepInhibitor) release() {}
