package judge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ToolSpec 单语言工具链配置（对标 LemonLime Compiler 名称/路径/参数）。
type ToolSpec struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	// Std is used for cpp/c: e.g. c++14. Empty keeps toolchain default.
	Std string `json:"std"`
	// CompileTimeoutSec 单次编译上限（秒），0 则用 Toolchain 缺省 30s。
	CompileTimeoutSec int `json:"compileTimeoutSec"`
	// CompileMemoryMiB 编译进程内存墙（MB），0 则不限。
	CompileMemoryMiB int `json:"compileMemoryMiB"`
	// TimeLimitRatio 运行时限倍率（对标 LemonLime Compiler
	// timeLimitRatio；解释型语言如 Python 建议 2~3），0 表示未设置（=1）。
	TimeLimitRatio float64 `json:"timeLimitRatio"`
	// MemoryLimitRatio 内存限制倍率（对标 memoryLimitRatio），0 = 1。
	MemoryLimitRatio float64 `json:"memoryLimitRatio"`
	// DisableMemoryLimitCheck 指针语义区分未设置与 false
	// （对标 disableMemoryLimitCheck：某些运行时自带内存管理，硬墙会误杀）。
	DisableMemoryLimitCheck *bool `json:"disableMemoryLimitCheck,omitempty"`
}

// ToolchainConfig 全语言工具链配置（JSON 文件，FORGE_TOOLCHAINS 指向）。
type ToolchainConfig struct {
	CPP    ToolSpec `json:"cpp"`
	C      ToolSpec `json:"c"`
	Go     ToolSpec `json:"go"`
	Python ToolSpec `json:"python"`
}

// DefaultToolchainConfig 开箱默认值：CCF 复赛口径（-O2 -std=c++14）。
func DefaultToolchainConfig() ToolchainConfig {
	return ToolchainConfig{
		CPP:    ToolSpec{Command: "g++", Std: "c++14"},
		C:      ToolSpec{Command: "gcc"},
		Go:     ToolSpec{Command: "go"},
		Python: ToolSpec{Command: "python"},
	}
}

// ParseToolchainConfig loads and strictly validates a toolchain JSON file.
// Configuration errors must be reported to the operator instead of silently
// falling back to a different compiler than the one they requested.
func ParseToolchainConfig(path string) (ToolchainConfig, error) {
	def := DefaultToolchainConfig()
	if path == "" {
		return def, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return def, fmt.Errorf("read FORGE_TOOLCHAINS: %w", err)
	}
	var c ToolchainConfig
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return def, fmt.Errorf("parse FORGE_TOOLCHAINS: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return def, errors.New("parse FORGE_TOOLCHAINS: trailing JSON value")
	}
	specs := map[string]ToolSpec{
		"cpp": c.CPP, "c": c.C, "go": c.Go, "python": c.Python,
	}
	for name, spec := range specs {
		if spec.CompileTimeoutSec < 0 {
			return def, fmt.Errorf("toolchain %s compileTimeoutSec cannot be negative", name)
		}
		if spec.CompileMemoryMiB < 0 {
			return def, fmt.Errorf("toolchain %s compileMemoryMib cannot be negative", name)
		}
		if spec.TimeLimitRatio < 0 || spec.TimeLimitRatio > 100 {
			return def, fmt.Errorf("toolchain %s timeLimitRatio out of range (0,100]", name)
		}
		if spec.MemoryLimitRatio < 0 || spec.MemoryLimitRatio > 100 {
			return def, fmt.Errorf("toolchain %s memoryLimitRatio out of range (0,100]", name)
		}
	}
	merge := func(d, o ToolSpec) ToolSpec {
		if o.Command != "" {
			d.Command = o.Command
		}
		if o.Args != nil {
			d.Args = o.Args
		}
		if o.Std != "" {
			d.Std = o.Std
		}
		if o.CompileTimeoutSec > 0 {
			d.CompileTimeoutSec = o.CompileTimeoutSec
		}
		if o.CompileMemoryMiB > 0 {
			d.CompileMemoryMiB = o.CompileMemoryMiB
		}
		if o.TimeLimitRatio > 0 {
			d.TimeLimitRatio = o.TimeLimitRatio
		}
		if o.MemoryLimitRatio > 0 {
			d.MemoryLimitRatio = o.MemoryLimitRatio
		}
		if o.DisableMemoryLimitCheck != nil {
			d.DisableMemoryLimitCheck = o.DisableMemoryLimitCheck
		}
		return d
	}
	def.CPP = merge(def.CPP, c.CPP)
	def.C = merge(def.C, c.C)
	def.Go = merge(def.Go, c.Go)
	def.Python = merge(def.Python, c.Python)
	return def, nil
}

// LoadToolchainConfig retains the forgiving helper for fixtures. Production
// startup must call ParseToolchainConfig and surface its error.
func LoadToolchainConfig(path string) ToolchainConfig {
	cfg, err := ParseToolchainConfig(path)
	if err != nil {
		return DefaultToolchainConfig()
	}
	return cfg
}

// CheckCommands verifies that every configured compiler/interpreter is
// resolvable before readiness advertises automatic judging.
func (c ToolchainConfig) CheckCommands() error {
	seen := make(map[string]bool)
	for _, spec := range []ToolSpec{c.CPP, c.C, c.Go, c.Python} {
		command := strings.TrimSpace(spec.Command)
		if command == "" || seen[command] {
			continue
		}
		seen[command] = true
		if _, err := exec.LookPath(command); err != nil {
			return fmt.Errorf("judge toolchain %q is unavailable: %w", command, err)
		}
	}
	return nil
}
