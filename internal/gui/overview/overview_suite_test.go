package overview

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOverview(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "overview")
}
