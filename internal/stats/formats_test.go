package stats_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

var _ = Describe("Debuts", func() {
	// history is n games an hour apart, newest first, ending 100 hours ago, with sets.
	history := func(n int, sets ...string) []game {
		var gs []game
		for i := range n {
			gs = append(gs, game{ago: time.Duration(100+i) * time.Hour, mine: "OP13-001", lobby: "XABCDEF", sets: sets})
		}
		return gs
	}
	matches := func(gs ...game) []stats.Match { return stats.Matches(build(gs...), me) }

	It("finds a set numbered past every earlier one of its kind", func() {
		gs := append([]game{{ago: time.Hour, mine: "OP13-001", lobby: "XABCDEF", sets: []string{"OP17", "OP18"}}}, history(10, "OP17", "EB05")...)
		ds := stats.Debuts(matches(gs...), nil, nil)
		Expect(ds).To(Equal([]stats.SetDebut{{Set: "OP18", First: now.Add(-time.Hour)}}))
	})

	It("ignores old sets turning up late, starter decks and new kinds", func() {
		gs := append([]game{{ago: time.Hour, mine: "OP13-001", lobby: "XABCDEF", sets: []string{"OP03", "ST40", "PRB02"}}}, history(10, "OP17", "ST32")...)
		Expect(stats.Debuts(matches(gs...), nil, nil)).To(BeEmpty())
	})

	It("counts ranked games in the Spoiler and Standard rooms only, since casual lobbies and other rooms play sets ranked bans", func() {
		casual := game{ago: 3 * time.Hour, mine: "OP13-001", lobby: "QWERTY", sets: []string{"OP18"}}
		otherQueue := game{ago: 2 * time.Hour, mine: "OP13-001", lobby: "UHJ6MGP", sets: []string{"OP18"}}
		gs := append([]game{otherQueue, casual}, history(10, "OP17")...)
		Expect(stats.Debuts(matches(gs...), nil, nil)).To(BeEmpty())
		gs = append([]game{{ago: time.Hour, mine: "OP13-001", lobby: "XABCDEF", sets: []string{"OP18"}}}, gs...)
		ds := stats.Debuts(matches(gs...), nil, nil)
		Expect(ds).To(HaveLen(1))
		Expect(ds[0].First).To(Equal(now.Add(-time.Hour)))
	})

	It("counts a Standard room game as much as a Spoiler room one", func() {
		gs := append([]game{{ago: time.Hour, mine: "OP13-001", lobby: "WABCDEF", sets: []string{"OP18"}}}, history(10, "OP17")...)
		Expect(stats.Debuts(matches(gs...), nil, nil)).To(HaveLen(1))
	})

	It("needs 10 earlier games", func() {
		gs := append([]game{{ago: time.Hour, mine: "OP13-001", lobby: "XABCDEF", sets: []string{"OP18"}}}, history(9, "OP17")...)
		Expect(stats.Debuts(matches(gs...), nil, nil)).To(BeEmpty())
	})

	It("leaves out dismissed sets and those a format already starts after", func() {
		gs := append([]game{{ago: time.Hour, mine: "OP13-001", lobby: "XABCDEF", sets: []string{"OP18", "EB06"}}}, history(10, "OP17", "EB05")...)
		ms := matches(gs...)
		Expect(stats.Debuts(ms, nil, []string{"EB06"})).To(HaveLen(1))
		fs := []store.Format{{Name: "OP18", Start: now.Add(-2 * time.Hour)}}
		Expect(stats.Debuts(ms, fs, nil)).To(HaveLen(2))
		fs[0].Start = now.Add(-time.Hour)
		Expect(stats.Debuts(ms, fs, nil)).To(BeEmpty())
	})
})

var _ = Describe("Ranges", func() {
	It("ends each format where the next starts", func() {
		a, b := now.AddDate(0, 0, -20), now.AddDate(0, 0, -5)
		rs := stats.Ranges([]store.Format{{Name: "A", Start: a}, {Name: "B", Start: b}})
		Expect(rs[0].End).To(Equal(b))
		Expect(rs[1].End.IsZero()).To(BeTrue())
	})
})

var _ = Describe("Scope until", func() {
	It("drops matches at or after the end", func() {
		ms := stats.Matches(build(game{ago: time.Hour, mine: "A"}, game{ago: 3 * time.Hour, mine: "A"}), me)
		Expect(stats.Scope{Until: now.Add(-2 * time.Hour)}.Apply(ms)).To(HaveLen(1))
	})
})
