package gui

import "github.com/egoist/mygo/ui"

func mustSVG(s string) *ui.SVG {
	svg, err := ui.ParseSVG([]byte(s))
	if err != nil {
		panic(err)
	}
	return svg
}

const svgHead = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">`

var (
	iconOverview = mustSVG(svgHead + `<path d="M4 11l8-7 8 7"/><path d="M6 10v10h12V10"/><path d="M10 20v-6h4v6"/></svg>`)
	iconMatches  = mustSVG(svgHead + `<path d="M9 6h11M9 12h11M9 18h11"/><circle cx="4.5" cy="6" r="1"/><circle cx="4.5" cy="12" r="1"/><circle cx="4.5" cy="18" r="1"/></svg>`)
	iconCompass  = mustSVG(svgHead + `<circle cx="12" cy="12" r="9"/><path d="M15.5 8.5l-2 5-5 2 2-5z"/></svg>`)
)
