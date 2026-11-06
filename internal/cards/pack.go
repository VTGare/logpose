package cards

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// readPackFile reads one file out of a Godot 4 .pck. The format is: "GDPC", the pack version,
// Godot's major, minor and patch version, pack flags, the files' base offset, 16 reserved
// uint32s, the file count, then per file its path length and path, offset, size, md5 and flags.
// Pack version 3 adds a uint64 directory offset after the base offset.
func readPackFile(path, want string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var head struct {
		Magic               [4]byte
		Version             uint32
		Major, Minor, Patch uint32
		Flags               uint32
		Base                uint64
	}
	if err := binary.Read(f, binary.LittleEndian, &head); err != nil {
		return nil, err
	}
	if string(head.Magic[:]) != "GDPC" {
		return nil, errors.New("cards: not a Godot pack")
	}
	skip := int64(16 * 4)
	if head.Version >= 3 {
		skip += 8
	}
	if _, err := f.Seek(skip, io.SeekCurrent); err != nil {
		return nil, err
	}
	var count uint32
	if err := binary.Read(f, binary.LittleEndian, &count); err != nil {
		return nil, err
	}
	for range count {
		e, err := readEntry(f)
		if err != nil {
			return nil, err
		}
		if strings.TrimPrefix(e.path, "res://") != want {
			continue
		}
		if e.flags&1 != 0 {
			return nil, fmt.Errorf("cards: %s is encrypted", want)
		}
		off := int64(e.offset)
		// Flag 2 means offsets are relative to the base offset.
		if head.Flags&2 != 0 {
			off += int64(head.Base)
		}
		buf := make([]byte, e.size)
		if _, err := f.ReadAt(buf, off); err != nil {
			return nil, err
		}
		return buf, nil
	}
	return nil, fmt.Errorf("cards: %s not in pack", want)
}

type packEntry struct {
	path         string
	offset, size uint64
	flags        uint32
}

func readEntry(r io.Reader) (packEntry, error) {
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return packEntry{}, err
	}
	if n > 4096 {
		return packEntry{}, errors.New("cards: bad pack entry")
	}
	name := make([]byte, n)
	if _, err := io.ReadFull(r, name); err != nil {
		return packEntry{}, err
	}
	var rest struct {
		Offset, Size uint64
		MD5          [16]byte
		Flags        uint32
	}
	if err := binary.Read(r, binary.LittleEndian, &rest); err != nil {
		return packEntry{}, err
	}
	return packEntry{path: strings.TrimRight(string(name), "\x00"), offset: rest.Offset, size: rest.Size, flags: rest.Flags}, nil
}
