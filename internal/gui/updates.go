package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type updatePhase int

const (
	// updateIdle is up to date, or not checked yet.
	updateIdle updatePhase = iota
	updateChecking
	// updateFound is a newer version that waits for the user to download it.
	updateFound
	updateDownloading
	// updateReady is a newer version that's installed and runs once logpose starts again.
	updateReady
	updateFailed
)

type updateState struct {
	// can is false for copies that can't replace themselves, such as dev builds.
	can     bool
	current string
	phase   updatePhase
	next    string
	// notes are the next version's release notes, in Markdown.
	notes   string
	done    float32
	checked time.Time
	err     string
}

// pending is true while a newer version is found, downloading or ready.
func (u updateState) pending() bool {
	return u.phase == updateFound || u.phase == updateDownloading || u.phase == updateReady
}

// busy is true while a check can't start: one is running, or a version is downloading or ready.
func (u updateState) busy() bool {
	return u.phase == updateChecking || u.phase == updateDownloading || u.phase == updateReady
}

func (u updateState) getting() string {
	return fmt.Sprintf("Getting %s · %d%%", u.next, int(u.done*100))
}

func (v *view) setUpdate(u updateState) {
	if u.phase == updateReady && v.upd.phase != updateReady {
		v.updNotes = false
	}
	v.upd = u
}

// updateBox sits above Formats and Settings in the sidebar while a newer version is found or ready. The
// rail shows it as an icon.
func (v *view) updateBox(c *ui.Context, wide bool) {
	u := v.upd
	if !u.pending() {
		return
	}
	t := c.Theme()
	title, action := u.next+" is ready", "Restart"
	icon := iconRestart
	switch u.phase {
	case updateFound:
		title, action, icon = u.next+" is out", "Get it", iconSave
	case updateDownloading:
		title, action, icon = u.getting(), "", iconSave
	}
	run := func() {
		switch u.phase {
		case updateReady:
			v.do(func(a actions) { a.RestartToUpdate() })
		case updateFound:
			v.upd.phase = updateDownloading
			v.do(func(a actions) { a.InstallUpdate() })
		}
	}
	if !wide {
		// Tinted with the accent so it reads as an action, not as the current page's grey fill.
		b := ui.ButtonBase(c).Label(title).Width(44).Height(40).Radius(8).Center().Background(railUpdateTint(t, false)).
			Tooltip(title + ". " + updateHint(u.phase))
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
			info := ui.ButtonBase(c).Label("What's new in " + u.next).Background(ui.Transparent)
			if info.Clicked() {
				v.updNotes = !v.updNotes
			}
			col := t.TextMuted
			if info.Hovered() || v.updNotes {
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
	v.updateNotes(c, box)
}

func railUpdateTint(t *ui.Theme, hovered bool) ui.Color {
	if hovered {
		return t.Accent.Alpha(0.26)
	}
	return t.Accent.Alpha(0.16)
}

func updateHint(p updatePhase) string {
	switch p {
	case updateReady:
		return "Click to restart into it."
	case updateFound:
		return "Click to download it."
	}
	return ""
}

// updateNotes is the next version's release notes, under the sidebar's box.
func (v *view) updateNotes(c *ui.Context, anchor ui.Element) {
	t := c.Theme()
	u := v.upd
	ui.PopoverBase(c, anchor, &v.updNotes, func(panel ui.Element) {
		popover(panel.Margin(0, 0, 6, 0).Padding(14, 16).Width(420), t)
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "logpose "+u.next).FontWeight(600)
			ui.Text(c, "You have "+u.current+".").FontSize(12).TextColor(t.TextMuted)
			ui.Box(c).Height(4)
			for _, l := range noteLines(u.notes) {
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

func (v *view) updateSettings(c *ui.Context) {
	u := v.upd
	ui.Text(c, "Updates").FontSize(13).FontWeight(600)
	prefRow(c, "Version "+u.current, v.updateStatus(), func() {
		if !u.can {
			return
		}
		b := ui.Button(c, "Check now").Disabled(u.busy())
		if b.Clicked() {
			v.upd.phase = updateChecking
			v.do(func(a actions) { a.CheckUpdate() })
		}
	})
	divider(c)
	auto := !v.prefs.updateManual
	if switchRow(c, "Download updates by itself", "Off only tells you when there's one", &auto, !u.can) {
		prefUpdateManual.set(v, !auto)
	}
	if u.phase == updateFailed {
		ui.Text(c, u.err).FontSize(12).TextColor(warnColor).Wrap()
	}
}

func (v *view) updateStatus() string {
	u := v.upd
	switch {
	case !u.can:
		return "Only logpose installed with its installer updates itself"
	case u.phase == updateChecking:
		return "Checking…"
	case u.phase == updateFound:
		return u.next + " is out"
	case u.phase == updateDownloading:
		return u.getting()
	case u.phase == updateReady:
		return u.next + " is ready and runs when logpose starts again"
	case u.phase == updateFailed:
		return "Couldn't check for updates"
	case u.checked.IsZero():
		return "Not checked yet"
	}
	return "Up to date · checked " + dayAndTime(v.now(), u.checked)
}
