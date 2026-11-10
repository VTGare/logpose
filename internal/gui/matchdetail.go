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
	// noteOpen shows the note box on a match without a note, after "Add note".
	noteOpen bool
	// focusNote puts the keyboard in the note box once, when the row menu opens the match to
	// write one.
	focusNote bool
	// replay is the match's open replay, or nil on the match page.
	replay *replayState
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
	// Replayable is false for logs from sim versions without the RZ1 move stream.
	Replayable bool
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
	if v.detail.replay != nil {
		v.replayPage(c, m)
		return
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				back := ui.Row(c).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label("Back to matches")
				if back.Clicked() {
					v.closeMatch()
				}
				back.Children(func() {
					ui.Icon(c, iconPrevStep).TextColor(t.TextMuted)
					ui.Text(c, "Matches").TextColor(t.TextMuted)
				})
				ui.Text(c, "/").TextColor(faintColor(t))
				ui.Text(c, m.PlayedAt.Format("Mon Jan 2, 15:04")).Bold()
				ui.Spacer(c)
				if stats.CanBeRanked(m.Game) {
					v.typeSelect(c, m)
				}
				if !v.noteShown() && iconButton(c, iconNote, "Add note", false).Label("Add note").Clicked() {
					v.detail.noteOpen = true
					v.detail.focusNote = true
				}
				v.bookmarkButton(c, m)
				v.replayButton(c, m)
			})
			v.faceOff(c, m)
			v.handsTile(c, m)
			if v.noteShown() {
				v.noteTile(c)
			}
			v.timeline(c, m)
		})
	})
}

// noteShown is whether the match page shows the note box: when the match has a note, or the
// user asked to add one.
func (v *view) noteShown() bool {
	d := v.detail
	return d.noteOpen || d.focusNote || strings.TrimSpace(d.note) != "" || d.savedNote != ""
}

// faceOff is both leaders' art coming in from the ends, each lit by its colors, with the names
// on it and the result between them.
func (v *view) faceOff(c *ui.Context, m stats.Match) {
	t := c.Theme()
	mine, theirs := v.art.get(m.Me.Leader, artFaceMine), v.art.get(m.Opp.Leader, artFaceTheirs)
	myCols, theirCols := colorsOf(v.snap.card(m.Me.Leader)), colorsOf(v.snap.card(m.Opp.Leader))
	base := panelColor(t)
	side := func(p store.GamePlayer, who string, me bool) {
		align := ui.Start
		col := youColor
		if !me {
			align, col = ui.End, oppColor
		}
		ui.Column(c).Gap(1).AlignItems(align).Children(func() {
			ui.Text(c, who).FontSize(12).Bold().TextColor(col)
			ui.Text(c, v.snap.card(p.Leader).Name).FontSize(26).Bold()
			ui.Text(c, setOf(p.Leader)+" · "+orderWord(m, me)).FontSize(12).TextColor(t.TextMuted)
		})
	}
	ui.Row(c).Height(180).Radius(12).Clip().Background(base).Border(1, t.Border.Alpha(0.6)).
		Shadow(0, 6, 20, 0, panelShade).AlignItems(ui.End).Padding(0, 20, 16, 20).Draw(func(p *ui.Painter, r ui.Rect) {
		p.FillRadialGradient(r, light(myCols[0].Glow, ui.Point{X: 0, Y: 1.1}, r.W*0.45, 0.3), 0)
		p.FillRadialGradient(r, light(theirCols[0].Glow, ui.Point{X: 1, Y: 1.1}, r.W*0.45, 0.3), 0)
		w := r.W * 0.44
		if mine != nil {
			p.Image(mine, ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}, ui.Cover)
		}
		if theirs != nil {
			p.Image(theirs, ui.Rect{X: r.X + r.W - w, Y: r.Y, W: w, H: r.H}, ui.Cover)
		}
		p.FillGradient(r, ui.LinearGradient{Angle: 0, Stops: []ui.GradientStop{
			{At: 0, Color: base.Alpha(0.94)}, {At: 0.4, Color: base.Alpha(0.5)}, {At: 0.7, Color: base.Alpha(0)},
		}}, 0)
	}).Children(func() {
		side(m.Me, "You", true)
		ui.Column(c).Grow(1).Gap(2).AlignItems(ui.Center).Children(func() {
			ui.Text(c, resultWord(m.Result)).FontSize(44).Bold().TextColor(resultColor(m.Result))
			ui.Text(c, fmt.Sprintf("%d turns · %s · %s", m.Turns, endWord(m.End.Kind), gameType(m))).FontSize(13)
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
	if m.Match > 0 || v.lobbyGames(m) == 1 {
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
	if m.Match > 0 {
		return fmt.Sprintf("Counted as ranked, since the sim logged it as OPBounty match %d.", m.Match)
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
			v.detail.noteOpen = true
		}
		focused := box.Focused()
		if v.detail.noteFocused && !focused {
			v.saveNote()
		}
		v.detail.noteFocused = focused
	})
}

// replayButton opens the replay once the log is read and has the moves a replay needs.
func (v *view) replayButton(c *ui.Context, m stats.Match) {
	l := v.detail.timeline
	switch {
	case l == nil:
		ui.Button(c, "Watch replay").Disabled(true).Tooltip("Reading the log")
	case !l.Replayable:
		ui.Button(c, "Watch replay").Disabled(true).Tooltip(noMovesText)
	default:
		b := ui.PrimaryButton(c, "").Label("Watch replay").Children(func() {
			ui.Icon(c, iconPlay)
			ui.Text(c, "Watch replay").SingleLine()
		})
		if b.Clicked() {
			v.openReplay(m, -1)
		}
	}
}

// bookmarkButton bookmarks the match, or removes its bookmark.
func (v *view) bookmarkButton(c *ui.Context, m stats.Match) {
	var b ui.Element
	if m.Bookmarked {
		b = ui.Button(c, "").Label("Bookmarked").Children(func() {
			ui.Icon(c, iconStarSet).TextColor(bookmarkColor)
			ui.Text(c, "Bookmarked").SingleLine()
		})
	} else {
		b = iconButton(c, iconStar, "Bookmark", false).Label("Bookmark")
	}
	if b.Clicked() {
		v.act.SetBookmark(m.Fingerprint, !m.Bookmarked)
	}
}
