package live_test

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"

	"github.com/VTGare/logpose/internal/live"
	"github.com/VTGare/logpose/internal/simlog"
)

// replay feeds a fixture to a tracker and returns the state after each line.
func replay(name string) []live.State {
	f, err := os.Open(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = f.Close() }()
	var t live.Tracker
	var states []live.State
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t.Line(sc.Text())
		states = append(states, t.State())
	}
	return states
}

// at returns the state right after the first line that has the given phase and n players.
func at(states []live.State, phase live.Phase, players int) live.State {
	for _, s := range states {
		if s.Phase == phase && len(s.Players) == players {
			return s
		}
	}
	Fail("no such state")
	return live.State{}
}

var _ = Describe("Tracker", func() {
	Describe("an OPBounty game with leaders in the log", func() {
		states := replay("ranked.log")

		It("shows the lobby code before the game starts", func() {
			s := at(states, live.Lobby, 0)
			Expect(s.Hosting).To(BeTrue())
			Expect(states[3].Lobby).To(Equal("XABCDEF"))
			Expect(states[3].Ranked()).To(BeTrue())
		})

		It("knows both players, their leaders and who is local", func() {
			var s live.State
			for _, st := range states {
				if st.First != "" {
					s = st
				}
			}
			me, ok := s.Local()
			Expect(ok).To(BeTrue())
			Expect(me).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
				"ID": Equal("Alice#1111"), "Owner": Equal(1), "Leader": Equal("OP13-001"),
				"Life": Equal(5), "Hand": Equal(5),
			}))
			opp, ok := s.Opponent()
			Expect(ok).To(BeTrue())
			Expect(opp).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
				"ID": Equal("Bob#2222"), "Owner": Equal(2), "Leader": Equal("OP17-058"),
				"Life": Equal(4), "Hand": Equal(6),
			}))
			Expect(s.First).To(Equal("Alice#1111"))
		})

		It("is idle after the game, through the stale header", func() {
			Expect(states[len(states)-1]).To(Equal(live.State{}))
		})
	})

	Describe("a joined game without leaders, where the opponent shuffles first", func() {
		states := replay("casual.log")
		last := states[len(states)-1]

		It("keeps the relay code", func() {
			Expect(last.Lobby).To(Equal("QWERTY"))
			Expect(last.Ranked()).To(BeFalse())
			Expect(last.Hosting).To(BeFalse())
		})

		It("gives each player the owner of the moves after their first shuffle", func() {
			me, _ := last.Local()
			opp, _ := last.Opponent()
			Expect(me.ID).To(Equal("Alice#1111"))
			Expect(me.Owner).To(Equal(1))
			Expect(opp.ID).To(Equal("Cat#3333"))
			Expect(opp.Owner).To(Equal(2))
			Expect(opp.Leader).To(BeEmpty())
		})

		It("treats a reshuffle as a mulligan, not a new game", func() {
			Expect(last.Players).To(HaveLen(2))
			Expect(last.First).To(Equal("Cat#3333"))
			me, _ := last.Local()
			Expect(me.Hand).To(Equal(6))
		})
	})

	It("names each saved log with the format its lobby validated decks against", func() {
		var t live.Tracker
		for _, l := range []string{
			"sim version: 1.44a",
			"Load eMultiStyle Nationals",
			"Validate Decks Nationals",
			"Host with 1.44a",
			"Return to Main",
			`[CombatLog SaveMyLogLines] wrote OK | lines=10 | file=CombatLogs\AutoSaved\2026-11-05T14.37.06.log | exists=True | length=1`,
		} {
			t.Line(l)
		}
		Expect(t.TakeSaved()).To(Equal([]live.Saved{{File: "2026-11-05T14.37.06.log", Format: "Nationals", SimVersion: "1.44a"}}))
		Expect(t.TakeSaved()).To(BeEmpty())
	})

	It("counts a game OPBounty numbered as ranked, though the joiner's code is short", func() {
		var t live.Tracker
		t.Line("Joined relay KF669D")
		Expect(t.State().Ranked()).To(BeFalse())
		t.Line("[ReplaySync] RZ1|META|f4a4|7849575")
		Expect(t.State().Match).To(Equal(7849575))
		Expect(t.State().Ranked()).To(BeTrue())
	})

	It("skips the empty PLY line the sim writes for each seat", func() {
		var t live.Tracker
		for _, l := range []string{
			"Joined relay QWERTY",
			"shuffle deck for A\u200b#1",
			"[ReplaySync] RZ1|PLY|2||",
			"[ReplaySync] RZ1|PLY|2|B\u200b#2|OP17-039",
		} {
			t.Line(l)
		}
		Expect(t.State().Players).To(ConsistOf(
			gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{"ID": Equal("A#1")}),
			gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{"ID": Equal("B#2"), "Owner": Equal(2), "Leader": Equal("OP17-039")}),
		))
	})

	It("starts over when the move numbers do", func() {
		var t live.Tracker
		for _, l := range []string{
			"Joined relay QWERTY",
			"shuffle deck for A\u200b#1",
			"[ReplaySync] RZ1|5|1|Don|4|0|5|0|1|1|0|0|0",
			"[ReplaySync] RZ1|CHK|5|1|38|5|0|5|9|1|0|0|1|0",
			"[ReplaySync] RZ1|1|1|OP01-001|0|3|1|0|1|1|0|0|0",
		} {
			t.Line(l)
		}
		s := t.State()
		Expect(s.First).To(BeEmpty())
		Expect(s.Players).To(ConsistOf(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{"ID": Equal("A#1"), "Life": Equal(0)})))
	})
})

