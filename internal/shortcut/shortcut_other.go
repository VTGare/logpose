//go:build !windows

package shortcut

import (
	"errors"
	"fmt"
)

func folder(Place) (string, error) {
	return "", fmt.Errorf("shortcut: finding the folder: %w", errors.ErrUnsupported)
}

func save(string, Link) error {
	return fmt.Errorf("shortcut: writing a shortcut: %w", errors.ErrUnsupported)
}
