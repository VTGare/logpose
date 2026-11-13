package gui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

func pct(rate float64) string { return fmt.Sprintf("%.0f%%", math.Round(rate*100)) }

// record is written "4–7", with an en dash.
func record(r stats.Record) string { return fmt.Sprintf("%d–%d", r.Wins, r.Losses) }

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// count is n with its noun, such as "1 game" or "3 games".
func count(n int, one, many string) string { return fmt.Sprintf("%d %s", n, plural(n, one, many)) }

func games(n int) string { return count(n, "game", "games") }

func turns(n int) string { return count(n, "turn", "turns") }

// dateText is "Oct 25", with the year when it isn't now's.
func dateText(now, t time.Time) string {
	if t.Year() != now.Year() {
		return t.Format("Jan 2, 2006")
	}
	return t.Format("Jan 2")
}

// weekdayText is "Sun Oct 25", with the year when it isn't now's.
func weekdayText(now, t time.Time) string { return t.Format("Mon ") + dateText(now, t) }

func (e *env) date(t time.Time) string    { return dateText(e.now(), t) }
func (e *env) weekday(t time.Time) string { return weekdayText(e.now(), t) }

// dateTime is the date and the time, such as "Oct 25 18:50".
func (e *env) dateTime(t time.Time) string { return e.date(t) + " " + t.Format("15:04") }

// span is "a – b", or a alone when the two are the same.
func span(a, b string) string {
	if a == b {
		return a
	}
	return a + " – " + b
}

// ago is how long before now then was, such as "4 h ago", or its date past a day.
func ago(now, then time.Time) string {
	d := now.Sub(then)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return dateText(now, then)
}

// dayAndTime is a past moment as "today at 18:50", "yesterday at 18:50" or "Nov 6 at 18:50".
func dayAndTime(now, then time.Time) string {
	day := dateText(now, then)
	switch stats.DayStart(then) {
	case stats.DayStart(now):
		day = "today"
	case stats.DayStart(now.AddDate(0, 0, -1)):
		day = "yesterday"
	}
	return day + " at " + then.Format("15:04")
}

func duration(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %02dm", h, m)
}

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

// thousands writes n with comma separators, such as 2,926.
func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func cardCount(n int) string {
	if n == 1 {
		return "1 card"
	}
	return thousands(n) + " cards"
}

// typeRank orders cards as a decklist does, with types the card data lacks last.
func typeRank(kind string) int {
	for i, t := range []string{cards.Character, cards.Event, cards.Stage} {
		if strings.EqualFold(kind, t) {
			return i
		}
	}
	return 3
}
