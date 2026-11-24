package page

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/gui/kit"
	"github.com/VTGare/logpose/internal/sim"
)

// ArtActions are the sim-art, gallery and card-image actions the env asks the app for.
type ArtActions interface {
	ChooseSimFolder()
	LoadSimArt(card cards.Card)
	LoadArts(card cards.Card, gen int)
	ScanSimArt()
	// UseSimArt makes the sim show choice for the card. prev is what it showed, for Undo.
	UseSimArt(card cards.Card, choice, prev sim.ArtChoice)
	SaveArt(key string)
	CopyArt(key string)
	OpenURL(url string)
}

// simArtToastTime is how long the message after a change shows, with its Undo.
const simArtToastTime = 10 * time.Second

const artOffNote = "Card art downloads are off in Settings, so only the sim's own art and your own file can be used."

// SimGold marks the art the sim shows.
var SimGold = ui.Hex("#E8B83A")

type SimArtInfo struct {
	NoSim bool
	Cur   sim.ArtChoice
	// OwnFile is the user's own file for the card, when there's one that isn't a printing.
	OwnFile string
	busy    bool
	Err     string
}

type SimArtState struct {
	cards   map[string]SimArtInfo
	loading map[string]bool
	// changed holds the cards the sim shows other art for.
	changed map[string]bool
	scanned bool
}

// For returns what the sim shows for card, starting a background read on the first call.
func (s *SimArtState) For(e *Env, card cards.Card) (SimArtInfo, bool) {
	if info, ok := s.cards[card.Code]; ok {
		return info, true
	}
	if s.loading == nil {
		s.loading = make(map[string]bool)
	}
	if !s.loading[card.Code] {
		s.loading[card.Code] = true
		if e.ArtAct != nil {
			e.ArtAct.LoadSimArt(card)
		}
	}
	return SimArtInfo{}, false
}

func (s *SimArtState) Scan(e *Env) {
	if s.scanned {
		return
	}
	s.scanned = true
	if e.ArtAct != nil {
		e.ArtAct.ScanSimArt()
	}
}

func (s *SimArtState) Changed(code string) bool { return s.changed[code] }

func (s *SimArtState) Set(code string, info SimArtInfo) {
	if s.cards == nil {
		s.cards = make(map[string]SimArtInfo)
	}
	s.cards[code] = info
	delete(s.loading, code)
	if s.changed == nil {
		s.changed = make(map[string]bool)
	}
	s.changed[code] = info.Cur.Kind != sim.ArtOwn
}

func (s *SimArtState) SetChanged(codes []string) {
	s.changed = make(map[string]bool, len(codes))
	for _, code := range codes {
		s.changed[code] = true
	}
	s.scanned = true
}

func (s *SimArtState) Reset() { *s = SimArtState{} }

func (s *SimArtState) Use(e *Env, card cards.Card, choice sim.ArtChoice) {
	info, ok := s.For(e, card)
	if !ok || info.NoSim || info.busy || info.Cur == choice {
		return
	}
	info.busy, info.Err = true, ""
	s.Set(card.Code, info)
	prev := info.Cur
	if e.ArtAct != nil {
		e.ArtAct.UseSimArt(card, choice, prev)
	}
}

// Done records the result of UseSimArt, drops cached bitmaps for the card and queues an Undo toast.
// Invalidation runs even on error because a failed write can still have removed the prior override.
func (s *SimArtState) Done(e *Env, card cards.Card, choice sim.ArtChoice, info SimArtInfo, undo sim.ArtChoice, err error) {
	if err != nil {
		info.Err = "Couldn't change the sim's art: " + err.Error()
	}
	s.Set(card.Code, info)
	if e.Art != nil {
		e.Art.Invalidate(card.Code)
	}
	if err != nil {
		return
	}
	t := ui.Toast{ID: "sim-art", Timeout: simArtToastTime}
	switch choice.Kind {
	case sim.ArtOwn:
		t.Title = "The sim shows its own art for " + card.Name + " again."
	default:
		ca, _ := e.Arts.For(e, card)
		t.Title = "The sim now shows " + card.Name + "'s " + choiceName(ca, choice) + "."
	}
	t.Description = "Restart the sim to see it."
	t.Action = "Undo"
	t.OnAction = func() { s.Use(e, card, undo) }
	e.Toast = &t
}

