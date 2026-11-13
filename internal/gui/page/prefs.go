package page

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/VTGare/logpose/internal/arts"
)

// The keys of the settings only the window and the app use. Flags store "1" for on and "" for
// off.
const (
	// KeyTheme is "dark" or "light". Empty follows Windows.
	KeyTheme        = "theme"
	KeyColorblind   = "colorblind"
	KeyCloseQuits   = "close_quits"
	KeyStartHidden  = "start_hidden"
	KeySidebarWidth = "sidebar_width"
	// keyCardPanelWidth is the width in DIPs of the Cards page's card panel.
	keyCardPanelWidth = "card_panel_width"
	// KeyCardSort is the Cards page's sort, a key such as "cost", with " reversed" after it for
	// the key's other direction.
	KeyCardSort = "card_sort"
	KeyCardList = "card_list"
	KeyArtOff   = "art_off"
	// KeyArtCap is how many MB of large card images are kept.
	KeyArtCap = "art_cap_mb"
	// KeyShortcutsOffered is on once the user answered the offer of shortcuts.
	KeyShortcutsOffered = "shortcuts_offered"
	keyDeckChanges      = "deck_show_changes"
	// KeyFolded is the IDs of the tiles the user collapsed, a list.
	KeyFolded = "folded_tiles"
	// KeyTables is the JSON of each table's sort, column order and widths, by table ID.
	KeyTables       = "tables"
	KeyUpdateManual = "update_manual"
	// KeyStartFormat is startLatest or startAll. Empty means the user hasn't picked.
	KeyStartFormat = "start_format"
	// KeySimSaves is the folder Export to sim writes to. Empty is sim-saves next to the
	// database.
	KeySimSaves = "sim_saves_dir"

	// KeyUpdateChecked is when logpose last asked for a newer version, in Unix ms.
	KeyUpdateChecked = "update_checked"
	// KeyLastVersion is the version of logpose that ran last.
	KeyLastVersion = "last_version"
	// KeyDismissedSets is the sets whose new-format suggestion the user dismissed, a list.
	KeyDismissedSets = "dismissed_sets"
)

const (
	StartLatest = "latest"
	StartAll    = "all"
)

// Prefs are the settings the window applies itself. The view owns them, and saves each change
// with savePref.
type Prefs struct {
	Theme            string
	Colorblind       bool
	CloseQuits       bool
	StartHidden      bool
	ShortcutsOffered bool
	SideWidth        float32
	PanelWidth       float32
	ArtOff           bool
	// ArtCap is the MB of large card images kept, one of artCaps.
	ArtCap int
	// DeckChanges marks the cards a build's list changed.
	DeckChanges bool
	// folded is the tiles collapsed, by ID.
	folded map[string]bool
	tables map[string]*kit.TableLayout
	// CardSort is the Cards page's order as stored, and cardList whether it shows a list.
	CardSort string
	CardList bool
	// UpdateManual waits for the user to download an update.
	UpdateManual bool
	// StartFormat is the format pages start on and Clear goes back to, formatCurrent or
	// formatAll.
	StartFormat int64
	SimSaves    string
}

// pref is one stored preference.
type pref interface {
	prefKey() string
	load(p *Prefs, stored string)
	text(p *Prefs) string
}

// prefEntry is a pref with its field in prefs, how the field is read from the stored text, where
// "" gives the default, and how it is written back.
type prefEntry[T any] struct {
	key   string
	field func(*Prefs) *T
	Read  func(string) T
	write func(T) string
}

func (e prefEntry[T]) prefKey() string              { return e.key }
func (e prefEntry[T]) load(p *Prefs, stored string) { *e.field(p) = e.Read(stored) }
func (e prefEntry[T]) text(p *Prefs) string         { return e.write(*e.field(p)) }

// Set changes the pref and saves it.
func (e prefEntry[T]) Set(env *Env, value T) {
	*e.field(&env.Prefs) = value
	env.savePref(e)
}

func flagPref(key string, field func(*Prefs) *bool) prefEntry[bool] {
	return prefEntry[bool]{key, field, func(s string) bool { return s == "1" }, flag}
}

