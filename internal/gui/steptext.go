package gui

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"
)

// stepKind is how the replay words one kind of step: its part of the turn, as the sim's game
// screen names it, and its title.
type stepKind struct {
	phase string
	title func(w *words, s *replay.Step)
}

func fixedTitle(t string) func(*words, *replay.Step) {
	return func(w *words, _ *replay.Step) { w.text(t) }
}

var stepKinds = [...]stepKind{
	replay.KindLobby:       {"Setup", fixedTitle("Lobby")},
	replay.KindOpeningHand: {"Setup", fixedTitle("Drew an opening hand")},
	replay.KindMulligan:    {"Setup", fixedTitle("Mulliganed")},
	replay.KindLife:        {"Setup", fixedTitle("Life cards placed")},
	replay.KindTurnStart:   {"Refresh · Draw · DON", turnStartTitle},
	replay.KindPlay: {"Main", func(w *words, s *replay.Step) {
		w.text("Played ")
		w.card(s.Cards[0])
	}},
	replay.KindAttach: {"Main", func(w *words, s *replay.Step) {
		w.text("Gave ")
		w.card(s.Cards[0])
		w.text(" " + strconv.Itoa(s.Don) + " DON")
	}},
	replay.KindAttack: {"Attack Step", func(w *words, s *replay.Step) {
		w.card(s.Cards[0])
		w.text(" attacked ")
		w.card(s.Cards[1])
	}},
	replay.KindBlock: {"Blocker Step", func(w *words, s *replay.Step) {
		w.text("Blocked with ")
		w.card(s.Cards[0])
	}},
	replay.KindCounter: {"Counter Step", func(w *words, s *replay.Step) {
		w.text("Countered with ")
		w.card(s.Cards[0])
		if s.Counter > 0 {
			w.text(" +" + strconv.Itoa(s.Counter))
		}
	}},
	replay.KindDamage: {"Damage", func(w *words, s *replay.Step) {
		bt := s.Battle
		w.card(bt.Attacker)
		w.text(" " + strconv.Itoa(bt.AttackerPower) + " vs ")
		w.card(bt.Defender())
		w.text(" " + strconv.Itoa(bt.TargetPower))
	}},
	// A battle effect shows the phase of the step it comes in. Its own is for a replay that
	// starts with one.
	replay.KindBattleEffect: {"Attack Step", battleEffectTitle},
	replay.KindEffect:       {"Main", effectTitle},
	replay.KindConcede:      {"Conceded", fixedTitle("Conceded")},
	replay.KindDeckOut:      {"Out of cards", fixedTitle("Ran out of cards")},
}

// stepTitle says what a replay step did, as "Monkey D. Luffy attacked Rocks D. Xebec".
func stepTitle(g *simlog.Game, s *replay.Step, col ui.Color) []textPiece {
	w := newWords(g, col)
	stepKinds[s.Kind].title(w, s)
	return w.out
}

// phaseWord names a step's part of the turn.
func phaseWord(steps []replay.Step, i int) string {
	for i > 0 && steps[i].Kind == replay.KindBattleEffect {
		i--
	}
	return stepKinds[steps[i].Kind].phase
}

func turnStartTitle(w *words, s *replay.Step) {
	if len(s.Drew) == 0 && s.Don == 0 {
		w.text("Turn " + strconv.Itoa(s.Turn))
		return
	}
	if len(s.Drew) > 0 {
		w.text("Drew ")
		w.cards(s.Drew)
	}
	if s.Don > 0 {
		if len(s.Drew) > 0 {
			w.text(" · ")
		}
		w.text("+" + strconv.Itoa(s.Don) + " DON")
	}
}

// effectTitle is the first thing the step's effect did, as "Nami: Set 5 DON active".
func effectTitle(w *words, s *replay.Step) {
	if s.Effect == nil {
		w.card(s.Cards[0])
		return
	}
	w.linked(s.Effect.Head)
	w.text(": ")
	for _, d := range s.Effect.Did {
		if d.Kind == replay.DidCount {
			w.text(countText(d.Count))
		} else {
			w.linked(d.Text)
		}
	}
}

// battleEffectTitle is what an effect during an attack did, with what it cost in brackets, as
// "Charlotte Linlin: +1000 to Stussy (trashed Edward Newgate)".
func battleEffectTitle(w *words, s *replay.Step) {
	var effects, paid []func()
	var trashed []string
	for _, d := range s.Effect.Did {
		switch {
		case d.Kind == replay.DidPower:
			effects = append(effects, func() {
				w.text(signedPower(d.N) + " to ")
				w.card(d.Card)
			})
		case d.Kind == replay.DidTrash:
			trashed = append(trashed, d.Card)
		case d.Kind == replay.DidCount && d.Count.Kind == replay.CountDonRested:
			paid = append(paid, func() { w.text("rested " + strconv.Itoa(d.Count.N) + " DON") })
		case d.Kind == replay.DidCount && d.Count.Kind == replay.CountDonReturned:
			paid = append(paid, func() { w.text("returned " + strconv.Itoa(d.Count.N) + " DON") })
		case d.Kind == replay.DidCount:
			effects = append(effects, func() { w.text(countText(d.Count)) })
		default:
			effects = append(effects, func() { w.linked(d.Text) })
		}
	}
	if len(trashed) > 0 {
		paid = append(paid, func() {
			w.text("trashed ")
			w.cards(trashed)
		})
	}
	list := func(parts []func()) {
		for i, p := range parts {
			if i > 0 {
				w.text(", ")
			}
			p()
		}
	}
	w.card(s.Cards[0])
	switch {
	case len(effects) > 0 && len(paid) > 0:
		w.text(": ")
		list(effects)
		w.text(" (")
		list(paid)
		w.text(")")
	case len(effects) > 0:
		w.text(": ")
		list(effects)
	case len(paid) > 0:
		w.text(": ")
		list(paid)
	}
}

func countText(c replay.Count) string {
	n := strconv.Itoa(c.N)
	switch c.Kind {
	case replay.CountDonActive:
		return "Set " + n + " DON active"
	case replay.CountDonRested:
		return "Rest " + n + " DON"
	case replay.CountDonReturned:
		return "Return " + n + " DON"
	case replay.CountDonAdded:
		return "Add " + n + " DON"
	case replay.CountRestedDonAdded:
		return "Add " + n + " rested DON"
	case replay.CountCardsDrawn:
		if c.N == 1 {
			return "Draw 1 card"
		}
		return "Draw " + n + " cards"
	}
	return c.Verb + " " + n + " " + c.Object
}

func signedPower(n int) string {
	if n < 0 {
		return strconv.Itoa(n)
	}
	return "+" + strconv.Itoa(n)
}

// stepResult is how an attack ended, as "Hit for 1" or "Attack failed", or "".
func stepResult(g *simlog.Game, s *replay.Step) string {
	var parts []string
	for _, o := range s.Outcome {
		switch o.Kind {
		case replay.OutcomeFailed:
			parts = append(parts, "Attack failed")
		case replay.OutcomeKO:
			parts = append(parts, cardName(g, o.Card)+" K.O.'d")
		case replay.OutcomeHit:
			parts = append(parts, "Hit for "+strconv.Itoa(o.N))
		}
	}
	if s.Lethal {
		parts = append(parts, "Lethal")
	}
	return strings.Join(parts, " · ")
}