type ArtsState struct {
	// gen bumps on each reset so in-flight groupings from before it are ignored.
	gen     int
	cards   map[string]arts.CardArts
	loading map[string]bool
}

// For returns card's grouped arts, falling back to Limitless printings until the gallery replies.
func (s *ArtsState) For(e *Env, card cards.Card) (arts.CardArts, bool) {
	if ca, ok := s.cards[card.Code]; ok {
		return ca, true
	}
	if s.loading == nil {
		s.loading = make(map[string]bool)
	}
	if !s.loading[card.Code] {
		s.loading[card.Code] = true
		gen := s.gen
		if e.ArtAct != nil {
			e.ArtAct.LoadArts(card, gen)
		}
	}
	return arts.LimitlessArts(card), false
}

func (s *ArtsState) Set(gen int, code string, ca arts.CardArts) {
	if gen != s.gen {
		return
	}
	if s.cards == nil {
		s.cards = make(map[string]arts.CardArts)
	}
	s.cards[code] = ca
}

func (s *ArtsState) Reset() { *s = ArtsState{gen: s.gen + 1} }

// choiceName names a choice for prose, such as "Alt art", "Base art", "own file" or "own art".
func choiceName(ca arts.CardArts, choice sim.ArtChoice) string {
	switch choice.Kind {
	case sim.ArtFile:
		return "own file"
	case sim.ArtOwn:
		return "own art"
	}
	art, _, ok := ca.Find(choice.Key)
	if !ok {
		return "art"
	}
	name := art.Name()
	if !strings.HasSuffix(name, " art") {
		name += " art"
	}
	return name
}

// SimArtStatus returns a short badge label for what the sim shows and whether it overrides
// the sim's own art.
func SimArtStatus(ca arts.CardArts, info SimArtInfo, loaded bool) (label string, changed bool) {
	if !loaded || info.NoSim {
		return "", false
	}
	switch info.Cur.Kind {
	case sim.ArtOwn:
		return "Sim's own", false
	case sim.ArtFile:
		return "Your file", true
	default:
		if art, _, ok := ca.Find(info.Cur.Key); ok {
			return art.Name(), true
		}
		return "Other art", true
	}
}

// SimArtInSim returns the "In the sim: …" phrase when the sim shows an override, or "".
func SimArtInSim(ca arts.CardArts, info SimArtInfo, loaded bool) string {
	label, changed := SimArtStatus(ca, info, loaded)
	if !changed {
		return ""
	}
	switch label {
	case "Your file":
		label = "your own file"
	case "Base":
		label = "the base card"
	case "Other art":
		label = "other art"
	}
	return "In the sim: " + label
}

// SimArtNotice renders the page-level banner when the sim isn't found or art downloads are off.
func SimArtNotice(c *ui.Context, e *Env, noSim bool) {
	if noSim {
		noSimBanner(c, e)
	} else if e.Prefs.ArtOff {
		ui.Text(c, artOffNote).FontSize(12).TextColor(c.Theme().TextMuted).Wrap()
	}
}

// ViewedArtKey resolves viewed against card's choices, falling back to the first art.
func ViewedArtKey(card cards.Card, info SimArtInfo, ca arts.CardArts, viewed string) string {
	switch viewed {
	case "":
	case arts.SimOwnRef(card.Code).String():
		return viewed
	case arts.FileRef(info.OwnFile).String():
		if info.OwnFile != "" {
			return viewed
		}
	default:
		if _, ver, ok := ca.Find(viewed); ok {
			return ver.Key.String()
		}
	}
	return ca.Arts[0].Head().Key.String()
}

