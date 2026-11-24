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

// simArtToastTime is how long the message after a change shows, with its Undo.
const simArtToastTime = 10 * time.Second

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
		if e.Act != nil {
			e.Act.LoadSimArt(card)
		}
	}
	return SimArtInfo{}, false
}

func (s *SimArtState) Scan(e *Env) {
	if s.scanned {
		return
	}
	s.scanned = true
	if e.Act != nil {
		e.Act.ScanSimArt()
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
	if e.Act != nil {
		e.Act.UseSimArt(card, choice, prev)
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
		t.Title = "The sim now shows " + card.Name + "'s " + ChoiceName(ca, choice) + "."
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
		if e.Act != nil {
			e.Act.LoadArts(card, gen)
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

// ChoiceName names a choice for prose, such as "Alt art", "Base art", "own file" or "own art".
func ChoiceName(ca arts.CardArts, choice sim.ArtChoice) string {
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

// ArtEntry is one art choice for a card. Art is nil for the user's file and the sim's own picture.
type ArtEntry struct {
	Key, Name, Note string
	Art             *arts.Art
	Choice          sim.ArtChoice
	InSim, Usable   bool
}

// ArtEntries lists a card's pictures in picker order: its arts, the user's file and the sim's own.
// An art's key is the version being viewed or its head, and its choice puts that version in the sim.
// The viewer, the Art tab, the art menu, the deck picker and the tab count all read this list.
func ArtEntries(e *Env, card cards.Card, ca arts.CardArts, info SimArtInfo, loaded bool, viewed string) []ArtEntry {
	usable := loaded && !info.NoSim && !info.busy
	var out []ArtEntry
	for i, art := range ca.Arts {
		ver := art.Head()
		if viewed != "" {
			if shown, ok := art.Find(viewed); ok {
				ver = shown
			}
		}
		key := ver.Key.String()
		out = append(out, ArtEntry{
			Key: key, Name: art.Name(), Note: ver.Product, Art: &ca.Arts[i],
			Choice: sim.ArtChoice{Kind: sim.ArtPrint, Key: key},
			InSim:  loaded && info.Cur.Kind == sim.ArtPrint && art.Has(info.Cur.Key),
			Usable: usable && !e.Prefs.ArtOff,
		})
	}
	if info.OwnFile != "" {
		out = append(out, ArtEntry{
			Key: FileRef(info.OwnFile), Name: "Your file", Note: "A file you added to the sim",
			Choice: sim.ArtChoice{Kind: sim.ArtFile}, InSim: info.Cur.Kind == sim.ArtFile, Usable: usable,
		})
	}
	if loaded && !info.NoSim {
		out = append(out, ArtEntry{
			Key: SimOwnRef(card.Code), Name: "Sim's own", Note: "The sim's picture",
			Choice: sim.ArtChoice{Kind: sim.ArtOwn}, InSim: info.Cur.Kind == sim.ArtOwn, Usable: usable,
		})
	}
	return out
}

type ArtColumnOpts struct {
	Key  string
	Name string
	// Subject prefixes accessible button labels when several cards share one page.
	Subject  string
	Versions int
	Viewed   bool
	InSim    bool
	Usable   bool
	// Compact hides the dot's text label and borders the active thumbnail in gold.
	Compact bool
}

// ArtColumn renders one art thumbnail with its label and sim radio dot.
func ArtColumn(c *ui.Context, e *Env, o ArtColumnOpts) (col ui.Element, look, use bool) {
	t := c.Theme()
	const w = 60
	target := o.Name
	if o.Subject != "" {
		target = o.Subject + " " + o.Name
	}
	col = ui.Column(c.Key(o.Key)).Gap(4).MinWidth(w).AlignItems(ui.Center).Children(func() {
		label := "Show " + target
		if o.Versions > 1 {
			label = fmt.Sprintf("Show %s, %d versions", target, o.Versions)
		}
		pic := ui.ButtonBase(c).Label(label).Background(ui.Transparent).Padding(0).Radius(6).FocusRing(false)
		if o.Compact && o.InSim {
			pic.Border(1.5, SimGold)
		}
		look = pic.Clicked()
		pic.Children(func() {
			e.CardImage(c, ExactPrintRef(o.Key), w).Radius(6)
			if o.Versions > 1 {
				ui.Box(c).Absolute().Top(-6).Right(-6).Size(20, 20).Radius(10).Background(ui.RGBA(20, 20, 22, 0.92)).
					Border(1, ui.RGBA(255, 255, 255, 0.3)).Center().Children(func() {
					ui.Text(c, fmt.Sprint(o.Versions)).FontSize(11).FontWeight(600).TextColor(ui.Hex("#FFFFFF"))
				})
			}
		})
		if !o.Compact {
			bar := ui.Box(c).Height(3).Width(w).Radius(2)
			if o.Viewed {
				bar.Background(t.Accent)
			}
		}
		tc := t.TextMuted
		weight := 400
		if o.Viewed || (o.Compact && o.InSim) {
			tc = t.Text
		}
		if o.Compact && o.InSim {
			weight = 600
		}
		ui.Text(c, o.Name).FontSize(11).FontWeight(weight).TextColor(tc).MaxLines(1)
		use = UseDot(c, target, o.InSim, o.Usable, !o.Compact)
	})
	return col, look, use
}

func UseDot(c *ui.Context, name string, on, usable, showText bool) bool {
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

// PendingColumn stands for printings whose pictures are still being compared. They often turn out
// to be one art, so they take one column.
func PendingColumn(c *ui.Context, e *Env, card cards.Card, n int, compact bool) {
	t := c.Theme()
	const w = 60
	ui.Column(c.Key("pending")).Gap(4).Width(w).AlignItems(ui.Center).Children(func() {
		e.CardPlaceholder(c, card.Code, w, CardHeight(w)).Radius(6)
		if !compact {
			// Reserves the height of ArtColumn's accent bar so captions line up.
			ui.Box(c).Height(3)
		}
		ui.Text(c, fmt.Sprintf("Checking %d more", n)).FontSize(11).TextColor(t.TextMuted).Wrap().TextAlign(ui.Center)
	})
}

func VersionPopover(
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
						e.CardImage(c, ExactPrintRef(key), 30).Radius(6)
						ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
							col := t.Text
							if viewed == key {
								col = t.Accent
							}
							ui.Text(c, ver.Short).FontSize(13).FontWeight(500).TextColor(col).MaxLines(1).Ellipsis("…")
							ui.Text(c, ver.Product).FontSize(12).TextColor(t.TextMuted).MaxLines(2).Ellipsis("…")
						})
					})
					if UseDot(c, ver.Short+", "+ver.Product, on, usable, true) && onUse != nil {
						onUse(sim.ArtChoice{Kind: sim.ArtPrint, Key: key})
					}
				})
			}
		})
	})
}

func NoSimBanner(c *ui.Context, e *Env) {
	t := c.Theme()
	ui.Row(c).Gap(10).Padding(10, 12).Radius(10).Background(kit.WarnColor.Alpha(0.10)).Border(1, kit.WarnColor.Alpha(0.35)).
		AlignItems(ui.Start).Children(func() {
		ui.Icon(c, kit.IconWarn).Size(16, 16).TextColor(kit.WarnColor)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, "logpose can't find the sim, so it can't change the sim's art.").FontSize(13).Wrap()
			if ui.ButtonBase(c).Label("Choose the sim's folder").Background(ui.Transparent).Padding(0).
				Children(func() { ui.Text(c, "Choose the sim's folder").FontSize(13).TextColor(t.Accent) }).Clicked() {
				if e.Act != nil {
					e.Act.ChooseSimFolder()
				}
			}
		})
	})
}
