package settings

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"
)

type updatePhase int

const (
	// UpdateIdle is up to date, or not checked yet.
	UpdateIdle updatePhase = iota
	UpdateChecking
	// UpdateFound is a newer version that waits for the user to download it.
	UpdateFound
	UpdateDownloading
	// UpdateReady is a newer version that's installed and runs once logpose starts again.
	UpdateReady
	UpdateFailed
)

type UpdateState struct {
	// Can is false for copies that Can't replace themselves, such as dev builds.
	Can     bool
	Current string
	Phase   updatePhase
	Next    string
	// Notes are the next version's release Notes, in Markdown.
	Notes   string
	Done    float32
	Checked time.Time
	Err     string
}

// pending is true while a newer version is found, downloading or ready.
func (u UpdateState) pending() bool {
	return u.Phase == UpdateFound || u.Phase == UpdateDownloading || u.Phase == UpdateReady
}

// Busy is true while a check can't start: one is running, or a version is downloading or ready.
func (u UpdateState) Busy() bool {
	return u.Phase == UpdateChecking || u.Phase == UpdateDownloading || u.Phase == UpdateReady
}

func (u UpdateState) getting() string {
	return fmt.Sprintf("Getting %s · %d%%", u.Next, int(u.Done*100))
}

func (pg *Page) SetUpdate(u UpdateState) {
	if u.Phase == UpdateReady && pg.upd.Phase != UpdateReady {
		pg.updNotes = false
	}
	pg.upd = u
}

// UpdateBox sits above Formats and Settings in the sidebar while a newer version is found or ready. The
// rail shows it as an icon.
func (pg *Page) UpdateBox(c *ui.Context, wide bool) {
	u := pg.upd
	if !u.pending() {
		return
	}
	t := c.Theme()
	title, action := u.Next+" is ready", "Restart"
	icon := kit.IconRestart
	switch u.Phase {
	case UpdateFound:
		title, action, icon = u.Next+" is out", "Get it", kit.IconSave
	case UpdateDownloading:
		title, action, icon = u.getting(), "", kit.IconSave
	}
	run := func() {
		switch u.Phase {
		case UpdateReady:
			pg.do(func(a Actions) { a.RestartToUpdate() })
		case UpdateFound:
			pg.upd.Phase = UpdateDownloading
			pg.do(func(a Actions) { a.InstallUpdate() })
		}
	}
	if !wide {
		// Tinted with the accent so it reads as an action, not as the current page's grey fill.
		b := ui.ButtonBase(c).Label(title).Width(44).Height(40).Radius(8).Center().Background(railUpdateTint(t, false)).
			Tooltip(title + ". " + updateHint(u.Phase))
		if b.Hovered() {
			b.Background(railUpdateTint(t, true))
		}
		if b.Clicked() {
			run()
		}
		b.Children(func() { ui.Icon(c, icon).Size(18, 18).TextColor(t.Accent) })
		return
	}
	box := ui.Column(c).Gap(6).Padding(10, 12).Radius(10).Background(t.Surface).Border(1, t.Border)
	box.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(t.Accent)
			ui.Text(c, title).FontSize(13).FontWeight(600).SingleLine().Grow(1).Basis(0).MinWidth(0)
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(0, 0, 0, 16).Children(func() {
			info := ui.ButtonBase(c).Label("What's new in " + u.Next).Background(ui.Transparent)
			if info.Clicked() {
				pg.updNotes = !pg.updNotes
			}
			col := t.TextMuted
			if info.Hovered() || pg.updNotes {
				col = t.Text
			}
			info.Children(func() { ui.Text(c, "What's new").FontSize(12).TextColor(col) })
			ui.Spacer(c)
			if action != "" {
				b := ui.PrimaryButton(c, action).Padding(4, 12).FontSize(12)
				if b.Clicked() {
					run()
				}
			}
		})
	})
	pg.updateNotes(c, box)
}