// ArtCount reports how many art choices a card offers.
func ArtCount(ca arts.CardArts, info SimArtInfo, loaded bool) int {
	n := len(ca.Arts)
	if info.OwnFile != "" {
		n++
	}
	if loaded && !info.NoSim {
		n++
	}
	return n
}

// artEntry is one art choice for a card. art is nil for the user's file and the sim's own picture.
type artEntry struct {
	key, name, note string
	art             *arts.Art
	choice          sim.ArtChoice
	inSim, usable   bool
}

// artEntries lists a card's pictures in picker order: its arts, the user's file and the sim's own.
// An art's key is the version being viewed or its head, and its choice puts that version in the sim.
func artEntries(e *Env, card cards.Card, ca arts.CardArts, info SimArtInfo, loaded bool, viewed string) []artEntry {
	usable := loaded && !info.NoSim && !info.busy
	var out []artEntry
	for i, art := range ca.Arts {
		ver := art.Head()
		if viewed != "" {
			if shown, ok := art.Find(viewed); ok {
				ver = shown
			}
		}
		key := ver.Key.String()
		out = append(out, artEntry{
			key: key, name: art.Name(), note: ver.Product, art: &ca.Arts[i],
			choice: sim.ArtChoice{Kind: sim.ArtPrint, Key: key},
			inSim:  loaded && info.Cur.Kind == sim.ArtPrint && art.Has(info.Cur.Key),
			usable: usable && !e.Prefs.ArtOff,
		})
	}
	if info.OwnFile != "" {
		out = append(out, artEntry{
			key: arts.FileRef(info.OwnFile).String(), name: "Your file", note: "A file you added to the sim",
			choice: sim.ArtChoice{Kind: sim.ArtFile}, inSim: info.Cur.Kind == sim.ArtFile, usable: usable,
		})
	}
	if loaded && !info.NoSim {
		out = append(out, artEntry{
			key: arts.SimOwnRef(card.Code).String(), name: "Sim's own", note: "The sim's picture",
			choice: sim.ArtChoice{Kind: sim.ArtOwn}, inSim: info.Cur.Kind == sim.ArtOwn, usable: usable,
		})
	}
	return out
}

// ArtStripState tracks which card and art have the version popover open.
type ArtStripState struct {
	card, head string
}

func (s *ArtStripState) Close() { s.card, s.head = "", "" }

// ArtChoicesOpts configures the interactive art-choice strip.
type ArtChoicesOpts struct {
	// Compact hides the accent bar and dot text, borders the active art in gold,
	// shows only per-card errors, and closes the version popover on pick.
	Compact bool
	// Subject prefixes accessible button labels when several cards share one page.
	Subject string
	// Viewed is the currently previewed art key when Compact is false.
	Viewed string
	// OnLook runs when the user clicks an art thumbnail or picks a version from the popover.
	OnLook func(key string)
}

// ArtChoices renders a card's art thumbnails, pending placeholder, version popover and status note.
func ArtChoices(c *ui.Context, e *Env, card cards.Card, state *ArtStripState, opts ArtChoicesOpts) {
	info, loaded := e.SimArt.For(e, card)
	ca, _ := e.Arts.For(e, card)
	entries := artEntries(e, card, ca, info, loaded, opts.Viewed)

	drawStrip := func() {
		ui.Row(c).Gap(10).Wrap().AlignItems(ui.Start).Children(func() {
			pending := ca.Pending > 0
			for _, en := range entries {
				if en.art == nil && pending {
					pendingColumn(c, e, card, ca.Pending, opts.Compact)
					pending = false
				}
				artChoiceEntry(c, e, card, info, en, state, opts)
			}
			if pending {
				pendingColumn(c, e, card, ca.Pending, opts.Compact)
			}
		})
	}

	if opts.Compact {
		drawStrip()
		if info.Err != "" {
			ui.Text(c, info.Err).FontSize(12).TextColor(kit.LossColor(c.Theme())).Wrap()
		}
		return
	}
	ui.Column(c).Gap(8).Children(func() {
		drawStrip()
		artNote(c, e, info)
	})
}

