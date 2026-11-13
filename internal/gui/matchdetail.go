package gui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

type detailState struct {
	fingerprint string
	// from is the page the match opened on. It stays chosen in the sidebar, and Back returns to it.
	from string
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
	// Game is the parsed log, which the replay is built from, or nil when Err says why it
	// couldn't be read.
	Game *simlog.Game
	Err  error
}

func (pg *matchesPage) openMatch(e *env, m stats.Match) {
	pg.saveNote()
	pg.detail = detailState{
		fingerprint: m.Fingerprint, from: e.page, gameType: int(m.Type), lifePoint: -1,
		note: m.Note, savedNote: m.Note,
	}
	pg.do(func(a matchActions) { a.LoadTimeline(m.Fingerprint, m.Me.ID, m.Opp.ID) })
}

func (pg *matchesPage) closeMatch() {
	pg.saveNote()
	pg.detail = detailState{}
}

func (pg *matchesPage) saveNote() {
	d := &pg.detail
	note := strings.TrimSpace(d.note)
	if d.fingerprint == "" || note == d.savedNote {
		return
	}
	d.savedNote = note
	pg.do(func(a matchActions) { a.SetGameNote(d.fingerprint, note) })
}

func (pg *matchesPage) setTimeline(l matchTimeline) {
	if pg.detail.fingerprint == l.Fingerprint {
		pg.detail.timeline = &l
		pg.buildReplay()
	}
}

func findMatch(e *env, fingerprint string) (stats.Match, bool) {
	for _, m := range e.snap.Matches {
		if m.Fingerprint == fingerprint {
			return m, true
		}
	}
	return stats.Match{}, false
}

func (pg *matchesPage) matchPage(c *ui.Context, e *env) {
	t := c.Theme()
	m, ok := findMatch(e, pg.detail.fingerprint)
	if !ok {
		pg.detail = detailState{}
		pg.draw(c, e)
		return
	}
	if pg.detail.replay != nil {
		pg.replayPage(c, e, m)
		return
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				from := pageLabel(pg.detail.from)
				back := ui.Row(c).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label("Back to " + strings.ToLower(from))
				if back.Clicked() {
					pg.closeMatch()
				}
				back.Children(func() {
					ui.Icon(c, iconPrevStep).TextColor(t.TextMuted)
					ui.Text(c, from).TextColor(t.TextMuted)
				})
				ui.Text(c, "/").TextColor(faintColor(t))
				ui.Text(c, e.weekday(m.PlayedAt)+", "+m.PlayedAt.Format("15:04")).Bold()
				ui.Spacer(c)
				if stats.CanBeRanked(m.Game) {
					pg.typeSelect(c, m)
				}
				if !pg.noteShown() && iconButton(c, iconNote, "Add note").Label("Add note").Clicked() {
					pg.detail.noteOpen = true
					pg.detail.focusNote = true
				}
				pg.bookmarkButton(c, m)
				pg.replayButton(c, e, m)
			})
			faceOff(c, e, m)
			handsTile(c, e, m)
			if pg.noteShown() {
				pg.noteTile(c)
			}
			pg.flowTile(c, e, m)
			pg.momentsTile(c, e, m)
		})
	})
}

func (pg *matchesPage) noteShown() bool {
	d := pg.detail
	return d.noteOpen || d.focusNote || strings.TrimSpace(d.note) != "" || d.savedNote != ""
}

