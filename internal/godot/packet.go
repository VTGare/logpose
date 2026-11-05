package godot

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

type Command uint8

const (
	CommandRemoteCall Command = iota
	CommandSimplifyPath
	CommandConfirmPath
	CommandRaw
	CommandSpawn
	CommandDespawn
	CommandSync
	CommandSys
)

const (
	commandMask    = 0x0F
	nodeWidthShift = 4
	methodWideFlag = 0x40
	noArgsFlag     = 0x80
	inlinePathFlag = 0x80000000
	inlinePathMask = 0x7FFFFFFF
)

var ErrShortPacket = errors.New("godot: packet too short")

type RemoteCall struct {
	NodeID uint32
	// NodePath is set when the sender hasn't had the path confirmed yet and sent it inline.
	NodePath string
	MethodID uint16
	// RawArgs holds a single PackedByteArray argument, or nothing, when the call carries no
	// argument count.
	RawArgs  bool
	ArgCount int
	Args     []byte
}

func PacketCommand(p []byte) (Command, error) {
	if len(p) == 0 {
		return 0, ErrShortPacket
	}
	return Command(p[0] & commandMask), nil
}

func ParseRemoteCall(p []byte) (RemoteCall, error) {
	cmd, err := PacketCommand(p)
	if err != nil {
		return RemoteCall{}, err
	}
	if cmd != CommandRemoteCall {
		return RemoteCall{}, fmt.Errorf("godot: command %d is not a remote call", cmd)
	}

	hdr := p[0]
	r := reader{buf: p, off: 1}
	var call RemoteCall

	switch (hdr >> nodeWidthShift) & 0x3 {
	case 0:
		call.NodeID = uint32(r.u8())
	case 1:
		call.NodeID = uint32(r.u16())
	default:
		call.NodeID = r.u32()
	}
	if hdr&methodWideFlag != 0 {
		call.MethodID = r.u16()
	} else {
		call.MethodID = uint16(r.u8())
	}

	end := len(p)
	if call.NodeID&inlinePathFlag != 0 {
		start := int(call.NodeID & inlinePathMask)
		if start < r.off || start >= len(p) {
			return RemoteCall{}, fmt.Errorf("godot: inline path offset %d out of range", start)
		}
		path, ok := cstring(p[start:])
		if !ok {
			return RemoteCall{}, errors.New("godot: inline path is not terminated")
		}
		call.NodePath = path
		end = start
	}

	if hdr&noArgsFlag != 0 {
		call.RawArgs = true
	} else {
		call.ArgCount = int(r.u8())
	}
	if r.err != nil {
		return RemoteCall{}, r.err
	}
	if r.off > end {
		return RemoteCall{}, ErrShortPacket
	}
	call.Args = p[r.off:end]
	return call, nil
}

func cstring(b []byte) (string, bool) {
	i := bytes.IndexByte(b, 0)
	if i < 0 {
		return "", false
	}
	return string(b[:i]), true
}

type reader struct {
	buf []byte
	off int
	err error
}

func (r *reader) take(n int) []byte {
	if r.err != nil {
		return make([]byte, n)
	}
	if r.off+n > len(r.buf) {
		r.err = ErrShortPacket
		return make([]byte, n)
	}
	b := r.buf[r.off : r.off+n]
	r.off += n
	return b
}

func (r *reader) u8() uint8   { return r.take(1)[0] }
func (r *reader) u16() uint16 { return binary.LittleEndian.Uint16(r.take(2)) }
func (r *reader) u32() uint32 { return binary.LittleEndian.Uint32(r.take(4)) }
