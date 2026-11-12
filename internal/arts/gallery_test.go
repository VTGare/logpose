package arts

import (
	"context"
	"errors"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/VTGare/logpose/internal/cards"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// imageOf is a 16×22 picture whose grid is g.
func imageOf(g grid) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, gridW, gridH))
	for i := range gridW * gridH {
		copy(img.Pix[i*4:], g[i*3:i*3+3])
		img.Pix[i*4+3] = 255
	}
	return img
}

// fakeBandai serves the card list: the trimmed real pages it has, and a page of its own for
// every other series. It records the series each read asked for, "" for the list's own page.
type fakeBandai struct {
	mu    sync.Mutex
	list  []bandaiSeries
	pages map[string][]byte
	reads []string
}

func (f *fakeBandai) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/cardlist/" {
		http.NotFound(w, r)
		return
	}
	id := r.URL.Query().Get("series")
	f.reads = append(f.reads, id)
	if page, ok := f.pages[id]; ok {
		_, _ = w.Write(page)
		return
	}
	_, _ = w.Write(seriesPage(f.list, id))
}

func (f *fakeBandai) took() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.reads
	f.reads = nil
	return out
}

var _ = Describe("Gallery", func() {
	var (
		dir     string
		catPath string
		pics    *fakePictures
	)
	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		catPath = filepath.Join(dir, "bandai.json")
		pics = newFakePictures()
		for _, card := range []func() (cards.Card, []printing){urouge, tashigi} {
			c, bandai := card()
			for _, p := range printsOf(c) {
				if g, ok := realGrids()(Key{Limitless, p.Key}); ok {
					pics.pics[Key{Limitless, p.Key}] = imageOf(g)
				}
			}
			for _, b := range bandai {
				if g, ok := realGrids()(Key{Bandai, b.Key}); ok {
					pics.pics[Key{Bandai, b.Key}] = imageOf(g)
				}
			}
		}
	})

	// open writes bandai.json with the printings given and opens a gallery over it.
	open := func(bandai ...[]printing) *Gallery {
		GinkgoHelper()
		if len(bandai) > 0 {
			c := newCatalog()
			s := &series{Name: "Promotion card", Fetched: day0}
			for _, b := range bandai {
				s.Prints = append(s.Prints, b...)
			}
			c.Series[promotionID] = s
			Expect(c.save(catPath)).To(Succeed())
		}
		g, err := Open(Config{CatalogPath: catPath, ArtDir: dir, Pictures: pics})
		Expect(err).NotTo(HaveOccurred())
		return g
	}
	shows := func(g *Gallery, card cards.Card) []CardArts {
		var out []CardArts
		g.Arts(context.Background(), card, func(ca CardArts) { out = append(out, ca) })
		return out
	}

	Describe("Arts", func() {
		It("shows Limitless's arts at once, then each new picture, and the four arts last", func() {
			card, bandai := urouge()
			got := shows(open(bandai), card)
			Expect(layout(got[0])).To(Equal([]string{
				"Base: OP07-021 | OP07-021_bd", "SP: OP07-021_p1 | ", "Alt art: OP07-021_p3 | ",
			}))
			Expect(got[0].Pending).To(Equal(8))
			last := got[len(got)-1]
			Expect(layout(last)).To(Equal([]string{
				"Base: OP07-021 | OP07-021_bd",
				"SP: OP07-021_p1 | OP07-021_p2_bd",
				"Alt art: OP07-021_p3 | OP07-021_p8_bd",
				"Regional: OP07-021_p1_bd OP07-021_p3_bd OP07-021_p4_bd OP07-021_p5_bd OP07-021_p6_bd OP07-021_p7_bd | ",
			}))
			Expect(last.Pending).To(BeZero())
			Expect(got).To(HaveLen(13))
			for _, ca := range got {
				Expect(layout(ca)).NotTo(ContainElement(HavePrefix("Promo")))
				Expect(layout(ca)).NotTo(ContainElement(HavePrefix("SP: OP07-021_p2_bd")))
			}
			Expect(pics.readsOf(Key{Bandai, "OP07-021"})).To(BeZero())
		})

		It("reads each picture once ever", func() {
			card, bandai := urouge()
			shows(open(bandai), card)
			first := pics.total.Load()
			Expect(first).To(Equal(int32(11)))
			got := shows(open(), card)
			Expect(got).To(HaveLen(1))
			Expect(got[0].Arts).To(HaveLen(4))
			Expect(pics.total.Load()).To(Equal(first))
		})

		It("reads no picture for a card whose Bandai printings are its own on Limitless", func() {
			card, bandai := tashigi()
			got := shows(open(bandai[:2]), card)
			Expect(got).To(HaveLen(1))
			Expect(layout(got[0])).To(Equal([]string{"Base: OP12-031 | OP12-031_bd", "Alt art: OP12-031_p1 | OP12-031_p1_bd"}))
			Expect(pics.total.Load()).To(BeZero())
		})

		It("shows what earlier pictures grouped while downloads are off, and leaves the rest out", func() {
			card, bandai := urouge()
			pics.pics = map[Key]image.Image{
				{Limitless, "OP07-021"}: pics.pics[Key{Limitless, "OP07-021"}], {Limitless, "OP07-021_p1"}: pics.pics[Key{Limitless, "OP07-021_p1"}],
				{Limitless, "OP07-021_p3"}: pics.pics[Key{Limitless, "OP07-021_p3"}], {Bandai, "OP07-021_p1"}: pics.pics[Key{Bandai, "OP07-021_p1"}],
				{Bandai, "OP07-021_p3"}: pics.pics[Key{Bandai, "OP07-021_p3"}],
			}
			got := shows(open(bandai), card)
			last := got[len(got)-1]
			Expect(layout(last)).To(Equal([]string{
				"Base: OP07-021 | OP07-021_bd", "SP: OP07-021_p1 | ", "Alt art: OP07-021_p3 | ",
				"Regional: OP07-021_p1_bd OP07-021_p3_bd | ",
			}))
			Expect(last.Pending).To(BeZero())
		})

		It("shows Limitless's arts alone without a gallery", func() {
			card, _ := urouge()
			var g *Gallery
			got := shows(g, card)
			Expect(got).To(Equal([]CardArts{LimitlessArts(card)}))
			_, ok := g.Match(context.Background(), card, imageOf(fixtureGrid("OP07-021")))
			Expect(ok).To(BeFalse())
			Expect(g.Stale(day0)).To(BeFalse())
		})
	})

	Describe("Match", func() {
		It("finds the closest stamp for a file, making the pictures it needs", func() {
			card, bandai := urouge()
			k, ok := open(bandai).Match(context.Background(), card, imageOf(fixtureGrid("OP07-021_p4_bd")))
			Expect(ok).To(BeTrue())
			Expect(k).To(Equal(Key{Bandai, "OP07-021_p4"}))
			Expect(pics.readsOf(Key{Bandai, "OP07-021"})).To(BeZero())
		})

		It("finds Tashigi's Treasure Cup printing, and nothing for another card", func() {
			card, bandai := tashigi()
			g := open(bandai)
			k, ok := g.Match(context.Background(), card, imageOf(fixtureGrid("OP12-031_p2_bd")))
			Expect(ok).To(BeTrue())
			Expect(k.String()).To(Equal("OP12-031_p2_bd"))
			_, ok = g.Match(context.Background(), card, imageOf(fixtureGrid("OP07-021_p1_bd")))
			Expect(ok).To(BeFalse())
		})
	})

	Describe("Refresh", func() {
		var (
			site *fakeBandai
			srv  *httptest.Server
		)
		BeforeEach(func() {
			list := parsePage(pageFixture("bandai-cardlist.html")).Series
			site = &fakeBandai{list: list, pages: map[string][]byte{
				"":          pageFixture("bandai-cardlist.html"),
				"569117":    pageFixture("bandai-cardlist.html"),
				promotionID: pageFixture("bandai-569901.html"),
				"569107":    pageFixture("bandai-569107.html"),
			}}
			srv = httptest.NewServer(site)
			DeferCleanup(srv.Close)
		})
		gallery := func() *Gallery {
			GinkgoHelper()
			g, err := Open(Config{CatalogPath: catPath, ArtDir: dir, Client: srv.Client(), Site: srv.URL})
			Expect(err).NotTo(HaveOccurred())
			return g
		}

		It("reads the list's own page, then every series once, and keeps them", func() {
			g := gallery()
			Expect(g.Stale(day0)).To(BeTrue())
			var last [2]int
			changed, err := g.Refresh(context.Background(), day0, func(done, total int) { last = [2]int{done, total} })
			Expect(err).NotTo(HaveOccurred())
			Expect(changed).To(BeTrue())
			reads := site.took()
			Expect(reads).To(HaveLen(61))
			Expect(reads[0]).To(BeEmpty())
			Expect(reads[1:]).To(ConsistOf(slices.DeleteFunc(ids(site.list), func(id string) bool { return id == "569117" })))
			Expect(last).To(Equal([2]int{61, 61}))
			Expect(g.Stale(day0.Add(time.Hour))).To(BeFalse())
			Expect(keysOf(gallery().catalog().forCard("OP07-021"))).To(HaveLen(7))
		})

		It("reads only the daily pages a day later, the promotions first, and finds nothing new", func() {
			_, err := gallery().Refresh(context.Background(), day0, nil)
			Expect(err).NotTo(HaveOccurred())
			site.took()
			g := gallery()
			Expect(g.Stale(day0.Add(21 * time.Hour))).To(BeTrue())
			changed, err := g.Refresh(context.Background(), day0.Add(21*time.Hour), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(changed).To(BeFalse())
			Expect(site.took()).To(Equal([]string{promotionID, "569801", "569302", "569205", "569117", "569036"}))
		})

		It("keeps the pages read before a cancel, and goes on from there", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			changed, err := gallery().Refresh(ctx, day0, func(done, _ int) {
				if done == 4 {
					cancel()
				}
			})
			Expect(err).To(MatchError(context.Canceled))
			Expect(changed).To(BeTrue())
			Expect(site.took()).To(Equal([]string{"", promotionID, "569801", "569302"}))

			g := gallery()
			Expect(g.Stale(day0.Add(time.Hour))).To(BeTrue())
			Expect(g.catalog().forCard("OP07-021")).To(HaveLen(6))
			_, err = g.Refresh(context.Background(), day0.Add(time.Hour), nil)
			Expect(err).NotTo(HaveOccurred())
			reads := site.took()
			Expect(reads).To(HaveLen(58))
			Expect(reads[0]).To(Equal(promotionID))
			Expect(reads).NotTo(ContainElements("", "569801", "569302"))
			Expect(g.Stale(day0.Add(2 * time.Hour))).To(BeFalse())
		})

		It("stops at a page of another shape and keeps what came before", func() {
			site.pages["569801"] = []byte("<html><body>Maintenance</body></html>")
			g := gallery()
			_, err := g.Refresh(context.Background(), day0, nil)
			Expect(err).To(MatchError(errBadPage))
			Expect(g.Stale(day0.Add(time.Hour))).To(BeTrue())
			Expect(g.catalog().forCard("OP07-021")).To(HaveLen(6))
		})

		It("leaves the catalog as it was when the site can't be reached", func() {
			srv.Close()
			g := gallery()
			changed, err := g.Refresh(context.Background(), day0, nil)
			Expect(err).To(HaveOccurred())
			Expect(changed).To(BeFalse())
			_, statErr := os.Stat(catPath)
			Expect(errors.Is(statErr, os.ErrNotExist)).To(BeTrue())
		})
	})
})
