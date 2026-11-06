package store

import (
	"context"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("formats", func() {
	var (
		ctx context.Context
		st  *Store
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		st, err = Open(ctx, filepath.Join(GinkgoT().TempDir(), "logpose.db"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(st.Close()).To(Succeed()) })
	})

	day := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.Local) }

	It("lists formats oldest first and edits them", func() {
		_, err := st.AddFormat(ctx, " OP18 + EB05 ", day(20))
		Expect(err).NotTo(HaveOccurred())
		id, err := st.AddFormat(ctx, "OP17", day(1))
		Expect(err).NotTo(HaveOccurred())
		Expect(st.UpdateFormat(ctx, Format{ID: id, Name: "OP17 + EB05", Start: day(2)})).To(Succeed())

		fs, err := st.Formats(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(fs).To(HaveLen(2))
		Expect(fs[0].Name).To(Equal("OP17 + EB05"))
		Expect(fs[0].Start.Equal(day(2))).To(BeTrue())
		Expect(fs[1].Name).To(Equal("OP18 + EB05"))

		Expect(st.DeleteFormat(ctx, id)).To(Succeed())
		fs, err = st.Formats(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(fs).To(HaveLen(1))
	})

	It("refuses two formats starting at the same moment", func() {
		_, err := st.AddFormat(ctx, "A", day(1))
		Expect(err).NotTo(HaveOccurred())
		_, err = st.AddFormat(ctx, "B", day(1))
		Expect(err).To(HaveOccurred())
	})
})
