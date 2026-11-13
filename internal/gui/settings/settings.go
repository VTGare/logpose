package settings

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/shortcut"
	"github.com/VTGare/logpose/internal/store"
)

// The parts of the Settings page an action can leave a note on.
const (
	NoteSim     = "sim"
	NoteLogs    = "logs"
	NoteCards   = "cards"
	NoteData    = "data"
	NoteStartup = "startup"
	NoteArt     = "art"
)

type Actions interface {
	CheckUpdate()
	InstallUpdate()
	RestartToUpdate()
	SetSetting(key, value string)
	SetOpenAtLogin(on bool)
	SetShortcut(place shortcut.Place, on bool)
	// ChooseSimFolder asks for the sim's folder and switches to it.
	ChooseSimFolder()
	// FindSim forgets the chosen folder and finds the sim on its own.
	FindSim()
	OpenPath(path string)
	ShowFile(path string)
	// ImportAgain reads every log in the sim's folder and adds what's missing.
	ImportAgain()
	RefreshCards()
	Backup()
	// LoadSettings reads what the Settings page shows and shows it with view.setSettingsInfo.
	LoadSettings()
	LoadArtUsage()
	ClearArt()
}

type Page struct {
	Act Actions

	// System is what Windows says about logpose's startup and shortcuts.
	System System
	upd    UpdateState
	// updNotes shows the next version's release notes.
	updNotes bool

	Notes map[string]string
	// allNames shows every player in the You list, not only the top ones.
	allNames bool
	// Art is nil until the card Art folder is counted.
	Art *arts.Usage
	// offerSkip holds the places unticked in the offer of shortcuts.
	offerSkip [2]bool
	// frame is the view's frame the page last showed in.
	frame int
	// Info is nil until the app first reads it, and infoFor is the snapshot it was asked for on.
	Info    *Info
	infoFor *page.Snapshot
}

func (pg *Page) do(fn func(Actions)) {
	if pg.Act != nil {
		fn(pg.Act)
	}
}

func (pg *Page) Note(part, text string) {
	if pg.Notes == nil {
		pg.Notes = make(map[string]string)
	}
	pg.Notes[part] = text
}

func (pg *Page) NoteOn(part string) string { return pg.Notes[part] }

const cardDataHelp = "Cards come from Limitless (onepiece.limitlesstcg.com), which has every card, its " +
	"text and its art, and bans come from Bandai's list. logpose checks both once a day, and never " +
	"during a game. The OPBounty client inside the sim and tcgcsv.com fill in cards Limitless doesn't " +
	"have yet. Names in your logs win over all of them.\n\n" +
	"Refresh now checks again. Without a connection logpose keeps the cards it has."

// topNames is how many players the You list shows before "Show all".
const topNames = 3

func (pg *Page) Draw(c *ui.Context, e *page.Env) {
	if pg.frame != e.Frame-1 {
		pg.do(func(a Actions) { a.LoadArtUsage() })
	}
	pg.frame = e.Frame
	// A new snapshot can mean new games, logs or card data, so the page reads them again.
	if pg.infoFor != e.Snap {
		pg.infoFor = e.Snap
		pg.do(func(a Actions) { a.LoadSettings() })
	}
	if pg.Info == nil {
		return
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			ui.Text(c, "Settings").FontSize(20).FontWeight(600)
			ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
				pg.simTile(c)
				pg.logsTile(c)
				pg.cardsTile(c, e)
				pg.dataTile(c, e)
			})
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
					kit.Panel(c).Padding(14, 16).Gap(4).Children(func() { pg.youSettings(c) })
					kit.Panel(c).Padding(14, 16).Gap(4).Children(func() { pg.artSettings(c, e) })
				})
				kit.Panel(c).Grow(1).Basis(0).Padding(14, 16).Gap(4).Children(func() {
					lookSettings(c, e)
					ui.Box(c).Height(14)
					pg.startupSettings(c, e)
					ui.Box(c).Height(14)
					pg.updateSettings(c, e)
				})
			})
		})
	})
}

