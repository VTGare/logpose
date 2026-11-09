package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/sim"
)

// printedPower loads card data as the app does: the sim's OPBounty pack, a cached catalog over
// it, and the cached Limitless library over both. It returns nil when given none of them.
func printedPower(simDir, catalog, library string) (func(string) int, error) {
	if simDir == "" && catalog == "" && library == "" {
		return nil, nil
	}
	db := cards.NewDB()
	if simDir != "" {
		install, err := sim.Open(simDir)
		if err != nil {
			return nil, err
		}
		if db, err = cards.LoadPack(install.OPBountyPack()); err != nil {
			return nil, err
		}
	}
	if catalog != "" {
		cat, err := cards.LoadCatalog(catalog)
		if err != nil {
			return nil, err
		}
		cat.Apply(db)
	}
	if library != "" {
		lib, err := cards.LoadLibrary(library)
		if err != nil {
			return nil, err
		}
		lib.Apply(db)
	}
	return func(code string) int { return db.Card(code).Power }, nil
}

var durationNames = map[string]replay.Duration{
	"line": replay.UntilLine, "this": replay.ThisTurn, "next": replay.NextTurn,
	"opp": replay.OpponentsNextTurn, "combat": replay.Combat, "refresh": replay.UntilRefresh,
}

// setDurations overrides how long phrases last, to test readings against the "vs" lines.
func setDurations(spec string) error {
	if spec == "" {
		return nil
	}
	for _, item := range strings.Split(spec, ";") {
		phrase, name, ok := strings.Cut(item, "=")
		d, known := durationNames[name]
		if _, listed := replay.Durations[replay.Phrase(phrase)]; !ok || !known || !listed {
			return fmt.Errorf("bad duration %q", item)
		}
		replay.Durations[replay.Phrase(phrase)] = d
	}
	return nil
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

func (p *powerReport) add(r *replay.Replay, game string) {
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
		p.byCard[fmt.Sprintf("%s %s %+d", c.Code, r.Name(c.Code), c.Want-c.Got)]++
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
				game, c.Step, side, r.Name(c.Code), c.Want, c.Got))
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
