package gui

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

type fakeActions struct {
	added     []store.Format
	dismissed []string
	loaded    []string
	types     []string
	notes     []string
	bookmarks []string
	replays   []string
	timers    []func()
	calls     []string
}

func (f *fakeActions) SetBookmark(fingerprint string, on bool) {
	f.bookmarks = append(f.bookmarks, fmt.Sprintf("%s=%t", fingerprint, on))
}

func (f *fakeActions) AddFormat(name string, start time.Time) {
	f.added = append(f.added, store.Format{Name: name, Start: start})
}
func (*fakeActions) UpdateFormat(store.Format) {}
func (*fakeActions) DeleteFormat(int64)        {}
func (f *fakeActions) DismissSet(set string)   { f.dismissed = append(f.dismissed, set) }

func (f *fakeActions) OpenReplay(fingerprint string) { f.replays = append(f.replays, fingerprint) }

// After keeps the timer for a spec to fire with fire.
func (f *fakeActions) After(_ time.Duration, fn func()) { f.timers = append(f.timers, fn) }

func (f *fakeActions) fire() {
	timers := f.timers
	f.timers = nil
	for _, fn := range timers {
		fn()
	}
}

func (f *fakeActions) LoadTimeline(fingerprint, _, _ string) {
	f.loaded = append(f.loaded, fingerprint)
}

func (f *fakeActions) SetGameNote(fingerprint, note string) {
	f.notes = append(f.notes, fingerprint+"="+note)
}

func (f *fakeActions) SetGameType(fingerprint string, t store.GameType) {
	f.types = append(f.types, fingerprint+"="+fmt.Sprint(t))
}

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

	It("starts on the current format and can show the older one or all", func() {
		v, tt := tester(withFormats())
		Expect(tt.HasText("New format")).To(BeTrue())
		Expect(tt.HasText("All of New format")).To(BeTrue())
		Expect(v.scoped().matches).To(HaveLen(4))
		v.filter.format = 1
		Expect(v.scoped().matches).To(HaveLen(6))
		v.filter.format = formatAll
		Expect(v.scoped().matches).To(HaveLen(10))
	})

	It("counts ranked games only in a format, with the type fixed to ranked", func() {
		s := withFormats()
		s.Matches[0].Ranked = false
		v, tt := tester(s)
		v.filter.gameType = int(stats.CasualGames)
		Expect(v.scoped().matches).To(HaveLen(3))
		tt.Frame()
		Expect(tt.HasText("Ranked")).To(BeTrue())
		Expect(tt.HasText("Casual")).To(BeFalse())
		v.filter.format = formatAll
		Expect(v.scoped().matches).To(HaveLen(1))
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
		v.formats.name = " OP18 "
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
