package store

import (
	"os"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/deck"
	"github.com/VTGare/logpose/internal/simlog"
)

// checkList is the names a migration's CHECK (column IN (...)) allows.
func checkList(file, column string) []string {
	text, err := os.ReadFile("migrations/" + file)
	Expect(err).NotTo(HaveOccurred())
	m := regexp.MustCompile(`CHECK \(` + column + ` IN \(([^)]*)\)\)`).FindStringSubmatch(string(text))
	Expect(m).NotTo(BeNil(), file)
	var out []string
	for _, name := range strings.Split(m[1], ",") {
		out = append(out, strings.Trim(strings.TrimSpace(name), "'"))
	}
	return out
}

var _ = Describe("stored kind names", func() {
	It("stores every end kind under a name the games table allows", func() {
		var names []string
		for _, k := range simlog.EndKinds {
			names = append(names, k.String())
			Expect(simlog.ParseEndKind(k.String())).To(Equal(k))
		}
		Expect(names).To(ConsistOf(checkList("0001_games.sql", "end_kind")))
	})

	It("stores every card event under a name the card events table allows", func() {
		var names []string
		for _, k := range deck.EventKinds {
			names = append(names, k.String())
			got, ok := deck.ParseEventKind(k.String())
			Expect(ok).To(BeTrue())
			Expect(got).To(Equal(k))
		}
		Expect(names).To(ConsistOf(checkList("0008_deck_facts.sql", "kind")))
	})
})
