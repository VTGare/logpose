package gui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

type detailState struct {
	fingerprint string
	// timeline is nil while the match's log loads.
	timeline *matchTimeline
	// gameType is a store.GameType, an int for ui.Segmented. It shows the user's choice before
	// the snapshot with it arrives.
	gameType int
	// typeChoice is gameType's label for ui.Select.
	typeChoice string
	// lifePoint is the turn under the pointer on the turn-by-turn chart, or -1, and flowTab the
	// chart's tab: life, hand or board.
	lifePoint, flowTab int
	// note is the note as typed, and savedNote the last note saved, trimmed.
	note, savedNote string
	noteFocused     bool
	// focusNote puts the keyboard in the note box once, when the row menu opens the match to
	// write one.
	focusNote bool
}

// matchTimeline is what the match page reads from a game's log.
type matchTimeline struct {
	Fingerprint string
	// Me and Opp are life by turn, as simlog.Game.LifeByTurn gives it, and the rest hand and
	// board sizes the same way.
	Me, Opp           []int
	MeHand, OppHand   []int
	MeBoard, OppBoard []int
	Moments           []simlog.Moment
	Marks             []simlog.Mark
}

func (v *view) openMatch(m stats.Match) {
	v.saveNote()
	v.page = pageMatches
	v.detail = detailState{
		fingerprint: m.Fingerprint, gameType: int(m.Type), lifePoint: -1,
		note: m.Note, savedNote: m.Note,
	}
	v.act.LoadTimeline(m.Fingerprint, m.Me.ID, m.Opp.ID)
}

// closeMatch goes back to the match list and saves the note being typed.
func (v *view) closeMatch() {
	v.saveNote()
	v.detail = detailState{}
}

func (v *view) saveNote() {
	d := &v.detail
	note := strings.TrimSpace(d.note)
	if d.fingerprint == "" || note == d.savedNote {
		return
	}
	d.savedNote = note
	v.act.SetGameNote(d.fingerprint, note)
}

// setTimeline shows a match's life once its log is read, unless another match opened meanwhile.
func (v *view) setTimeline(l matchTimeline) {
	if v.detail.fingerprint == l.Fingerprint {
		v.detail.timeline = &l
	}
}

func (v *view) findMatch(fingerprint string) (stats.Match, bool) {
	for _, m := range v.snap.Matches {
		if m.Fingerprint == fingerprint {
			return m, true
		}
	}
	return stats.Match{}, false
}

func (v *view) matchPage(c *ui.Context) {
	t := c.Theme()
	m, ok := v.findMatch(v.detail.fingerprint)
	if !ok {
		v.detail = detailState{}
		v.matchesPage(c)
		return
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			ui.Row(c).Gap(12).Children(func() {
				if ui.Button(c, "Back to matches").Clicked() {
					v.closeMatch()
				}
				v.bookmarkButton(c, m)
				ui.Button(c, "Watch replay").Disabled(true).Tooltip("Replays aren't built yet")
				// Only room lobbies can be ranked, so private ones have no choice to make.
				if m.Lobby.InRoom() {
					ui.Text(c, "Game type").TextColor(t.TextMuted).Margin(0, 0, 0, 12)
					v.typeSelect(c, m)
				}
				ui.Spacer(c)
				ui.Text(c, m.PlayedAt.Format("Mon Jan 2 2006, 15:04")).TextColor(t.TextMuted)
			})
			v.faceOff(c, m)
			ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
				v.handsTile(c, m).Grow(2).Basis(0)
				v.noteTile(c).Grow(1).Basis(0)
			})
			v.timeline(c, m)
		})
	})
}

