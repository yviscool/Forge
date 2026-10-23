//go:build !linux

package judge

// CheckResourceLimitSupport is a no-op on platforms whose limiter uses a
// native process/job primitive instead of prlimit.
func CheckResourceLimitSupport() error { return nil }