// statusTile is one of the tiles across the top of Settings: a dot that's green when the part
// works, what logpose found, a note from the last action and the part's buttons.
func (pg *Page) statusTile(c *ui.Context, part, label string, ok bool, value, sub, help string, buttons func()) {
	t := c.Theme()
	kit.Panel(c).Grow(1).Basis(0).Padding(12, 14).Gap(2).Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			dot := kit.WinColor(t)
			if !ok {
				dot = kit.WarnColor
			}
			ui.Box(c).Size(8, 8).Radius(4).Background(dot)
			ui.Text(c, label).FontSize(12).TextColor(kit.FaintColor(t))
			if help != "" {
				ui.Icon(c, kit.IconInfo).Size(14, 14).TextColor(t.TextMuted).Label("About " + strings.ToLower(label)).Tooltip(help)
			}
		})
		ui.Text(c, value).FontSize(15).FontWeight(600).Margin(2, 0, 0, 0).SingleLine()
		ui.Text(c, sub).FontSize(12).TextColor(kit.FaintColor(t)).SingleLine().Tooltip(sub)
		if n := pg.Notes[part]; n != "" {
			ui.Text(c, n).FontSize(12).TextColor(t.Accent).Wrap()
		}
		ui.Spacer(c)
		ui.Row(c).Gap(6).Wrap().Margin(10, 0, 0, 0).Children(buttons)
	})
}

func (pg *Page) simTile(c *ui.Context) {
	s := pg.Info
	if s.SimDir == "" {
		pg.statusTile(c, NoteSim, "OPTCGSim", false, "Not found", "Start the sim, or choose its folder", "", func() {
			if ui.Button(c, "Choose folder…").Clicked() {
				pg.do(func(a Actions) { a.ChooseSimFolder() })
			}
		})
		return
	}
	value := "Found"
	if s.SimVersion != "" {
		value = "Version " + s.SimVersion
	}
	sub := s.SimDir
	switch {
	case !s.SimExe:
		sub = "No OPTCGSim.exe in " + s.SimDir
	case s.SimFlag:
		sub += " · set with -sim"
	case s.SimChosen:
		sub += " · chosen by you"
	}
	pg.statusTile(c, NoteSim, "OPTCGSim", s.SimExe, value, sub, "", func() {
		if ui.Button(c, "Change…").Label("Change the sim's folder").Clicked() {
			pg.do(func(a Actions) { a.ChooseSimFolder() })
		}
		if ui.Button(c, "Open").Label("Open the sim's folder").Clicked() {
			pg.do(func(a Actions) { a.OpenPath(s.SimDir) })
		}
		if s.SimChosen && !s.SimFlag && ui.Button(c, "Find again").Label("Find the sim on its own").Clicked() {
			pg.do(func(a Actions) { a.FindSim() })
		}
	})
}

func (pg *Page) logsTile(c *ui.Context) {
	s := pg.Info
	if s.SimDir == "" {
		pg.statusTile(c, NoteLogs, "Logs", false, "No logs", "They're in the sim's folder", "", func() {
			ui.Button(c, "Import again").Disabled(true)
		})
		return
	}
	pg.statusTile(c, NoteLogs, "Logs", true, fmt.Sprintf("%d in AutoSaved", s.Logs), "Watching for new games", "", func() {
		if ui.Button(c, "Import again").Label("Import every log again").Clicked() {
			pg.do(func(a Actions) { a.ImportAgain() })
		}
	})
}

