package config

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Config 是进程级配置基石：全部来自环境变量，开箱即用。
type Config struct {
	Addr    string
	DataDir string
	Locale  string
	// Store 持久化后端：sqlite（默认，文件）或 memory（纯内存，测试/临时）。
	Store  string
	DBPath string
	// AutoJudge 是否启用本地自动评测 worker（默认开；FORGE_AUTOJUDGE=0 关）。
	AutoJudge bool
	// AdminPassword 种子管理员密码（FORGE_ADMIN_PASSWORD；缺省 demo 值，首启即改）。
	AdminPassword string
	// Toolchains 工具链配置文件路径（FORGE_TOOLCHAINS；为空用内置默认）。
	Toolchains string
	// JudgeWorkers 评测并发数（FORGE_JUDGE_WORKERS；缺省 CPU 数，最小 1）。
	JudgeWorkers int
}

func Load() Config {
	addr := os.Getenv("FORGE_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dataDir := os.Getenv("FORGE_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	locale := os.Getenv("FORGE_LOCALE")
	if locale == "" {
		locale = "zh-CN"
	}
	store := strings.ToLower(strings.TrimSpace(os.Getenv("FORGE_STORE")))
	if store != "memory" {
		store = "sqlite"
	}
	auto := true
	if strings.TrimSpace(os.Getenv("FORGE_AUTOJUDGE")) == "0" {
		auto = false
	}
	adminPw := os.Getenv("FORGE_ADMIN_PASSWORD")
	if adminPw == "" {
		adminPw = "admin123"
	}
	workers := runtime.NumCPU()
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("FORGE_JUDGE_WORKERS"))); err == nil && n >= 1 {
		workers = n
	}
	return Config{
		Addr: addr, DataDir: dataDir, Locale: strings.TrimSpace(locale),
		Store: store, DBPath: dataDir + "/forge.db", AutoJudge: auto,
		AdminPassword: adminPw, Toolchains: os.Getenv("FORGE_TOOLCHAINS"),
		JudgeWorkers: workers,
	}
}
