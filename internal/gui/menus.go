package gui

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// Every right-click menu is a PopoverMenu, drawn in the app's look. Menus use one wording:
// "Open on …" goes to a page about the thing, "Show in Matches" to its games, and "Filter to …"
// sets a pill and stays on the page. Every item can also be reached without right-click.

// gameMenu is a game's menu, the same wherever the game shows.
func (v *view) gameMenu(m stats.Match) func(*ui.Menu) {
	return func(menu *ui.Menu) {
		if menu.Item("Open match").Chosen() {
			v.openMatch(m)
		}
		if menu.Item("Watch replay").Chosen() {
			v.openReplay(m, -1)
		}
		menu.Separator()
		if stats.CanBeRanked(m.Game) {
			menu.Submenu("Game type", func(sub *ui.Menu) {
				for i, label := range v.typeOptions(m) {
					if sub.Item(label).Radio(int(m.Type) == i).Chosen() {
						v.do(func(a actions) { a.SetGameType(m.Fingerprint, store.GameType(i)) })
					}
				}
			})
		} else {
			menu.Item("Game type: private lobby").Disabled(true)
		}
		noteLabel := "Add note…"
		if m.Note != "" {
			noteLabel = "Edit note…"
		}
		if menu.Item(noteLabel).Chosen() {
			v.openMatch(m)
			v.detail.focusNote = true
		}
		bookmark := "Bookmark"
		if m.Bookmarked {
			bookmark = "Remove bookmark"
		}
		if menu.Item(bookmark).Chosen() {
			v.do(func(a actions) { a.SetBookmark(m.Fingerprint, !m.Bookmarked) })
		}
		menu.Separator()
		if menu.Item("Filter to games vs " + m.Opp.Name).Chosen() {
			v.filter.opp = m.Opp.ID
		}
		matchup := v.snap.card(m.Me.Leader).Name + " vs " + v.snap.card(m.Opp.Leader).Name
		if menu.Item("Filter to " + matchup).Chosen() {
			v.filter.deck, v.filter.oppLeader = m.Me.Leader, m.Opp.Leader
		}
	}
}

func (e *env) yourDeckMenu(leader string) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("Open on Decks").Chosen() {
			e.openLeader(leader)
		}
		if m.Item("Open on Matchups").Chosen() {
			e.setDeck(leader)
			e.page = pageMatchups
		}
		if m.Item("Show in Matches").Chosen() {
			e.setDeck(leader)
			e.page = pageMatches
		}
		m.Separator()
		e.leaderCardItems(m, leader)
	}
}

// opponentMenu is the menu of an opposing leader. deck is the user's deck it's faced with, or
// "" to keep the deck filter as it is.
func (e *env) opponentMenu(deck, leader string) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("Show in Matches").Chosen() {
			e.showOpponentGames(deck, leader)
		}
		if m.Item("Filter to games vs " + e.snap.card(leader).Name).Chosen() {
			e.filterOpponent(leader)
		}
		m.Separator()
		e.leaderCardItems(m, leader)
	}
}

func (e *env) showOpponentGames(deck, leader string) {
	if deck != "" {
		e.filter.deck = deck
	}
	e.filter.oppLeader = leader
	e.page = pageMatches
}

func (e *env) filterOpponent(leader string) {
	e.filter.oppLeader = leader
}

// leaderCardItems are the items of a leader as a card, after what the menu does with it as a
// deck.
func (e *env) leaderCardItems(m *ui.Menu, code string) {
	if m.Item("Open in Cards").Chosen() {
		e.nav.openInCards(code)
	}
	if m.Item("View art").Chosen() {
		e.nav.viewArt(code)
	}
}

func (v *view) openInCards(code string) {
	s := &v.cardsUI
	if s.selected != code {
		s.selected, s.viewed, s.versions = code, "", ""
	}
	s.viewer = false
	v.page = pageCards
}

// viewArt opens the art viewer over the page, which build draws off the Cards page.
func (v *view) viewArt(code string) {
	s := &v.cardsUI
	if s.selected != code {
		s.selected, s.viewed, s.versions = code, "", ""
	}
	s.viewer = true
}

// showsHands reports whether the page is a Decks tab where "Show hands with" fits a card.
func (v *view) showsHands() bool {
	return v.page == pageDecks && v.detail.fingerprint == "" && v.decks.tab != tabResults
}

// showHands opens the Hands tab on the hands the card was dealt in.
func (v *view) showHands(code string) {
	s := &v.decks
	s.tab, s.handsCard, s.allHands, s.edit = tabHands, code, false, nil
}

// pillMenu gives a pill, which opens its own menu on left click, menu on right-click. mygo
// draws one menu per element, so menu sits on a box around the pill.
func pillMenu(c *ui.Context, pill func(), menu func(*ui.Menu)) {
	ui.Row(c).PopoverMenu(menu).Children(pill)
}

// clearPillMenu is a pill's right-click menu: one item to reset it, greyed out while it's
// already there, and Clear all filters.
func (e *env) clearPillMenu(label string, at bool, reset func()) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item(label).Disabled(at).Chosen() {
			reset()
		}
		if m.Item("Clear all filters").Disabled(!e.canClear()).Chosen() {
			e.clearFilters()
		}
	}
}
