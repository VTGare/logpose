package store

import (
	"context"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Builds", func() {
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

	It("keeps a build with its core sorted, without blanks or repeats", func() {
		id, err := st.SaveBuild(ctx, Build{Leader: "OP13-001", Name: " Wano/Miracle ", Core: []string{"ST30-004", "OP01-055", " ", "ST30-004"}})
		Expect(err).NotTo(HaveOccurred())
		builds, err := st.Builds(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(builds).To(HaveLen(1))
		Expect(builds[0].ID).To(Equal(id))
		Expect(builds[0].Leader).To(Equal("OP13-001"))
		Expect(builds[0].Name).To(Equal("Wano/Miracle"))
		Expect(builds[0].Core).To(Equal([]string{"OP01-055", "ST30-004"}))
	})

	It("renames a build and replaces its core", func() {
		id, err := st.SaveBuild(ctx, Build{Leader: "OP13-001", Name: "Wano", Core: []string{"ST30-004", "OP01-055"}})
		Expect(err).NotTo(HaveOccurred())
		_, err = st.SaveBuild(ctx, Build{ID: id, Leader: "OP13-001", Name: "Miracle", Core: []string{"ST30-004"}})
		Expect(err).NotTo(HaveOccurred())
		builds, err := st.Builds(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(builds).To(HaveLen(1))
		Expect(builds[0].Name).To(Equal("Miracle"))
		Expect(builds[0].Core).To(Equal([]string{"ST30-004"}))
	})

	It("lists builds oldest first", func() {
		_, err := st.SaveBuild(ctx, Build{Leader: "OP13-001", Name: "First", Core: []string{"A-1"}})
		Expect(err).NotTo(HaveOccurred())
		_, err = st.SaveBuild(ctx, Build{Leader: "EB05-010", Name: "Second", Core: []string{"B-1"}})
		Expect(err).NotTo(HaveOccurred())
		builds, err := st.Builds(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(builds[0].Name).To(Equal("First"))
		Expect(builds[1].Name).To(Equal("Second"))
	})

	It("needs a name and at least one core card", func() {
		_, err := st.SaveBuild(ctx, Build{Leader: "OP13-001", Name: "  ", Core: []string{"A-1"}})
		Expect(err).To(MatchError(ErrNoName))
		_, err = st.SaveBuild(ctx, Build{Leader: "OP13-001", Name: "Empty", Core: []string{" "}})
		Expect(err).To(MatchError(ErrNoCore))
		builds, err := st.Builds(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(builds).To(BeEmpty())
	})

	It("deletes a build with its core", func() {
		id, err := st.SaveBuild(ctx, Build{Leader: "OP13-001", Name: "Wano", Core: []string{"A-1"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(st.DeleteBuild(ctx, id)).To(Succeed())
		builds, err := st.Builds(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(builds).To(BeEmpty())
		var cards int
		Expect(st.read.QueryRowContext(ctx, `SELECT count(*) FROM build_cards`).Scan(&cards)).To(Succeed())
		Expect(cards).To(BeZero())
	})

	It("says when the build to change or delete isn't stored", func() {
		_, err := st.SaveBuild(ctx, Build{ID: 7, Leader: "OP13-001", Name: "Gone", Core: []string{"A-1"}})
		Expect(err).To(MatchError(ErrNotFound))
		Expect(st.DeleteBuild(ctx, 7)).To(MatchError(ErrNotFound))
	})

	It("remembers a dismissed suggestion once, whatever order its cards came in", func() {
		Expect(st.DismissSuggestion(ctx, Dismissal{Leader: "OP13-001", Core: []string{"ST24-002", "OP14-013"}})).To(Succeed())
		Expect(st.DismissSuggestion(ctx, Dismissal{Leader: "OP13-001", Core: []string{"OP14-013", "ST24-002"}})).To(Succeed())
		ds, err := st.Dismissals(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(ds).To(Equal([]Dismissal{{Leader: "OP13-001", Core: []string{"OP14-013", "ST24-002"}}}))
	})
})