func (pg *Page) cardsTile(c *ui.Context, e *page.Env) {
	s := pg.Info
	// Card data counts as current until three of its daily checks were missed.
	ok := !s.Fetched.IsZero() && e.Now().Sub(s.Fetched) < 3*cards.LibraryMaxAge
	count := 0
	if e.Snap.Cards != nil {
		count = e.Snap.Cards.Len()
	}
	sub := "Only the sim's card list"
	if !s.Fetched.IsZero() {
		sub = "Refreshed " + kit.DayAndTime(e.Now(), s.Fetched)
	}
	pg.statusTile(c, NoteCards, "Card data", ok, fmt.Sprintf("%s cards", kit.Thousands(count)), sub, cardDataHelp, func() {
		if ui.Button(c, "Refresh now").Label("Refresh card data").Clicked() {
			pg.do(func(a Actions) { a.RefreshCards() })
		}
	})
}

func (pg *Page) dataTile(c *ui.Context, e *page.Env) {
	s := pg.Info
	sub := kit.Megabytes(s.DBSize) + " in " + s.DBFolder
	pg.statusTile(c, NoteData, "Database", true, kit.Games(e.Snap.Stored), sub, "", func() {
		if ui.Button(c, "Back up…").Label("Back up the database").Clicked() {
			pg.do(func(a Actions) { a.Backup() })
		}
		if ui.Button(c, "Show").Label("Show the database in its folder").Clicked() {
			pg.do(func(a Actions) { a.ShowFile(s.DBPath) })
		}
	})
}

func (pg *Page) youSettings(c *ui.Context) {
	t := c.Theme()
	s := pg.Info
	ui.Text(c, "You").FontSize(13).FontWeight(600)
	ui.Text(c, "Your games are the ones with these names. Pick more than one for an alt account.").FontSize(12).TextColor(t.TextMuted).Wrap()
	auto := len(s.You) == 0
	if switchRow(c, "Pick automatically", "The name in the most games", &auto, false) {
		switch {
		case auto:
			pg.setYou(nil)
		case len(s.Players) > 0:
			pg.setYou([]string{s.Players[0].ID})
		}
	}
	picked := s.You
	if auto && len(s.Players) > 0 {
		picked = []string{s.Players[0].ID}
	}
	rows := s.Players
	if !pg.allNames && len(rows) > topNames {
		rows = slices.Clone(rows[:topNames])
		for _, p := range s.Players[topNames:] {
			if slices.Contains(picked, p.ID) {
				rows = append(rows, p)
			}
		}
	}
	list := ui.Column(c).Gap(2)
	if auto {
		list.Opacity(0.5)
	}
	list.Children(func() {
		for _, p := range rows {
			on := slices.Contains(picked, p.ID)
			was := on
			ui.Row(c.Key(p.ID)).Gap(10).AlignItems(ui.Center).Padding(3, 0).Children(func() {
				ui.Checkbox(c, &on, p.ID).Disabled(auto)
				ui.Spacer(c)
				ui.Text(c, kit.Games(p.Games)).FontSize(12).TextColor(kit.FaintColor(t))
			})
			switch {
			case auto || on == was:
			case on:
				pg.setYou(append(slices.Clone(picked), p.ID))
			case len(picked) > 1:
				pg.setYou(slices.DeleteFunc(slices.Clone(picked), func(n string) bool { return n == p.ID }))
			}
		}
	})
	if len(s.Players) > topNames {
		label := fmt.Sprintf("Show all %d names", len(s.Players))
		if pg.allNames {
			label = "Show fewer"
		}
		if ui.Text(c, label).FontSize(12).TextColor(t.Accent).Cursor(ui.CursorPointer).Clicked() {
			pg.allNames = !pg.allNames
		}
	}
}

// setYou saves the user's names. The page shows them before the new snapshot arrives.
func (pg *Page) setYou(names []string) {
	pg.Info.You = names
	pg.do(func(a Actions) { a.SetSetting(store.SettingYou, store.JoinNames(names)) })
}

var themes = []string{"", "dark", "light"}

