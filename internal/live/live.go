package live

import (
	"strconv"
	"strings"
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

func (s State) Ranked() bool { return len(s.Lobby) == 7 }

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
}

func (t *Tracker) State() State {
	s := t.state
	s.Players = append([]Player(nil), s.Players...)
	return s
}

const rzPrefix = "[ReplaySync] RZ1|"

// Line takes one line of Player.log and reports whether the state changed.
func (t *Tracker) Line(line string) bool {
	line = strings.TrimRight(line, "\r")
	before := t.State()
	switch {
	case strings.HasPrefix(line, rzPrefix):
		if t.state.Phase == Playing {
			t.rz(strings.Split(line[len(rzPrefix):], "|"))
		}
	case line == "Return to Main",
		strings.HasPrefix(line, "OPB_RESULT|"),
		strings.HasPrefix(line, "[CombatLog SaveMyLogLines] wrote OK"):
		t.reset()
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
		t.shuffled(playerID(strings.TrimPrefix(line, "shuffle deck for ")))
	}
	return !same(before, t.State())
}

func (t *Tracker) reset() {
	*t = Tracker{}
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

func (t *Tracker) rz(parts []string) {
	if len(parts) == 0 {
		return
	}
	switch parts[0] {
	case "HDR":
	case "PLY":
		if len(parts) != 4 {
			return
		}
		owner, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}
		p := t.add(playerID(parts[2]))
		p.Owner, p.Leader = owner, parts[3]
	case "CHK":
		t.check(parts)
	default:
		t.move(parts)
	}
}

func (t *Tracker) move(parts []string) {
	if len(parts) < 7 {
		return
	}
	seq, err := strconv.Atoi(parts[0])
	if err != nil {
		return
	}
	owner, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}
	if seq < t.lastSeq {
		t.rematch()
	}
	t.lastSeq = seq
	if p := t.player(t.pending); p != nil && p.Owner == 0 && t.byOwner(owner) == nil {
		p.Owner = owner
	}
	t.pending = ""
	from, to := parts[3], parts[5]
	if t.state.First == "" && from == "4" && to == "5" {
		if p := t.byOwner(owner); p != nil {
			t.state.First = p.ID
		}
	}
}

func (t *Tracker) check(parts []string) {
	if len(parts) != 13 {
		return
	}
	n := make([]int, len(parts)-1)
	for i := range n {
		v, err := strconv.Atoi(parts[i+1])
		if err != nil {
			return
		}
		n[i] = v
	}
	if p := t.byOwner(n[1]); p != nil {
		p.Deck, p.Hand, p.Life = n[2], n[3], n[5]
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

// playerID drops the zero-width space the sim puts before a player's tag.
func playerID(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "\u200b", "")
}

func same(a, b State) bool {
	if a.Phase != b.Phase || a.Lobby != b.Lobby || a.Hosting != b.Hosting || a.First != b.First ||
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
