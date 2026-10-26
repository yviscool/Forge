package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yviscool/forge/internal/judge"
)

// Config 是进程级配置基石：全部来自环境变量，开箱即用。
type Config struct {
	Addr        string
	Environment string
	DataDir     string
	Locale      string
	// Store 持久化后端：sqlite（默认，文件）或 memory（纯内存，测试/临时）。
	Store  string
	DBPath string
	// AutoJudge 是否启用本地自动评测 worker（默认关；需显式开启）。
	AutoJudge bool
	// AllowUnsafeLocalJudge acknowledges that the local runner has no
	// filesystem/network sandbox. Both it and AutoJudge must be enabled before
	// untrusted submissions are executed on the host.
	AllowUnsafeLocalJudge bool
	// WinSandbox Windows 评测沙箱模式(FORGE_WIN_SANDBOX;auto 缺省):
	// auto = LowBox 令牌 + 工作目录 ACL(探测失败自动退回 Job-only),
	// off = 仅 Job 资源墙。
	WinSandbox string
	// WinSandboxKeepAcl 停机时是否保留运行时目录的包 SID 授权
	// (FORGE_WIN_SANDBOX_KEEP_ACL;缺省 false = 优雅停机时回滚,
	// 授权不越过服务生命周期)。
	WinSandboxKeepAcl bool
	// AdminPassword 种子管理员密码（FORGE_ADMIN_PASSWORD；缺省随机生成）。
	AdminPassword string
	// AdminPasswordGenerated indicates a one-time generated bootstrap password.
	AdminPasswordGenerated bool
	// Toolchains 工具链配置文件路径（FORGE_TOOLCHAINS；为空用内置默认）。
	Toolchains string
	// JudgeWorkers 评测并发数（FORGE_JUDGE_WORKERS；缺省 CPU 数，最小 1）。
	JudgeWorkers int
	// MinFreeDiskMiB is the readiness floor for the data volume.
	MinFreeDiskMiB int64
	// TrustProxy 是否信任 X-Forwarded-For / X-Real-IP。
	// 默认关闭：机位绑定依赖真实连接地址，若服务不在可信反向代理之后，
	// 允许客户端自报 IP 会让考生直接伪造机位绕过考务绑定。
	TrustProxy bool

	loadErr error
}

