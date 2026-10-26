package config

import (
	"runtime"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("FORGE_ADDR", "")
	t.Setenv("FORGE_ENV", "")
	t.Setenv("FORGE_DATA_DIR", "")
	t.Setenv("FORGE_STORE", "")
	t.Setenv("FORGE_JUDGE_WORKERS", "")
	t.Setenv("FORGE_TRUST_PROXY", "")
	t.Setenv("FORGE_AUTOJUDGE", "")
	t.Setenv("FORGE_ALLOW_UNSAFE_LOCAL_JUDGE", "")
	t.Setenv("FORGE_ADMIN_PASSWORD", "")
	t.Setenv("FORGE_MIN_FREE_DISK_MIB", "")
	c := Load()
	if c.Addr != "127.0.0.1:8080" || c.DataDir != "./data" || c.Store != "sqlite" {
		t.Fatalf("defaults: %+v", c)
	}
	if c.Environment != "development" || c.MinFreeDiskMiB != 512 {
		t.Fatalf("environment/disk defaults: %+v", c)
	}
	// 缺省物理核数（SMT 上小于逻辑核数）；区间断言不绑定具体机器。
	if c.JudgeWorkers < 1 || c.JudgeWorkers > runtime.NumCPU() {
		t.Fatalf("workers default out of range: %+v", c)
	}
	if c.TrustProxy {
		t.Fatalf("proxy headers must be untrusted by default: %+v", c)
	}
	if c.AutoJudge {
		t.Fatalf("automatic judging must be opt-in: %+v", c)
	}
	if c.AllowUnsafeLocalJudge {
		t.Fatalf("host code execution must be opt-in: %+v", c)
	}
	if !c.AdminPasswordGenerated || len(c.AdminPassword) != 32 {
		t.Fatalf("missing admin password must be a generated 32-character value: %+v", c)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("FORGE_STORE", "memory")
	t.Setenv("FORGE_JUDGE_WORKERS", "3")
	t.Setenv("FORGE_AUTOJUDGE", "1")
	t.Setenv("FORGE_ALLOW_UNSAFE_LOCAL_JUDGE", "true")
	t.Setenv("FORGE_TRUST_PROXY", "true")
	c := Load()
	if c.Store != "memory" || c.JudgeWorkers != 3 || !c.AutoJudge ||
		!c.AllowUnsafeLocalJudge || !c.TrustProxy {
		t.Fatalf("overrides: %+v", c)
	}
	t.Setenv("FORGE_JUDGE_WORKERS", "0")
	if err := Load().Validate(); err == nil {
		t.Fatal("zero workers must fail closed")
	}
	t.Setenv("FORGE_JUDGE_WORKERS", "abc")
	if err := Load().Validate(); err == nil {
		t.Fatal("non-numeric workers must fail closed")
	}
}

func TestValidateRejectsUnknownStore(t *testing.T) {
	t.Setenv("FORGE_STORE", "sqlit")
	c := Load()
	if err := c.Validate(); err == nil {
		t.Fatalf("unknown store must fail closed: %+v", c)
	}
}

func TestValidateRejectsMalformedBooleansAndUnsafeAutoJudge(t *testing.T) {
	t.Setenv("FORGE_TRUST_PROXY", "sometimes")
	if err := Load().Validate(); err == nil {
		t.Fatal("malformed boolean must fail closed")
	}

	t.Setenv("FORGE_TRUST_PROXY", "")
	t.Setenv("FORGE_AUTOJUDGE", "1")
	t.Setenv("FORGE_ALLOW_UNSAFE_LOCAL_JUDGE", "0")
	if err := Load().Validate(); err == nil {
		t.Fatal("auto-judge without explicit unsafe opt-in must fail closed")
	}
}

func TestProductionRejectsEphemeralStoreAndUnsafeJudge(t *testing.T) {
	t.Setenv("FORGE_ENV", "production")
	t.Setenv("FORGE_STORE", "memory")
	if err := Load().Validate(); err == nil {
		t.Fatal("production must reject the in-memory store")
	}

	t.Setenv("FORGE_STORE", "sqlite")
	t.Setenv("FORGE_AUTOJUDGE", "1")
	t.Setenv("FORGE_ALLOW_UNSAFE_LOCAL_JUDGE", "1")
	if err := Load().Validate(); err == nil {
		t.Fatal("production must reject the unisolated local judge")
	}
}

func TestValidateRejectsUnknownEnvironmentAndNegativeDiskFloor(t *testing.T) {
	t.Setenv("FORGE_ENV", "live-ish")
	if err := Load().Validate(); err == nil {
		t.Fatal("unknown environment must fail closed")
	}

	t.Setenv("FORGE_ENV", "development")
	t.Setenv("FORGE_MIN_FREE_DISK_MIB", "-1")
	if err := Load().Validate(); err == nil {
		t.Fatal("negative disk floor must fail closed")
	}
}
