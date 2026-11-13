package page

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/VTGare/logpose/internal/stats"
)

// Side is one player of a match from the user's seat. Its color and its words come from here.
type Side struct {
	Mine bool
	Opp  string
}

// SideOf is the side of the player with the ID. Anyone but the user is the opponent.
func SideOf(m stats.Match, id string) Side { return Side{Mine: id == m.Me.ID, Opp: m.Opp.Name} }

func MySide(m stats.Match) Side { return Side{Mine: true, Opp: m.Opp.Name} }

func (s Side) Other() Side { return Side{Mine: !s.Mine, Opp: s.Opp} }

func (s Side) Color() ui.Color {
	if s.Mine {
		return kit.YouColor
	}
	return kit.OppColor
}

// Name starts a sentence: "You" or the opponent's Name.
func (s Side) Name() string {
	if s.Mine {
		return "You"
	}
	return s.Opp
}

// Object ends one: "you" or the opponent's name.
func (s Side) Object() string {
	if s.Mine {
		return "you"
	}
	return s.Opp
}

func (s Side) Possessive() string {
	if s.Mine {
		return "Your"
	}
	return s.Opp + "'s"
}