var (
	PrefTheme            = prefEntry[string]{KeyTheme, func(p *Prefs) *string { return &p.Theme }, same, same}
	PrefColorblind       = flagPref(KeyColorblind, func(p *Prefs) *bool { return &p.Colorblind })
	PrefCloseQuits       = flagPref(KeyCloseQuits, func(p *Prefs) *bool { return &p.CloseQuits })
	PrefStartHidden      = flagPref(KeyStartHidden, func(p *Prefs) *bool { return &p.StartHidden })
	PrefShortcutsOffered = flagPref(KeyShortcutsOffered, func(p *Prefs) *bool { return &p.ShortcutsOffered })
	PrefSideWidth        = prefEntry[float32]{KeySidebarWidth, func(p *Prefs) *float32 { return &p.SideWidth }, widthFrom(SideDefault, SnapSide), widthText}
	PrefPanelWidth       = prefEntry[float32]{keyCardPanelWidth, func(p *Prefs) *float32 { return &p.PanelWidth }, widthFrom(PanelDefault, SnapPanel), widthText}
	PrefArtOff           = flagPref(KeyArtOff, func(p *Prefs) *bool { return &p.ArtOff })
	PrefArtCap           = prefEntry[int]{KeyArtCap, func(p *Prefs) *int { return &p.ArtCap }, artCapFrom, strconv.Itoa}
	PrefDeckChanges      = flagPref(keyDeckChanges, func(p *Prefs) *bool { return &p.DeckChanges })
	prefFolded           = prefEntry[map[string]bool]{KeyFolded, func(p *Prefs) *map[string]bool { return &p.folded }, foldedFrom, foldedText}
	prefTables           = prefEntry[map[string]*kit.TableLayout]{KeyTables, func(p *Prefs) *map[string]*kit.TableLayout { return &p.tables }, tablesFrom, tablesText}
	PrefCardSort         = prefEntry[string]{KeyCardSort, func(p *Prefs) *string { return &p.CardSort }, same, same}
	PrefCardList         = flagPref(KeyCardList, func(p *Prefs) *bool { return &p.CardList })
	PrefUpdateManual     = flagPref(KeyUpdateManual, func(p *Prefs) *bool { return &p.UpdateManual })
	prefStartFormat      = prefEntry[int64]{KeyStartFormat, func(p *Prefs) *int64 { return &p.StartFormat }, startFormatFrom, startFormatText}
	PrefSimSaves         = prefEntry[string]{KeySimSaves, func(p *Prefs) *string { return &p.SimSaves }, same, same}
)

var prefTable = []pref{
	PrefTheme, PrefColorblind, PrefCloseQuits, PrefStartHidden, PrefShortcutsOffered, PrefSideWidth,
	PrefPanelWidth, PrefArtOff, PrefArtCap, PrefDeckChanges, prefFolded, prefTables, PrefCardSort,
	PrefCardList, PrefUpdateManual, prefStartFormat, PrefSimSaves,
}

// PrefsFrom reads the stored settings. A nil map gives the defaults.
func PrefsFrom(set map[string]string) Prefs {
	var p Prefs
	for _, e := range prefTable {
		e.load(&p, set[e.prefKey()])
	}
	return p
}

// savePref stores p as the prefs have it, and gives the app the prefs, which acts on the ones
// that changed.
func (e *Env) savePref(p pref) {
	if e.Act == nil {
		return
	}
	e.Act.SetSetting(p.prefKey(), p.text(&e.Prefs))
	e.Act.ApplyPrefs(e.Prefs)
}

// saveDelay waits for changes that come every frame, such as a drag, to stop.
const saveDelay = 400 * time.Millisecond

// later runs fn once the calls under key stop for saveDelay.
func (e *Env) later(key string, fn func()) {
	if e.pending == nil {
		e.pending = make(map[string]int)
	}
	e.pending[key]++
	n := e.pending[key]
	if e.Act == nil {
		return
	}
	e.Act.After(saveDelay, func() {
		if e.pending[key] == n {
			fn()
		}
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

// ArtCaps are the sizes in MB the large card images can be capped at.
var ArtCaps = []int{100, 150, 300, 600}

const artCapDefault = arts.DefaultCap >> 20

func artCapFrom(s string) int {
	mb, err := strconv.Atoi(s)
	if err != nil || !slices.Contains(ArtCaps, mb) {
		return artCapDefault
	}
	return mb
}

// SplitList reads a stored list, which is comma-separated.
func SplitList(s string) []string {
	var out []string
	for item := range strings.SplitSeq(s, ",") {
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func JoinList(items []string) string { return strings.Join(items, ",") }

func foldedFrom(s string) map[string]bool {
	out := make(map[string]bool)
	for _, id := range SplitList(s) {
		out[id] = true
	}
	return out
}

func foldedText(folded map[string]bool) string {
	return JoinList(slices.Sorted(maps.Keys(folded)))
}

func tablesFrom(s string) map[string]*kit.TableLayout {
	out := make(map[string]*kit.TableLayout)
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	return out
}

func tablesText(tables map[string]*kit.TableLayout) string {
	b, err := json.Marshal(tables)
	if err != nil {
		return ""
	}
	return string(b)
}

func startFormatFrom(s string) int64 {
	switch s {
	case StartLatest:
		return FormatCurrent
	case StartAll:
		return FormatAll
	}
	return startUnpicked
}

func startFormatText(format int64) string {
	if format == FormatCurrent {
		return StartLatest
	}
	return StartAll
}

// The sidebar is resized by dragging its edge. Narrower than sideSnap it folds to an icon rail.
const (
	SideDefault = 200
	SideMin     = 160
	SideMax     = 320
	sideSnap    = 120
	SideRail    = 60
)

func SnapSide(w float32) float32 {
	switch {
	case w < sideSnap:
		return SideRail
	case w < SideMin:
		return SideMin
	}
	return min(w, SideMax)
}

// The card panel is resized by dragging its edge. Narrower than panelSnap it folds to a strip.
const (
	PanelDefault = 470
	panelMin     = 360
	panelMax     = 760
	panelSnap    = 280
	PanelStrip   = 44
)

func SnapPanel(w float32) float32 {
	switch {
	case w < panelSnap:
		return PanelStrip
	case w < panelMin:
		return panelMin
	}
	return min(w, panelMax)
}
