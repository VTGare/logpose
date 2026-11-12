package simfile_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSimfile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Simfile Suite")
}
