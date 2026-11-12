package gui

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// side is one player of a match from the user's seat. Its color and its words come from here.
type side struct {
	mine bool
	opp  string
}

// sideOf is the side of the player with the ID. Anyone but the user is the opponent.
func sideOf(m stats.Match, id string) side { return side{mine: id == m.Me.ID, opp: m.Opp.Name} }

func mySide(m stats.Match) side { return side{mine: true, opp: m.Opp.Name} }

func (s side) other() side { return side{mine: !s.mine, opp: s.opp} }

func (s side) color() ui.Color {
	if s.mine {
		return youColor
	}
	return oppColor
}

// name starts a sentence: "You" or the opponent's name.
func (s side) name() string {
	if s.mine {
		return "You"
	}
	return s.opp
}

// object ends one: "you" or the opponent's name.
func (s side) object() string {
	if s.mine {
		return "you"
	}
	return s.opp
}

func (s side) possessive() string {
	if s.mine {
		return "Your"
	}
	return s.opp + "'s"
}
