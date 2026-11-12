package gui

import (
	"time"

	"github.com/egoist/mygo/ui"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

var _ = Describe("formats", func() {
	// withFormats splits the 10 test games, an hour apart, at 4 hours ago.
	withFormats := func() *snapshot {
		s := testSnapshot(10)
		s.Formats = stats.Ranges([]store.Format{
			{ID: 1, Name: "Old format", Start: now.Add(-48 * time.Hour)},
			{ID: 2, Name: "New format", Start: now.Add(-4*time.Hour - 30*time.Minute)},
		})
		return s
	}

	latestFirst := func(s *snapshot) *ui.Tester {
		v, tt := tester(s)
		v.setPrefs(prefsFrom(map[string]string{keyStartFormat: startLatest}))
		tt.Frame()
		return tt
	}

	It("starts on the current format and can show the older one or all", func() {
		tt := latestFirst(withFormats())
		Expect(tt.HasText("All of New format")).To(BeTrue())
		Expect(tt.Texts()).To(ContainElements("New format", "4 games"))
		Expect(tt.Click("New format")).To(Succeed())
		Expect(choose(tt, "Old format · since Nov 4")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("6 games"))
		Expect(tt.Click("Old format")).To(Succeed())
		Expect(choose(tt, "All formats")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("10 games"))
	})

	It("counts ranked games only in a format, with the type fixed to ranked", func() {
		s := withFormats()
		s.Matches[0].Ranked = false
		_, tt := tester(s)
		Expect(tt.Click("All games")).To(Succeed())
		Expect(choose(tt, "Casual")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("1 game"))
		Expect(tt.Click("All formats")).To(Succeed())
		Expect(choose(tt, "New format · since Nov 6")).To(Succeed())
		Expect(tt.Texts()).To(ContainElements("Ranked", "3 games"))
		Expect(tt.Texts()).NotTo(ContainElement("Casual"))
	})

	It("lists formats with their games, and the games before the first", func() {
		v, tt := tester(withFormats())
		v.page = pageFormats
		tt.Frame()
		Expect(tt.HasText("New format")).To(BeTrue())
		Expect(tt.HasText("4 games ranked · 2–2")).To(BeTrue(), "%v", tt.Texts())
	})

	It("asks for a name before adding a format", func() {
		v, tt := tester(withFormats())
		fa := &fakeActions{}
		v.act = fa
		v.page = pageFormats
		tt.Frame()
		Expect(tt.Click("Add")).To(Succeed())
		Expect(tt.HasText("Give the format a name.")).To(BeTrue())
		Expect(fa.added).To(BeEmpty())
		Expect(tt.Click("Format name")).To(Succeed())
		tt.Type(" OP18 ")
		Expect(tt.Click("Add")).To(Succeed())
		Expect(fa.added).To(HaveLen(1))
		Expect(fa.added[0].Name).To(Equal("OP18"))
	})

	It("offers a format where a new set showed up, on the overview", func() {
		s := testSnapshot(10)
		s.Debuts = []stats.SetDebut{{Set: "OP18", First: time.Date(2026, 11, 2, 18, 30, 0, 0, time.Local)}}
		v, tt := tester(s)
		fa := &fakeActions{}
		v.act = fa
		tt.Frame()
		Expect(tt.HasText("OP18 cards first showed up in your games on Nov 2")).To(BeTrue())
		Expect(tt.Click("Start a format on Nov 2")).To(Succeed())
		Expect(fa.added).To(Equal([]store.Format{{Name: "OP18", Start: time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)}}))
		Expect(tt.Click("Dismiss")).To(Succeed())
		Expect(fa.dismissed).To(Equal([]string{"OP18"}))
	})
})

var _ = Describe("gameType", func() {
	It("names the ranked room by its letter, and the sim's format for casual games", func() {
		var m stats.Match
		m.Ranked, m.Lobby.Code = true, "EABCDEF"
		Expect(gameType(m)).To(Equal("Ranked · Extra Regulation"))
		m.Lobby.Code = "WABCDEF"
		Expect(gameType(m)).To(Equal("Ranked · Standard"))
		m.Lobby.Code = "UABCDEF"
		Expect(gameType(m)).To(Equal("Ranked"))
		m.Ranked, m.Lobby.Code, m.SimFormat = false, "QWERTY", "Spoiler"
		Expect(gameType(m)).To(Equal("Casual · Spoiler"))
	})
})
