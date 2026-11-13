package kit

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

func Pct(rate float64) string { return fmt.Sprintf("%.0f%%", math.Round(rate*100)) }

// Record is written "4–7", with an en dash.
func Record(r stats.Record) string { return fmt.Sprintf("%d–%d", r.Wins, r.Losses) }

func Plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// count is n with its noun, such as "1 game" or "3 games".
func count(n int, one, many string) string { return fmt.Sprintf("%d %s", n, Plural(n, one, many)) }

func Games(n int) string { return count(n, "game", "games") }

func Turns(n int) string { return count(n, "turn", "turns") }

// DateText is "Oct 25", with the year when it isn't now's.
func DateText(now, t time.Time) string {
	if t.Year() != now.Year() {
		return t.Format("Jan 2, 2006")
	}
	return t.Format("Jan 2")
}

// WeekdayText is "Sun Oct 25", with the year when it isn't now's.
func WeekdayText(now, t time.Time) string { return t.Format("Mon ") + DateText(now, t) }

// Span is "a – b", or a alone when the two are the same.
func Span(a, b string) string {
	if a == b {
		return a
	}
	return a + " – " + b
}

// Ago is how long before now then was, such as "4 h Ago", or its date past a day.
func Ago(now, then time.Time) string {
	d := now.Sub(then)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return DateText(now, then)
}

// DayAndTime is a past moment as "today at 18:50", "yesterday at 18:50" or "Nov 6 at 18:50".
func DayAndTime(now, then time.Time) string {
	day := DateText(now, then)
	switch stats.DayStart(then) {
	case stats.DayStart(now):
		day = "today"
	case stats.DayStart(now.AddDate(0, 0, -1)):
		day = "yesterday"
	}
	return day + " at " + then.Format("15:04")
}

func Duration(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %02dm", h, m)
}

// Megabytes is a size such as "8.4 MB" or "258 MB".
func Megabytes(n int64) string {
	mb := float64(n) / (1 << 20)
	if n == 0 {
		return "0 MB"
	}
	if mb < 10 {
		return fmt.Sprintf("%.1f MB", mb)
	}
	return fmt.Sprintf("%.0f MB", mb)
}

// Thousands writes n with comma separators, such as 2,926.
func Thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func CardCount(n int) string {
	if n == 1 {
		return "1 card"
	}
	return Thousands(n) + " cards"
}

// TypeRank orders cards as a decklist does, with types the card data lacks last.
func TypeRank(kind string) int {
	for i, t := range []string{cards.Character, cards.Event, cards.Stage} {
		if strings.EqualFold(kind, t) {
			return i
		}
	}
	return 3
}

// CounterText writes a counter as "+1000", or "+1k" when short. none stands for no counter.
func CounterText(n int, short bool, none string) string {
	switch {
	case n == 0:
		return none
	case short:
		return fmt.Sprintf("+%dk", n/1000)
	}
	return fmt.Sprintf("+%d", n)
}