func railUpdateTint(t *ui.Theme, hovered bool) ui.Color {
	if hovered {
		return t.Accent.Alpha(0.26)
	}
	return t.Accent.Alpha(0.16)
}

func updateHint(p updatePhase) string {
	switch p {
	case UpdateReady:
		return "Click to restart into it."
	case UpdateFound:
		return "Click to download it."
	}
	return ""
}

// updateNotes is the next version's release notes, under the sidebar's box.
func (pg *Page) updateNotes(c *ui.Context, anchor ui.Element) {
	t := c.Theme()
	u := pg.upd
	ui.PopoverBase(c, anchor, &pg.updNotes, func(panel ui.Element) {
		kit.Popover(panel.Margin(0, 0, 6, 0).Padding(14, 16).Width(420), t)
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "logpose "+u.Next).FontWeight(600)
			ui.Text(c, "You have "+u.Current+".").FontSize(12).TextColor(t.TextMuted)
			ui.Box(c).Height(4)
			for _, l := range noteLines(u.Notes) {
				if !l.bullet {
					ui.Text(c, l.text).FontSize(13).Wrap()
					continue
				}
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, "•").FontSize(13).TextColor(t.TextMuted)
					ui.Text(c, l.text).FontSize(13).Wrap().Grow(1).Basis(0).MinWidth(0)
				})
			}
		})
	})
}

type noteLine struct {
	text   string
	bullet bool
}

// noteLines reads the Markdown of release notes as paragraphs and bullets, which is all
// CHANGELOG.md uses. Emphasis and code marks are dropped.
func noteLines(md string) []noteLine {
	var out []noteLine
	strip := strings.NewReplacer("**", "", "__", "", "`", "")
	for _, l := range strings.Split(md, "\n") {
		l = strings.TrimSpace(l)
		switch {
		case l == "", strings.HasPrefix(l, "#"):
		case strings.HasPrefix(l, "- "), strings.HasPrefix(l, "* "):
			out = append(out, noteLine{text: strip.Replace(l[2:]), bullet: true})
		case len(out) > 0 && out[len(out)-1].text != "":
			// A wrapped line goes on with the line before.
			out[len(out)-1].text += " " + strip.Replace(l)
		default:
			out = append(out, noteLine{text: strip.Replace(l)})
		}
	}
	return out
}

func (pg *Page) updateSettings(c *ui.Context, e *page.Env) {
	u := pg.upd
	ui.Text(c, "Updates").FontSize(13).FontWeight(600)
	prefRow(c, "Version "+u.Current, pg.updateStatus(e), func() {
		if !u.Can {
			return
		}
		b := ui.Button(c, "Check now").Disabled(u.Busy())
		if b.Clicked() {
			pg.upd.Phase = UpdateChecking
			pg.do(func(a Actions) { a.CheckUpdate() })
		}
	})
	divider(c)
	auto := !e.Prefs.UpdateManual
	if switchRow(c, "Download updates by itself", "Off only tells you when there's one", &auto, !u.Can) {
		page.PrefUpdateManual.Set(e, !auto)
	}
	if u.Phase == UpdateFailed {
		ui.Text(c, u.Err).FontSize(12).TextColor(kit.WarnColor).Wrap()
	}
}

func (pg *Page) updateStatus(e *page.Env) string {
	u := pg.upd
	switch {
	case !u.Can:
		return "Only logpose installed with its installer updates itself"
	case u.Phase == UpdateChecking:
		return "Checking…"
	case u.Phase == UpdateFound:
		return u.Next + " is out"
	case u.Phase == UpdateDownloading:
		return u.getting()
	case u.Phase == UpdateReady:
		return u.Next + " is ready and runs when logpose starts again"
	case u.Phase == UpdateFailed:
		return "Couldn't check for updates"
	case u.Checked.IsZero():
		return "Not checked yet"
	}
	return "Up to date · checked " + kit.DayAndTime(e.Now(), u.Checked)
}
