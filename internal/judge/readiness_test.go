package judge

import (
	"context"
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

func TestToolchainConfigRejectsMissingCommand(t *testing.T) {
	cfg := ToolchainConfig{
		CPP: ToolSpec{Command: "forge-definitely-missing-compiler"},
	}
	if err := cfg.CheckCommands(); err == nil {
		t.Fatal("missing compiler must fail readiness")
	}
}

func TestPoolHealthBeforeAndAfterStart(t *testing.T) {
	pool := NewPool(2, func(context.Context, Request) (domain.JudgeOutcome, error) {
		return domain.JudgeOutcome{}, nil
	}, nil)
	if err := pool.Healthy(); err == nil {
		t.Fatal("unstarted pool must not be healthy")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx, 1)
	if err := pool.Healthy(); err != nil {
		t.Fatal(err)
	}
	stats := pool.Stats()
	if stats.Workers != 1 || stats.Capacity != 2 || stats.Queued != 0 {
		t.Fatalf("unexpected pool stats: %+v", stats)
	}
	cancel()
	pool.Wait()
}
