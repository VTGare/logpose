package formats

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFormats(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "formats")
}
