//go:build !windows

package sim

import (
	"errors"
	"fmt"
)

func runningExe() (string, error) {
	return "", fmt.Errorf("sim: finding the running sim: %w", errors.ErrUnsupported)
}

func usualDirs() []string { return nil }
