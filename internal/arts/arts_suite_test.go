package arts

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestArts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Arts Suite")
}
