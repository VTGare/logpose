package shortcut

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShortcut(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shortcut Suite")
}
