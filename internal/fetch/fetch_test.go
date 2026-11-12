package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Get", func() {
	var srv *httptest.Server

	BeforeEach(func() {
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", 10)))
		}))
		DeferCleanup(srv.Close)
	})

	It("reads a body up to the limit", func() {
		Expect(Get(context.Background(), srv.Client(), srv.URL, 10)).To(HaveLen(10))
	})

	It("fails on a body over the limit", func() {
		_, err := Get(context.Background(), srv.Client(), srv.URL, 9)
		Expect(err).To(MatchError(ContainSubstring("over 9 bytes")))
	})
})
