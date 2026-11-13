package gui

import (
	"image/color"

	"github.com/egoist/mygo/ui"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/gui/guitest"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/gui/settings"
)

var _ = Describe("going from one page to another", func() {
	It("opens Matches filtered to the matchup", func() {
		s := guitest.Snapshot(10)
		s.Matches[0].Opp.Leader = "OP09-042"
		s.Cards.Name("OP09-042", "Buggy")
		_, tt := tester(s)
		Expect(tt.Click("Matchups")).To(Succeed())
		tt.Frame()
		Expect(tt.Click("Show in Matches")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElements("Monkey D. Luffy", "vs Kaido", "9 games · 4–5"))
		Expect(tt.HasText("Sort by 2 leaders faced")).To(BeFalse())
	})

	It("tints the rail's update button blue, unlike the grey of the current page", func() {
		v, tt := tester(guitest.Snapshot(4))
		v.settings.SetUpdate(settings.UpdateState{Can: true, Current: "0.1.0", Phase: settings.UpdateReady, Next: "0.1.1"})
		v.setPrefs(page.PrefsFrom(map[string]string{page.KeySidebarWidth: "90"}))
		v.Page = page.Formats
		tt.Frame()
		fill := func(label string) color.RGBA {
			GinkgoHelper()
			r, ok := tt.Find(label)
			Expect(ok).To(BeTrue(), label)
			return tt.Image().RGBAAt(int(r.X+6), int(r.Y+r.H/2))
		}
		upd, page := fill("0.1.1 is ready"), fill("Formats")
		Expect(upd).NotTo(Equal(page))
		Expect(upd.B).To(BeNumerically(">", upd.R+20))
		Expect(page.B).To(BeNumerically("<", page.R+8))
	})

	It("opens a match from its hand and goes back to the Hands tab", func() {
		v, tt := tester(guitest.HandsSnapshot())
		v.setActions(&fakeActions{})
		tt.SetSize(1180, 2000)
		v.open("decks/hands")
		tt.Frame()
		Expect(tt.Click("Show the hands dealt Shanks")).To(Succeed())
		tt.Frame()
		Expect(tt.Click("Open the match against Opp")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Back to decks")).To(BeTrue())
		Expect(tt.Click("Back to decks")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Back to decks")).To(BeFalse())
		Expect(tt.HasText("The 5 games you were dealt Shanks")).To(BeTrue())
	})
})

var _ = Describe("an open match and the other pages", func() {
	var fa *fakeActions
	open := func(s *page.Snapshot) (*view, *ui.Tester) {
		v, tt := tester(s)
		fa = &fakeActions{}
		v.setActions(fa)
		return v, tt
	}

	It("opens a match from its tile on the overview and goes back there", func() {
		_, tt := open(guitest.Snapshot(1))
		Expect(tt.Click("Open the match against Opp")).To(Succeed())
		Expect(tt.HasText("Opening hands")).To(BeTrue())
		Expect(tt.Click("Back to overview")).To(Succeed())
		Expect(tt.HasText("Opening hands")).To(BeFalse())
		Expect(tt.HasText("Open the match against Opp")).To(BeTrue())
	})

	It("closes the match when its page is picked in the sidebar", func() {
		_, tt := open(guitest.Snapshot(1))
		Expect(tt.Click("Open the match against Opp")).To(Succeed())
		Expect(tt.Click("Overview")).To(Succeed())
		Expect(tt.HasText("Opening hands")).To(BeFalse())
		Expect(tt.HasText("Open the match against Opp")).To(BeTrue())
	})

	It("closes the match when another page is picked", func() {
		_, tt := open(guitest.Snapshot(1))
		Expect(tt.Click("Open the match against Opp")).To(Succeed())
		Expect(tt.Click("Matches")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Opening hands")).To(BeFalse())
		Expect(tt.HasText("Back to overview")).To(BeFalse())
	})

	It("saves the note being typed when the page changes", func() {
		s := guitest.Snapshot(3)
		s.Matches[1].Note = "old note"
		v, tt := open(s)
		v.OpenMatch(s.Matches[1])
		tt.Frame()
		Expect(tt.Click("Match note")).To(Succeed())
		tt.Type(" and more")
		Expect(tt.Click("Overview")).To(Succeed())
		Expect(fa.notes).To(Equal([]string{"1=old note and more"}))
		Expect(tt.HasText("Match note")).To(BeFalse())
	})
})
