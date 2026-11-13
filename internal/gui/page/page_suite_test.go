package page

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "page")
}
