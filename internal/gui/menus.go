package gui

import (
	"github.com/egoist/mygo/ui"
)

// Every right-click menu is a PopoverMenu, drawn in the app's look. Menus use one wording:
// "Open on …" goes to a page about the thing, "Show in Matches" to its games, and "Filter to …"
// sets a pill and stays on the page. Every item can also be reached without right-click.

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
	s := &v.cards.state
	if s.selected != code {
		s.selected, s.viewed, s.versions = code, "", ""
	}
	s.viewer = false
	v.page = pageCards
}

// viewArt opens the art viewer over the page, which build draws off the Cards page.
func (v *view) viewArt(code string) {
	s := &v.cards.state
	if s.selected != code {
		s.selected, s.viewed, s.versions = code, "", ""
	}
	s.viewer = true
}

// showsHands reports whether the page is a Decks tab where "Show hands with" fits a card.
func (v *view) showsHands() bool {
	return v.page == pageDecks && v.matches.detail.fingerprint == "" && v.decks.tab != tabResults
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
