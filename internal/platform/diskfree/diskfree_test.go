package diskfree

import (
	"context"
	"testing"
)

func TestAvailableBytesAndThreshold(t *testing.T) {
	ctx := context.Background()
	available, err := AvailableBytes(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if available == 0 {
		t.Fatal("expected a non-zero available disk size")
	}
	if err := RequireAtLeast(ctx, t.TempDir(), 1); err != nil {
		t.Fatal(err)
	}
	if err := RequireAtLeast(ctx, t.TempDir(), ^uint64(0)); err == nil {
		t.Fatal("impossible disk floor must fail")
	}
}
