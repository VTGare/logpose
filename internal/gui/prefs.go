package gui

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/arts"
)

// The keys of the settings only the window and the app use. Flags store "1" for on and "" for
// off.
const (
	// keyTheme is "dark" or "light". Empty follows Windows.
	keyTheme        = "theme"
	keyColorblind   = "colorblind"
	keyCloseQuits   = "close_quits"
	keyStartHidden  = "start_hidden"
	keySidebarWidth = "sidebar_width"
	// keyCardPanelWidth is the width in DIPs of the Cards page's card panel.
	keyCardPanelWidth = "card_panel_width"
	// keyCardSort is the Cards page's sort, a key such as "cost", with " reversed" after it for
	// the key's other direction.
	keyCardSort = "card_sort"
	keyCardList = "card_list"
	keyArtOff   = "art_off"
	// keyArtCap is how many MB of large card images are kept.
	keyArtCap = "art_cap_mb"
	// keyShortcutsOffered is on once the user answered the offer of shortcuts.
	keyShortcutsOffered = "shortcuts_offered"
	keyDeckChanges      = "deck_show_changes"
	// keyFolded is the IDs of the tiles the user collapsed, a list.
	keyFolded = "folded_tiles"
	// keyTables is the JSON of each table's sort, column order and widths, by table ID.
	keyTables       = "tables"
	keyUpdateManual = "update_manual"
	// keyStartFormat is startLatest or startAll. Empty means the user hasn't picked.
	keyStartFormat = "start_format"

	// keyUpdateChecked is when logpose last asked for a newer version, in Unix ms.
	keyUpdateChecked = "update_checked"
	// keyLastVersion is the version of logpose that ran last.
	keyLastVersion = "last_version"
	// keyDismissedSets is the sets whose new-format suggestion the user dismissed, a list.
	keyDismissedSets = "dismissed_sets"
)

const (
	startLatest = "latest"
	startAll    = "all"
)

// prefs are the settings the window applies itself. The view owns them, and saves each change
// with savePref.
type prefs struct {
	theme            string
	colorblind       bool
	closeQuits       bool
	startHidden      bool
	shortcutsOffered bool
	sideWidth        float32
	panelWidth       float32
	artOff           bool
	// artCap is the MB of large card images kept, one of artCaps.
	artCap int
	// deckChanges marks the cards a build's list changed.
	deckChanges bool
	// folded is the tiles collapsed, by ID.
	folded map[string]bool
	tables map[string]*tableLayout
	// cardSort and cardList are the Cards page's order and whether it shows a list.
	cardSort cardSort
	cardList bool
	// updateManual waits for the user to download an update.
	updateManual bool
	// startFormat is the format pages start on and Clear goes back to, formatCurrent or
	// formatAll.
	startFormat int64
}

// pref is one stored preference.
type pref interface {
	prefKey() string
	load(p *prefs, stored string)
	text(p *prefs) string
}

// prefEntry is a pref with its field in prefs, how the field is read from the stored text, where
// "" gives the default, and how it is written back.
type prefEntry[T any] struct {
	key   string
	field func(*prefs) *T
	read  func(string) T
	write func(T) string
}

func (e prefEntry[T]) prefKey() string              { return e.key }
func (e prefEntry[T]) load(p *prefs, stored string) { *e.field(p) = e.read(stored) }
func (e prefEntry[T]) text(p *prefs) string         { return e.write(*e.field(p)) }

// set changes the pref in v.prefs and saves it.
func (e prefEntry[T]) set(v *view, value T) {
	*e.field(&v.prefs) = value
	v.savePref(e)
}

func flagPref(key string, field func(*prefs) *bool) prefEntry[bool] {
	return prefEntry[bool]{key, field, func(s string) bool { return s == "1" }, flag}
}

