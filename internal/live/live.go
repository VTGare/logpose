package live

import (
	"strings"

	"github.com/VTGare/logpose/internal/simlog"
)

type Phase int

const (
	Idle Phase = iota
	// Lobby is a lobby that has no game yet: hosting, joining or matched by OPBounty.
	Lobby
	Playing
)

type Player struct {
	// ID is name#tag, the same as store.GamePlayer.ID.
	ID    string
	Owner int
	// Leader is "" when the sim didn't log it. It logs the leaders for most games but not all.
	Leader string
	Local  bool
	Life   int
	Hand   int
	Deck   int
}

type State struct {
	Phase Phase
	// Lobby is the lobby code once the sim logs it. OPBounty's are 7 characters.
	Lobby   string
	Hosting bool
	// Match is OPBounty's match number when the sim's ranked panel set the game up, or 0.
	Match   int
	Players []Player
	// First is the ID of the player who goes first, once they get their first DON.
	First string
}

func (s State) Local() (Player, bool) {
	for _, p := range s.Players {
		if p.Local {
			return p, true
		}
	}
	return Player{}, false
}

func (s State) Opponent() (Player, bool) {
	for _, p := range s.Players {
		if !p.Local {
			return p, true
		}
	}
	return Player{}, false
}

// Ranked reports whether the game could be ranked. Stored games also need their lobby code to be
// in no other game, which a game in progress can't know.
func (s State) Ranked() bool { return simlog.CouldBeRanked(simlog.Lobby{Code: s.Lobby}, s.Match) }

// Saved is a combat log the sim wrote, with the format its lobby validated decks against.
type Saved struct {
	// File is the log's base name, as store.Source.File has it.
	File string
	// Format is the sim's name for the format's slot, such as "Nationals", or "" when the log
	// didn't say. SimVersion is the sim's version, such as "1.44a".
	Format, SimVersion string
}

// Tracker follows the sim's Player.log line by line.
//
// The sim logs a deck shuffle for each player when a game starts and after each mulligan. The
// local player's first one comes right after "deck filled, do shuffle". The RZ1 moves after a
// shuffle carry that player's owner number. After a game the sim logs a stale RZ1 header, so RZ1
// lines only count during a game.
type Tracker struct {
	state     State
	localNext bool
	// pending is the player whose owner number the next move gives.
	pending string
	lastSeq int

	// format and simVersion outlive games, since the sim logs them before it opens a lobby.
	format, simVersion string
	saved              []Saved
}

// TakeSaved returns the logs saved since the last call.
func (t *Tracker) TakeSaved() []Saved {
	s := t.saved
	t.saved = nil
	return s
}

func (t *Tracker) State() State {
	s := t.state
	s.Players = append([]Player(nil), s.Players...)
	return s
}

// rzPrefix comes before each RZ1 line in Player.log.
const rzPrefix = "[ReplaySync] "

// Line takes one line of Player.log.
func (t *Tracker) Line(line string) {
	line = strings.TrimRight(line, "\r")
	switch {
	case strings.HasPrefix(line, rzPrefix+"RZ1|"):
		t.rz(line[len(rzPrefix):])
	case strings.HasPrefix(line, "[CombatLog SaveMyLogLines] wrote OK"):
		t.wrote(line)
		t.reset()
	case line == "Return to Main", strings.HasPrefix(line, "OPB_RESULT|"):
		t.reset()
	case strings.HasPrefix(line, "Validate Decks "):
		t.format = strings.TrimSpace(strings.TrimPrefix(line, "Validate Decks "))
	case strings.HasPrefix(line, "sim version: "):
		t.simVersion = strings.TrimSpace(strings.TrimPrefix(line, "sim version: "))
	case strings.HasPrefix(line, "Host with "):
		t.reset()
		t.state.Phase, t.state.Hosting = Lobby, true
	case strings.HasPrefix(line, "Join with "):
		t.reset()
		t.state.Phase = Lobby
	case strings.HasPrefix(line, "match ready on Room ID "):
		code, _, _ := strings.Cut(strings.TrimPrefix(line, "match ready on Room ID "), " ")
		t.lobby(code)
	case strings.HasPrefix(line, "Joined relay "):
		t.lobby(strings.TrimSpace(strings.TrimPrefix(line, "Joined relay ")))
	case line == "deck filled, do shuffle":
		t.localNext = true
	case strings.HasPrefix(line, "shuffle deck for "):
		t.shuffled(simlog.PlayerID(strings.TrimSpace(strings.TrimPrefix(line, "shuffle deck for "))))
	}
}

