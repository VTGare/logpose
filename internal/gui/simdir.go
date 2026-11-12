package gui

import (
	"context"

	"github.com/VTGare/logpose/internal/sim"
	"github.com/VTGare/logpose/internal/store"
)

const (
	// keySimDir is the sim's folder, saved whenever logpose finds it.
	keySimDir = "sim_dir"
	// keySimDirChosen is a sim folder the user picked. It wins over the folder logpose finds.
	keySimDirChosen = "sim_dir_chosen"
)

// LocateSim finds the sim from dir, the -sim flag, then the folder the user chose, the running
// process, the usual folders and the folder it was last found in. It remembers the folder it
// finds, so it's known the next time logpose starts without the sim running.
func LocateSim(ctx context.Context, st *store.Store, dir string) (sim.Install, error) {
	if dir != "" {
		return sim.Open(dir)
	}
	if chosen, err := st.Setting(ctx, keySimDirChosen); err == nil && chosen != "" {
		if in, err := sim.Open(chosen); err == nil {
			return in, nil
		}
	}
	in, err := sim.Locate()
	if err != nil {
		if saved, serr := st.Setting(ctx, keySimDir); serr == nil && saved != "" {
			in, err = sim.Open(saved)
		}
	}
	if err != nil {
		return sim.Install{}, err
	}
	return in, rememberSim(ctx, st, in, false)
}

// rememberSim saves the sim's folder, and with chosen as the folder the user picked, which wins
// over the one logpose finds.
func rememberSim(ctx context.Context, st *store.Store, in sim.Install, chosen bool) error {
	if err := st.SetSetting(ctx, keySimDir, in.Root); err != nil {
		return err
	}
	if chosen {
		return st.SetSetting(ctx, keySimDirChosen, in.Root)
	}
	return nil
}

func forgetChosenSim(ctx context.Context, st *store.Store) error {
	return st.SetSetting(ctx, keySimDirChosen, "")
}
