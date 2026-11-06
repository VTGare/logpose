package sim

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
// card, in a folder per set. Small images are JPEGs, and full ones JPEG or, in about half the
// sets, PNG.
func (i Install) CardImage(code string, small bool) string {
	set := code
	for j := range code {
		if code[j] == '-' {
			set = code[:j]
			break
		}
	}
	dir := filepath.Join(i.streamingAssets(), "Cards", set)
	if small {
		return filepath.Join(dir, code+"_small.jpg")
	}
	jpg := filepath.Join(dir, code+".jpg")
	if _, err := os.Stat(jpg); err == nil {
		return jpg
	}
	return filepath.Join(dir, code+".png")
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

// PlayerLog is the sim's Unity log. The sim writes it as it plays, and renames it to
// Player-prev.log when it starts again.
func PlayerLog() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "AppData", "LocalLow", "Batsu", "OPTCGSim", "Player.log"), nil
}

func Running() bool {
	_, err := runningExe()
	return err == nil
}

// FormatName is the name the sim shows for one of its format slots, such as "Spoiler" for the
// slot "Nationals". The sim keeps each version's formats next to Player.log. Without the file
// it's the slot's own name.
func FormatName(simVersion, slot string) string {
	log, err := PlayerLog()
	// Both names come from Player.log, so they can't reach outside the formats folder.
	if err != nil || simVersion == "" || slot == "" || strings.ContainsAny(slot+simVersion, `/\`) {
		return slot
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(log), simVersion, "Formats", slot+".json"))
	if err != nil {
		return slot
	}
	var f struct {
		Name     string `json:"formatName"`
		Override string `json:"sNameOverride"`
	}
	if json.Unmarshal(data, &f) != nil {
		return slot
	}
	if f.Override != "" {
		return f.Override
	}
	if f.Name != "" {
		return f.Name
	}
	return slot
}
