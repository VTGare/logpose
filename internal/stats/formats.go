package stats

import (
	"slices"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/store"
)

// FormatRange is a format with its end, which is the next format's start, or zero for the
// current one.
type FormatRange struct {
	store.Format
	End time.Time
}

// Scope is the format's games. A format holds only ranked games, since casual lobbies can play
// any cards.
func (f FormatRange) Scope() Scope {
	return Scope{Type: RankedGames, Since: f.Start, Until: f.End}
}

// Ranges gives each format, oldest first, its end.
func Ranges(fs []store.Format) []FormatRange {
	out := make([]FormatRange, len(fs))
	for i, f := range fs {
		out[i].Format = f
		if i+1 < len(fs) {
			out[i].End = fs[i+1].Start
		}
	}
	return out
}

// SetDebut is a set whose cards showed up in the local player's games for the first time,
// numbered past every set of its kind seen before, so likely just released.
type SetDebut struct {
	Set   string
	First time.Time
}

// Rooms names the sim room type of an OPBounty lobby by the first letter of its code. Ranked
// plays in the Spoiler room at some times and the Standard room at others.
var Rooms = map[byte]string{'X': "Spoiler", 'W': "Standard", 'E': "Extra Regulation"}

// formatRooms are the rooms ranked's main queue plays in. Other rooms have their own card
// rules, such as Extra Regulation's.
const formatRooms = "XW"

// debutHistory is how many earlier games a debut needs, so the first games logpose sees don't
// count every set in them as new.
const debutHistory = 10

// Debuts finds sets that likely started a new format: ranked games in the Spoiler or Standard
// room, given newest first, show them for the first time after at least debutHistory such
// games, numbered past every set of the same kind before them. Only those count. The sim lets
// casual lobbies play sets before ranked allows them, and other rooms have their own rules.
// Debuts at or before the latest format's start, or dismissed, are left out. They come oldest
// first.
func Debuts(ms []Match, fs []store.Format, dismissed []string) []SetDebut {
	ms = slices.DeleteFunc(slices.Clone(ms), func(m Match) bool {
		return !m.Ranked || !strings.ContainsRune(formatRooms, rune(m.Lobby.Room()))
	})
	highest := make(map[string]int)
	var out []SetDebut
	for i := len(ms) - 1; i >= 0; i-- {
		earlier := len(ms) - 1 - i
		m := ms[i]
		for _, set := range m.Sets {
			kind, n, ok := cards.SplitSet(set)
			if !ok || !formatKinds[kind] {
				continue
			}
			prev, seen := highest[kind]
			if seen && n <= prev {
				continue
			}
			highest[kind] = n
			if seen && earlier >= debutHistory {
				out = append(out, SetDebut{Set: set, First: m.PlayedAt})
			}
		}
	}
	return slices.DeleteFunc(out, func(d SetDebut) bool {
		if slices.Contains(dismissed, d.Set) {
			return true
		}
		return len(fs) > 0 && !d.First.After(fs[len(fs)-1].Start)
	})
}

// formatKinds are the kinds of set whose release starts a format. Starter decks come out often
// and old ones turn up late in rare decks, so they would suggest formats that aren't there.
var formatKinds = map[string]bool{"OP": true, "EB": true}
