package gui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/store"
)

// The parts of the Settings page an action can leave a note on.
const (
	noteSim     = "sim"
	noteLogs    = "logs"
	noteCards   = "cards"
	noteData    = "data"
	noteStartup = "startup"
	noteArt     = "art"
)

type settingsState struct {
	notes map[string]string
	// allNames shows every player in the You list, not only the top ones.
	allNames bool
	// art is nil until the card art folder is counted.
	art *artUsage
}

func (s *settingsState) note(part, text string) {
	if s.notes == nil {
		s.notes = make(map[string]string)
	}
	s.notes[part] = text
}

const cardDataHelp = "Cards come from Limitless (onepiece.limitlesstcg.com), which has every card, its " +
	"text and its art, and bans come from Bandai's list. logpose checks both once a day, and never " +
	"during a game. The OPBounty client inside the sim and tcgcsv.com fill in cards Limitless doesn't " +
	"have yet. Names in your logs win over all of them.\n\n" +
	"Refresh now checks again. Without a connection logpose keeps the cards it has."

// topNames is how many players the You list shows before "Show all".
const topNames = 3

func (v *view) settingsPage(c *ui.Context) {
	if pageEntered && v.snap.Settings.ArtDir != "" {
		v.do(func(a actions) { a.LoadArtUsage() })
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			ui.Text(c, "Settings").FontSize(20).FontWeight(600)
			ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
				v.simTile(c)
				v.logsTile(c)
				v.cardsTile(c)
				v.dataTile(c)
			})
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
					panel(c).Padding(14, 16).Gap(4).Children(func() { v.youSettings(c) })
					panel(c).Padding(14, 16).Gap(4).Children(func() { v.artSettings(c) })
				})
				panel(c).Grow(1).Basis(0).Padding(14, 16).Gap(4).Children(func() {
					v.lookSettings(c)
					ui.Box(c).Height(14)
					v.startupSettings(c)
				})
			})
		})
	})
}

// statusTile is one of the tiles across the top of Settings: a dot that's green when the part
// works, what logpose found, a note from the last action and the part's buttons.
func (v *view) statusTile(c *ui.Context, part, label string, ok bool, value, sub, help string, buttons func()) {
	t := c.Theme()
	panel(c).Grow(1).Basis(0).Padding(12, 14).Gap(2).Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			dot := winColor
			if !ok {
				dot = warnColor
			}
			ui.Box(c).Size(8, 8).Radius(4).Background(dot)
			ui.Text(c, label).FontSize(12).TextColor(faintColor(t))
			if help != "" {
				ui.Icon(c, iconInfo).Size(14, 14).TextColor(t.TextMuted).Label("About " + strings.ToLower(label)).Tooltip(help)
			}
		})
		ui.Text(c, value).FontSize(15).FontWeight(600).Margin(2, 0, 0, 0).SingleLine()
		ui.Text(c, sub).FontSize(12).TextColor(faintColor(t)).SingleLine().Tooltip(sub)
		if n := v.settings.notes[part]; n != "" {
			ui.Text(c, n).FontSize(12).TextColor(t.Accent).Wrap()
		}
		ui.Spacer(c)
		ui.Row(c).Gap(6).Wrap().Margin(10, 0, 0, 0).Children(buttons)
	})
}

