package deck

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"

	"github.com/VTGare/logpose/internal/simlog"
)

// The fixtures are two of the user's games with the players renamed Alice and Bob, the lobby
// code replaced, the seal zeroed and the SEM lines left out.
func alice(file string) Facts {
	text, err := os.ReadFile("testdata/" + file)
	Expect(err).NotTo(HaveOccurred())
	games, err := simlog.Parse(strings.NewReader(string(text)))
	Expect(err).NotTo(HaveOccurred())
	Expect(games).To(HaveLen(1))
	facts, err := Read(&games[0])
	Expect(err).NotTo(HaveOccurred())
	for _, f := range facts {
		if f.Player == "Alice#1111" {
			return f
		}
	}
	Fail("no facts for Alice")
	return Facts{}
}

func count(f Facts, k EventKind) int {
	n := 0
	for _, e := range f.Events {
		if e.Kind == k {
			n++
		}
	}
	return n
}

var _ = Describe("Read", func() {
	It("names the whole deck by following every card through the game", func() {
		f := alice("luffy-samurai.log")
		Expect(f.Named()).To(BeNumerically(">=", 49))
		Expect(f.List).To(HaveKeyWithValue("ST30-004", 4))
		Expect(f.List).To(HaveKeyWithValue("OP01-055", 3))
		Expect(f.List).To(HaveKeyWithValue("OP14-037", 1))
	})

	It("tells plays the player paid for from cards an effect put out", func() {
		f := alice("luffy-searches.log")
		Expect(count(f, PaidPlay)).To(Equal(12))
		Expect(count(f, EffectPlay)).To(BeNumerically("<=", 4))
	})

	It("counts characters discarded for their counter and counter events", func() {
		Expect(count(alice("luffy-searches.log"), Counter)).To(Equal(3))
		Expect(count(alice("luffy-samurai.log"), Counter)).To(Equal(4))
	})

	It("marks an event leaving the hand after its own effect lines", func() {
		f := alice("luffy-samurai.log")
		var samurai []Event
		for _, e := range f.Events {
			if e.Card == "OP01-055" && e.Kind == OwnLines {
				samurai = append(samurai, e)
			}
		}
		Expect(samurai).NotTo(BeEmpty())
	})

	It("reads each search with the card taken and the ones passed over", func() {
		f := alice("luffy-samurai.log")
		Expect(f.Searches).To(HaveLen(3))
		Expect(f.Searches[0]).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Source": Equal("OP07-022"),
			"Taken":  Equal("OP01-055"),
			"Passed": HaveLen(4),
		}))
	})

	It("gives a search that took nothing to the card deployed before it", func() {
		var empty []Search
		for _, s := range alice("luffy-searches.log").Searches {
			if s.Taken == "" {
				empty = append(empty, s)
			}
		}
		Expect(empty).To(HaveLen(1))
		Expect(empty[0].Source).To(Equal("ST31-005"))
		Expect(empty[0].Passed).NotTo(BeEmpty())
	})

	It("reads the DON in play each turn, up to 10", func() {
		f := alice("luffy-samurai.log")
		Expect(f.Don[2]).To(Equal(2))
		Expect(f.Don[4]).To(Equal(4))
		Expect(f.Don[len(f.Don)-1]).To(BeNumerically("<=", 10))
	})

	It("has nothing to read in a log without RZ1", func() {
		games, err := simlog.Parse(strings.NewReader("Version is 1.30a\r\n"))
		Expect(err).NotTo(HaveOccurred())
		for i := range games {
			facts, err := Read(&games[i])
			Expect(err).NotTo(HaveOccurred())
			Expect(facts).To(BeEmpty())
		}
	})
})
