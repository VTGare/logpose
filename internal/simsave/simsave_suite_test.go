package simsave

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSimsave(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Simsave Suite")
}