func lookSettings(c *ui.Context, e *page.Env) {
	ui.Text(c, "Appearance").FontSize(13).FontWeight(600)
	theme := max(slices.Index(themes, e.Prefs.Theme), 0)
	prefRow(c, "Theme", "", func() { ui.Segmented(c, &theme, "Windows", "Dark", "Light").Label("Theme") })
	if themes[theme] != e.Prefs.Theme {
		page.PrefTheme.Set(e, themes[theme])
	}
	divider(c)
	cb := e.Prefs.Colorblind
	if switchRow(c, "Colorblind-safe colors", "Wins and losses in blue and orange instead of green and red", &cb, false) {
		page.PrefColorblind.Set(e, cb)
	}
}

func (pg *Page) startupSettings(c *ui.Context, e *page.Env) {
	t := c.Theme()
	ui.Text(c, "Startup and tray").FontSize(13).FontWeight(600)
	login := pg.System.OpenAtLogin
	if switchRow(c, "Start with Windows", "", &login, false) {
		pg.System.OpenAtLogin = login
		pg.do(func(a Actions) { a.SetOpenAtLogin(login) })
	}
	pg.shortcutRows(c, e)
	divider(c)
	hidden := e.Prefs.StartHidden
	if switchRow(c, "Start hidden in the tray", "Only when it starts with Windows", &hidden, !pg.System.OpenAtLogin) {
		page.PrefStartHidden.Set(e, hidden)
	}
	divider(c)
	closing := 0
	if e.Prefs.CloseQuits {
		closing = 1
	}
	prefRow(c, "Closing the window", "", func() {
		ui.Segmented(c, &closing, "Hides it to the tray", "Quits logpose").Label("Closing the window")
	})
	if quits := closing == 1; quits != e.Prefs.CloseQuits {
		page.PrefCloseQuits.Set(e, quits)
	}
	if n := pg.Notes[NoteStartup]; n != "" {
		ui.Text(c, n).FontSize(12).TextColor(t.Accent)
	}
}

func (pg *Page) artSettings(c *ui.Context, e *page.Env) {
	t := c.Theme()
	ui.Text(c, "Card art").FontSize(13).FontWeight(600)
	ui.Text(c, "Pictures of cards come from Limitless and stay on this PC.").FontSize(12).TextColor(t.TextMuted).Wrap()
	on := !e.Prefs.ArtOff
	if switchRow(c, "Download card art", "Off shows the sim's own images", &on, false) {
		page.PrefArtOff.Set(e, !on)
	}
	dir := pg.Info.ArtDir
	if dir == "" {
		return
	}
	divider(c)
	capIndex := slices.Index(page.ArtCaps, e.Prefs.ArtCap)
	u := pg.Art
	label, sub := "Counting…", ""
	if u != nil {
		label = kit.Megabytes(u.Total()) + " on disk"
		sub = fmt.Sprintf("Thumbnails %s · large images %s of %d MB", kit.Megabytes(u.ThumbBytes), kit.Megabytes(u.FullBytes), page.ArtCaps[capIndex])
	}
	prefRow(c, label, sub, func() {
		ui.Row(c).Gap(6).Children(func() {
			if ui.Button(c, "Show").Label("Show the card art folder").Clicked() {
				pg.do(func(a Actions) { a.OpenPath(dir) })
			}
			if ui.Button(c, "Clear").Label("Clear card art").Disabled(u == nil || u.Total() == 0).Clicked() {
				pg.do(func(a Actions) { a.ClearArt() })
			}
		})
	})
	if n := pg.Notes[NoteArt]; n != "" {
		ui.Text(c, n).FontSize(12).TextColor(t.Accent)
	}
	divider(c)
	picked := capIndex
	// The four sizes leave too little room beside them for a label, so it sits above.
	ui.Column(c).Gap(8).Padding(6, 0).AlignItems(ui.Start).Children(func() {
		ui.Text(c, "Keep large images up to").FontWeight(500)
		ui.Segmented(c, &picked, artCapLabels...).Label("Keep large images up to")
	})
	if picked != capIndex {
		page.PrefArtCap.Set(e, page.ArtCaps[picked])
	}
	ui.Text(c, "Past this, the ones you opened longest ago are deleted. Thumbnails stay.").
		FontSize(12).TextColor(t.TextMuted).Wrap()
}

