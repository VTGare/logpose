package simlog

import (
	"regexp"
	"strconv"
	"strings"
)

const rematchLine = "Opponent is Ready for Rematch"

type LineKind int

const (
	LineOther LineKind = iota
	LineEndTurn
	LineConcede
	// LineDeckOut is "Is Out of Cards, Loses the Game!".
	LineDeckOut
	// LineAttack is "X attacking Y". Cards holds the attacker and then the target.
	LineAttack
	LineAttackFails
	// LineHit is "X hit for N damage". N is the damage.
	LineHit
	LineBlock
	// LineCounterDiscard is "Discard X for Counter N". N is the counter.
	LineCounterDiscard
	// LinePlay is "Deploy X", a card played from the hand.
	LinePlay
	// LineAttach is "Attach N Don to X" or "Attach N Rested Don to X". N is the DON.
	LineAttach
	// LineKO is "X Destroyed".
	LineKO
	// LineEffect is "Card [CODE]: effect". Source is the card, and EffectText the part after
	// the colon.
	LineEffect
	// LineTrigger and LineCounter are the effect lines "Card [CODE]: Activate Trigger" and
	// "Card [CODE]: Activate Counter". Source is set as on LineEffect. An event's counter the sim
	// writes as "Card [CODE]: Counter N" is a LineCounter too, with N its counter.
	LineTrigger
	LineCounter
	// LinePrintout is one of the sim's board printouts: "Hand: [...]", "Board: [...]",
	// "Trash: [...]" or "Life: N".
	LinePrintout
	// LineDraw is "Drew card from deck: X".
	LineDraw
)

type Line struct {
	// Turn is the game turn the line belongs to, starting at 1. Lines before the first turn,
	// such as the lobby and mulligans, have turn 0.
	Turn int
	// Player is the ID of the player the line starts with. It is empty for lines without one,
	// such as battle results, and for the sim's prompts to the local player.
	Player string
	// Local is set on the sim's prompts to the local player, which start with "[You]".
	Local bool
	// Text is the line with player prefix and TextMeshPro tags removed, and card links written
	// as "Name [CODE]".
	Text  string
	Cards []string
	Kind  LineKind
	// N is the number a hit, a counter, a counter discard or an attach carries, or a Life
	// printout's count.
	N int
	// Source is the card an effect line comes from, or "".
	Source string
	// Printout is the zone a printout line shows: "Hand", "Board", "Trash" or "Life", or "" when
	// its content doesn't read. Shown is the codes a Hand, Board or Trash printout lists.
	Printout string
	Shown    []string
}

// EffectText is the part of an effect line after "Card [CODE]: ".
func (l Line) EffectText() string {
	_, rest, _ := strings.Cut(l.Text, ": ")
	return rest
}

var (
	// The sim writes a zero-width space between the name and the '#'.
	playerPrefix = regexp.MustCompile(`^\[([^\]]*?)\x{200B}?#(\d+)\] `)
	cardLink     = regexp.MustCompile(`<mark><link="([^"]*)">[^<]*</link></mark>`)
	richTag      = regexp.MustCompile(`</?(?:b|i|u|s|size|color)(?:=[^>]*)?>`)
	codeList     = regexp.MustCompile(`^\[([^\]]*)\]$`)
	hitLine      = regexp.MustCompile(`^.* \[[A-Z0-9-]+\] hit for (\d+) damage$`)
	counterLine  = regexp.MustCompile(`^Discard .* for Counter (\d+)$`)
	attachLine   = regexp.MustCompile(`^Attach (\d+) (?:Rested )?Don to `)
	counterText  = regexp.MustCompile(`^Counter (-?\d+)$`)
	cardCode     = regexp.MustCompile(`\[([A-Z]{1,3}\d{0,2}-\d{3}[A-Z]?)\]`)
	// namedCard is a card link with the text before it, back to the line's start, a "] " or a
	// ": ". That text can start with a verb, as in "Discard Otama", so a card's name is the
	// shortest text seen before its links.
	namedCard = regexp.MustCompile(`(?:^|\] |: )([^\[\]:]+?) \[<mark><link="([^"]+)">`)
)

var printoutZones = []string{"Hand", "Board", "Trash", "Life"}

func parseLine(raw string) Line {
	var l Line
	rest := raw
	if m := playerPrefix.FindStringSubmatch(raw); m != nil {
		l.Player = m[1] + "#" + m[2]
		rest = raw[len(m[0]):]
	} else if after, ok := strings.CutPrefix(raw, "[You] "); ok {
		l.Local = true
		rest = after
	}
	rest = cardLink.ReplaceAllString(rest, "$1")
	rest = richTag.ReplaceAllString(rest, "")
	l.Text = rest
	for _, m := range cardCode.FindAllStringSubmatch(rest, -1) {
		l.Cards = append(l.Cards, m[1])
	}
	l.classify()
	return l
}

var (
	wholeLines = map[string]LineKind{
		"End Turn": LineEndTurn, "Concedes!": LineConcede, "Is Out of Cards, Loses the Game!": LineDeckOut,
		"Attack Fails": LineAttackFails,
	}
	effectLines = map[string]LineKind{"Activate Trigger": LineTrigger, "Activate Counter": LineCounter}
	// numberLines carry their N in the first group.
	numberLines = []struct {
		re   *regexp.Regexp
		kind LineKind
	}{{hitLine, LineHit}, {counterLine, LineCounterDiscard}, {attachLine, LineAttach}}
)

