package simlog

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"slices"
	"strings"
)

type LobbyRole int

const (
	RoleUnknown LobbyRole = iota
	RoleHost
	RoleJoin
)

type Lobby struct {
	Code string
	Role LobbyRole
}

// OPBounty reports whether OPBounty made the lobby. Its codes are 7 characters, starting with
// the queue's prefix letter, and the sim's own codes are 6.
func (l Lobby) OPBounty() bool { return len(l.Code) == 7 }

type Player struct {
	Name string
	// Tag is the number after '#', which tells apart players with the same name.
	Tag string
	// Owner is the player's number in the RZ1 stream, 1 or 2. It is 0 when the game has no
	// RZ1 stream.
	Owner  int
	Leader string
}

func (p Player) ID() string { return p.Name + "#" + p.Tag }

type Mulligan struct {
	Before     []string
	Mulliganed bool
	After      []string
}

type EndKind int

const (
	EndUnknown EndKind = iota
	EndConcede
	EndDeckOut
	EndLethal
)

func (k EndKind) String() string {
	switch k {
	case EndConcede:
		return "concede"
	case EndDeckOut:
		return "deck out"
	case EndLethal:
		return "lethal"
	default:
		return "unknown"
	}
}

type End struct {
	Kind EndKind
	// Loser is the losing player's ID. It is empty when the end is unknown.
	Loser string
}

type Game struct {
	Lobby      Lobby
	SimVersion string
	Players    []Player
	// First is the ID of the player who went first, or empty when the log doesn't say.
	First     string
	Mulligans map[string]Mulligan
	// Turns counts the turns both players ended. The turn in progress when the game ended is not
	// counted.
	Turns int
	End   End
	Lines []Line
	// Moves and Checks are the RZ1 stream. Both are empty in logs from sim versions before it.
	Moves  []Move
	Checks []Check
	// Fingerprint identifies the game across files. In a rematch, the sim saves the whole lobby
	// log after every game, so the same game shows up in several files.
	Fingerprint string
}

func (g *Game) Player(id string) (Player, bool) {
	for _, p := range g.Players {
		if p.ID() == id {
			return p, true
		}
	}
	return Player{}, false
}

func (g *Game) HasRZ() bool { return len(g.Moves) > 0 }

// Parse reads one AutoSaved combat log, which holds one game or, after rematches, several. A
// lobby opened after the last game but never played isn't returned.
func Parse(r io.Reader) ([]Game, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var games []Game
	b := newBuilder()
	keep := func() {
		if g := b.finish(); g.Turns > 0 || g.End.Kind != EndUnknown {
			games = append(games, g)
		}
	}
	for sc.Scan() {
		line := strings.TrimPrefix(strings.TrimRight(sc.Text(), "\r"), "\ufeff")
		if b.startsNewGame(line) {
			keep()
			b = newBuilder()
		}
		b.add(line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if b.started {
		keep()
	}
	return games, nil
}

// fingerprintMoves is how many RZ1 moves go into a fingerprint. The file saved at the end of a
// game and a later rematch file can differ after the game, such as in chat, but not in its first
// moves.
const fingerprintMoves = 60

type builder struct {
	game    Game
	started bool
	turn    int
	ended   bool
	// begun is set at the first line past the lobby and mulligans.
	begun   bool
	headers int
	print   []string
	ends    endTracker
}

func newBuilder() *builder {
	return &builder{
		game: Game{Mulligans: make(map[string]Mulligan)},
		ends: newEndTracker(),
	}
}

func (b *builder) startsNewGame(line string) bool {
	if line == rematchLine {
		return false
	}
	if b.ended && b.started {
		return true
	}
	return strings.HasPrefix(line, "RZ1|HDR|") && b.headers > 0
}

func (b *builder) add(line string) {
	if line == "" {
		return
	}
	if line == rematchLine {
		b.ended = true
		return
	}
	b.started = true
	if strings.HasPrefix(line, "RZ1|") {
		b.addRZ(line)
		return
	}
	b.addText(line)
}

func (b *builder) finish() Game {
	b.inferOwners()
	g := b.game
	g.Turns = b.turn
	if g.End.Kind == EndUnknown {
		g.End = b.ends.lethal(&g)
	}
	g.Fingerprint = b.fingerprint()
	return g
}

func (b *builder) fingerprint() string {
	h := sha256.New()
	ids := make([]string, 0, len(b.game.Players))
	for _, p := range b.game.Players {
		ids = append(ids, p.ID()+"="+p.Leader)
	}
	slices.Sort(ids)
	for _, s := range append(ids, b.print...) {
		h.Write([]byte(s))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

func (b *builder) player(id string) *Player {
	for i := range b.game.Players {
		if b.game.Players[i].ID() == id {
			return &b.game.Players[i]
		}
	}
	name, tag, _ := strings.Cut(id, "#")
	b.game.Players = append(b.game.Players, Player{Name: name, Tag: tag})
	return &b.game.Players[len(b.game.Players)-1]
}

func (b *builder) other(id string) string {
	for _, p := range b.game.Players {
		if p.ID() != id {
			return p.ID()
		}
	}
	return ""
}
