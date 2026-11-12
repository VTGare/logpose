package cards

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Load", func() {
	It("layers the pack, the catalog and the library in that order", func() {
		dir := GinkgoT().TempDir()
		fetched := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		cat := &Catalog{Version: catalogVersion, Fetched: fetched, Groups: make(map[string]string), Cards: map[string]catalogCard{
			"OP01-001": {Name: "Zoro (catalog)", Type: Leader},
			"OP01-002": {Name: "Nami (catalog)"},
		}}
		Expect(cat.Save(filepath.Join(dir, "cat.json"))).To(Succeed())
		lib := newLibrary()
		lib.Cards["OP01-001"] = Card{Code: "OP01-001", Name: "Zoro (library)", Detailed: true}
		Expect(lib.Save(filepath.Join(dir, "lib.json"))).To(Succeed())
		pack := NewDB()
		pack.Name("OP01-003", "Luffy (pack)")

		l, err := Load(pack, filepath.Join(dir, "cat.json"), filepath.Join(dir, "lib.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(l.DB.Card("OP01-001").Name).To(Equal("Zoro (library)"))
		Expect(l.DB.Card("OP01-002").Name).To(Equal("Nami (catalog)"))
		Expect(l.DB.Card("OP01-003").Name).To(Equal("Luffy (pack)"))
		Expect(l.Fetched).To(BeTemporally("==", fetched))
		Expect(pack.Len()).To(Equal(1))
	})

	It("keeps the sources that load when one can't be read", func() {
		dir := GinkgoT().TempDir()
		bad := filepath.Join(dir, "cat.json")
		Expect(os.Mkdir(bad, 0o700)).To(Succeed())
		lib := newLibrary()
		lib.Cards["OP01-001"] = Card{Code: "OP01-001", Name: "Zoro"}
		Expect(lib.Save(filepath.Join(dir, "lib.json"))).To(Succeed())
		l, err := Load(nil, bad, filepath.Join(dir, "lib.json"))
		Expect(err).To(HaveOccurred())
		Expect(l.DB.Card("OP01-001").Name).To(Equal("Zoro"))
	})

	It("reads a library file that doesn't parse as an empty library", func() {
		dir := GinkgoT().TempDir()
		bad := filepath.Join(dir, "lib.json")
		Expect(os.WriteFile(bad, []byte(`{"cards": 3}`), 0o600)).To(Succeed())
		l, err := Load(nil, "", bad)
		Expect(err).NotTo(HaveOccurred())
		Expect(l.DB.Len()).To(BeZero())
		Expect(l.Checked).To(BeZero())
	})
})
