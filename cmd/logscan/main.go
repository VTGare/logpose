// Command logscan parses every combat log in a folder and reports what the parser found. It is a
// development tool for checking simlog against real logs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simfile"
	"github.com/VTGare/logpose/internal/simlog"
)

type stats struct {
	files, games, unique, rz, withFirst, mulliganBoth int
	ends                                              map[simlog.EndKind]int
	lobbies                                           map[string]int
	unknown                                           []string
	reparseFailures                                   []string
	textBytes                                         int
}

func main() {
	dir := flag.String("dir", "", "folder with the sim's AutoSaved combat logs")
	verbose := flag.Bool("v", false, "list games whose end is unknown")
	show := flag.String("show", "", "print the details of the games in one log file")
	asJSON := flag.Bool("json", false, "print one JSON line per unique game instead of the report")
	replays := flag.Bool("replay", false, "rebuild every game's board and report where it disagrees with the log")
	simDir := flag.String("sim", "", "the sim's folder, whose card data gives printed power for -replay")
	catalog := flag.String("catalog", "", "a cached tcgcsv card catalog, for printed power newer sets lack in the sim's data")
	library := flag.String("library", "", "the app's library.json of Limitless cards, whose printed power wins over the rest")
	durations := flag.String("durations", "", "phrase=duration overrides for -replay, separated by ';', as \"Buff X N=next\"")
	steps := flag.String("steps", "", "print the replay steps of the games in one log file")
	flag.Parse()
	if *steps != "" {
		if err := showSteps(*steps); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *show != "" {
		if err := showFile(*show); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: logscan -dir <CombatLogs/AutoSaved>")
		os.Exit(2)
	}
	if *replays {
		power, err := printedPower(*simDir, *catalog, *library)
		var opts []replay.Option
		if err == nil {
			opts, err = lastings(*durations)
		}
		if err == nil {
			err = replayReport(*dir, 40, power, opts)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *asJSON {
		if err := dumpJSON(*dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	s, err := scan(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report(s, *verbose)
}

func scan(dir string) (stats, error) {
	s := stats{ends: make(map[simlog.EndKind]int), lobbies: make(map[string]int)}
	files, err := eachGame(dir, func(file string, i int, g *simlog.Game, seen bool) error {
		s.games++
		if !seen {
			s.count(g, fmt.Sprintf("%s#%d", file, i))
		}
		return nil
	})
	s.files = files
	return s, err
}

// eachGame parses every log in dir in name order and calls fn with each game, and whether an
// earlier file had it. It returns how many files it read.
func eachGame(dir string, fn func(file string, i int, g *simlog.Game, seen bool) error) (int, error) {
	paths, err := simfile.CombatLogs(dir)
	if err != nil {
		return 0, err
	}
	seen := make(map[string]bool)
	for n, path := range paths {
		games, err := parseFile(path)
		if err != nil {
			return n, fmt.Errorf("%s: %w", path, err)
		}
		for i := range games {
			g := &games[i]
			if err := fn(filepath.Base(path), i, g, seen[g.Fingerprint]); err != nil {
				return n, err
			}
			seen[g.Fingerprint] = true
		}
	}
	return len(paths), nil
}

func parseFile(path string) ([]simlog.Game, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return simlog.Parse(f)
}

func (s *stats) count(g *simlog.Game, name string) {
	s.unique++
	s.textBytes += len(g.Text)
	if again, err := simlog.ParseOne(g.Text); err != nil ||
		again.Fingerprint != g.Fingerprint || again.End != g.End || again.Lobby != g.Lobby {
		s.reparseFailures = append(s.reparseFailures, name)
	}
	if g.HasRZ() {
		s.rz++
	}
	if g.First != "" {
		s.withFirst++
	}
	if len(g.Mulligans) == 2 {
		s.mulliganBoth++
	}
	s.ends[g.End.Kind]++
	s.lobbies[lobbyKind(g.Lobby)]++
	if g.End.Kind == simlog.EndUnknown {
		s.unknown = append(s.unknown, fmt.Sprintf("%s turns=%d rz=%v last=%q", name, g.Turns, g.HasRZ(), lastLine(g)))
	}
}

func lobbyKind(l simlog.Lobby) string {
	if l.Code == "" {
		return "no code"
	}
	return fmt.Sprintf("%s, %d chars, starts %c", l.Role, len(l.Code), l.Code[0])
}

func lastLine(g *simlog.Game) string {
	for i := len(g.Lines) - 1; i >= 0; i-- {
		if l := g.Lines[i]; l.Kind != simlog.LinePrintout {
			return l.Text
		}
	}
	return ""
}

func report(s stats, verbose bool) {
	fmt.Printf("files %d, games %d, unique games %d (%d repeated across rematch files)\n", s.files, s.games, s.unique, s.games-s.unique)
	fmt.Printf("with RZ1 stream %d, turn order known %d, both mulligans %d\n", s.rz, s.withFirst, s.mulliganBoth)
	fmt.Printf("game text %d KB, %d games don't parse the same from their own text %v\n", s.textBytes/1024, len(s.reparseFailures), s.reparseFailures)
	fmt.Println("ends:")
	for _, k := range simlog.EndKinds {
		fmt.Printf("  %-8s %d\n", k, s.ends[k])
	}
	fmt.Println("lobbies:")
	keys := make([]string, 0, len(s.lobbies))
	for k := range s.lobbies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %-28s %d\n", k, s.lobbies[k])
	}
	if verbose {
		fmt.Println("unknown ends:")
		for _, u := range s.unknown {
			fmt.Println("  " + u)
		}
	}
}

func showFile(path string) error {
	games, err := parseFile(path)
	if err != nil {
		return err
	}
	for i, g := range games {
		fmt.Printf("game %d: sim %s, lobby %q, turns %d, first %q, end %s, loser %q\n", i, g.SimVersion, g.Lobby.Code, g.Turns, g.First, g.End.Kind, g.End.Loser)
		for _, p := range g.Players {
			fmt.Printf("  player %s owner %d leader %s mulligan %+v\n", p.ID(), p.Owner, p.Leader, g.Mulligans[p.ID()])
		}
		if n := len(g.Checks); n > 0 {
			fmt.Printf("  last checks %+v %+v\n", g.Checks[max(0, n-2)], g.Checks[n-1])
		}
	}
	return nil
}

type gameJSON struct {
	File    string   `json:"file"`
	Index   int      `json:"index"`
	Lobby   string   `json:"lobby"`
	Players []string `json:"players"`
	Leaders []string `json:"leaders"`
	First   string   `json:"first"`
	Turns   int      `json:"turns"`
	End     string   `json:"end"`
	Loser   string   `json:"loser"`
}

func dumpJSON(dir string) error {
	enc := json.NewEncoder(os.Stdout)
	_, err := eachGame(dir, func(file string, i int, g *simlog.Game, seen bool) error {
		if seen {
			return nil
		}
		out := gameJSON{File: file, Index: i, Lobby: g.Lobby.Code, First: g.First, Turns: g.Turns, End: g.End.Kind.String(), Loser: g.End.Loser}
		for _, p := range g.Players {
			out.Players = append(out.Players, p.ID())
			out.Leaders = append(out.Leaders, p.Leader)
		}
		return enc.Encode(out)
	})
	return err
}