func (l *Line) classify() {
	t := l.Text
	if k, ok := wholeLines[t]; ok {
		l.Kind = k
		return
	}
	for _, z := range printoutZones {
		if rest, ok := strings.CutPrefix(t, z+": "); ok {
			l.Kind = LinePrintout
			l.printout(z, rest)
			return
		}
	}
	if strings.HasPrefix(t, "Drew card from deck: ") {
		l.Kind = LineDraw
		return
	}
	if len(l.Cards) > 0 {
		if head, rest, ok := strings.Cut(t, ": "); ok && strings.HasSuffix(head, "["+l.Cards[0]+"]") {
			l.Source, l.Kind = l.Cards[0], LineEffect
			if k, ok := effectLines[rest]; ok {
				l.Kind = k
			} else if m := counterText.FindStringSubmatch(rest); m != nil {
				l.Kind, l.N = LineCounter, atoi(m[1])
			}
			return
		}
	}
	for _, n := range numberLines {
		if m := n.re.FindStringSubmatch(t); m != nil {
			l.Kind, l.N = n.kind, atoi(m[1])
			return
		}
	}
	switch {
	case strings.HasPrefix(t, "Deploy "):
		l.Kind = LinePlay
	case strings.Contains(t, " attacking "):
		l.Kind = LineAttack
	case strings.HasSuffix(t, " Blocks"):
		l.Kind = LineBlock
	case strings.HasSuffix(t, " Destroyed"):
		l.Kind = LineKO
	}
}

func (l *Line) printout(zone, rest string) {
	if zone == "Life" {
		n, err := strconv.Atoi(rest)
		if err == nil {
			l.Printout, l.N = zone, n
		}
		return
	}
	if codeList.MatchString(rest) {
		l.Printout, l.Shown = zone, codes(rest)
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func (b *builder) addText(raw string) {
	b.learnNames(raw)
	l := parseLine(raw)
	header := b.headerLine(l)
	if !b.begun && !header && !isPregame(l.Text) {
		b.begun = true
	}
	if b.begun {
		l.Turn = b.turn + 1
	}
	if !header && !b.playerLine(l) {
		b.battleLine(l)
	}
	b.game.Lines = append(b.game.Lines, l)
}

var pregamePrefixes = []string{
	"Leader is ", "Chose to go ", "Will select turn order", "Hand before Mulligan", "Mulligan",
	"Hand after Mulligan", "Drew card from deck", "Setting up a Relay Room", "Connected to Server: ",
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

const (
	joinPrefix = "Attempting to connect to "
	hostPrefix = "Waiting for a Connection with Room ID:"
)

func lobbyLine(l Lobby) string {
	if l.Role == RoleHost {
		return hostPrefix + l.Code
	}
	return joinPrefix + l.Code
}

func (b *builder) headerLine(l Line) bool {
	t := l.Text
	switch {
	case strings.HasPrefix(t, joinPrefix):
		b.game.Lobby = Lobby{Code: strings.TrimPrefix(t, joinPrefix), Role: RoleJoin}
	case strings.HasPrefix(t, hostPrefix):
		b.game.Lobby = Lobby{Code: strings.TrimPrefix(t, hostPrefix), Role: RoleHost}
	case strings.HasPrefix(t, "Version is "):
		if b.game.SimVersion == "" {
			b.game.SimVersion = strings.TrimPrefix(t, "Version is ")
		}
	default:
		return false
	}
	return true
}

// playerLine reads the lines a named player says. The local player's prompts name no one.
func (b *builder) playerLine(l Line) bool {
	if l.Player == "" {
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
	case l.Kind == LineDraw && !b.begun:
		if len(l.Cards) > 0 {
			b.draws[l.Player] = append(b.draws[l.Player], l.Cards[len(l.Cards)-1])
		}
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
	case l.Kind == LineEndTurn:
		b.turn++
	case l.Kind == LineConcede:
		b.game.End = End{Kind: EndConcede, Loser: l.Player}
	case l.Kind == LineDeckOut:
		b.game.End = End{Kind: EndDeckOut, Loser: l.Player}
	case l.Printout == "Life":
		b.ends.lifeByPlayer[l.Player] = l.N
	case l.Kind == LineAttack:
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
	switch l.Kind {
	case LineAttackFails:
		b.ends.failed()
	case LineHit:
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

// actionPrefix is the start of the sim's action text before a card's name, as the line survey
// of real logs found it.
var actionPrefix = regexp.MustCompile(`^(?:(?:Leader is|Deploy|Trash|Discard|Rest|Destroy|Buff|Grant|Return|Send|Set|Reveal and Draw|attacking|Play|Add|Attach \d+(?: Rested)? Don to) )+`)

func (b *builder) learnNames(raw string) {
	for _, m := range namedCard.FindAllStringSubmatch(richTag.ReplaceAllString(raw, ""), -1) {
		name, code := actionPrefix.ReplaceAllString(strings.TrimSpace(m[1]), ""), m[2]
		if name == "" {
			continue
		}
		if old, ok := b.game.CardNames[code]; !ok || len(name) < len(old) {
			b.game.CardNames[code] = name
		}
	}
}
