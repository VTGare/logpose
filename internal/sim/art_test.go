package sim_test

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"time"

	"github.com/VTGare/logpose/internal/sim"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SimArt", func() {
	var (
		art   *sim.SimArt
		cards string
	)
	card := func(c color.Color) image.Image {
		img := image.NewRGBA(image.Rect(0, 0, 60, 84))
		for y := range 84 {
			for x := range 60 {
				img.Set(x, y, c)
			}
		}
		return img
	}
	path := func(name string) string { return filepath.Join(cards, "OP07", name) }
	exists := func(name string) bool {
		_, err := os.Stat(path(name))
		return err == nil
	}
	inspect := func() sim.ArtState {
		GinkgoHelper()
		st, err := art.Inspect("OP07-019")
		Expect(err).NotTo(HaveOccurred())
		return st
	}

	BeforeEach(func() {
		root := GinkgoT().TempDir()
		cards = filepath.Join(root, "OPTCGSim_Data", "StreamingAssets", "Cards")
		Expect(os.MkdirAll(filepath.Join(cards, "OP07"), 0o700)).To(Succeed())
		Expect(os.WriteFile(path("OP07-019.png"), []byte("own"), 0o600)).To(Succeed())
		art = &sim.SimArt{Install: sim.Install{Root: root}, Dir: filepath.Join(root, "logpose")}
	})

	It("shows the sim's own art until a printing is used", func() {
		Expect(inspect()).To(Equal(sim.ArtState{Choice: sim.ArtChoice{Kind: sim.ArtOwn}}))
		Expect(art.UsePrint("OP07-019", "OP07-019_p1", card(color.White))).To(Succeed())
		st := inspect()
		Expect(st.Choice).To(Equal(sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP07-019_p1"}))
		Expect(st.Override).To(Equal(path("OP07-019_OVERRIDE.png")))
		Expect(st.OverrideHash).To(HaveLen(64))
		Expect(st.OwnFile).To(BeEmpty())
		Expect(art.Install.Override("OP07-019")).To(Equal(path("OP07-019_OVERRIDE.png")))
		Expect(art.Install.Overrides()).To(Equal([]string{"OP07-019"}))

		f, err := os.Open(path("OP07-019_small_OVERRIDE.jpg"))
		Expect(err).NotTo(HaveOccurred())
		small, err := jpeg.Decode(f)
		Expect(f.Close()).To(Succeed())
		Expect(err).NotTo(HaveOccurred())
		Expect(small.Bounds().Size()).To(Equal(image.Pt(120, 167)))

		Expect(art.UseOwn("OP07-019")).To(Succeed())
		Expect(inspect()).To(Equal(sim.ArtState{Choice: sim.ArtChoice{Kind: sim.ArtOwn}}))
		Expect(exists("OP07-019_OVERRIDE.png")).To(BeFalse())
		Expect(exists("OP07-019_small_OVERRIDE.jpg")).To(BeFalse())
		Expect(exists("OP07-019.png")).To(BeTrue())
	})

	It("keeps the user's own file before replacing it, and puts it back", func() {
		Expect(os.WriteFile(path("OP07-019_OVERRIDE.jpg"), []byte("mine"), 0o600)).To(Succeed())
		st := inspect()
		Expect(st.Choice).To(Equal(sim.ArtChoice{Kind: sim.ArtFile}))
		Expect(st.OwnFile).To(Equal(path("OP07-019_OVERRIDE.jpg")))
		Expect(st.Override).To(Equal(path("OP07-019_OVERRIDE.jpg")))
		Expect(st.OverrideHash).To(HaveLen(64))

		Expect(art.UsePrint("OP07-019", "OP07-019", card(color.Black))).To(Succeed())
		Expect(exists("OP07-019_OVERRIDE.jpg")).To(BeFalse())
		st = inspect()
		Expect(st.Choice).To(Equal(sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP07-019"}))
		Expect(filepath.Base(st.OwnFile)).To(Equal("OP07-019_OVERRIDE.jpg"))
		Expect(os.ReadFile(st.OwnFile)).To(Equal([]byte("mine")))

		Expect(art.UseFile("OP07-019")).To(Succeed())
		st = inspect()
		Expect(st.Choice).To(Equal(sim.ArtChoice{Kind: sim.ArtFile}))
		Expect(st.OwnFile).To(Equal(path("OP07-019_OVERRIDE.jpg")))
		Expect(os.ReadFile(path("OP07-019_OVERRIDE.jpg"))).To(Equal([]byte("mine")))
		Expect(exists("OP07-019_OVERRIDE.png")).To(BeFalse())
		Expect(exists("OP07-019_small_OVERRIDE.jpg")).To(BeFalse())
	})

	It("keeps two different files of the user's under different names", func() {
		Expect(os.WriteFile(path("OP07-019_OVERRIDE.png"), []byte("first"), 0o600)).To(Succeed())
		Expect(art.UseOwn("OP07-019")).To(Succeed())
		Expect(os.WriteFile(path("OP07-019_OVERRIDE.png"), []byte("second"), 0o600)).To(Succeed())
		Expect(art.UseOwn("OP07-019")).To(Succeed())
		kept, err := filepath.Glob(filepath.Join(art.Dir, "kept", "OP07-019_OVERRIDE*.png"))
		Expect(err).NotTo(HaveOccurred())
		Expect(kept).To(HaveLen(2))
		Expect(os.ReadFile(inspect().OwnFile)).To(Equal([]byte("second")))
	})

	It("offers the user's file kept last when a JPEG and a PNG were kept", func() {
		Expect(os.WriteFile(path("OP07-019_OVERRIDE.jpg"), []byte("jpeg"), 0o600)).To(Succeed())
		Expect(art.UseOwn("OP07-019")).To(Succeed())
		Expect(os.WriteFile(path("OP07-019_OVERRIDE.png"), []byte("png"), 0o600)).To(Succeed())
		Expect(art.UseOwn("OP07-019")).To(Succeed())
		jpg := filepath.Join(art.Dir, "kept", "OP07-019_OVERRIDE.jpg")
		png := filepath.Join(art.Dir, "kept", "OP07-019_OVERRIDE.png")
		old := time.Now().Add(-time.Hour)
		Expect(os.Chtimes(jpg, old, old)).To(Succeed())
		Expect(inspect().OwnFile).To(Equal(png))

		Expect(os.Chtimes(png, old.Add(-time.Hour), old.Add(-time.Hour))).To(Succeed())
		Expect(inspect().OwnFile).To(Equal(jpg))
	})

	It("reports a user's file as the printing it was matched to by hash", func() {
		Expect(os.WriteFile(path("OP07-019_OVERRIDE.png"), []byte("alt art"), 0o600)).To(Succeed())
		st := inspect()
		Expect(art.RecordMatch(st.OverrideHash, "OP07-019_p1")).To(Succeed())
		st = inspect()
		Expect(st.Choice).To(Equal(sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP07-019_p1"}))
		Expect(st.OwnFile).To(BeEmpty())
	})
})
