//go:build windows

package diskfree

import (
	"context"

	"golang.org/x/sys/windows"
)

func availableBytes(ctx context.Context, path string) (uint64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var available, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &available, &total, &totalFree); err != nil {
		return 0, err
	}
	return available, nil
}
