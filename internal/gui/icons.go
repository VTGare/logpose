package gui

import "github.com/egoist/mygo/ui"

func mustSVG(s string) *ui.SVG {
	svg, err := ui.ParseSVG([]byte(s))
	if err != nil {
		panic(err)
	}
	return svg
}

// svgBold is for icons drawn small on badges, where the usual stroke would vanish.
const svgBold = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round">`

const svgHead = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">`

var (
	iconOverview = mustSVG(svgHead + `<path d="M4 11l8-7 8 7"/><path d="M6 10v10h12V10"/><path d="M10 20v-6h4v6"/></svg>`)
	iconMatches  = mustSVG(svgHead + `<path d="M9 6h11M9 12h11M9 18h11"/><circle cx="4.5" cy="6" r="1"/><circle cx="4.5" cy="12" r="1"/><circle cx="4.5" cy="18" r="1"/></svg>`)
	iconLeaders  = mustSVG(svgHead + `<path d="M4 8l4 4 4-7 4 7 4-4-2 11H6z"/></svg>`)
	iconMatchups = mustSVG(svgHead + `<rect x="3" y="4" width="7" height="10" rx="1.5"/><rect x="14" y="10" width="7" height="10" rx="1.5"/><path d="M12 7h4M8 17h4"/></svg>`)
	iconFormats  = mustSVG(svgHead + `<rect x="4" y="5" width="16" height="15" rx="2"/><path d="M4 10h16M9 3v4M15 3v4"/></svg>`)
	iconSettings = mustSVG(svgHead + `<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/></svg>`)
	iconInfo     = mustSVG(svgHead + `<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/></svg>`)
	iconCompass  = mustSVG(svgHead + `<circle cx="12" cy="12" r="9"/><path d="M15.5 8.5l-2 5-5 2 2-5z"/></svg>`)

	iconPrevTurn    = mustSVG(svgHead + `<path d="M6 5v14"/><path d="M18 6l-8 6 8 6z"/></svg>`)
	iconNextTurn    = mustSVG(svgHead + `<path d="M18 5v14"/><path d="M6 6l8 6-8 6z"/></svg>`)
	iconPrevStep    = mustSVG(svgHead + `<path d="M15 5l-7 7 7 7"/></svg>`)
	iconNextStep    = mustSVG(svgHead + `<path d="M9 5l7 7-7 7"/></svg>`)
	iconRestart     = mustSVG(svgHead + `<path d="M4 5v5h5"/><path d="M4.6 14.5A8 8 0 1 0 6.3 6.3L4 10"/></svg>`)
	iconPlay        = mustSVG(svgHead + `<path d="M7 4.5l12 7.5-12 7.5z" fill="currentColor"/></svg>`)
	iconPause       = mustSVG(svgHead + `<rect x="6" y="5" width="4" height="14" rx="1" fill="currentColor"/><rect x="14" y="5" width="4" height="14" rx="1" fill="currentColor"/></svg>`)
	iconStar        = mustSVG(svgHead + `<path d="M12 3.5l2.6 5.4 5.9.8-4.3 4.1 1 5.8-5.2-2.8-5.2 2.8 1-5.8-4.3-4.1 5.9-.8z"/></svg>`)
	iconStarSet     = mustSVG(svgHead + `<path d="M12 3.5l2.6 5.4 5.9.8-4.3 4.1 1 5.8-5.2-2.8-5.2 2.8 1-5.8-4.3-4.1 5.9-.8z" fill="currentColor"/></svg>`)
	iconNote        = mustSVG(svgHead + `<path d="M12 20h8"/><path d="M16.5 4.5a2.1 2.1 0 0 1 3 3L8 19l-4 1 1-4z"/></svg>`)
	iconNoRest      = mustSVG(svgBold + `<path d="M7 11V7a5 5 0 0 1 10 0v4"/><rect x="5" y="11" width="14" height="10" rx="2"/></svg>`)
	iconFrozen      = mustSVG(svgBold + `<path d="M12 2v20M3.3 7l17.4 10M3.3 17 20.7 7"/><path d="m9 4 3 3 3-3M9 20l3-3 3 3"/></svg>`)
	iconNoAttack    = mustSVG(svgBold + `<path d="M14.5 17.5 3 6V3h3l11.5 11.5"/><path d="m13 19 6-6M16 16l4 4M19 21l2-2"/><path d="M21 3 3 21"/></svg>`)
	iconNegated     = mustSVG(svgBold + `<path d="M12 3l1.9 4.6L18.5 9l-4.6 1.9L12 15.5l-1.9-4.6L5.5 9l4.6-1.4z"/><path d="M3 21 21 3"/></svg>`)
	iconUnblockable = mustSVG(svgBold + `<path d="M19.7 14a7 7 0 0 0 .3-2V5l-8-3-3.2 1.2"/><path d="M4.7 4.7 4 5v7c0 6 8 10 8 10a20 20 0 0 0 5.6-4.3"/><path d="m2 2 20 20"/></svg>`)
	iconImmune      = mustSVG(svgBold + `<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="M12 7.5l1 2.5 2.5 1-2.5 1-1 2.5-1-2.5-2.5-1 2.5-1z"/></svg>`)
	iconClose       = mustSVG(svgHead + `<path d="M6.5 6.5l11 11M17.5 6.5l-11 11"/></svg>`)
	iconCheck       = mustSVG(svgHead + `<path d="M5 12.5l4.5 4.5L19 7.5"/></svg>`)
	iconChevron     = mustSVG(svgHead + `<path d="M6 9l6 6 6-6"/></svg>`)
	iconTrendUp     = mustSVG(svgHead + `<path d="M12 19V5M6 11l6-6 6 6"/></svg>`)
	iconTrendDown   = mustSVG(svgHead + `<path d="M12 5v14M6 13l6 6 6-6"/></svg>`)
)
