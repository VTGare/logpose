package gui

import (
	"context"
	"path/filepath"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/store"
)

var _ = Describe("app.load", func() {
	It("builds snapshots on several goroutines at once without sharing their card names", func() {
		ctx := context.Background()
		st, err := store.Open(ctx, filepath.Join(GinkgoT().TempDir(), "logpose.db"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(st.Close()).To(Succeed()) })
		a := &app{opts: Options{Store: st}}
		var wg sync.WaitGroup
		snaps := make([]*snapshot, 4)
		for i := range snaps {
			wg.Go(func() {
				defer GinkgoRecover()
				s, err := a.load(ctx)
				Expect(err).NotTo(HaveOccurred())
				snaps[i] = s
			})
		}
		wg.Wait()
		snaps[0].Cards.Name("ZZ-001", "Only in the first")
		later, err := a.load(ctx)
		Expect(err).NotTo(HaveOccurred())
		for _, s := range append(snaps[1:], later) {
			Expect(s.Cards.Card("ZZ-001").Name).NotTo(Equal("Only in the first"))
		}
	})
})
