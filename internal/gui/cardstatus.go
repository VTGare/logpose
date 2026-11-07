package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
)

// cardMatch is a physical card's identity across frames, so a card that moves between zones
// glides from where it was.
type cardMatch int

// glide moves a card from where the frame before showed it, when the step just moved by one.
func (sv *stepView) glide(el ui.Element, id int) ui.Element {
	if sv.glideFor == 0 || id == 0 {
		return el
	}
	return el.Transition(ui.ElementTransition{Duration: sv.glideFor, Position: true, Match: cardMatch(id)})
}

// glideTime is how long cards take to move on a one-step change: shorter while playing fast,
// and not at all at the fastest speed.
func glideTime(playing bool, speed int) time.Duration {
	if !playing {
		return 250 * time.Millisecond
	}
	return []time.Duration{250 * time.Millisecond, 150 * time.Millisecond, 0}[speed]
}

func signed(n int) string {
	if n < 0 {
		return fmt.Sprintf("−%d", -n)
	}
	return fmt.Sprintf("+%d", n)
}

// statusBadge is how a status shows on a card: a power change as one pill for all of them, a
// set cost as its number, and the rest as icons. Rush has none, since it only matters on the turn
// the card is played, and the hover preview names it.
type statusBadge struct {
	power int
	cost  int
	icons []*ui.SVG
	names []string
}

func badgesOf(card replay.Card) statusBadge {
	var b statusBadge
	b.cost = -1
	for _, st := range card.Statuses {
		switch st.Kind {
		case replay.StatusPower:
			b.power += st.N
		case replay.StatusCost:
			b.cost = st.N
		case replay.StatusRush:
		default:
			if icon := statusIcons[st.Kind]; icon != nil {
				b.icons = append(b.icons, icon)
				b.names = append(b.names, statusNames[st.Kind])
			}
		}
	}
	return b
}

var statusNames = map[replay.StatusKind]string{
	replay.StatusFrozen:      "Frozen",
	replay.StatusNoRest:      "Can't be rested",
	replay.StatusNoAttack:    "Can't attack",
	replay.StatusNegated:     "Negated",
	replay.StatusProtected:   "Immune to effects",
	replay.StatusRush:        "Rush",
	replay.StatusUnblockable: "Unblockable",
}

// statusNotes say what each status does, whose effect it is, and when it ends, for the
// card's hover preview.
func (v *view) statusNotes(card replay.Card) []string {
	var out []string
	for _, st := range card.Statuses {
		var what string
		switch st.Kind {
		case replay.StatusPower:
			what = signed(st.N) + " power"
			if st.Counter {
				what = signed(st.N) + " counter"
			}
		case replay.StatusCost:
			what = fmt.Sprintf("Cost %d", st.N)
		case replay.StatusFrozen:
			what = "Stays rested in its next refresh"
		default:
			what = statusNames[st.Kind]
		}
		if st.Source != "" {
			what += " (" + v.snap.card(st.Source).Name + ")"
		}
		switch {
		case st.ForAttack:
			what += ", for this attack"
		case st.EndsTurn > 0:
			what += fmt.Sprintf(", until the end of turn %d", st.EndsTurn)
		}
		out = append(out, what)
	}
	return out
}

// powerText is a power's value, marked when it's an estimate.
func powerText(p replay.Power) string {
	if p.Estimated {
		return fmt.Sprintf("≈%d", p.Value)
	}
	return fmt.Sprint(p.Value)
}

// powerSum spells out how a power is made, as "7000 = 5000 + 1000 DON + 1000 counter (Nami)".
// The sim shows power below 0 as 0, as "0: 5000 − 7000 (Charlotte Linlin) = −2000, shown as 0".
func (v *view) powerSum(p replay.Power) string {
	var b strings.Builder
	total := 0
	for i, part := range p.Parts {
		total += part.N
		n := part.N
		sep := " + "
		if n < 0 {
			sep, n = " − ", -n
		}
		if i == 0 {
			sep = ""
			if part.N < 0 {
				sep = "−"
			}
		}
		b.WriteString(sep + fmt.Sprint(n))
		switch part.Kind {
		case replay.PartPrinted:
			if n == 0 {
				b.WriteString(" (printed power unknown)")
			}
		case replay.PartDon:
			b.WriteString(" DON")
		case replay.PartCounter:
			b.WriteString(" counter")
		case replay.PartUnlogged:
			b.WriteString(" from effects not in the log")
		}
		if part.Source != "" {
			b.WriteString(" (" + v.snap.card(part.Source).Name + ")")
		}
	}
	if p.Floored() {
		return fmt.Sprintf("%s: %s = %s, shown as 0", powerText(p), b.String(), signed(total))
	}
	return powerText(p) + " = " + b.String()
}

// chips draws a card's status chips down its top right corner, away from where the attack
// arrow usually comes in.
// badges draws a card's statuses over it: the power pill across its bottom edge, and the
// icons down its top right corner, white on near black so they read on any art.
func badges(c *ui.Context, b statusBadge, w, h float32) {
	size := max(w*0.26, 16)
	if b.power != 0 {
		col := winColor
		if b.power < 0 {
			col = lossColor
		}
		ui.Row(c).Absolute().Left(0).Right(0).Top(h - size*0.45).Justify(ui.Center).Children(func() {
			ui.Text(c, signed(b.power)).FontSize(max(w*0.15, 10)).Bold().TextColor(pillText).Padding(0, 6).Radius(8).
				Background(col).Border(2, badgeInk)
		})
	}
	if len(b.icons) == 0 && b.cost < 0 {
		return
	}
	ui.Column(c).Absolute().Top(-size * 0.3).Right(-size * 0.3).Gap(3).Children(func() {
		disc := func(key int) ui.Element {
			return ui.Row(c.Key(key)).Size(size, size).Radius(size/2).Background(badgeInk).Border(1.5, pillText).
				AlignItems(ui.Center).Justify(ui.Center)
		}
		for i, icon := range b.icons {
			disc(i).Tooltip(b.names[i]).Children(func() {
				ui.Icon(c, icon).Size(size*0.6, size*0.6).TextColor(pillText)
			})
		}
		if b.cost >= 0 {
			disc(len(b.icons)).Tooltip(fmt.Sprintf("Cost %d", b.cost)).Children(func() {
				ui.Text(c, fmt.Sprint(b.cost)).FontSize(size * 0.55).Bold().TextColor(pillText)
			})
		}
	})
}

var badgeInk = ui.Hex("#1D1D22")

var statusIcons = map[replay.StatusKind]*ui.SVG{
	replay.StatusFrozen:      iconFrozen,
	replay.StatusNoRest:      iconNoRest,
	replay.StatusNoAttack:    iconNoAttack,
	replay.StatusNegated:     iconNegated,
	replay.StatusProtected:   iconImmune,
	replay.StatusUnblockable: iconUnblockable,
}
