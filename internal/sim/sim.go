package sim

import (
	"errors"
	"os"
	"path/filepath"
)

var ErrNotFound = errors.New("sim: OPTCGSim not found")

// Install is an OPTCGSim folder, the one holding OPTCGSim.exe.
type Install struct {
	Root string
}

func (i Install) Logs() string { return filepath.Join(i.Root, "CombatLogs", "AutoSaved") }

func (i Install) streamingAssets() string {
	return filepath.Join(i.Root, "OPTCGSim_Data", "StreamingAssets")
}

// CardImage returns the path of a card's image. The sim ships a full image and a small one per
// card, in a folder per set.
func (i Install) CardImage(code string, small bool) string {
	set := code
	for j := range code {
		if code[j] == '-' {
			set = code[:j]
			break
		}
	}
	name := code + ".jpg"
	if small {
		name = code + "_small.jpg"
	}
	return filepath.Join(i.streamingAssets(), "Cards", set, name)
}

// OPBountyPack is the OPBounty client the sim ships, whose pack holds a card database.
func (i Install) OPBountyPack() string {
	return filepath.Join(i.streamingAssets(), "OPBounty", "win", "OPBounty.pck")
}

// Open checks that dir is an OPTCGSim folder.
func Open(dir string) (Install, error) {
	if _, err := os.Stat(filepath.Join(dir, "OPTCGSim_Data")); err != nil {
		return Install{}, ErrNotFound
	}
	return Install{Root: dir}, nil
}

// Locate finds the sim from its running process, then from the folders it's usually in.
func Locate() (Install, error) {
	if exe, err := runningExe(); err == nil {
		if in, err := Open(filepath.Dir(exe)); err == nil {
			return in, nil
		}
	}
	for _, dir := range usualDirs() {
		if in, err := Open(dir); err == nil {
			return in, nil
		}
	}
	return Install{}, ErrNotFound
}