func artChoiceEntry(c *ui.Context, e *Env, card cards.Card, info SimArtInfo, en artEntry, state *ArtStripState, opts ArtChoicesOpts) {
	if en.art == nil {
		_, look, use := artColumn(c, e, artColumnOpts{
			key: en.key, name: en.name, subject: opts.Subject, versions: 1,
			viewed: !opts.Compact && opts.Viewed == en.key, inSim: en.inSim, usable: en.usable, compact: opts.Compact,
		})
		if look && opts.OnLook != nil {
			opts.OnLook(en.key)
		}
		if use {
			e.SimArt.Use(e, card, en.choice)
		}
		return
	}
	art := *en.art
	head := art.Head().Key.String()
	col, look, use := artColumn(c, e, artColumnOpts{
		key: head, name: en.name, subject: opts.Subject, versions: len(art.Versions),
		viewed: !opts.Compact && art.Has(opts.Viewed), inSim: en.inSim, usable: en.usable, compact: opts.Compact,
	})
	if look {
		onArtLook(card.Code, head, en.key, len(art.Versions) > 1, state, opts)
	}
	if use {
		e.SimArt.Use(e, card, en.choice)
	}
	if len(art.Versions) > 1 && state != nil {
		open := state.card == card.Code && state.head == head
		versionPopover(c, e, col, art, info, en.usable, opts.Viewed, &open, func(k string) {
			if opts.Compact {
				state.Close()
			}
			if opts.OnLook != nil {
				opts.OnLook(k)
			}
		}, func(ch sim.ArtChoice) {
			e.SimArt.Use(e, card, ch)
		})
		if !open && state.card == card.Code && state.head == head {
			state.Close()
		}
	}
}

func onArtLook(code, head, key string, multi bool, state *ArtStripState, opts ArtChoicesOpts) {
	if !multi {
		if opts.OnLook != nil {
			opts.OnLook(key)
		}
		return
	}
	if state != nil {
		state.card, state.head = code, head
	}
	if !opts.Compact && opts.OnLook != nil {
		opts.OnLook(head)
	}
}

type artColumnOpts struct {
	key, name string
	// subject prefixes accessible button labels when several cards share one page.
	subject  string
	versions int
	viewed   bool
	inSim    bool
	usable   bool
	// compact hides the dot's text label and borders the active thumbnail in gold.
	compact bool
}

// artColumn renders one art thumbnail with its label and sim radio dot.
func artColumn(c *ui.Context, e *Env, o artColumnOpts) (col ui.Element, look, use bool) {
	t := c.Theme()
	const w = 60
	target := o.name
	if o.subject != "" {
		target = o.subject + " " + o.name
	}
	col = ui.Column(c.Key(o.key)).Gap(4).MinWidth(w).AlignItems(ui.Center).Children(func() {
		label := "Show " + target
		if o.versions > 1 {
			label = fmt.Sprintf("Show %s, %d versions", target, o.versions)
		}
		pic := ui.ButtonBase(c).Label(label).Background(ui.Transparent).Padding(0).Radius(6).FocusRing(false)
		if o.compact && o.inSim {
			pic.Border(1.5, SimGold)
		}
		look = pic.Clicked()
		pic.Children(func() {
			e.CardImage(c, arts.ExactPrintRef(o.key).String(), w).Radius(6)
			if o.versions > 1 {
				ui.Box(c).Absolute().Top(-6).Right(-6).Size(20, 20).Radius(10).Background(ui.RGBA(20, 20, 22, 0.92)).
					Border(1, ui.RGBA(255, 255, 255, 0.3)).Center().Children(func() {
					ui.Text(c, fmt.Sprint(o.versions)).FontSize(11).FontWeight(600).TextColor(ui.Hex("#FFFFFF"))
				})
			}
		})
		if !o.compact {
			bar := ui.Box(c).Height(3).Width(w).Radius(2)
			if o.viewed {
				bar.Background(t.Accent)
			}
		}
		tc := t.TextMuted
		weight := 400
		if o.viewed || (o.compact && o.inSim) {
			tc = t.Text
		}
		if o.compact && o.inSim {
			weight = 600
		}
		ui.Text(c, o.name).FontSize(11).FontWeight(weight).TextColor(tc).MaxLines(1)
		use = useDot(c, target, o.inSim, o.usable, !o.compact)
	})
	return col, look, use
}