func (v *view) simTile(c *ui.Context) {
	s := v.snap.Settings
	if s.SimDir == "" {
		v.statusTile(c, noteSim, "OPTCGSim", false, "Not found", "Start the sim, or choose its folder", "", func() {
			if ui.Button(c, "Choose folder…").Clicked() {
				v.do(func(a actions) { a.ChooseSimFolder() })
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
	v.statusTile(c, noteSim, "OPTCGSim", s.SimExe, value, sub, "", func() {
		if ui.Button(c, "Change…").Label("Change the sim's folder").Clicked() {
			v.do(func(a actions) { a.ChooseSimFolder() })
		}
		if ui.Button(c, "Open").Label("Open the sim's folder").Clicked() {
			v.do(func(a actions) { a.OpenPath(s.SimDir) })
		}
		if s.SimChosen && !s.SimFlag && ui.Button(c, "Find again").Label("Find the sim on its own").Clicked() {
			v.do(func(a actions) { a.FindSim() })
		}
	})
}

func (v *view) logsTile(c *ui.Context) {
	s := v.snap.Settings
	if s.SimDir == "" {
		v.statusTile(c, noteLogs, "Logs", false, "No logs", "They're in the sim's folder", "", func() {
			ui.Button(c, "Import again").Disabled(true)
		})
		return
	}
	v.statusTile(c, noteLogs, "Logs", true, fmt.Sprintf("%d in AutoSaved", s.Logs), "Watching for new games", "", func() {
		if ui.Button(c, "Import again").Label("Import every log again").Clicked() {
			v.do(func(a actions) { a.ImportAgain() })
		}
	})
}

func (v *view) cardsTile(c *ui.Context) {
	s := v.snap.Settings
	ok := !s.Fetched.IsZero() && v.snap.Now.Sub(s.Fetched) < 3*24*time.Hour
	sub := "Only the sim's card list"
	if !s.Fetched.IsZero() {
		sub = "Refreshed " + dayAndTime(v.snap.Now, s.Fetched)
	}
	v.statusTile(c, noteCards, "Card data", ok, fmt.Sprintf("%s cards", thousands(s.Cards)), sub, cardDataHelp, func() {
		if ui.Button(c, "Refresh now").Label("Refresh card data").Clicked() {
			v.do(func(a actions) { a.RefreshCards() })
		}
	})
}

func (v *view) dataTile(c *ui.Context) {
	s := v.snap.Settings
	sub := fmt.Sprintf("%.1f MB in %s", float64(s.DBSize)/(1<<20), s.DBFolder)
	v.statusTile(c, noteData, "Database", true, games(s.Games), sub, "", func() {
		if ui.Button(c, "Back up…").Label("Back up the database").Clicked() {
			v.do(func(a actions) { a.Backup() })
		}
		if ui.Button(c, "Show").Label("Show the database in its folder").Clicked() {
			v.do(func(a actions) { a.ShowFile(s.DBPath) })
		}
	})
}

func (v *view) youSettings(c *ui.Context) {
	t := c.Theme()
	s := &v.snap.Settings
	ui.Text(c, "You").FontSize(13).FontWeight(600)
	ui.Text(c, "Your games are the ones with these names. Pick more than one for an alt account.").FontSize(12).TextColor(t.TextMuted).Wrap()
	auto := len(s.You) == 0
	if switchRow(c, "Pick automatically", "The name in the most games", &auto, false) {
		switch {
		case auto:
			v.setYou(nil)
		case len(s.Players) > 0:
			v.setYou([]string{s.Players[0].ID})
		}
	}
	picked := s.You
	if auto && len(s.Players) > 0 {
		picked = []string{s.Players[0].ID}
	}
	rows := s.Players
	if !v.settings.allNames && len(rows) > topNames {
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
				ui.Text(c, games(p.Games)).FontSize(12).TextColor(faintColor(t))
			})
			switch {
			case auto || on == was:
			case on:
				v.setYou(append(slices.Clone(picked), p.ID))
			case len(picked) > 1:
				v.setYou(slices.DeleteFunc(slices.Clone(picked), func(n string) bool { return n == p.ID }))
			}
		}
	})
	if len(s.Players) > topNames {
		label := fmt.Sprintf("Show all %d names", len(s.Players))
		if v.settings.allNames {
			label = "Show fewer"
		}
		if ui.Text(c, label).FontSize(12).TextColor(t.Accent).Cursor(ui.CursorPointer).Clicked() {
			v.settings.allNames = !v.settings.allNames
		}
	}
}

// setYou saves the user's names. The page shows them before the new snapshot arrives.
func (v *view) setYou(names []string) {
	v.snap.Settings.You = names
	v.do(func(a actions) { a.SetSetting(store.SettingYou, strings.Join(names, "\n")) })
}

var themes = []string{"", "dark", "light"}

func (v *view) lookSettings(c *ui.Context) {
	ui.Text(c, "Appearance").FontSize(13).FontWeight(600)
	theme := max(slices.Index(themes, v.prefs.theme), 0)
	prefRow(c, "Theme", "", func() { ui.Segmented(c, &theme, "Windows", "Dark", "Light").Label("Theme") })
	if themes[theme] != v.prefs.theme {
		v.prefs.theme = themes[theme]
		v.do(func(a actions) { a.SetSetting(store.SettingTheme, themes[theme]) })
	}
	divider(c)
	cb := v.prefs.colorblind
	if switchRow(c, "Colorblind-safe colors", "Wins and losses in blue and orange instead of green and red", &cb, false) {
		v.prefs.colorblind = cb
		v.do(func(a actions) { a.SetSetting(store.SettingColorblind, flag(cb)) })
	}
}

func (v *view) startupSettings(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "Startup and tray").FontSize(13).FontWeight(600)
	login := v.prefs.openAtLogin
	if switchRow(c, "Start with Windows", "", &login, false) {
		v.prefs.openAtLogin = login
		v.do(func(a actions) { a.SetOpenAtLogin(login) })
	}
	divider(c)
	hidden := v.prefs.startHidden
	if switchRow(c, "Start hidden in the tray", "Only when it starts with Windows", &hidden, !v.prefs.openAtLogin) {
		v.prefs.startHidden = hidden
		v.do(func(a actions) { a.SetSetting(store.SettingStartHidden, flag(hidden)) })
	}
	divider(c)
	closing := 0
	if v.prefs.closeQuits {
		closing = 1
	}
	prefRow(c, "Closing the window", "", func() {
		ui.Segmented(c, &closing, "Hides it to the tray", "Quits logpose").Label("Closing the window")
	})
	if quits := closing == 1; quits != v.prefs.closeQuits {
		v.prefs.closeQuits = quits
		v.do(func(a actions) { a.SetSetting(store.SettingCloseQuits, flag(quits)) })
	}
	if n := v.settings.notes[noteStartup]; n != "" {
		ui.Text(c, n).FontSize(12).TextColor(t.Accent)
	}
}

