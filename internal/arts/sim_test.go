package arts

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
)

func solidRect(w, h int, col color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = col.R, col.G, col.B, col.A
	}
	return img
}

func writePNGFile(path string, img image.Image) {
	GinkgoHelper()
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	f, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	Expect(png.Encode(f, img)).To(Succeed())
	Expect(f.Close()).To(Succeed())
}

func writeJPEGFile(path string, img image.Image) {
	GinkgoHelper()
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	f, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	Expect(jpeg.Encode(f, img, &jpeg.Options{Quality: 90})).To(Succeed())
	Expect(f.Close()).To(Succeed())
}

var _ = Describe("Ref", func() {
	It("round-trips every reference kind through its string form", func() {
		cases := []string{
			"OP07-021",
			ExactPrintRef("OP07-021").String(),
			"OP07-021_p3_bd",
			SimOwnRef("OP07-021").String(),
			FileRef(`C:\art\zoro.jpg`).String(),
			DonRef().String(),
			DonBackRef().String(),
		}
		for _, s := range cases {
			Expect(ParseRef(s).String()).To(Equal(s))
		}
		Expect(ExactPrintRef("OP07-021").IsExact()).To(BeTrue())
		Expect(PrintRef("OP07-021").IsExact()).To(BeFalse())
		Expect(ExactPrintRef(SimOwnRef("OP07-021").String())).To(Equal(SimOwnRef("OP07-021")))
		Expect(ExactPrintRef(FileRef("/art/zoro.jpg").String())).To(Equal(FileRef("/art/zoro.jpg")))
		Expect(ExactPrintRef("")).To(Equal(PrintRef("")))
	})

	It("reports the card code and default saved file name", func() {
		Expect(ParseRef("OP07-021_p3_bd").Card()).To(Equal("OP07-021"))
		Expect(ExactPrintRef("OP07-021").Card()).To(Equal("OP07-021"))
		Expect(SimOwnRef("OP07-021").Card()).To(Equal("OP07-021"))
		Expect(FileRef("/art/zoro.jpg").Card()).To(BeEmpty())
		Expect(DonRef().Card()).To(BeEmpty())
		Expect(DonBackRef().Card()).To(BeEmpty())

		Expect(ExactPrintRef("OP07-021").FileName()).To(Equal("OP07-021.png"))
		Expect(SimOwnRef("OP07-021").FileName()).To(Equal("OP07-021_sim.png"))
		Expect(FileRef("/art/zoro.jpg").FileName()).To(Equal("zoro.png"))
		Expect(PrintRef("OP07-021_p1").FileName()).To(Equal("OP07-021_p1.png"))
	})
})

