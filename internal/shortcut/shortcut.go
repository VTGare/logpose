// Package shortcut adds and removes Windows shortcuts to a program in the Start menu and on the
// desktop.
package shortcut

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Place int

const (
	StartMenu Place = iota
	Desktop
)

var Places = []Place{StartMenu, Desktop}

// Link is what a shortcut starts.
type Link struct {
	Name        string // the shortcut's file name without ".lnk", which Windows shows
	Target      string
	Args        []string
	Dir         string
	Description string // the shortcut's tooltip
}

func Path(place Place, name string) (string, error) {
	dir, err := folder(place)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".lnk"), nil
}

func Exists(place Place, name string) bool {
	path, err := Path(place, name)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Create writes the shortcut to place, replacing one with the same name.
func Create(place Place, l Link) error {
	path, err := Path(place, l.Name)
	if err != nil {
		return err
	}
	return save(path, l)
}

// Remove deletes the shortcut named name from place. A missing one is not an error.
func Remove(place Place, name string) error {
	path, err := Path(place, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
