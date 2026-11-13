package decks

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDecks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "decks")
}
