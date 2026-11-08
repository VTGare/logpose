package arts

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Key", func() {
	It("reads every key sim-art.json holds today as Limitless's", func() {
		for _, s := range []string{"OP07-021", "OP07-021_p1", "P-001", "ST10-001_p12"} {
			k, err := ParseKey(s)
			Expect(err).NotTo(HaveOccurred(), s)
			Expect(k).To(Equal(Key{Limitless, s}))
			Expect(k.String()).To(Equal(s))
		}
	})

	It("writes Bandai's keys with a suffix that reads back, cuts to the card and holds no ':'", func() {
		k := Key{Bandai, "OP07-021_p3"}
		Expect(k.String()).To(Equal("OP07-021_p3_bd"))
		Expect(k.String()).NotTo(ContainSubstring(":"))
		back, err := ParseKey(k.String())
		Expect(err).NotTo(HaveOccurred())
		Expect(back).To(Equal(k))
		Expect(k.Code()).To(Equal("OP07-021"))
		Expect(Key{Bandai, "OP07-021"}.String()).To(Equal("OP07-021_bd"))
		Expect(Key{Bandai, "OP07-021"}.Code()).To(Equal("OP07-021"))
	})

	It("finds each site's picture in its own format", func() {
		lim := Key{Limitless, "OP07-021_p1"}
		Expect(lim.URL()).To(Equal("https://limitlesstcg.nyc3.cdn.digitaloceanspaces.com/one-piece/OP07/OP07-021_p1_EN.webp"))
		Expect(lim.Ext()).To(Equal(".webp"))
		bd := Key{Bandai, "OP07-021_p1"}
		Expect(bd.URL()).To(Equal("https://en.onepiece-cardgame.com/images/cardlist/card/OP07-021_p1.png"))
		Expect(bd.Ext()).To(Equal(".png"))
	})

	It("refuses what isn't a printing's picture", func() {
		for _, s := range []string{"", "sim:OP07-021", `file:C:\sim\OP07-021_OVERRIDE.png`, "OP07-021_r1", "OP07-021_r1_bd", "OP07-021_bd_bd"} {
			_, err := ParseKey(s)
			Expect(err).To(HaveOccurred(), s)
		}
	})
})