func useDot(c *ui.Context, name string, on, usable, showText bool) bool {
	t := c.Theme()
	label := "Use " + name + " in the sim"
	if on {
		label = name + " is in the sim"
	}
	dot := ui.ButtonBase(c).Label(label).Background(ui.Transparent).Padding(1, 4).Radius(6).Gap(5).
		AlignItems(ui.Center).Disabled(!usable || on)
	clicked := dot.Clicked()
	if dot.Hovered() && usable && !on {
		dot.Background(t.SurfaceHover)
	}
	dot.Children(func() {
		ring := t.TextMuted
		text, tc := "Use", t.TextMuted
		if on {
			ring, text, tc = SimGold, "In sim", SimGold
		}
		alpha := float32(1)
		if !usable && !on {
			alpha = 0.4
		}
		ui.Box(c).Size(14, 14).Radius(7).Border(1.5, ring.Alpha(alpha)).Justify(ui.Center).AlignItems(ui.Center).
			Children(func() {
				if on {
					ui.Box(c).Size(7, 7).Radius(3.5).Background(SimGold)
				}
			})
		if showText {
			ui.Text(c, text).FontSize(11).TextColor(tc.Alpha(alpha)).FontWeight(map[bool]int{true: 600, false: 400}[on])
		}
	})
	return clicked
}

// pendingColumn stands for printings whose pictures are still being compared. They often turn out
// to be one art, so they take one column.
func pendingColumn(c *ui.Context, e *Env, card cards.Card, n int, compact bool) {
	t := c.Theme()
	const w = 60
	ui.Column(c.Key("pending")).Gap(4).Width(w).AlignItems(ui.Center).Children(func() {
		e.CardPlaceholder(c, card.Code, w, CardHeight(w)).Radius(6)
		if !compact {
			// Reserves the height of artColumn's accent bar so captions line up.
			ui.Box(c).Height(3)
		}
		ui.Text(c, fmt.Sprintf("Checking %d more", n)).FontSize(11).TextColor(t.TextMuted).Wrap().TextAlign(ui.Center)
	})
}