var _ = Describe("SimManager", func() {
	var (
		simRoot string
		simDir  string
		artDir  string
		store   *Store
	)

	cardPath := func(root, set, name string) string {
		return filepath.Join(root, "OPTCGSim_Data", "StreamingAssets", "Cards", set, name)
	}

	BeforeEach(func() {
		base := GinkgoT().TempDir()
		simRoot = filepath.Join(base, "sim")
		simDir = filepath.Join(base, "sim-art")
		artDir = filepath.Join(base, "arts")
		store = NewStore(artDir)
		writePNGFile(cardPath(simRoot, "OP07", "OP07-021.png"), solidRect(80, 112, color.RGBA{200, 0, 0, 255}))
		writeJPEGFile(cardPath(simRoot, "OP07", "OP07-021_small.jpg"), solidRect(60, 84, color.RGBA{180, 20, 20, 255}))
	})

	Describe("NoSim handling", func() {
		It("reports NoSim on Inspect, ErrNotFound on Use and empty Scan without an install", func() {
			mgr := NewSimManager(nil, simDir, store, nil)
			card := cards.Card{Code: "OP07-021", Name: "Urouge"}

			Expect(mgr.Inspect(context.Background(), card)).To(Equal(SimStatus{NoSim: true}))
			Expect(mgr.Scan()).To(BeEmpty())

			st, err := mgr.Use(context.Background(), card, sim.ArtChoice{Kind: sim.ArtOwn})
			Expect(err).To(MatchError(sim.ErrNotFound))
			Expect(st.NoSim).To(BeTrue())
		})
	})

	Describe("Inspect and perceptual Match deduplication", func() {
		It("runs Match once per override hash, re-runs when the file changes and persists matches", func() {
			card, bandai := urouge()
			pics := newFakePictures()
			for _, p := range printsOf(card) {
				if g, ok := realGrids()(Key{Limitless, p.Key}); ok {
					pics.pics[Key{Limitless, p.Key}] = imageOf(g)
				}
			}

			catPath := filepath.Join(artDir, "bandai.json")
			c := newCatalog()
			c.Series[promotionID] = &series{Name: "Promotion card", Fetched: day0, Prints: bandai}
			Expect(c.save(catPath)).To(Succeed())

			g, err := Open(Config{CatalogPath: catPath, ArtDir: artDir, Pictures: pics})
			Expect(err).NotTo(HaveOccurred())

			mgr := NewSimManager(&sim.Install{Root: simRoot}, simDir, store, g)
			override := cardPath(simRoot, "OP07", "OP07-021_OVERRIDE.png")
			// Write a Bandai printing whose grid is not yet available in pics, so the first match fails.
			writePNGFile(override, imageOf(fixtureGrid("OP07-021_p4_bd")))

			st := mgr.Inspect(context.Background(), card)
			Expect(st.Err).NotTo(HaveOccurred())
			Expect(st.Cur).To(Equal(sim.ArtChoice{Kind: sim.ArtFile}))
			Expect(st.OwnFile).To(Equal(override))
			readsAfterFirst := pics.total.Load()
			Expect(readsAfterFirst).To(BeNumerically(">", 0))

			// Make the Bandai printing available now. Because the override hash did not change,
			// the second Inspect must not re-run Gallery.Match.
			pics.mu.Lock()
			for _, b := range bandai {
				if gr, ok := realGrids()(Key{Bandai, b.Key}); ok {
					pics.pics[Key{Bandai, b.Key}] = imageOf(gr)
				}
			}
			pics.mu.Unlock()

			st = mgr.Inspect(context.Background(), card)
			Expect(st.Cur).To(Equal(sim.ArtChoice{Kind: sim.ArtFile}))
			Expect(pics.total.Load()).To(Equal(readsAfterFirst))

			// Replace the override with another Bandai printing (new hash). Inspect now runs Match,
			// identifies the printing and persists it in sim-art.json.
			writePNGFile(override, imageOf(fixtureGrid("OP07-021_p1_bd")))
			st = mgr.Inspect(context.Background(), card)
			Expect(st.Err).NotTo(HaveOccurred())
			Expect(st.Cur).To(Equal(sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP07-021_p1_bd"}))
			Expect(st.OwnFile).To(BeEmpty())

			// A fresh manager over the same simDir sees the recorded match without running Match.
			fresh := NewSimManager(&sim.Install{Root: simRoot}, simDir, store, nil)
			st = fresh.Inspect(context.Background(), card)
			Expect(st.Cur).To(Equal(sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP07-021_p1_bd"}))
			Expect(st.OwnFile).To(BeEmpty())
		})
	})

	Describe("Use, Scan and kept-file safety", func() {
		It("applies ArtPrint, keeps user files in sim-art/kept, restores ArtFile and resets to ArtOwn", func() {
			card := cards.Card{Code: "OP07-021", Name: "Urouge"}
			k := Key{Source: Limitless, Print: "OP07-021_p1"}
			writePNGFile(store.fullPath(k), solidRect(200, 280, color.RGBA{0, 0, 220, 255}))

			overrideJPG := cardPath(simRoot, "OP07", "OP07-021_OVERRIDE.jpg")
			writeJPEGFile(overrideJPG, solidRect(100, 140, color.RGBA{220, 180, 0, 255}))
			origBytes, err := os.ReadFile(overrideJPG)
			Expect(err).NotTo(HaveOccurred())

			mgr := NewSimManager(&sim.Install{Root: simRoot}, simDir, store, nil)
			Expect(mgr.Scan()).To(ConsistOf("OP07-021"))

			st, err := mgr.Use(context.Background(), card, sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP07-021_p1"})
			Expect(err).NotTo(HaveOccurred())
			Expect(st.Cur).To(Equal(sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP07-021_p1"}))
			Expect(st.OwnFile).To(Equal(filepath.Join(simDir, "kept", "OP07-021_OVERRIDE.jpg")))
			Expect(os.ReadFile(st.OwnFile)).To(Equal(origBytes))
			Expect(mgr.Scan()).To(ConsistOf("OP07-021"))

			st, err = mgr.Use(context.Background(), card, sim.ArtChoice{Kind: sim.ArtFile})
			Expect(err).NotTo(HaveOccurred())
			Expect(st.Cur).To(Equal(sim.ArtChoice{Kind: sim.ArtFile}))
			Expect(st.OwnFile).To(Equal(overrideJPG))
			Expect(os.ReadFile(overrideJPG)).To(Equal(origBytes))

			st, err = mgr.Use(context.Background(), card, sim.ArtChoice{Kind: sim.ArtOwn})
			Expect(err).NotTo(HaveOccurred())
			Expect(st.Cur).To(Equal(sim.ArtChoice{Kind: sim.ArtOwn}))
			Expect(st.OwnFile).To(Equal(filepath.Join(simDir, "kept", "OP07-021_OVERRIDE.jpg")))
			Expect(mgr.Scan()).To(BeEmpty())
		})

		It("returns ErrNoPicture when using a printing without a web store", func() {
			card := cards.Card{Code: "OP07-021", Name: "Urouge"}
			mgr := NewSimManager(&sim.Install{Root: simRoot}, simDir, nil, nil)
			st, err := mgr.Use(context.Background(), card, sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP07-021_p1"})
			Expect(err).To(MatchError(ErrNoPicture))
			Expect(st.Cur).To(Equal(sim.ArtChoice{Kind: sim.ArtOwn}))
		})
	})

	Describe("Resolve", func() {
		It("handles mirroring, ExactPrintRef, SimOwnRef, FileRef, DON cards and simFallback", func() {
			// Base sim art is red; web art is blue; override art is green.
			webKey := Key{Source: Limitless, Print: "OP07-021"}
			writePNGFile(store.fullPath(webKey), solidRect(300, 420, color.RGBA{0, 0, 220, 255}))
			writePNGFile(cardPath(simRoot, "OP07", "OP07-021_OVERRIDE.png"), solidRect(1200, 1680, color.RGBA{0, 220, 0, 255}))

			// Second card has no web art and a corrupt override, testing fallback.
			writePNGFile(cardPath(simRoot, "OP07", "OP07-022.png"), solidRect(80, 112, color.RGBA{220, 0, 0, 255}))
			Expect(os.WriteFile(cardPath(simRoot, "OP07", "OP07-022_OVERRIDE.png"), []byte("bad png"), 0o600)).To(Succeed())

			// DON!! front and back.
			writePNGFile(filepath.Join(simRoot, "OPTCGSim_Data", "StreamingAssets", "Cards", "Don", "Don.png"), solidRect(60, 84, color.RGBA{240, 200, 40, 255}))
			writePNGFile(filepath.Join(simRoot, "OPTCGSim_Data", "StreamingAssets", "CardBacks", "CardBackDon.png"), solidRect(60, 84, color.RGBA{40, 40, 200, 255}))

			// Local file.
			localFile := filepath.Join(GinkgoT().TempDir(), "custom.png")
			writePNGFile(localFile, solidRect(400, 560, color.RGBA{180, 0, 180, 255}))

			mgr := NewSimManager(&sim.Install{Root: simRoot}, simDir, store, nil)

			// mirror=true uses override, capped at LargeWidth for SizeFull and SmallWidth for SizeSmall.
			img, err := mgr.Resolve(PrintRef("OP07-021"), SizeFull, true, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(img.Bounds().Dx()).To(Equal(LargeWidth))
			r, g, b, _ := img.At(5, 5).RGBA()
			Expect(g).To(BeNumerically(">", r))
			Expect(g).To(BeNumerically(">", b))

			small, err := mgr.Resolve(PrintRef("OP07-021"), SizeSmall, true, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(small.Bounds().Dx()).To(Equal(SmallWidth))

			// mirror=false ignores override and returns web art (blue).
			noMirror, err := mgr.Resolve(PrintRef("OP07-021"), SizeFull, false, true)
			Expect(err).NotTo(HaveOccurred())
			_, g, b, _ = noMirror.At(5, 5).RGBA()
			Expect(b).To(BeNumerically(">", g))

			// ExactPrintRef ignores override even when mirror=true.
			exact, err := mgr.Resolve(ExactPrintRef("OP07-021"), SizeFull, true, true)
			Expect(err).NotTo(HaveOccurred())
			_, g, b, _ = exact.At(5, 5).RGBA()
			Expect(b).To(BeNumerically(">", g))

			// SimOwnRef returns sim's own image (red) even when web and override exist.
			simOwn, err := mgr.Resolve(SimOwnRef("OP07-021"), SizeFull, true, false)
			Expect(err).NotTo(HaveOccurred())
			r, g, b, _ = simOwn.At(5, 5).RGBA()
			Expect(r).To(BeNumerically(">", g))
			Expect(r).To(BeNumerically(">", b))

			// FileRef decodes local file and scales for SizeSmall.
			fileFull, err := mgr.Resolve(FileRef(localFile), SizeFull, true, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(fileFull.Bounds().Dx()).To(Equal(400))
			fileSmall, err := mgr.Resolve(FileRef(localFile), SizeSmall, true, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(fileSmall.Bounds().Dx()).To(Equal(SmallWidth))

			// DON!! cards resolve regardless of simFallback.
			don, err := mgr.Resolve(DonRef(), SizeThumb, false, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(don.Bounds().Dx()).To(Equal(60))
			donBack, err := mgr.Resolve(DonBackRef(), SizeThumb, false, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(donBack.Bounds().Dx()).To(Equal(60))

			// simFallback=true falls back to sim on missing web art and corrupt override;
			// simFallback=false returns an error when web downloads are off.
			store.SetOff(true)
			fb, err := mgr.Resolve(PrintRef("OP07-022"), SizeFull, true, true)
			Expect(err).NotTo(HaveOccurred())
			r, g, _, _ = fb.At(5, 5).RGBA()
			Expect(r).To(BeNumerically(">", g))

			_, err = mgr.Resolve(PrintRef("OP07-022"), SizeFull, false, false)
			Expect(err).To(MatchError(ErrNoPicture))

			// Without an install, SimOwnRef fails with sim.ErrNotFound and DonRef fails with ErrNoPicture.
			mgr.SetInstall(nil)
			_, err = mgr.Resolve(SimOwnRef("OP07-021"), SizeFull, false, false)
			Expect(err).To(MatchError(sim.ErrNotFound))
			_, err = mgr.Resolve(DonRef(), SizeFull, false, false)
			Expect(err).To(MatchError(ErrNoPicture))
		})
	})

	Describe("SetInstall and concurrency", func() {
		It("swaps installs cleanly and handles concurrent Inspect and Resolve safely", func() {
			otherRoot := filepath.Join(GinkgoT().TempDir(), "sim2")
			writePNGFile(cardPath(otherRoot, "OP07", "OP07-021.png"), solidRect(80, 112, color.RGBA{0, 120, 220, 255}))
			writePNGFile(cardPath(otherRoot, "OP07", "OP07-021_OVERRIDE.png"), solidRect(80, 112, color.RGBA{0, 220, 0, 255}))

			mgr := NewSimManager(&sim.Install{Root: simRoot}, simDir, nil, nil)
			card := cards.Card{Code: "OP07-021", Name: "Urouge"}
			Expect(mgr.Inspect(context.Background(), card).Cur).To(Equal(sim.ArtChoice{Kind: sim.ArtOwn}))

			mgr.SetInstall(&sim.Install{Root: otherRoot})
			Expect(mgr.Inspect(context.Background(), card).Cur).To(Equal(sim.ArtChoice{Kind: sim.ArtFile}))
			Expect(mgr.Scan()).To(ConsistOf("OP07-021"))

			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					defer GinkgoRecover()
					st := mgr.Inspect(context.Background(), card)
					Expect(st.Err).NotTo(HaveOccurred())
					img, err := mgr.Resolve(PrintRef("OP07-021"), SizeFull, true, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(img).NotTo(BeNil())
				})
			}
			wg.Wait()
		})
	})
})
