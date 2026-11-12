package live

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"

	"github.com/VTGare/logpose/internal/simfile"
)

// Follower reads Player.log as the sim writes it. It opens the file for each read and closes it
// again, because the sim renames it to Player-prev.log when it starts.
type Follower struct {
	Path     string
	Interval time.Duration
	// Running reports whether the sim is running. While it isn't, the state is Idle whatever the
	// log says, since the log ends wherever the sim stopped. Nil means always running.
	Running func() bool
	// OnChange gets each new state. It runs on the follower's goroutine.
	OnChange func(State)
	// OnSaved gets each combat log the sim saved, including those already in the log when the
	// follower starts. It runs on the follower's goroutine.
	OnSaved func(Saved)

	tracker Tracker
	offset  int64
	info    os.FileInfo
	partial []byte
	last    State
	sent    bool
}

func (f *Follower) Run(ctx context.Context) error {
	interval := f.Interval
	if interval == 0 {
		interval = time.Second
	}
	for {
		f.poll()
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

func (f *Follower) poll() {
	f.read()
	for _, sv := range f.tracker.TakeSaved() {
		if f.OnSaved != nil {
			f.OnSaved(sv)
		}
	}
	s := f.tracker.State()
	if f.Running != nil && !f.Running() {
		s = State{}
	}
	if f.sent && same(s, f.last) {
		return
	}
	f.last, f.sent = s, true
	if f.OnChange != nil {
		f.OnChange(s)
	}
}

func (f *Follower) read() {
	file, err := simfile.Open(f.Path)
	if err != nil {
		f.restart(nil)
		return
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return
	}
	if f.info != nil && (!os.SameFile(f.info, info) || info.Size() < f.offset) {
		f.restart(info)
	}
	f.info = info
	if info.Size() == f.offset {
		return
	}
	if _, err := file.Seek(f.offset, io.SeekStart); err != nil {
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, info.Size()-f.offset))
	if err != nil {
		return
	}
	f.offset += int64(len(data))
	f.feed(data)
}

func (f *Follower) restart(info os.FileInfo) {
	f.tracker = Tracker{}
	f.offset, f.info, f.partial = 0, info, nil
}

func (f *Follower) feed(data []byte) {
	data = append(f.partial, data...)
	for {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			break
		}
		f.tracker.Line(string(data[:i]))
		data = data[i+1:]
	}
	f.partial = append([]byte(nil), data...)
}