var (
	prefTheme            = prefEntry[string]{keyTheme, func(p *prefs) *string { return &p.theme }, same, same}
	prefColorblind       = flagPref(keyColorblind, func(p *prefs) *bool { return &p.colorblind })
	prefCloseQuits       = flagPref(keyCloseQuits, func(p *prefs) *bool { return &p.closeQuits })
	prefStartHidden      = flagPref(keyStartHidden, func(p *prefs) *bool { return &p.startHidden })
	prefShortcutsOffered = flagPref(keyShortcutsOffered, func(p *prefs) *bool { return &p.shortcutsOffered })
	prefSideWidth        = prefEntry[float32]{keySidebarWidth, func(p *prefs) *float32 { return &p.sideWidth }, widthFrom(sideDefault, snapSide), widthText}
	prefPanelWidth       = prefEntry[float32]{keyCardPanelWidth, func(p *prefs) *float32 { return &p.panelWidth }, widthFrom(panelDefault, snapPanel), widthText}
	prefArtOff           = flagPref(keyArtOff, func(p *prefs) *bool { return &p.artOff })
	prefArtCap           = prefEntry[int]{keyArtCap, func(p *prefs) *int { return &p.artCap }, artCapFrom, strconv.Itoa}
	prefDeckChanges      = flagPref(keyDeckChanges, func(p *prefs) *bool { return &p.deckChanges })
	prefFolded           = prefEntry[map[string]bool]{keyFolded, func(p *prefs) *map[string]bool { return &p.folded }, foldedFrom, foldedText}
	prefTables           = prefEntry[map[string]*tableLayout]{keyTables, func(p *prefs) *map[string]*tableLayout { return &p.tables }, tablesFrom, tablesText}
	prefCardSort         = prefEntry[cardSort]{keyCardSort, func(p *prefs) *cardSort { return &p.cardSort }, cardSortFrom, cardSort.String}
	prefCardList         = flagPref(keyCardList, func(p *prefs) *bool { return &p.cardList })
	prefUpdateManual     = flagPref(keyUpdateManual, func(p *prefs) *bool { return &p.updateManual })
	prefStartFormat      = prefEntry[int64]{keyStartFormat, func(p *prefs) *int64 { return &p.startFormat }, startFormatFrom, startFormatText}
)

var prefTable = []pref{
	prefTheme, prefColorblind, prefCloseQuits, prefStartHidden, prefShortcutsOffered, prefSideWidth,
	prefPanelWidth, prefArtOff, prefArtCap, prefDeckChanges, prefFolded, prefTables, prefCardSort,
	prefCardList, prefUpdateManual, prefStartFormat,
}

// prefsFrom reads the stored settings. A nil map gives the defaults.
func prefsFrom(set map[string]string) prefs {
	var p prefs
	for _, e := range prefTable {
		e.load(&p, set[e.prefKey()])
	}
	return p
}

// savePref stores e as v.prefs has it, and gives the app the prefs, which acts on the ones that
// changed.
func (v *view) savePref(e pref) {
	key, value := e.prefKey(), e.text(&v.prefs)
	p := v.prefs
	v.do(func(a actions) {
		a.SetSetting(key, value)
		a.ApplyPrefs(p)
	})
}

// saveDelay waits for changes that come every frame, such as a drag, to stop.
const saveDelay = 400 * time.Millisecond

// later runs fn once the calls under key stop for saveDelay.
func (v *view) later(key string, fn func()) {
	if v.pending == nil {
		v.pending = make(map[string]int)
	}
	v.pending[key]++
	n := v.pending[key]
	v.do(func(a actions) {
		a.After(saveDelay, func() {
			if v.pending[key] == n {
				fn()
			}
		})
	})
}

func same(s string) string { return s }

func flag(on bool) string {
	if on {
		return "1"
	}
	return ""
}

func widthFrom(def float32, snap func(float32) float32) func(string) float32 {
	return func(s string) float32 {
		w, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return def
		}
		return snap(float32(w))
	}
}

func widthText(w float32) string { return fmt.Sprint(int(w)) }

// artCaps are the sizes in MB the large card images can be capped at.
var artCaps = []int{100, 150, 300, 600}

const artCapDefault = arts.DefaultCap >> 20

func artCapFrom(s string) int {
	mb, err := strconv.Atoi(s)
	if err != nil || !slices.Contains(artCaps, mb) {
		return artCapDefault
	}
	return mb
}

// splitList reads a stored list, which is comma-separated.
func splitList(s string) []string {
	var out []string
	for item := range strings.SplitSeq(s, ",") {
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func joinList(items []string) string { return strings.Join(items, ",") }

func foldedFrom(s string) map[string]bool {
	out := make(map[string]bool)
	for _, id := range splitList(s) {
		out[id] = true
	}
	return out
}

func foldedText(folded map[string]bool) string {
	return joinList(slices.Sorted(maps.Keys(folded)))
}

func tablesFrom(s string) map[string]*tableLayout {
	out := make(map[string]*tableLayout)
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	return out
}

func tablesText(tables map[string]*tableLayout) string {
	b, err := json.Marshal(tables)
	if err != nil {
		return ""
	}
	return string(b)
}

func startFormatFrom(s string) int64 {
	switch s {
	case startLatest:
		return formatCurrent
	case startAll:
		return formatAll
	}
	return startUnpicked
}

func startFormatText(format int64) string {
	if format == formatCurrent {
		return startLatest
	}
	return startAll
}
