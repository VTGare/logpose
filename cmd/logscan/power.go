package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/sim"
	"github.com/VTGare/logpose/internal/simlog"
)

// printedPower loads card data as the app does: the sim's OPBounty pack, a cached catalog over
// it, and the cached Limitless library over both. It returns nil when given none of them.
func printedPower(simDir, catalog, library string) (func(string) int, error) {
	if simDir == "" && catalog == "" && library == "" {
		return nil, nil
	}
	var pack *cards.DB
	if simDir != "" {
		install, err := sim.Open(simDir)
		if err != nil {
			return nil, err
		}
		if pack, err = cards.LoadPack(install.OPBountyPack()); err != nil {
			return nil, err
		}
	}
	l, err := cards.Load(pack, catalog, library)
	if err != nil {
		return nil, err
	}
	return func(code string) int { return l.DB.Card(code).Power }, nil
}

var durationNames = map[string]replay.Duration{
	"line": replay.UntilLine, "this": replay.ThisTurn, "next": replay.NextTurn,
	"opp": replay.OpponentsNextTurn, "combat": replay.Combat, "refresh": replay.UntilRefresh,
}

// lastings overrides how long phrases last, to test readings against the "vs" lines.
func lastings(spec string) ([]replay.Option, error) {
	if spec == "" {
		return nil, nil
	}
	var opts []replay.Option
	for item := range strings.SplitSeq(spec, ";") {
		phrase, name, ok := strings.Cut(item, "=")
		d, known := durationNames[name]
		if !ok || !known || !slices.Contains(replay.Phrases(), phrase) {
			return nil, fmt.Errorf("bad duration %q", item)
		}
		opts = append(opts, replay.Lasting(phrase, d))
	}
	return opts, nil
}

// powerReport counts how often the worked-out power matches the sim's "vs" lines, and sorts
// the misses.
type powerReport struct {
	compared, missed, unknown, guesses, corrected int
	byDiff                                        map[int]int
	byCard                                        map[string]int
	bySide                                        map[string]int
	examples                                      []string
}

func newPowerReport() *powerReport {
	return &powerReport{byDiff: make(map[int]int), byCard: make(map[string]int), bySide: make(map[string]int)}
}

func (p *powerReport) add(g *simlog.Game, r *replay.Replay, game string) {
	name := func(code string) string {
		if n, ok := g.CardNames[code]; ok {
			return n
		}
		return code
	}
	p.compared += r.PowerCompared
	p.guesses += r.PowerGuesses
	p.corrected += r.PowerCorrected
	for _, c := range r.PowerChecks {
		if !c.Known {
			p.unknown++
			continue
		}
		p.missed++
		p.byDiff[c.Want-c.Got]++
		p.byCard[fmt.Sprintf("%s %s %+d", c.Code, name(c.Code), c.Want-c.Got)]++
		if r.Steps[c.Step].Board.Side(c.Owner).Leader.Code == c.Code {
			p.bySide["leader"]++
		} else {
			p.bySide["character"]++
		}
		side := "defender"
		if c.Attacker {
			side = "attacker"
		}
		p.bySide[side]++
		if len(p.examples) < 25 {
			p.examples = append(p.examples, fmt.Sprintf("%s step %d %s %s: vs %d, worked out %d",
				game, c.Step, side, name(c.Code), c.Want, c.Got))
		}
	}
}

func (p *powerReport) print() {
	known := p.compared - p.unknown
	fmt.Printf("power: %d compared with vs lines, %d without card data, %d of %d match (%.1f%%)\n",
		p.compared, p.unknown, known-p.missed, known, 100*float64(known-p.missed)/float64(max(known, 1)))
	fmt.Printf("power buffs naming one of several copies %d, corrected by vs lines %d\n", p.guesses, p.corrected)
	fmt.Printf("misses by side: %v\n", p.bySide)
	printTop("misses by vs minus worked out", p.byDiff, 15)
	printTop("misses by card", p.byCard, 20)
	for _, e := range p.examples {
		fmt.Println("  " + e)
	}
}

func printTop[K comparable](title string, m map[K]int, n int) {
	type kv struct {
		k K
		v int
	}
	var s []kv
	for k, v := range m {
		s = append(s, kv{k, v})
	}
	sort.Slice(s, func(i, j int) bool { return s[i].v > s[j].v })
	fmt.Println(title + ":")
	for _, x := range s[:min(n, len(s))] {
		fmt.Printf("  %5d  %v\n", x.v, x.k)
	}
}
