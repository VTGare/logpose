package godot

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Packets below are copied from a captured OPBounty 2.7.8 session.
var _ = Describe("ParseRemoteCall", func() {
	It("reads a cached node call without arguments", func() {
		call, err := ParseRemoteCall([]byte{0x80, 0x03, 0x01})
		Expect(err).NotTo(HaveOccurred())
		Expect(call.NodeID).To(Equal(uint32(3)))
		Expect(call.MethodID).To(Equal(uint16(1)))
		Expect(call.RawArgs).To(BeTrue())
		Expect(call.Args).To(BeEmpty())
		Expect(call.NodePath).To(BeEmpty())
	})

	It("reads the inline path after the arguments", func() {
		p := append([]byte{0xa0, 0x06, 0x00, 0x00, 0x80, 0x01}, "root_main/Main/MatchResultV3\x00"...)
		call, err := ParseRemoteCall(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(call.NodePath).To(Equal("root_main/Main/MatchResultV3"))
		Expect(call.MethodID).To(Equal(uint16(1)))
		Expect(call.Args).To(BeEmpty())
	})

	It("separates the argument bytes from the inline path", func() {
		p := []byte{
			0x20, 0x17, 0x00, 0x00, 0x80, 0x00, 0x01,
			0x04, 0x00, 0x00, 0x00, 0x05, 0x00, 0x00, 0x00, '2', '.', '7', '.', '8', 0x00, 0x00, 0x00,
		}
		p = append(p, "root_main\x00"...)
		call, err := ParseRemoteCall(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(call.NodePath).To(Equal("root_main"))
		Expect(call.MethodID).To(Equal(uint16(0)))
		Expect(call.ArgCount).To(Equal(1))
		Expect(call.Args).To(HaveLen(16))
	})

	It("reads a call with an argument count", func() {
		call, err := ParseRemoteCall([]byte{0x00, 0x01, 0x8b, 0x03, 0x01})
		Expect(err).NotTo(HaveOccurred())
		Expect(call.NodeID).To(Equal(uint32(1)))
		Expect(call.MethodID).To(Equal(uint16(0x8b)))
		Expect(call.ArgCount).To(Equal(3))
		Expect(call.Args).To(Equal([]byte{0x01}))
	})

	It("rejects other commands", func() {
		_, err := ParseRemoteCall([]byte{0x02, 0x01, 'x', 0x00})
		Expect(err).To(MatchError(ContainSubstring("not a remote call")))
	})

	It("rejects truncated packets", func() {
		_, err := ParseRemoteCall([]byte{0x20, 0x17, 0x00})
		Expect(err).To(MatchError(ErrShortPacket))
	})

	It("rejects an inline path offset inside the header", func() {
		_, err := ParseRemoteCall([]byte{0xa0, 0x01, 0x00, 0x00, 0x80, 0x01, 'x', 0x00})
		Expect(err).To(MatchError(ContainSubstring("out of range")))
	})
})
