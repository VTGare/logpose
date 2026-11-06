//go:build !windows

package live

import "os"

func open(path string) (*os.File, error) { return os.Open(path) }