func Load() Config {
	var parseErrs []error
	addr := os.Getenv("FORGE_ADDR")
	if addr == "" {
		// Safe-by-default: a single binary should not expose an unfinished
		// deployment to the LAN merely because an environment file is missing.
		addr = "127.0.0.1:8080"
	}
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("FORGE_ENV")))
	if environment == "" {
		environment = "development"
	}
	dataDir := os.Getenv("FORGE_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	dataDir = strings.TrimSpace(dataDir)
	locale := os.Getenv("FORGE_LOCALE")
	if locale == "" {
		locale = "zh-CN"
	}
	store := strings.ToLower(strings.TrimSpace(os.Getenv("FORGE_STORE")))
	switch store {
	case "", "sqlite":
		store = "sqlite"
	case "memory":
	default:
		// Keep the invalid value so main can fail closed instead of silently
		// selecting SQLite after an operator typo.
	}
	auto, err := envBool("FORGE_AUTOJUDGE")
	if err != nil {
		parseErrs = append(parseErrs, err)
	}
	allowUnsafeJudge, err := envBool("FORGE_ALLOW_UNSAFE_LOCAL_JUDGE")
	if err != nil {
		parseErrs = append(parseErrs, err)
	}
	adminPw := os.Getenv("FORGE_ADMIN_PASSWORD")
	adminGenerated := false
	if adminPw == "" {
		adminPw = randomPassword()
		adminGenerated = true
	}
	// 缺省取物理核数（SMT 超订阅会污染 CPU-time 计时，对标 LemonLime
	// cputopology）；探测失败回退逻辑核数。显式配置优先。
	workers := judge.DefaultWorkers()
	if raw := strings.TrimSpace(os.Getenv("FORGE_JUDGE_WORKERS")); raw != "" {
		n, parseErr := strconv.Atoi(raw)
		if parseErr != nil || n < 1 {
			parseErrs = append(parseErrs, fmt.Errorf("FORGE_JUDGE_WORKERS must be a positive integer, got %q", raw))
		} else {
			workers = n
		}
	}
	trustProxy, err := envBool("FORGE_TRUST_PROXY")
	if err != nil {
		parseErrs = append(parseErrs, err)
	}
	winSandbox := strings.TrimSpace(os.Getenv("FORGE_WIN_SANDBOX"))
	if winSandbox == "" {
		winSandbox = "auto"
	}
	keepAcl, err := envBool("FORGE_WIN_SANDBOX_KEEP_ACL")
	if err != nil {
		parseErrs = append(parseErrs, err)
	}
	minFreeDiskMiB := int64(512)
	if raw := strings.TrimSpace(os.Getenv("FORGE_MIN_FREE_DISK_MIB")); raw != "" {
		n, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || n < 0 {
			parseErrs = append(parseErrs, fmt.Errorf(
				"FORGE_MIN_FREE_DISK_MIB must be a non-negative integer, got %q",
				raw,
			))
		} else {
			minFreeDiskMiB = n
		}
	}
	return Config{
		Addr: addr, Environment: environment, DataDir: dataDir, Locale: strings.TrimSpace(locale),
		Store: store, DBPath: filepath.Join(dataDir, "forge.db"), AutoJudge: auto,
		AllowUnsafeLocalJudge:  allowUnsafeJudge,
		AdminPassword:          adminPw,
		AdminPasswordGenerated: adminGenerated,
		Toolchains:             os.Getenv("FORGE_TOOLCHAINS"),
		JudgeWorkers:           workers,
		MinFreeDiskMiB:         minFreeDiskMiB,
		WinSandbox:             winSandbox,
		WinSandboxKeepAcl:      keepAcl,
		TrustProxy:             trustProxy,
		loadErr:                errors.Join(parseErrs...),
	}
}

// Validate rejects fail-open configuration values before a process starts.
func (c Config) Validate() error {
	if c.loadErr != nil {
		return fmt.Errorf("invalid environment: %w", c.loadErr)
	}
	switch c.Store {
	case "sqlite", "memory":
	default:
		return fmt.Errorf("invalid FORGE_STORE %q (want sqlite or memory)", c.Store)
	}
	switch c.Environment {
	case "development", "staging", "production":
	default:
		return fmt.Errorf(
			"invalid FORGE_ENV %q (want development, staging, or production)",
			c.Environment,
		)
	}
	if c.Environment == "production" && c.Store != "sqlite" {
		return fmt.Errorf("production requires durable FORGE_STORE=sqlite")
	}
	if c.Environment == "production" && c.AutoJudge {
		return fmt.Errorf(
			"production automatic judging is disabled until an isolated sandbox worker is configured",
		)
	}
	if c.JudgeWorkers < 1 {
		return fmt.Errorf("FORGE_JUDGE_WORKERS must be at least 1")
	}
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("FORGE_ADDR cannot be empty")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("FORGE_DATA_DIR cannot be empty")
	}
	if c.AutoJudge && !c.AllowUnsafeLocalJudge {
		return fmt.Errorf("FORGE_AUTOJUDGE requires FORGE_ALLOW_UNSAFE_LOCAL_JUDGE=1")
	}
	switch c.WinSandbox {
	case "", "auto", "off":
	default:
		return fmt.Errorf("invalid FORGE_WIN_SANDBOX %q (want auto or off)", c.WinSandbox)
	}
	if c.MinFreeDiskMiB < 0 {
		return fmt.Errorf("FORGE_MIN_FREE_DISK_MIB cannot be negative")
	}
	return nil
}

func envBool(key string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "", "0", "false", "no", "off":
		return false, nil
	case "1", "true", "yes", "on":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be one of true/false, 1/0, yes/no, or on/off", key)
	}
}

func randomPassword() string {
	// Never ship a public default admin password. A random 32-character
	// bootstrap value is printed once by main and can be rotated immediately.
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic("config: unable to generate administrator password")
	}
	return base64.RawURLEncoding.EncodeToString(raw[:])
}