var _ = Describe("dropped connections", func() {
	game := []string{
		"[ReplaySync] RZ1|1|2|OP13-004|0|40|1|0|1|1|0|0|0",
		"Joined relay QWERTY",
		"[ReplaySync] RZ1|HDR|1.45a|2|RZ1",
		"[ReplaySync] RZ1|PLY|1||",
		"[ReplaySync] RZ1|PLY|2|B\u200b#2|OP13-004",
		"[ReplaySync] RZ1|PLY|1|A\u200b#1|OP17-020",
		"deck filled, do shuffle",
		"shuffle deck for A\u200b#1",
		"[ReplaySync] RZ1|1|1|OP17-020|0|40|1|0|1|1|0|0|0",
		"[ReplaySync] RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
		"shuffle deck for B\u200b#2",
		"[ReplaySync] RZ1|2|2|OP13-004|0|40|1|0|1|1|0|0|0",
	}
	rz := func(line string) string { return strings.TrimPrefix(line, "[ReplaySync] ") }
	feed := func(lines ...string) []live.Drop {
		var t live.Tracker
		for _, l := range lines {
			t.Line(l)
		}
		return t.TakeDrops()
	}

	It("names the opponent and the game by its fingerprint, from the lines since its header", func() {
		drops := feed(append(game, "opponent connection lost", "Return to Main")...)
		players := []simlog.Player{
			{Name: "B", Tag: "2", Owner: 2, Leader: "OP13-004"},
			{Name: "A", Tag: "1", Owner: 1, Leader: "OP17-020"},
		}
		Expect(drops).To(Equal([]live.Drop{{
			Fingerprint: simlog.Fingerprint(players, []string{rz(game[8]), rz(game[11])}),
			Player:      "B#2",
		}}))
	})

	It("ignores a dropped connection outside a game", func() {
		Expect(feed(append(game, "Return to Main", "opponent connection lost")...)).To(BeEmpty())
	})
})

var _ = Describe("Follower", func() {
	var (
		dir, path string
		states    chan live.State
		running   atomic.Bool
		cancel    context.CancelFunc
	)

	write := func(name string, lines ...string) {
		f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		Expect(err).NotTo(HaveOccurred())
		for _, l := range lines {
			_, err = f.WriteString(l)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(f.Close()).To(Succeed())
	}

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		path = filepath.Join(dir, "Player.log")
		states = make(chan live.State, 100)
		running.Store(true)
		f := &live.Follower{
			Path:     path,
			Interval: 5 * time.Millisecond,
			Running:  running.Load,
			OnChange: func(s live.State) { states <- s },
		}
		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = f.Run(ctx)
		}()
		DeferCleanup(func() {
			cancel()
			<-done
		})
	})

	It("reads lines as they're written, including ones split across writes", func() {
		Eventually(states).Should(Receive(Equal(live.State{})))
		write(path, "Join with 1.44a\r\nJoined rel")
		Eventually(states).Should(Receive(HaveField("Phase", live.Lobby)))
		write(path, "ay QWERTY\r\n")
		Eventually(states).Should(Receive(HaveField("Lobby", "QWERTY")))
	})

	It("starts over when the sim starts a new log", func() {
		write(path, "Join with 1.44a\r\nJoined relay QWERTY\r\n")
		Eventually(states).Should(Receive(HaveField("Lobby", "QWERTY")))
		Expect(os.Rename(path, filepath.Join(dir, "Player-prev.log"))).To(Succeed())
		write(path, "Host with 1.44a\r\n")
		Eventually(states).Should(Receive(And(HaveField("Hosting", true), HaveField("Lobby", ""))))
	})

	It("is idle while the sim isn't running", func() {
		write(path, "Join with 1.44a\r\nJoined relay QWERTY\r\n")
		Eventually(states).Should(Receive(HaveField("Lobby", "QWERTY")))
		running.Store(false)
		Eventually(states).Should(Receive(Equal(live.State{})))
	})
})
