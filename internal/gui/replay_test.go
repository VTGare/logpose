package gui

import (
	"fmt"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// sixTurns is the replay package's fixture: a real game's first six turns between Alice and
// Bob, as match "0".
func sixTurns() loadedReplay {
	text, err := os.ReadFile("../replay/testdata/six-turns.log")
	Expect(err).NotTo(HaveOccurred())
	g := parseOne(string(text))
	r, err := replay.Build(g, nil)
	Expect(err).NotTo(HaveOccurred())
	return loadedReplay{Fingerprint: "0", Game: g, Replay: r}
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
		v.act = fa
	})

	// watch opens match 0's replay from its page and shows it built.
	watch := func() *replayState {
		v.page = pageMatches
		v.openMatch(v.snap.Matches[0])
		v.setTimeline(matchTimeline{Fingerprint: "0", Replayable: true})
		tt.Frame()
		Expect(tt.Click("Watch replay")).To(Succeed())
		Expect(fa.replays).To(Equal([]string{"0"}))
		Expect(tt.HasText("Building the replay")).To(BeTrue())
		v.setReplay(sixTurns())
		tt.Frame()
		return v.detail.replay
	}

	It("opens from the match page without the sidebar, and Back returns to the match", func() {
		rs := watch()
		Expect(rs.step).To(Equal(0))
		for _, text := range []string{"Setup", "Restart this turn", "Turn 3 · Bob", "Played Kyo", "Step 1 of 30"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("Overview")).To(BeFalse())

		Expect(tt.Click("Back to match")).To(Succeed())
		Expect(v.detail.replay).To(BeNil())
		Expect(v.detail.fingerprint).To(Equal("0"))
		Expect(tt.HasText("Back to matches")).To(BeTrue())
	})

	It("keeps Watch replay off until the log is read, and for a log without moves", func() {
		v.page = pageMatches
		v.openMatch(v.snap.Matches[0])
		tt.Frame()
		Expect(tt.Click("Watch replay")).To(Succeed())
		v.setTimeline(matchTimeline{Fingerprint: "0"})
		tt.Frame()
		Expect(tt.Click("Watch replay")).To(Succeed())
		Expect(fa.replays).To(BeEmpty())
		Expect(v.detail.replay).To(BeNil())
	})

	It("says why a game without moves can't be replayed", func() {
		v.page = pageMatches
		v.openReplay(v.snap.Matches[0], -1)
		v.setReplay(loadedReplay{Fingerprint: "0", Err: replay.ErrNoMoves})
		tt.Frame()
		Expect(tt.HasText(noMovesText)).To(BeTrue())
	})

	It("opens from the Matches row menu", func() {
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(tt.RightClick("19:00")).To(Succeed())
		Expect(tt.ChooseMenuItem("Watch replay")).To(Succeed())
		Expect(fa.replays).To(Equal([]string{"0"}))
		Expect(v.detail.fingerprint).To(Equal("0"))
		Expect(v.detail.replay).NotTo(BeNil())
	})

	It("opens a key moment's pin at that moment's step", func() {
		l := sixTurns()
		line := l.Replay.Steps[19].Lines[0]
		v.page = pageMatches
		v.openMatch(v.snap.Matches[0])
		v.setTimeline(matchTimeline{
			Fingerprint: "0", Replayable: true, Me: []int{5, 4, 3}, Opp: []int{5, 5, 4},
			Moments: []simlog.Moment{{
				Turn: 5, Player: "Bob#2222", Line: line,
				Reasons: []simlog.Reason{{Kind: simlog.ReasonLifeLost, Player: "Alice#1111", N: 2}},
			}},
		})
		tt.Frame()
		Expect(tt.Click("Replay moment 1")).To(Succeed())
		Expect(fa.replays).To(Equal([]string{"0"}))
		v.setReplay(l)
		tt.Frame()
		Expect(v.detail.replay.step).To(Equal(19))
		Expect(tt.HasText("Rocks D. Xebec attacked Monkey D. Garp")).To(BeTrue())
	})

	It("steps with the buttons and the arrow keys, and goes to either end", func() {
		rs := watch()
		Expect(tt.Click("Next step")).To(Succeed())
		Expect(rs.step).To(Equal(1))
		tt.Key(0, ui.KeyRight)
		Expect(rs.step).To(Equal(2))
		tt.Key(0, ui.KeyLeft)
		Expect(tt.Click("Previous step")).To(Succeed())
		Expect(rs.step).To(Equal(0))
		tt.Key(0, ui.KeyLeft)
		Expect(rs.step).To(Equal(0))
		tt.Key(0, ui.KeyEnd)
		Expect(rs.step).To(Equal(29))
		tt.Key(0, ui.KeyHome)
		Expect(rs.step).To(Equal(0))
	})

	It("moves by turn with the buttons and keys, and restarts the turn", func() {
		rs := watch()
		tt.Key(0, ui.KeyPageDown)
		Expect(rs.step).To(Equal(4))
		Expect(tt.HasText("Turn 2")).To(BeTrue())
		Expect(tt.Click("Next turn")).To(Succeed())
		Expect(rs.step).To(Equal(5))
		tt.Key(0, ui.KeyPageUp)
		Expect(rs.step).To(Equal(4))
		Expect(tt.HasText("Setup")).To(BeTrue())
		Expect(tt.Click("Previous turn")).To(Succeed())
		Expect(rs.step).To(Equal(0))
		rs.jump(27)
		tt.Frame()
		Expect(tt.HasText("Counter Step")).To(BeTrue())
		Expect(tt.HasText("Your turn")).To(BeTrue())
		rs.jump(10)
		tt.Frame()
		Expect(tt.HasText("Turn 3")).To(BeTrue())
		Expect(tt.HasText("Attack Step")).To(BeTrue())
		Expect(tt.Click("Restart this turn")).To(Succeed())
		Expect(rs.step).To(Equal(7))
	})

	It("jumps to a step clicked in the action list", func() {
		rs := watch()
		Expect(tt.Click("Played Kyo")).To(Succeed())
		Expect(rs.step).To(Equal(8))
		Expect(tt.HasText("Main")).To(BeTrue())
		Expect(tt.Click("Setup steps")).To(Succeed())
		Expect(rs.step).To(Equal(0))
	})

	It("shows an effect during an attack as a step of the attack", func() {
		rs := watch()
		rs.jump(26)
		tt.Frame()
		Expect(tt.HasText("Attack Step")).To(BeTrue())
		Expect(tt.HasText("Charlotte Linlin: +1000 to Rocks D. Xebec (trashed Wang Zhi)")).To(BeTrue())
		Expect(tt.HasText("+1000")).To(BeTrue())
	})

	It("shows statuses as chips, and both powers during an attack", func() {
		rs := watch()
		rs.jump(27)
		tt.Frame()
		Expect(tt.HasText("Counter Step")).To(BeTrue())
		Expect(tt.HasText("+2000")).To(BeTrue())
		Expect(tt.HasText("6000")).To(BeTrue())
		Expect(tt.HasText("7000")).To(BeTrue())
	})

	It("shows a card clicked on the mat in the inspector", func() {
		rs := watch()
		rs.jump(1)
		tt.Frame()
		b := rs.cur().Board
		leader := v.snap.card(b.Side(1).Leader.Code).Name
		Expect(tt.Click(leader)).To(Succeed())
		Expect(tt.HasText("Selected card")).To(BeTrue())
	})

	It("shows a deck in order and a trash by card when the pile is clicked on the mat", func() {
		rs := watch()
		rs.jump(len(rs.r.Steps) - 1)
		tt.Frame()
		Expect(tt.Click("Your deck")).To(Succeed())
		Expect(tt.HasText("Top first. 1 is the next draw.")).To(BeTrue())
		Expect(rs.pile).To(Equal(pileRef{kind: pileDeck, mine: true}))

		board := rs.cur().Board
		trash := len(board.Side(1).Trash)
		Expect(trash).To(BeNumerically(">", 0))
		Expect(tt.Click(fmt.Sprintf("Trash %d", trash))).To(Succeed())
		Expect(rs.pile).To(Equal(pileRef{kind: pileTrash, mine: true}))
		tt.Frame()
		Expect(tt.HasText("Last trashed")).To(BeTrue())

		Expect(tt.Click("Show this step")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("This step")).To(BeTrue())
	})

	It("shows a life pile top first when it's clicked on the mat", func() {
		rs := watch()
		rs.jump(len(rs.r.Steps) - 1)
		tt.Frame()
		board := rs.cur().Board
		life := len(board.Side(1).Life)
		Expect(life).To(BeNumerically(">", 0))
		Expect(tt.Click("Your life")).To(Succeed())
		tt.Frame()
		Expect(rs.pile).To(Equal(pileRef{kind: pileLife, mine: true}))
		Expect(tt.HasText("Top first. 1 is the next to be taken.")).To(BeTrue())
		Expect(tt.HasText(fmt.Sprintf("Life %d", life))).To(BeTrue())
	})

	It("keeps the controls in place from step to step", func() {
		rs := watch()
		var at []ui.Rect
		for _, i := range []int{1, len(rs.r.Steps) / 2, len(rs.r.Steps) - 1} {
			rs.jump(i)
			tt.Frame()
			r, ok := tt.Find("Next step")
			Expect(ok).To(BeTrue())
			at = append(at, r)
		}
		Expect(at[1]).To(Equal(at[0]))
		Expect(at[2]).To(Equal(at[0]))
	})

	It("glides cards on a single step only, and not at the fastest speed", func() {
		rs := watch()
		rs.jump(1)
		Expect(rs.glide).To(Equal(glideTime(false, 0)))
		Expect(rs.glide).To(BeNumerically(">", 0))
		rs.jump(5)
		Expect(rs.glide).To(BeZero())
		rs.speed, rs.playing = 2, true
		rs.jump(6)
		Expect(rs.glide).To(BeZero())
		rs.speed = 1
		rs.jump(7)
		Expect(rs.glide).To(BeNumerically("<", glideTime(false, 0)))
	})

	It("plays a step per tick until paused", func() {
		rs := watch()
		Expect(tt.Click("Play")).To(Succeed())
		Expect(rs.playing).To(BeTrue())
		fa.fire()
		Expect(rs.step).To(Equal(1))
		fa.fire()
		Expect(rs.step).To(Equal(2))
		tt.Frame()
		Expect(tt.Click("Pause")).To(Succeed())
		fa.fire()
		Expect(rs.step).To(Equal(2))
		tt.Key(0, ui.KeySpace)
		Expect(rs.playing).To(BeTrue())
		Expect(tt.Click("4×")).To(Succeed())
		Expect(rs.speed).To(Equal(2))
		fa.fire()
		Expect(rs.step).To(Equal(3), fmt.Sprint(len(fa.timers)))
	})
})

var _ = Describe("battlePair", func() {
	It("draws the attack from the copy that attacked, not the first one rested", func() {
		shanks := func(id int, rested bool) placed {
			return placed{card: replay.Card{ID: id, Code: "OP17-022", Rested: rested}, mine: true}
		}
		cards := []placed{
			shanks(1001, true), shanks(1002, true), shanks(1003, false),
			{card: replay.Card{ID: 2999, Code: "OP17-039", Rested: true}},
		}
		bt := &replay.Battle{Attacker: "OP17-022", Target: "OP17-039", AttackerOwner: 1, AttackerID: 1002, DefenderID: 2999}
		a, t := battlePair(cards, bt, true)
		Expect([]int{a, t}).To(Equal([]int{1, 3}))
	})
})