func faceOff(c *ui.Context, e *env, m stats.Match) {
	t := c.Theme()
	mine, theirs := e.art.get(m.Me.Leader, artFaceMine), e.art.get(m.Opp.Leader, artFaceTheirs)
	myCols, theirCols := colorsOf(e.snap.card(m.Me.Leader)), colorsOf(e.snap.card(m.Opp.Leader))
	base := panelColor(t)
	you := mySide(m)
	column := func(p store.GamePlayer, s side) {
		align := ui.Start
		if !s.mine {
			align = ui.End
		}
		ui.Column(c).Gap(1).AlignItems(align).Children(func() {
			ui.Text(c, s.name()).FontSize(12).Bold().TextColor(s.color())
			ui.Text(c, e.snap.card(p.Leader).Name).FontSize(26).Bold()
			ui.Text(c, cards.SetOf(p.Leader)+" · "+orderWord(m, s.mine)).FontSize(12).TextColor(t.TextMuted)
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
		column(m.Me, you)
		ui.Column(c).Grow(1).Gap(2).AlignItems(ui.Center).Children(func() {
			ui.Text(c, resultWord(m.Result)).FontSize(44).Bold().TextColor(resultColor(t, m.Result))
			ui.Text(c, fmt.Sprintf("%s · %s · %s", turns(m.Turns), endWord(m.End.Kind), gameType(m))).FontSize(13)
		})
		column(m.Opp, you.other())
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

func (pg *matchesPage) typeSelect(c *ui.Context, m stats.Match) ui.Element {
	opts := typeOptions(m)
	pg.detail.typeChoice = opts[pg.detail.gameType]
	sel := ui.Select(c, &pg.detail.typeChoice, opts).Label("Game type").Tooltip(pg.typeNote(m))
	if i := slices.Index(opts, pg.detail.typeChoice); i >= 0 && i != pg.detail.gameType {
		pg.detail.gameType = i
		pg.do(func(a matchActions) { a.SetGameType(m.Fingerprint, store.GameType(i)) })
	}
	return sel
}

// typeOptions labels the game types in store.GameType order, the first saying what the guess
// gives.
func typeOptions(m stats.Match) []string {
	guess := "Auto: Friendly"
	if m.RankedGuess {
		guess = "Auto: Ranked"
	}
	return []string{guess, "Ranked", "Friendly"}
}

func (pg *matchesPage) typeNote(m stats.Match) string {
	switch store.GameType(pg.detail.gameType) {
	case store.TypeRanked:
		return "Counted as ranked, as you set it."
	case store.TypeFriendly:
		return "Counted as a friendly, as you set it."
	}
	switch {
	case m.Match > 0:
		return fmt.Sprintf("Counted as ranked, since the sim logged it as OPBounty match %d.", m.Match)
	case m.RankedGuess:
		return "Counted as ranked, since it's the only game in its lobby."
	}
	return fmt.Sprintf("Counted as a friendly, since its lobby has %s.", games(m.LobbyGames))
}

// noteTile saves the note when the user leaves the box, and closeMatch saves it when they leave
// the page while still typing.
func (pg *matchesPage) noteTile(c *ui.Context) ui.Element {
	return tile(c, "Note", func() {
		box := ui.TextArea(c, &pg.detail.note).Placeholder("Add a note").Lines(3, 20).Grow(1).Label("Match note")
		if pg.detail.focusNote {
			box.Focus()
			pg.detail.focusNote = false
			pg.detail.noteOpen = true
		}
		focused := box.Focused()
		if pg.detail.noteFocused && !focused {
			pg.saveNote()
		}
		pg.detail.noteFocused = focused
	})
}

func (pg *matchesPage) replayButton(c *ui.Context, e *env, m stats.Match) {
	l := pg.detail.timeline
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
			pg.openReplay(e, m, -1)
		}
	}
}

func (pg *matchesPage) bookmarkButton(c *ui.Context, m stats.Match) {
	var b ui.Element
	if m.Bookmarked {
		b = ui.Button(c, "").Label("Bookmarked").Children(func() {
			ui.Icon(c, iconStarSet).TextColor(bookmarkColor)
			ui.Text(c, "Bookmarked").SingleLine()
		})
	} else {
		b = iconButton(c, iconStar, "Bookmark").Label("Bookmark")
	}
	if b.Clicked() {
		pg.do(func(a matchActions) { a.SetBookmark(m.Fingerprint, !m.Bookmarked) })
	}
}
