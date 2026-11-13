package matchups

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMatchups(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "matchups")
}
