package simlog

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

var roleNames = map[LobbyRole]string{RoleHost: "host", RoleJoin: "join"}

// String is the role's stored name, or "" for RoleUnknown.
func (r LobbyRole) String() string { return roleNames[r] }

// ParseLobbyRole reads a stored name. An unknown name reads as RoleUnknown.
func ParseLobbyRole(name string) LobbyRole {
	for r, n := range roleNames {
		if n == name {
			return r
		}
	}
	return RoleUnknown
}

type Lobby struct {
	Code string
	Role LobbyRole
}

// InRoom reports whether the lobby was made in one of the sim's rooms, such as Spoiler or
// Standard. Their codes are 7 characters, starting with the room's letter, and the sim's open
// lobbies have 6. OPBounty's ranked lobbies are in rooms, and so are friendlies hosted there.
func (l Lobby) InRoom() bool { return len(l.Code) == 7 }

// Room is the letter of the room the lobby was made in, such as 'X' for Spoiler, or 0 for a
// lobby outside a room. A joiner's code from the sim's ranked panel lacks the letter.
func (l Lobby) Room() byte {
	if !l.InRoom() {
		return 0
	}
	return l.Code[0]
}

// CouldBeRanked reports whether a game in the lobby with OPBounty's match number could have been
// ranked. The joiner's log has the room code without its room letter when the sim's ranked panel
// joined, so the match number counts too.
func CouldBeRanked(l Lobby, match int) bool { return l.InRoom() || match > 0 }

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

// SplitID reads a player ID as "name#tag". The sim writes a zero-width space before the '#',
// which neither part keeps.
func SplitID(id string) (name, tag string) {
	name, tag, ok := strings.Cut(id, "\u200b#")
	if !ok {
		name, tag, _ = strings.Cut(id, "#")
	}
	return name, tag
}

// PlayerID is the ID of a player as the sim writes them, with a zero-width space before the '#'.
func PlayerID(s string) string {
	name, tag := SplitID(s)
	return Player{Name: name, Tag: tag}.ID()
}

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
	endKindCount
)

var EndKinds = func() []EndKind {
	out := make([]EndKind, 0, endKindCount)
	for k := EndUnknown; k < endKindCount; k++ {
		out = append(out, k)
	}
	return out
}()

var endNames = map[EndKind]string{EndUnknown: "unknown", EndConcede: "concede", EndDeckOut: "deck_out", EndLethal: "lethal"}

// String is the kind's stored name, which the games table's CHECK lists.
func (k EndKind) String() string {
	if name, ok := endNames[k]; ok {
		return name
	}
	return endNames[EndUnknown]
}

// ParseEndKind reads a stored name. An unknown name reads as EndUnknown.
func ParseEndKind(name string) EndKind {
	for k, n := range endNames {
		if n == name {
			return k
		}
	}
	return EndUnknown
}

type End struct {
	Kind EndKind
	// Loser is the losing player's ID. It is empty when the result is unknown. Kind can be
	// unknown with a loser, when only the sim's END line says who won.
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
	// Match is OPBounty's match number when the sim's own ranked panel set the game up, and 0
	// otherwise, including games queued through the OPBounty app.
	Match int
	Lines []Line
	// Moves and Checks are the RZ1 stream. Both are empty in logs from sim versions before it.
	Moves  []Move
	Checks []Check
	// Fingerprint identifies the game across files. In a rematch, the sim saves the whole lobby
	// log after every game, so the same game shows up in several files.
	Fingerprint string
	// Sets is every card set the game's cards come from, such as "OP13", sorted.
	Sets []string
	// Closed is set when the log has the sim's RZ1|END line. A game read from a .partial.log
	// while it was being played doesn't.
	Closed bool
	// CardNames maps the codes of the cards the log mentions to the names it gives them.
	CardNames map[string]string
	// Text is the game's own part of the log, one line per line ending in "\n". Parsing it again
	// gives the same game.
	Text string
}

// Version is the parser's version. It goes up whenever parsing the same text gives different
// results, so stored games know to be parsed again.
const Version = 9

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
	var lobby Lobby
	b := newBuilder()
	keep := func() {
		g := b.finish(lobby)
		lobby = g.Lobby
		if g.Turns > 0 || g.End.Kind != EndUnknown {
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

// ParseOne reads a game's Text, which parses back to exactly that one game.
func ParseOne(text string) (*Game, error) {
	games, err := Parse(strings.NewReader(text))
	if err != nil {
		return nil, err
	}
	if len(games) != 1 {
		return nil, fmt.Errorf("log parses to %d games", len(games))
	}
	return &games[0], nil
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
	// draws is each player's cards drawn before the first turn, in order.
	draws map[string][]string
	print []string
	ends  endTracker
	seal  *seal
	text  strings.Builder
}

func newBuilder() *builder {
	return &builder{
		game:  Game{Mulligans: make(map[string]Mulligan), CardNames: make(map[string]string)},
		draws: make(map[string][]string),
		ends:  newEndTracker(),
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
	b.text.WriteString(line)
	b.text.WriteByte('\n')
	if strings.HasPrefix(line, "RZ1|") {
		b.addRZ(line)
		return
	}
	b.addText(line)
}

// A rematch's log has no lobby line, so it gets the lobby of the game before it, and its text
// gets the line too, so parsing the text alone finds it.
func (b *builder) finish(prev Lobby) Game {
	b.inferOwners()
	b.handsFromDraws()
	g := b.game
	g.Text = b.text.String()
	if g.Lobby.Code == "" && prev.Code != "" {
		g.Lobby = prev
		g.Text = lobbyLine(prev) + "\n" + g.Text
	}
	g.Turns = b.turn
	if g.End.Kind == EndUnknown {
		g.End = b.ends.lethal(&g)
	}
	if g.End.Loser == "" {
		g.End = b.sealedEnd(&g)
	}
	g.Fingerprint = b.fingerprint()
	g.Sets = setsOf(&g)
	return g
}

// handsFromDraws fills the opening hand of a player the log gives no "Hand before Mulligan"
// for, usually the opponent. The sim logs their 5 opening draws, and after a mulligan 5 more
// draws and then "Mulligan".
func (b *builder) handsFromDraws() {
	for id, drawn := range b.draws {
		m := b.game.Mulligans[id]
		if len(m.Before) > 0 || len(drawn) < handSize {
			continue
		}
		m.Before = drawn[:handSize]
		if m.Mulliganed && len(drawn) >= 2*handSize {
			m.After = drawn[handSize : 2*handSize]
		}
		b.game.Mulligans[id] = m
	}
}

const handSize = 5

// setsOf collects the sets of the leaders and of every card that moved. DON moves have no code.
func setsOf(g *Game) []string {
	seen := make(map[string]bool)
	add := func(code string) {
		if set, _, ok := strings.Cut(code, "-"); ok && set != "" {
			seen[set] = true
		}
	}
	for _, p := range g.Players {
		add(p.Leader)
	}
	for _, m := range g.Moves {
		add(m.Card)
	}
	sets := make([]string, 0, len(seen))
	for s := range seen {
		sets = append(sets, s)
	}
	slices.Sort(sets)
	return sets
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
	name, tag := SplitID(id)
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
