//go:build linux

package judge

import (
	"errors"
	"os/exec"
)

// CheckResourceLimitSupport refuses to advertise a Linux judge that cannot
// apply the address-space, CPU, and process-count walls used by the runner.
func CheckResourceLimitSupport() error {
	if _, err := exec.LookPath("prlimit"); err != nil {
		return errors.New("prlimit is required for the local judge resource walls")
	}
	return nil
}
