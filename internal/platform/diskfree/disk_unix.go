//go:build linux || darwin

package diskfree

import (
	"context"

	"golang.org/x/sys/unix"
)

func availableBytes(ctx context.Context, path string) (uint64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}