func (v *view) artSettings(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "Card art").FontSize(13).FontWeight(600)
	ui.Text(c, "Pictures of cards come from Limitless and stay on this PC.").FontSize(12).TextColor(t.TextMuted).Wrap()
	on := !v.prefs.artOff
	if switchRow(c, "Download card art", "Off shows the sim's own images", &on, false) {
		v.prefs.artOff = !on
		v.do(func(a actions) { a.SetArtDownloads(on) })
	}
	dir := v.snap.Settings.ArtDir
	if dir == "" {
		return
	}
	divider(c)
	capIndex := slices.Index(artCaps, v.prefs.artCap)
	if capIndex < 0 {
		capIndex = slices.Index(artCaps, artCapDefault)
	}
	u := v.settings.art
	label, sub := "Counting…", ""
	if u != nil {
		label = megabytes(u.total()) + " on disk"
		sub = fmt.Sprintf("Thumbnails %s · large images %s of %d MB", megabytes(u.ThumbBytes), megabytes(u.FullBytes), artCaps[capIndex])
	}
	prefRow(c, label, sub, func() {
		ui.Row(c).Gap(6).Children(func() {
			if ui.Button(c, "Show").Label("Show the card art folder").Clicked() {
				v.do(func(a actions) { a.OpenPath(dir) })
			}
			if ui.Button(c, "Clear").Label("Clear card art").Disabled(u == nil || u.total() == 0).Clicked() {
				v.do(func(a actions) { a.ClearArt() })
			}
		})
	})
	if n := v.settings.notes[noteArt]; n != "" {
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
		mb := artCaps[picked]
		v.prefs.artCap = mb
		v.do(func(a actions) { a.SetArtCap(mb) })
	}
	ui.Text(c, "Past this, the ones you opened longest ago are deleted. Thumbnails stay.").
		FontSize(12).TextColor(t.TextMuted).Wrap()
}

var artCapLabels = func() []string {
	var out []string
	for _, mb := range artCaps {
		out = append(out, fmt.Sprintf("%d MB", mb))
	}
	return out
}()

// megabytes is a size such as "8.4 MB" or "258 MB".
func megabytes(n int64) string {
	mb := float64(n) / (1 << 20)
	if n == 0 {
		return "0 MB"
	}
	if mb < 10 {
		return fmt.Sprintf("%.1f MB", mb)
	}
	return fmt.Sprintf("%.0f MB", mb)
}

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
				ui.Text(c, sub).FontSize(12).TextColor(faintColor(t)).Wrap()
			}
		})
		control()
	})
}

func divider(c *ui.Context) {
	ui.Box(c).Height(1).Background(c.Theme().Border.Alpha(0.5))
}

// setupPage asks for the sim's folder when logpose can't find the sim and has no games yet.
func (v *view) setupPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Padding(32).Children(func() {
		panel(c).Width(560).Padding(24, 26).Gap(8).Children(func() {
			ui.Text(c, "Where is OPTCGSim?").FontSize(18).Bold()
			ui.Text(c, "logpose reads the games the sim saves. Start the sim and logpose finds it, or choose the folder with OPTCGSim.exe in it.").
				TextColor(t.TextMuted).Wrap()
			ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(8, 0, 0, 0).Children(func() {
				if ui.PrimaryButton(c, "Choose folder…").Clicked() {
					v.do(func(a actions) { a.ChooseSimFolder() })
				}
				ui.Box(c).Size(8, 8).Radius(4).Background(t.Accent.Alpha(0.8))
				ui.Text(c, "Looking for a running sim").FontSize(13).TextColor(faintColor(t))
			})
			if n := v.settings.notes[noteSim]; n != "" {
				ui.Text(c, n).FontSize(13).TextColor(warnColor).Wrap()
			}
			ui.Box(c).Height(1).Margin(10, 0, 4, 0).Background(t.Border)
			ui.Text(c, `Checked Program Files, Program Files (x86) and AppData\Local. The folder you pick is remembered.`).
				FontSize(12).TextColor(faintColor(t)).Wrap()
		})
	})
}

// dayAndTime is a past moment as "today at 18:50", "yesterday at 18:50" or "Nov 6 at 18:50".
func dayAndTime(now, then time.Time) string {
	day := then.Format("Jan 2")
	switch dayStart(then) {
	case dayStart(now):
		day = "today"
	case dayStart(now.AddDate(0, 0, -1)):
		day = "yesterday"
	}
	return day + " at " + then.Format("15:04")
}

// thousands writes n with comma separators, such as 2,926.
func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
