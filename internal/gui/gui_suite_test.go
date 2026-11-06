package gui

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGUI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GUI Suite")
}
