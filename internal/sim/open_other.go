//go:build !windows

package sim

import "os"

func OpenShared(path string) (*os.File, error) { return os.Open(path) }
