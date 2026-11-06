package simlog

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSimlog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Simlog Suite")
}
