package godot

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGodot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Godot Suite")
}
