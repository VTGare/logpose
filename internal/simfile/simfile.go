package simfile

import (
	"path/filepath"
	"slices"
)

// CombatLogs lists the combat logs in the sim's AutoSaved folder, oldest first. The sim names
// each one by the time it began it, and a .partial.log is a game still being written.
func CombatLogs(dir string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.log"))
	slices.Sort(paths)
	return paths, err
}