var artCapLabels = func() []string {
	var out []string
	for _, mb := range page.ArtCaps {
		out = append(out, fmt.Sprintf("%d MB", mb))
	}
	return out
}()

// switchRow is prefRow with a switch, where a click anywhere on the row flips it. It reports
// whether the switch changed.
func switchRow(c *ui.Context, label, sub string, on *bool, disabled bool) bool {
	was := *on
	row := prefRowElement(c, label, sub, func() { ui.Switch(c, on).Label(label).Disabled(disabled) })
	if !disabled {
		row.Cursor(ui.CursorPointer)
		if row.Clicked() {
			*on = !*on
		}
	}
	return *on != was
}

// prefRow is a setting's name, with an optional line under it, and its control at the right.
func prefRow(c *ui.Context, label, sub string, control func()) {
	prefRowElement(c, label, sub, control)
}

func prefRowElement(c *ui.Context, label, sub string, control func()) ui.Element {
	t := c.Theme()
	return ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(6, 0).Children(func() {
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			ui.Text(c, label).FontWeight(500)
			if sub != "" {
				ui.Text(c, sub).FontSize(12).TextColor(kit.FaintColor(t)).Wrap()
			}
		})
		control()
	})
}

func divider(c *ui.Context) {
	ui.Box(c).Height(1).Background(c.Theme().Border.Alpha(0.5))
}

// SetupPage asks for the sim's folder when logpose can't find the sim and has no games yet.
func (pg *Page) SetupPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Padding(32).Children(func() {
		kit.Panel(c).Width(560).Padding(24, 26).Gap(8).Children(func() {
			ui.Text(c, "Where is OPTCGSim?").FontSize(18).Bold()
			ui.Text(c, "logpose reads the games the sim saves. Start the sim and logpose finds it, or choose the folder with OPTCGSim.exe in it.").
				TextColor(t.TextMuted).Wrap()
			ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(8, 0, 0, 0).Children(func() {
				if ui.PrimaryButton(c, "Choose folder…").Clicked() {
					pg.do(func(a Actions) { a.ChooseSimFolder() })
				}
				ui.Box(c).Size(8, 8).Radius(4).Background(t.Accent.Alpha(0.8))
				ui.Text(c, "Looking for a running sim").FontSize(13).TextColor(kit.FaintColor(t))
			})
			if n := pg.Notes[NoteSim]; n != "" {
				ui.Text(c, n).FontSize(13).TextColor(kit.WarnColor).Wrap()
			}
			ui.Box(c).Height(1).Margin(10, 0, 4, 0).Background(t.Border)
			ui.Text(c, `Checked Program Files, Program Files (x86) and AppData\Local. The folder you pick is remembered.`).
				FontSize(12).TextColor(kit.FaintColor(t)).Wrap()
		})
	})
}

// Info is what the Settings page shows about the sim, the logs, the card data and the
// database. The app reads it when the page asks.
type Info struct {
	// SimDir is empty when the sim isn't found.
	SimDir     string
	SimChosen  bool
	SimFlag    bool
	SimVersion string
	SimExe     bool
	Logs       int

	Fetched time.Time

	DBPath string
	// ArtDir is the card art folder, empty when there's none.
	ArtDir string
	// DBFolder is the database's folder, short for the page.
	DBFolder string
	DBSize   int64

	Players []store.PlayerCount
	// You is the names the user picked, empty when logpose picks the name in the most games.
	You []string
}

// System is what Windows says about logpose, which isn't stored.
type System struct {
	OpenAtLogin bool
	Shortcuts   [2]bool
	// CanShortcut is false where shortcuts can't be made.
	CanShortcut bool
	// Installed is true when logpose runs from its installer's folder, which has its shortcuts.
	Installed bool
}
