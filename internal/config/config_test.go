package config

import (
	"runtime"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("FORGE_ADDR", "")
	t.Setenv("FORGE_DATA_DIR", "")
	t.Setenv("FORGE_STORE", "")
	t.Setenv("FORGE_JUDGE_WORKERS", "")
	c := Load()
	if c.Addr != ":8080" || c.DataDir != "./data" || c.Store != "sqlite" {
		t.Fatalf("defaults: %+v", c)
	}
	if c.JudgeWorkers != runtime.NumCPU() {
		t.Fatalf("workers default: %+v", c)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("FORGE_STORE", "memory")
	t.Setenv("FORGE_JUDGE_WORKERS", "3")
	t.Setenv("FORGE_AUTOJUDGE", "0")
	c := Load()
	if c.Store != "memory" || c.JudgeWorkers != 3 || c.AutoJudge {
		t.Fatalf("overrides: %+v", c)
	}
	t.Setenv("FORGE_JUDGE_WORKERS", "0")
	if c := Load(); c.JudgeWorkers != runtime.NumCPU() {
		t.Fatal("invalid workers must fall back")
	}
	t.Setenv("FORGE_JUDGE_WORKERS", "abc")
	if c := Load(); c.JudgeWorkers != runtime.NumCPU() {
		t.Fatal("non-numeric workers must fall back")
	}
}
