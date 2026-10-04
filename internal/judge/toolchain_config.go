package judge

import (
	"encoding/json"
	"os"
)

// ToolSpec 单语言工具链配置（对标 LemonLime Compiler 名称/路径/参数）。
type ToolSpec struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	// Std is used for cpp/c: e.g. c++17. Empty keeps toolchain default.
	Std string `json:"std"`
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

// LoadToolchainConfig 从 JSON 文件加载；path 为空或读失败返回默认。
func LoadToolchainConfig(path string) ToolchainConfig {
	def := DefaultToolchainConfig()
	if path == "" {
		return def
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	var c ToolchainConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return def
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
		return d
	}
	def.CPP = merge(def.CPP, c.CPP)
	def.C = merge(def.C, c.C)
	def.Go = merge(def.Go, c.Go)
	def.Python = merge(def.Python, c.Python)
	return def
}