func versionPopover(
	c *ui.Context, e *Env, anchor ui.Element, art arts.Art, info SimArtInfo,
	usable bool, viewed string, open *bool, onLook func(key string), onUse func(choice sim.ArtChoice),
) {
	t := c.Theme()
	ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		kit.Popover(panel.Margin(6, 0, 0, 0).Padding(12, 14).Width(430), t)
		ui.Column(c).Gap(8).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, fmt.Sprintf("%s · %d versions", art.Name(), len(art.Versions))).FontSize(13).FontWeight(600)
				ui.Text(c, "The same art, stamped for each printing.").FontSize(12).TextColor(t.TextMuted)
			})
			shown, inArt := art.Find(info.Cur.Key)
			inArt = inArt && info.Cur.Kind == sim.ArtPrint
			for _, ver := range art.Versions {
				key := ver.Key.String()
				on := inArt && shown.Key == ver.Key
				ui.Row(c.Key(key)).Gap(10).AlignItems(ui.Center).Children(func() {
					row := ui.ButtonBase(c).Label("Show " + ver.Short + ", " + ver.Product).Background(ui.Transparent).Padding(2).Radius(6).
						Gap(10).AlignItems(ui.Center).Grow(1).Basis(0).MinWidth(0)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					if row.Clicked() && onLook != nil {
						onLook(key)
					}
					row.Children(func() {
						e.CardImage(c, arts.ExactPrintRef(key).String(), 30).Radius(6)
						ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
							col := t.Text
							if viewed == key {
								col = t.Accent
							}
							ui.Text(c, ver.Short).FontSize(13).FontWeight(500).TextColor(col).MaxLines(1).Ellipsis("…")
							ui.Text(c, ver.Product).FontSize(12).TextColor(t.TextMuted).MaxLines(2).Ellipsis("…")
						})
					})
					if useDot(c, ver.Short+", "+ver.Product, on, usable, true) && onUse != nil {
						onUse(sim.ArtChoice{Kind: sim.ArtPrint, Key: key})
					}
				})
			}
		})
	})
}

func artNote(c *ui.Context, e *Env, info SimArtInfo) {
	t := c.Theme()
	switch {
	case info.NoSim:
		noSimBanner(c, e)
	case info.Err != "":
		ui.Text(c, info.Err).FontSize(12).TextColor(kit.LossColor(t)).Wrap()
	case e.Prefs.ArtOff:
		ui.Text(c, artOffNote).FontSize(12).TextColor(t.TextMuted).Wrap()
	case info.Cur.Kind == sim.ArtFile:
		ui.Text(c, "The sim shows a file you added yourself. If you pick other art, logpose keeps your file "+
			"and it stays here to pick again.").FontSize(12).TextColor(t.TextMuted).Wrap()
	default:
		ui.Text(c, "Click a picture to look at it. Pick a dot to use that art in the sim.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
	}
}

func noSimBanner(c *ui.Context, e *Env) {
	t := c.Theme()
	ui.Row(c).Gap(10).Padding(10, 12).Radius(10).Background(kit.WarnColor.Alpha(0.10)).Border(1, kit.WarnColor.Alpha(0.35)).
		AlignItems(ui.Start).Children(func() {
		ui.Icon(c, kit.IconWarn).Size(16, 16).TextColor(kit.WarnColor)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, "logpose can't find the sim, so it can't change the sim's art.").FontSize(13).Wrap()
			if ui.ButtonBase(c).Label("Choose the sim's folder").Background(ui.Transparent).Padding(0).
				Children(func() { ui.Text(c, "Choose the sim's folder").FontSize(13).TextColor(t.Accent) }).Clicked() {
				if e.ArtAct != nil {
					e.ArtAct.ChooseSimFolder()
				}
			}
		})
	})
}

func SmallButton(c *ui.Context, label string, icon *ui.SVG, text bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Height(28).Padding(0, 9).Radius(7).Gap(6).AlignItems(ui.Center).
		Background(ui.Transparent).Border(1, t.Border)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Tooltip(label)
	kit.PressScale(b)
	b.Children(func() {
		ui.Icon(c, icon).Size(14, 14)
		if text {
			ui.Text(c, label).FontSize(12.5).FontWeight(500)
		}
	})
	return b
}

func ArtButtons(c *ui.Context, e *Env, key string) {
	if SmallButton(c, "Save image…", kit.IconSave, true).Clicked() && e.ArtAct != nil {
		e.ArtAct.SaveArt(key)
	}
	if SmallButton(c, "Copy image", kit.IconCopy, false).Clicked() && e.ArtAct != nil {
		e.ArtAct.CopyArt(key)
	}
}

