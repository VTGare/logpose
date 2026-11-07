package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VTGare/logpose/internal/replay"
)

// replayReport rebuilds every unique game's board and counts where it disagrees with the log.
func replayReport(dir string, examples int, power func(string) int) error {
	paths, err := filepath.Glob(filepath.Join(dir, "*.log"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	seen := make(map[string]bool)
	var games, clean, countClean, moves, checked, steps, guesses, corrected int
	kinds := make(map[replay.IssueKind]int)
	stepKinds := make(map[replay.Kind]int)
	gamesWith := make(map[replay.IssueKind]int)
	var shown []string
	pw := newPowerReport()
	for _, path := range paths {
		gs, err := parseFile(path)
		if err != nil {
			return err
		}
		for i := range gs {
			g := &gs[i]
			if seen[g.Fingerprint] {
				continue
			}
			seen[g.Fingerprint] = true
			r, err := replay.Build(g, power)
			if err != nil {
				fmt.Printf("%s#%d: %v\n", filepath.Base(path), i, err)
				continue
			}
			pw.add(r, fmt.Sprintf("%s#%d", filepath.Base(path), i))
			games++
			moves += len(g.Moves)
			checked += len(g.Checks)
			steps += len(r.Steps)
			for _, st := range r.Steps {
				stepKinds[st.Kind]++
			}
			guesses += r.Guesses
			corrected += r.Corrected
			if len(r.Issues) == 0 {
				clean++
			}
			if r.Unverified < 0 {
				countClean++
			}
			got := make(map[replay.IssueKind]bool)
			for _, is := range r.Issues {
				kinds[is.Kind]++
				if !got[is.Kind] {
					got[is.Kind] = true
					gamesWith[is.Kind]++
					if len(shown) < examples {
						shown = append(shown, fmt.Sprintf("%s#%d %s", filepath.Base(path), i, is))
					}
				}
			}
		}
	}
	fmt.Printf("games %d, moves %d, checks %d, steps %d\n", games, moves, checked, steps)
	fmt.Printf("attacks %d, blocks %d, counters %d, effects during attacks %d, damage steps %d\n",
		stepKinds[replay.KindAttack], stepKinds[replay.KindBlock], stepKinds[replay.KindCounter],
		stepKinds[replay.KindBattleEffect], stepKinds[replay.KindDamage])
	fmt.Printf("games with no issue %d, games whose counts match every check %d\n", clean, countClean)
	fmt.Printf("lines naming one of several copies %d, of which later attacks corrected %d\n", guesses, corrected)
	for _, k := range []replay.IssueKind{replay.IssueCount, replay.IssueMove, replay.IssueSnapshot, replay.IssueRest, replay.IssueNoAttack, replay.IssueTwice} {
		fmt.Printf("  %-9s %5d issues in %d games\n", k, kinds[k], gamesWith[k])
	}
	for _, s := range shown {
		fmt.Println("  " + s)
	}
	if power != nil {
		pw.print()
	}
	return nil
}

// showSteps prints every step of the games in one log, with each side's counts after it.
func showSteps(path string) error {
	gs, err := parseFile(path)
	if err != nil {
		return err
	}
	for gi := range gs {
		g := &gs[gi]
		r, err := replay.Build(g, nil)
		if err != nil {
			return err
		}
		fmt.Printf("game %d: %d steps, unverified from %d\n", gi, len(r.Steps), r.Unverified)
		for i, s := range r.Steps {
			title := s.Title
			if s.Result != "" {
				title += " → " + s.Result
			}
			fmt.Printf("%3d T%-2d %-6s %-16s %s  [%d moves]\n", i, s.Turn, s.Phase, short(s.Player), title, len(s.Moves))
			for o := 1; o <= 2; o++ {
				fmt.Printf("        %s\n", sideLine(s.Board.Side(o)))
			}
		}
		for _, is := range r.Issues {
			fmt.Println("  " + is.String())
		}
	}
	return nil
}

func short(id string) string {
	name, _, _ := strings.Cut(id, "#")
	return strings.TrimSuffix(name, "\u200b")
}

func sideLine(s *replay.Side) string {
	var chars []string
	for _, c := range s.Characters {
		chars = append(chars, cardText(c))
	}
	return fmt.Sprintf("%-8s L%s life %d hand %d deck %d trash %d don %d/%d+%d [%s] stage %d",
		short(s.Player), cardText(s.Leader), len(s.Life), len(s.Hand), s.DeckCount(), len(s.Trash),
		s.DonActive(), s.DonRested(), s.DonAttached(), strings.Join(chars, " "), len(s.Stage))
}

func cardText(c replay.Card) string {
	t := c.Code
	if c.Rested {
		t += "(R)"
	}
	if c.Don > 0 {
		t += fmt.Sprintf("+%d", c.Don)
	}
	return t
}
