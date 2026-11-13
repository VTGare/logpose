package gui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/VTGare/logpose/internal/gui/matches"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// sixTurns is the replay package's fixture: a real game's first six turns between Alice and
// Bob, as match "0".
func sixTurns() matches.LoadedReplay {
	text, err := os.ReadFile("../replay/testdata/six-turns.log")
	Expect(err).NotTo(HaveOccurred())
	g := parseOne(string(text))
	r, err := replay.Build(g, nil)
	Expect(err).NotTo(HaveOccurred())
	return matches.LoadedReplay{Fingerprint: "0", Game: g, Replay: r}
}

func parseOne(text string) *simlog.Game {
	games, err := simlog.Parse(strings.NewReader(text))
	Expect(err).NotTo(HaveOccurred())
	Expect(games).To(HaveLen(1))
	return &games[0]
}

var _ = Describe("replay", func() {
	var (
		fa *fakeActions
		v  *view
		tt *ui.Tester
	)

	BeforeEach(func() {
		s := testSnapshot(3)
		s.Matches[0].Me.ID, s.Matches[0].Me.Name = "Alice#1111", "Alice"
		s.Matches[0].Opp.ID, s.Matches[0].Opp.Name = "Bob#2222", "Bob"
		v, tt = tester(s)
		fa = &fakeActions{}
		v.setActions(fa)
	})

	// watch opens match 0's replay from its page and shows it built.
	watch := func() matches.LoadedReplay {
		v.Page = page.Matches
		v.OpenMatch(v.Snap.Matches[0])
		v.matches.SetTimeline(matches.MatchTimeline{Fingerprint: "0", Replayable: true})
		tt.Frame()
		Expect(tt.Click("Watch replay")).To(Succeed())
		Expect(fa.replays).To(Equal([]string{"0"}))
		Expect(tt.HasText("Building the replay")).To(BeTrue())
		l := sixTurns()
		v.matches.SetReplay(l)
		tt.Frame()
		return l
	}
	// at reports whether the replay shows step i, counted from 0, of the fixture's 30.
	at := func(i int) bool {
		tt.Frame()
		return tt.HasText(fmt.Sprintf("Step %d of 30", i+1))
	}
	goTo := func(i int) {
		tt.Key(0, ui.KeyHome)
		for range i {
			tt.Key(0, ui.KeyRight)
		}
		Expect(at(i)).To(BeTrue())
	}

	It("opens from the match page without the sidebar, and Back returns to the match", func() {
		watch()
		Expect(at(0)).To(BeTrue())
		for _, text := range []string{"Setup", "Restart this turn", "Turn 3 · Bob", "Played Kyo"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("Overview")).To(BeFalse())

		Expect(tt.Click("Back to match")).To(Succeed())
		Expect(at(0)).To(BeFalse())
		Expect(tt.HasText("Back to matches")).To(BeTrue())
		Expect(tt.HasText("Watch replay")).To(BeTrue())
	})

	It("keeps Watch replay off until the log is read, and for a log without moves", func() {
		v.Page = page.Matches
		v.OpenMatch(v.Snap.Matches[0])
		tt.Frame()
		Expect(tt.Click("Watch replay")).To(Succeed())
		v.matches.SetTimeline(matches.MatchTimeline{Fingerprint: "0"})
		tt.Frame()
		Expect(tt.Click("Watch replay")).To(Succeed())
		Expect(fa.replays).To(BeEmpty())
		Expect(tt.HasText("Building the replay")).To(BeFalse())
		Expect(tt.HasText("Back to matches")).To(BeTrue())
	})

	It("says why a game without moves can't be replayed", func() {
		v.Page = page.Matches
		v.matches.OpenReplay(&v.Env, v.Snap.Matches[0], -1)
		v.matches.SetReplay(matches.LoadedReplay{Fingerprint: "0", Err: replay.ErrNoMoves})
		tt.Frame()
		Expect(tt.HasText(matches.NoMovesText)).To(BeTrue())
	})

	It("opens from the Matches row menu", func() {
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(tt.RightClick("19:00")).To(Succeed())
		Expect(choose(tt, "Watch replay")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Building the replay")).To(BeTrue())
		Expect(fa.loaded).To(Equal([]string{"0"}))
		Expect(fa.replays).To(BeEmpty())
		v.matches.SetTimeline(matches.MatchTimeline{Fingerprint: "0", Replayable: true, Game: sixTurns().Game})
		Expect(fa.replays).To(Equal([]string{"0"}))
		v.matches.SetTimeline(matches.MatchTimeline{Fingerprint: "0", Replayable: true, Game: sixTurns().Game})
		Expect(fa.replays).To(HaveLen(1))
	})

	It("says when the match's log can't be read", func() {
		v.Page = page.Matches
		v.matches.OpenReplay(&v.Env, v.Snap.Matches[0], -1)
		v.matches.SetTimeline(matches.MatchTimeline{Fingerprint: "0", Err: errors.New("no such game")})
		tt.Frame()
		Expect(fa.replays).To(BeEmpty())
		Expect(tt.HasText("The game's log couldn't be read: no such game")).To(BeTrue())
	})

	It("opens a key moment's pin at that moment's step", func() {
		l := sixTurns()
		line := l.Replay.Steps[19].Lines[0]
		v.Page = page.Matches
		v.OpenMatch(v.Snap.Matches[0])
		v.matches.SetTimeline(matches.MatchTimeline{
			Fingerprint: "0", Replayable: true, Me: []int{5, 4, 3}, Opp: []int{5, 5, 4},
			Moments: []simlog.Moment{{
				Turn: 5, Player: "Bob#2222", Line: line,
				Reasons: []simlog.Reason{{Kind: simlog.ReasonLifeLost, Player: "Alice#1111", N: 2}},
			}},
		})
		tt.Frame()
		Expect(tt.Click("Replay moment 1")).To(Succeed())
		Expect(fa.replays).To(Equal([]string{"0"}))
		v.matches.SetReplay(l)
		Expect(at(19)).To(BeTrue())
		Expect(tt.HasText("Rocks D. Xebec attacked Monkey D. Garp")).To(BeTrue())
	})

	It("steps with the buttons and the arrow keys, and goes to either end", func() {
		watch()
		Expect(tt.Click("Next step")).To(Succeed())
		Expect(at(1)).To(BeTrue())
		tt.Key(0, ui.KeyRight)
		Expect(at(2)).To(BeTrue())
		tt.Key(0, ui.KeyLeft)
		Expect(tt.Click("Previous step")).To(Succeed())
		Expect(at(0)).To(BeTrue())
		tt.Key(0, ui.KeyLeft)
		Expect(at(0)).To(BeTrue())
		tt.Key(0, ui.KeyEnd)
		Expect(at(29)).To(BeTrue())
		tt.Key(0, ui.KeyHome)
		Expect(at(0)).To(BeTrue())
	})

	It("moves by turn with the buttons and keys, and restarts the turn", func() {
		watch()
		tt.Key(0, ui.KeyPageDown)
		Expect(at(4)).To(BeTrue())
		Expect(tt.HasText("Turn 2")).To(BeTrue())
		Expect(tt.Click("Next turn")).To(Succeed())
		Expect(at(5)).To(BeTrue())
		tt.Key(0, ui.KeyPageUp)
		Expect(at(4)).To(BeTrue())
		Expect(tt.HasText("Setup")).To(BeTrue())
		Expect(tt.Click("Previous turn")).To(Succeed())
		Expect(at(0)).To(BeTrue())
		goTo(27)
		Expect(tt.HasText("Counter Step")).To(BeTrue())
		Expect(tt.HasText("Your turn")).To(BeTrue())
		goTo(10)
		Expect(tt.HasText("Turn 3")).To(BeTrue())
		Expect(tt.HasText("Attack Step")).To(BeTrue())
		Expect(tt.Click("Restart this turn")).To(Succeed())
		Expect(at(7)).To(BeTrue())
	})

	It("jumps to a step clicked in the action list", func() {
		watch()
		Expect(tt.Click("Played Kyo")).To(Succeed())
		Expect(at(8)).To(BeTrue())
		Expect(tt.HasText("Main")).To(BeTrue())
		Expect(tt.Click("Setup steps")).To(Succeed())
		Expect(at(0)).To(BeTrue())
	})

	It("shows an effect during an attack as a step of the attack", func() {
		watch()
		goTo(26)
		Expect(tt.HasText("Attack Step")).To(BeTrue())
		Expect(tt.HasText("Charlotte Linlin: +1000 to Rocks D. Xebec (trashed Wang Zhi)")).To(BeTrue())
		Expect(tt.HasText("+1000")).To(BeTrue())
	})

	It("shows statuses as chips, and both powers during an attack", func() {
		watch()
		goTo(27)
		Expect(tt.HasText("Counter Step")).To(BeTrue())
		Expect(tt.HasText("+2000")).To(BeTrue())
		Expect(tt.HasText("6000")).To(BeTrue())
		Expect(tt.HasText("7000")).To(BeTrue())
	})

	It("shows a card clicked on the mat in the inspector", func() {
		l := watch()
		goTo(1)
		leader := v.Snap.Card(l.Replay.Steps[1].Board.Side(1).Leader.Code).Name
		Expect(tt.HasText("Selected card")).To(BeFalse())
		Expect(tt.Click(leader)).To(Succeed())
		Expect(tt.HasText("Selected card")).To(BeTrue())
	})

	It("shows a deck in order and a trash by card when the pile is clicked on the mat", func() {
		l := watch()
		goTo(29)
		Expect(tt.Click("Your deck")).To(Succeed())
		Expect(tt.HasText("Top first. 1 is the next draw.")).To(BeTrue())

		trash := len(l.Replay.Steps[29].Board.Side(1).Trash)
		Expect(trash).To(BeNumerically(">", 0))
		Expect(tt.Click(fmt.Sprintf("Trash %d", trash))).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Last trashed")).To(BeTrue())
		Expect(tt.HasText("Top first. 1 is the next draw.")).To(BeFalse())

		Expect(tt.Click("Show this step")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("This step")).To(BeTrue())
	})

	It("shows a life pile top first when it's clicked on the mat", func() {
		l := watch()
		goTo(29)
		life := len(l.Replay.Steps[29].Board.Side(1).Life)
		Expect(life).To(BeNumerically(">", 0))
		Expect(tt.Click("Your life")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Top first. 1 is the next to be taken.")).To(BeTrue())
		Expect(tt.HasText(fmt.Sprintf("Life %d", life))).To(BeTrue())
	})

	It("keeps the controls in place from step to step", func() {
		watch()
		var places []ui.Rect
		for _, i := range []int{1, 15, 29} {
			goTo(i)
			r, ok := tt.Find("Next step")
			Expect(ok).To(BeTrue())
			places = append(places, r)
		}
		Expect(places[1]).To(Equal(places[0]))
		Expect(places[2]).To(Equal(places[0]))
	})

	// The specs' tester turns animations off, so this reads how long the mat glides cards for.
	It("glides cards on a single step only, and not at the fastest speed", func() {
		watch()
		rs := v.matches.Detail.Replay
		Expect(tt.Click("Next step")).To(Succeed())
		byHand := rs.Glide
		Expect(byHand).To(BeNumerically(">", 0))
		tt.Key(0, ui.KeyEnd)
		Expect(rs.Glide).To(BeZero())
		tt.Key(0, ui.KeyHome)
		Expect(rs.Glide).To(BeZero())

		Expect(tt.Click("2×")).To(Succeed())
		Expect(tt.Click("Play")).To(Succeed())
		fa.fire()
		Expect(at(1)).To(BeTrue())
		Expect(rs.Glide).To(SatisfyAll(BeNumerically(">", 0), BeNumerically("<", byHand)))
		Expect(tt.Click("4×")).To(Succeed())
		fa.fire()
		Expect(at(2)).To(BeTrue())
		Expect(rs.Glide).To(BeZero())
	})

	It("plays a step per tick until paused", func() {
		watch()
		Expect(tt.Click("Play")).To(Succeed())
		fa.fire()
		Expect(at(1)).To(BeTrue())
		fa.fire()
		Expect(at(2)).To(BeTrue())
		Expect(tt.Click("Pause")).To(Succeed())
		fa.fire()
		Expect(at(2)).To(BeTrue())
		tt.Key(0, ui.KeySpace)
		Expect(tt.HasText("Pause")).To(BeTrue())
		Expect(tt.Click("4×")).To(Succeed())
		fa.fire()
		Expect(at(3)).To(BeTrue())
	})

	It("stops playing at the last step", func() {
		watch()
		tt.Key(0, ui.KeyEnd)
		tt.Key(0, ui.KeyLeft)
		Expect(tt.Click("Play")).To(Succeed())
		fa.fire()
		Expect(at(29)).To(BeTrue())
		Expect(tt.HasText("Play")).To(BeTrue())
		Expect(fa.timers).To(BeEmpty())
	})
})

var _ = Describe("battlePair", func() {
	It("draws the attack from the copy that attacked, not the first one rested", func() {
		shanks := func(id int, rested bool) matches.Placed {
			return matches.Placed{Card: replay.Card{ID: id, Code: "OP17-022", Rested: rested}, Mine: true}
		}
		cards := []matches.Placed{
			shanks(1001, true), shanks(1002, true), shanks(1003, false),
			{Card: replay.Card{ID: 2999, Code: "OP17-039", Rested: true}},
		}
		bt := &replay.Battle{Attacker: "OP17-022", Target: "OP17-039", AttackerOwner: 1, AttackerID: 1002, DefenderID: 2999}
		a, t := matches.BattlePair(cards, bt, true)
		Expect([]int{a, t}).To(Equal([]int{1, 3}))
	})
})