func (e *Env) CardMenu(card cards.Card) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if e.Page != Cards && e.Nav != nil {
			if m.Item("Open in Cards").Chosen() {
				e.Nav.OpenInCards(card.Code)
			}
			if e.Nav.ShowsHands() && m.Item("Show hands with "+card.Name).Chosen() {
				e.Nav.ShowHands(card.Code)
			}
			m.Separator()
		}
		info, loaded := e.SimArt.For(e, card)
		m.Submenu("Art in the sim", func(m *ui.Menu) { e.artMenu(m, card, info, loaded) }).Disabled(loaded && info.NoSim)
		key := shownArtKey(e, card, info, loaded)
		if m.Item("View art").Chosen() {
			if key != card.Code {
				e.ViewArtAt(card.Code, key)
			} else {
				e.ViewArt(card.Code)
			}
		}
		if m.Item("Save image…").Chosen() && e.ArtAct != nil {
			e.ArtAct.SaveArt(key)
		}
		if m.Item("Copy image").Chosen() && e.ArtAct != nil {
			e.ArtAct.CopyArt(key)
		}
		m.Separator()
		if m.Item("Open on Limitless").Chosen() && e.ArtAct != nil {
			e.ArtAct.OpenURL(cards.LimitlessBase + "/cards/" + card.Code)
		}
	}
}

// shownArtKey is the art reference visible on the right-clicked card: the sim's override off the
// Cards page when mirroring is on, otherwise the base code.
func shownArtKey(e *Env, card cards.Card, info SimArtInfo, loaded bool) string {
	if e.Page == Cards || e.Prefs.SimArtOff || !loaded || info.NoSim {
		return card.Code
	}
	switch info.Cur.Kind {
	case sim.ArtPrint:
		if info.Cur.Key != "" {
			return info.Cur.Key
		}
	case sim.ArtFile:
		if info.OwnFile != "" {
			return arts.FileRef(info.OwnFile).String()
		}
	}
	return card.Code
}

func (e *Env) artMenu(m *ui.Menu, card cards.Card, info SimArtInfo, loaded bool) {
	ca, _ := e.Arts.For(e, card)
	cur := ""
	if loaded && info.Cur.Kind == sim.ArtPrint {
		cur = info.Cur.Key
	}
	own := false
	for _, en := range artEntries(e, card, ca, info, loaded, "") {
		switch {
		case en.choice.Kind == sim.ArtOwn:
			m.Separator()
			own = true
			if m.Item("The sim's own").Radio(en.inSim).Disabled(!en.usable).Chosen() {
				e.SimArt.Use(e, card, en.choice)
			}
		case en.art == nil:
			if m.Item(en.name).Radio(en.inSim).Disabled(!en.usable).Chosen() {
				e.SimArt.Use(e, card, en.choice)
			}
		case len(en.art.Versions) == 1:
			if m.Item(menuName(card, en.art.Head())).Radio(en.inSim).Disabled(!en.usable).Chosen() {
				e.SimArt.Use(e, card, en.choice)
			}
		default:
			art := *en.art
			m.Submenu(fmt.Sprintf("%s · %d versions", art.Name(), len(art.Versions)), func(m *ui.Menu) {
				shown, inArt := art.Find(cur)
				for _, ver := range art.Versions {
					key := ver.Key.String()
					on := inArt && shown.Key == ver.Key
					label := ver.Product
					if label == "" {
						label = ver.Name
					}
					if m.Item(label).Radio(on).Disabled(!en.usable).Chosen() {
						e.SimArt.Use(e, card, sim.ArtChoice{Kind: sim.ArtPrint, Key: key})
					}
				}
			}).Disabled(!en.usable)
		}
	}
	if !own {
		// Without the sim's state, the entry shows but can't be picked.
		m.Separator()
		m.Item("The sim's own").Disabled(true)
	}
}

// menuName adds the product to non-base printings so duplicate kinds can be told apart without pictures.
func menuName(card cards.Card, ver arts.Version) string {
	if ver.Key.String() == card.Code || ver.Product == "" {
		return ver.Name
	}
	return ver.Name + ", " + ver.Product
}