// faceOff is both leaders with the result between them.
func (v *view) faceOff(c *ui.Context, m stats.Match) {
	side := func(p store.GamePlayer, who string, mine bool) {
		info := func() {
			align := ui.Start
			if !mine {
				align = ui.End
			}
			ui.Column(c).Gap(4).AlignItems(align).Children(func() {
				ui.Text(c, who).FontSize(12).TextColor(bannerTextMuted)
				ui.Text(c, v.snap.card(p.Leader).Name).FontSize(20).Bold().TextColor(bannerText)
				ui.Text(c, setOf(p.Leader)+" · "+orderWord(m, mine)).FontSize(12).TextColor(bannerTextMuted)
			})
		}
		justify := ui.Start
		if !mine {
			justify = ui.End
		}
		ui.Row(c).Grow(1).Basis(0).Gap(14).Justify(justify).Children(func() {
			if mine {
				v.cardThumb(c, p.Leader, 80)
				info()
				return
			}
			info()
			v.cardThumb(c, p.Leader, 80)
		})
	}
	ui.Row(c).Padding(18, 22).Gap(16).Radius(12).Background(bannerBase).Children(func() {
		side(m.Me, "You", true)
		ui.Column(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Text(c, resultWord(m.Result)).FontSize(28).Bold().TextColor(resultColor(m.Result))
			ui.Text(c, fmt.Sprintf("%d turns · %s", m.Turns, endWord(m.End.Kind))).TextColor(bannerText)
			ui.Text(c, gameType(m)).FontSize(12).TextColor(bannerTextMuted)
		})
		side(m.Opp, m.Opp.Name, false)
	})
}

func orderWord(m stats.Match, mine bool) string {
	switch {
	case !m.TurnOrderKnown:
		return "turn order unknown"
	case m.WentFirst == mine:
		return "went first"
	default:
		return "went second"
	}
}

// typeSelect is the game type as a drop-down, its first option saying what the guess gives,
// with why the game counts as it does on hover.
func (v *view) typeSelect(c *ui.Context, m stats.Match) ui.Element {
	opts := v.typeOptions(m)
	v.detail.typeChoice = opts[v.detail.gameType]
	sel := ui.Select(c, &v.detail.typeChoice, opts).Label("Game type").Tooltip(v.typeNote(m))
	if i := slices.Index(opts, v.detail.typeChoice); i >= 0 && i != v.detail.gameType {
		v.detail.gameType = i
		v.act.SetGameType(m.Fingerprint, store.GameType(i))
	}
	return sel
}

// typeOptions labels the game types in store.GameType order, the first saying what the guess
// gives.
func (v *view) typeOptions(m stats.Match) []string {
	guess := "Auto: Friendly"
	if v.lobbyGames(m) == 1 {
		guess = "Auto: Ranked"
	}
	return []string{guess, "Ranked", "Friendly"}
}

func (v *view) lobbyGames(m stats.Match) int {
	n := 0
	for _, o := range v.snap.Matches {
		if o.Lobby.Code == m.Lobby.Code {
			n++
		}
	}
	return n
}

func (v *view) typeNote(m stats.Match) string {
	switch store.GameType(v.detail.gameType) {
	case store.TypeRanked:
		return "Counted as ranked, as you set it."
	case store.TypeFriendly:
		return "Counted as a friendly, as you set it."
	}
	n := 0
	for _, o := range v.snap.Matches {
		if o.Lobby.Code == m.Lobby.Code {
			n++
		}
	}
	if n == 1 {
		return "Counted as ranked, since it's the only game in its lobby."
	}
	return fmt.Sprintf("Counted as a friendly, since its lobby has %s.", games(n))
}

// noteTile saves the note when the user leaves the box, and closeMatch saves it when they leave
// the page while still typing.
func (v *view) noteTile(c *ui.Context) ui.Element {
	return tile(c, "Note", func() {
		box := ui.TextArea(c, &v.detail.note).Placeholder("Add a note").Lines(3, 20).Grow(1).Label("Match note")
		if v.detail.focusNote {
			box.Focus()
			v.detail.focusNote = false
		}
		focused := box.Focused()
		if v.detail.noteFocused && !focused {
			v.saveNote()
		}
		v.detail.noteFocused = focused
	})
}

func legend(c *ui.Context, col ui.Color, label string) {
	ui.Row(c).Gap(6).Children(func() {
		ui.Box(c).Size(10, 10).Radius(5).Background(col)
		ui.Text(c, label).FontSize(12)
	})
}

// bookmarkButton bookmarks the match, or removes its bookmark.
func (v *view) bookmarkButton(c *ui.Context, m stats.Match) {
	label := "☆  Bookmark"
	if m.Bookmarked {
		label = "★  Bookmarked"
	}
	if ui.Button(c, label).Clicked() {
		v.act.SetBookmark(m.Fingerprint, !m.Bookmarked)
	}
}