func (t *Tracker) reset() {
	*t = Tracker{format: t.format, simVersion: t.simVersion, saved: t.saved}
}

// wrote records the log a "wrote OK" line names in its "file=" part, a path under the sim.
func (t *Tracker) wrote(line string) {
	for part := range strings.SplitSeq(line, "|") {
		file, ok := strings.CutPrefix(strings.TrimSpace(part), "file=")
		if !ok {
			continue
		}
		if i := strings.LastIndexAny(file, `\/`); i >= 0 {
			file = file[i+1:]
		}
		t.saved = append(t.saved, Saved{File: file, Format: t.format, SimVersion: t.simVersion})
	}
}

func (t *Tracker) lobby(code string) {
	if t.state.Phase == Idle {
		t.state.Phase = Lobby
	}
	t.state.Lobby = code
}

func (t *Tracker) shuffled(id string) {
	t.state.Phase = Playing
	p := t.add(id)
	if t.localNext {
		p.Local = true
		t.localNext = false
	}
	t.pending = id
}

func (t *Tracker) rz(line string) {
	if match, ok := simlog.ParseMeta(line); ok {
		t.state.Match = match
		return
	}
	if t.state.Phase != Playing {
		return
	}
	switch simlog.RZKind(line) {
	case "PLY":
		if pl, ok := simlog.ParsePlayer(line); ok {
			p := t.add(pl.ID())
			p.Owner, p.Leader = pl.Owner, pl.Leader
		}
	case "CHK":
		if c, ok := simlog.ParseCheck(line); ok {
			if p := t.byOwner(c.Owner); p != nil {
				p.Deck, p.Hand, p.Life = c.Deck, c.Hand, c.Life
			}
		}
	default:
		if m, ok := simlog.ParseMove(line); ok {
			t.move(m)
		}
	}
}

func (t *Tracker) move(m simlog.Move) {
	if m.Seq < t.lastSeq {
		t.rematch()
	}
	t.lastSeq = m.Seq
	if p := t.player(t.pending); p != nil && p.Owner == 0 && t.byOwner(m.Owner) == nil {
		p.Owner = m.Owner
	}
	t.pending = ""
	if t.state.First == "" && m.From == simlog.ZoneDonDeck && m.To == simlog.ZoneDonField {
		if p := t.byOwner(m.Owner); p != nil {
			t.state.First = p.ID
		}
	}
}

// rematch starts a new game with the same players. The move numbers start again with each game.
func (t *Tracker) rematch() {
	t.state.First = ""
	for i := range t.state.Players {
		p := &t.state.Players[i]
		p.Life, p.Hand, p.Deck = 0, 0, 0
	}
}

func (t *Tracker) add(id string) *Player {
	if p := t.player(id); p != nil {
		return p
	}
	t.state.Players = append(t.state.Players, Player{ID: id})
	return &t.state.Players[len(t.state.Players)-1]
}

func (t *Tracker) player(id string) *Player {
	for i := range t.state.Players {
		if t.state.Players[i].ID == id {
			return &t.state.Players[i]
		}
	}
	return nil
}

func (t *Tracker) byOwner(owner int) *Player {
	for i := range t.state.Players {
		if t.state.Players[i].Owner == owner {
			return &t.state.Players[i]
		}
	}
	return nil
}

func same(a, b State) bool {
	if a.Phase != b.Phase || a.Lobby != b.Lobby || a.Hosting != b.Hosting || a.Match != b.Match || a.First != b.First ||
		len(a.Players) != len(b.Players) {
		return false
	}
	for i := range a.Players {
		if a.Players[i] != b.Players[i] {
			return false
		}
	}
	return true
}
