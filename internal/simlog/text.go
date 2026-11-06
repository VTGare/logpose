package simlog

import (
	"regexp"
	"strconv"
	"strings"
)

const rematchLine = "Opponent is Ready for Rematch"

type Line struct {
	// Turn is the game turn the line belongs to, starting at 1. Lines before the first turn,
	// such as the lobby and mulligans, have turn 0.
	Turn int
	// Player is the ID of the player the line starts with. It is empty for lines without one,
	// such as battle results, and "You" for prompts the sim writes to the local player.
	Player string
	// Text is the line with player prefix and TextMeshPro tags removed, and card links written
	// as "Name [CODE]".
	Text  string
	Cards []string
}

var (
	// The sim writes a zero-width space between the name and the '#'.
	playerPrefix = regexp.MustCompile(`^\[([^\]]*?)\x{200B}?#(\d+)\] `)
	cardLink     = regexp.MustCompile(`<mark><link="([^"]*)">[^<]*</link></mark>`)
	richTag      = regexp.MustCompile(`</?(?:b|i|u|s|size|color)(?:=[^>]*)?>`)
	codeList     = regexp.MustCompile(`^\[([^\]]*)\]$`)
	hitLine      = regexp.MustCompile(`^.* \[([A-Z0-9-]+)\] hit for \d+ damage$`)
	cardCode     = regexp.MustCompile(`\[([A-Z]{1,3}\d{0,2}-\d{3}[A-Z]?)\]`)
)

func parseLine(raw string) Line {
	var l Line
	rest := raw
	if m := playerPrefix.FindStringSubmatch(raw); m != nil {
		l.Player = m[1] + "#" + m[2]
		rest = raw[len(m[0]):]
	} else if after, ok := strings.CutPrefix(raw, "[You] "); ok {
		l.Player = "You"
		rest = after
	}
	rest = cardLink.ReplaceAllString(rest, "$1")
	rest = richTag.ReplaceAllString(rest, "")
	l.Text = rest
	for _, m := range cardCode.FindAllStringSubmatch(rest, -1) {
		l.Cards = append(l.Cards, m[1])
	}
	return l
}

func (b *builder) addText(raw string) {
	l := parseLine(raw)
	header := b.headerLine(l)
	if !header && !b.playerLine(l) {
		b.battleLine(l)
	}
	if !b.begun && !header && !isPregame(l.Text) {
		b.begun = true
	}
	if b.begun {
		l.Turn = b.turn + 1
		if l.Text == "End Turn" {
			l.Turn = b.turn
		}
	}
	b.game.Lines = append(b.game.Lines, l)
}

var pregamePrefixes = []string{
	"Leader is ", "Chose to go ", "Will select turn order", "Hand before Mulligan", "Mulligan",
	"Hand after Mulligan", "Drew card from deck",
}

func isPregame(text string) bool {
	if strings.HasSuffix(text, " Has Connected") {
		return true
	}
	for _, p := range pregamePrefixes {
		if strings.HasPrefix(text, p) {
			return true
		}
	}
	return false
}

func (b *builder) headerLine(l Line) bool {
	t := l.Text
	switch {
	case strings.HasPrefix(t, "Attempting to connect to "):
		b.game.Lobby = Lobby{Code: strings.TrimPrefix(t, "Attempting to connect to "), Role: RoleJoin}
	case strings.HasPrefix(t, "Waiting for a Connection with Room ID:"):
		b.game.Lobby = Lobby{Code: strings.TrimPrefix(t, "Waiting for a Connection with Room ID:"), Role: RoleHost}
	case strings.HasPrefix(t, "Version is "):
		if b.game.SimVersion == "" {
			b.game.SimVersion = strings.TrimPrefix(t, "Version is ")
		}
	default:
		return false
	}
	return true
}

func (b *builder) playerLine(l Line) bool {
	if l.Player == "" || l.Player == "You" {
		return false
	}
	t := l.Text
	switch {
	case strings.HasPrefix(t, "Leader is "):
		if len(l.Cards) > 0 {
			b.player(l.Player).Leader = l.Cards[len(l.Cards)-1]
		}
	case t == "Chose to go First":
		b.player(l.Player)
		b.game.First = l.Player
	case t == "Chose to go Second":
		b.player(l.Player)
		b.game.First = b.other(l.Player)
	case strings.HasPrefix(t, "Hand before Mulligan: "):
		m := b.game.Mulligans[l.Player]
		m.Before = codes(strings.TrimPrefix(t, "Hand before Mulligan: "))
		b.game.Mulligans[l.Player] = m
	case t == "Mulligan":
		m := b.game.Mulligans[l.Player]
		m.Mulliganed = true
		b.game.Mulligans[l.Player] = m
	case strings.HasPrefix(t, "Hand after Mulligan: "):
		m := b.game.Mulligans[l.Player]
		m.After = codes(strings.TrimPrefix(t, "Hand after Mulligan: "))
		b.game.Mulligans[l.Player] = m
	case t == "End Turn":
		b.turn++
	case t == "Concedes!":
		b.game.End = End{Kind: EndConcede, Loser: l.Player}
	case t == "Is Out of Cards, Loses the Game!":
		b.game.End = End{Kind: EndDeckOut, Loser: l.Player}
	case strings.HasPrefix(t, "Life: "):
		if n, err := strconv.Atoi(strings.TrimPrefix(t, "Life: ")); err == nil {
			b.ends.lifeByPlayer[l.Player] = n
		}
	case strings.Contains(t, " attacking "):
		b.attackLine(l)
	default:
		return false
	}
	return true
}

func (b *builder) attackLine(l Line) {
	if len(l.Cards) == 0 {
		return
	}
	defender := b.other(l.Player)
	target := l.Cards[len(l.Cards)-1]
	p, ok := b.game.Player(defender)
	b.ends.attacked(defender, ok && p.Leader == target)
}

func (b *builder) battleLine(l Line) {
	switch {
	case l.Text == "Attack Fails":
		b.ends.failed()
	case hitLine.MatchString(l.Text):
		b.ends.hit(&b.game)
	}
}

func codes(list string) []string {
	m := codeList.FindStringSubmatch(list)
	if m == nil || m[1] == "" {
		return nil
	}
	return strings.Split(m[1], ",")
}
