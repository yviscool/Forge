package config

import (
	"os"
	"strings"
)

// Config 是进程级配置基石：全部来自环境变量，开箱即用。
type Config struct {
	Addr    string
	DataDir string
	Locale  string
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
	return Config{Addr: addr, DataDir: dataDir, Locale: strings.TrimSpace(locale)}
}
