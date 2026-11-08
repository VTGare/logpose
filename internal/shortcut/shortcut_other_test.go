//go:build !windows

package shortcut

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("shortcuts outside Windows", func() {
	It("can't be made", func() {
		_, err := Path(Desktop, "logpose")
		Expect(errors.Is(err, errors.ErrUnsupported)).To(BeTrue())
		Expect(Exists(StartMenu, "logpose")).To(BeFalse())
		Expect(errors.Is(Create(StartMenu, Link{Name: "logpose"}), errors.ErrUnsupported)).To(BeTrue())
	})
})
