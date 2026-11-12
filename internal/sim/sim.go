package sim

import (
	"cmp"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/simfile"
)

var ErrNotFound = errors.New("sim: OPTCGSim not found")

// Install is an OPTCGSim folder, the one holding OPTCGSim.exe.
type Install struct {
	Root string
}

func (i Install) Logs() string { return filepath.Join(i.Root, "CombatLogs", "AutoSaved") }

func (i Install) LogCount() int {
	paths, _ := simfile.CombatLogs(i.Logs())
	return len(paths)
}

// HasExe reports whether the folder holds OPTCGSim.exe. Open only checks for the data folder.
func (i Install) HasExe() bool { return fileExists(filepath.Join(i.Root, "OPTCGSim.exe")) }

func (i Install) streamingAssets() string {
	return filepath.Join(i.Root, "OPTCGSim_Data", "StreamingAssets")
}

func (i Install) cardDir(code string) string {
	set, _, _ := strings.Cut(code, "-")
	return filepath.Join(i.streamingAssets(), "Cards", set)
}

// CardImage returns the path of a card's image. The sim ships a full image and a small one per
// card, in a folder per set. Small images are JPEGs, and full ones JPEG or, in about half the
// sets, PNG.
func (i Install) CardImage(code string, small bool) string {
	dir := i.cardDir(code)
	if small {
		return filepath.Join(dir, code+"_small.jpg")
	}
	jpg := filepath.Join(dir, code+".jpg")
	if _, err := os.Stat(jpg); err == nil {
		return jpg
	}
	return filepath.Join(dir, code+".png")
}

// DonImage returns the path of the DON!! card's image. Some installs keep it only in the
// folder's default subfolder.
func (i Install) DonImage() string {
	dir := filepath.Join(i.streamingAssets(), "Cards", "Don")
	if p := filepath.Join(dir, "Don.png"); fileExists(p) {
		return p
	}
	return filepath.Join(dir, "default", "Don.png")
}

func (i Install) DonBack() string {
	return filepath.Join(i.streamingAssets(), "CardBacks", "CardBackDon.png")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
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

// Version is the newest version folder the sim keeps next to Player.log, such as "1.44a", or ""
// when there's none. Not every version makes one: 1.45a doesn't, so a game's log is the better
// source once there is one (see Newer).
func Version() string {
	log, err := PlayerLog()
	if err != nil {
		return ""
	}
	entries, err := os.ReadDir(filepath.Dir(log))
	if err != nil {
		return ""
	}
	var newest string
	var at time.Time
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "" || e.Name()[0] < '0' || e.Name()[0] > '9' {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(at) {
			newest, at = e.Name(), info.ModTime()
		}
	}
	return newest
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

// Newer returns the later of two sim versions such as "1.44a", "1.44a.24" and "1.45a". Numbers
// compare as numbers and letters as text. An empty version is older than any other.
func Newer(a, b string) string {
	if compareVersions(a, b) < 0 {
		return b
	}
	return a
}

func compareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	for i := range min(len(pa), len(pb)) {
		x, y := pa[i], pb[i]
		nx, errx := strconv.Atoi(x)
		ny, erry := strconv.Atoi(y)
		var c int
		if errx == nil && erry == nil {
			c = cmp.Compare(nx, ny)
		} else {
			c = strings.Compare(x, y)
		}
		if c != 0 {
			return c
		}
	}
	return cmp.Compare(len(pa), len(pb))
}

// versionParts splits "1.44a.24" into 1, 44, a, 24.
func versionParts(v string) []string {
	var parts []string
	start := -1
	digit := false
	for i, r := range v {
		d := r >= '0' && r <= '9'
		switch {
		case r == '.':
			if start >= 0 {
				parts = append(parts, v[start:i])
			}
			start = -1
		case start < 0:
			start, digit = i, d
		case d != digit:
			parts = append(parts, v[start:i])
			start, digit = i, d
		}
	}
	if start >= 0 {
		parts = append(parts, v[start:])
	}
	return parts
}
