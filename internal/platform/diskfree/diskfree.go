package diskfree

import (
	"context"
	"fmt"
)

// AvailableBytes reports free space available to the process on the volume
// containing path.
func AvailableBytes(ctx context.Context, path string) (uint64, error) {
	return availableBytes(ctx, path)
}

// RequireAtLeast fails when the volume has less than minBytes available.
func RequireAtLeast(ctx context.Context, path string, minBytes uint64) error {
	available, err := AvailableBytes(ctx, path)
	if err != nil {
		return err
	}
	if available < minBytes {
		return fmt.Errorf(
			"insufficient disk space: %d MiB available, need at least %d MiB",
			available>>20,
			minBytes>>20,
		)
	}
	return nil
}
