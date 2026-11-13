package page

import (
	"github.com/egoist/mygo/ui"
)

// Every right-click menu is a PopoverMenu, drawn in the app's look. Menus use one wording:
// "Open on …" goes to a page about the thing, "Show in Matches" to its games, and "Filter to …"
// sets a pill and stays on the page. Every item can also be reached without right-click.

func (e *Env) YourDeckMenu(leader string) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("Open on Decks").Chosen() {
			e.OpenLeader(leader)
		}
		if m.Item("Open on Matchups").Chosen() {
			e.SetDeck(leader)
			e.Page = Matchups
		}
		if m.Item("Show in Matches").Chosen() {
			e.SetDeck(leader)
			e.Page = Matches
		}
		m.Separator()
		e.leaderCardItems(m, leader)
	}
}

// OpponentMenu is the menu of an opposing leader. deck is the user's deck it's faced with, or
// "" to keep the deck filter as it is.
func (e *Env) OpponentMenu(deck, leader string) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("Show in Matches").Chosen() {
			e.ShowOpponentGames(deck, leader)
		}
		if m.Item("Filter to games vs " + e.Snap.Card(leader).Name).Chosen() {
			e.FilterOpponent(leader)
		}
		m.Separator()
		e.leaderCardItems(m, leader)
	}
}

func (e *Env) ShowOpponentGames(deck, leader string) {
	if deck != "" {
		e.Filter.Deck = deck
	}
	e.Filter.OppLeader = leader
	e.Page = Matches
}

func (e *Env) FilterOpponent(leader string) {
	e.Filter.OppLeader = leader
}

// leaderCardItems are the items of a leader as a card, after what the menu does with it as a
// deck.
func (e *Env) leaderCardItems(m *ui.Menu, code string) {
	if m.Item("Open in Cards").Chosen() {
		e.Nav.OpenInCards(code)
	}
	if m.Item("View art").Chosen() {
		e.Nav.ViewArt(code)
	}
}

// pillMenu gives a pill, which opens its own menu on left click, menu on right-click. mygo
// draws one menu per element, so menu sits on a box around the pill.
func pillMenu(c *ui.Context, pill func(), menu func(*ui.Menu)) {
	ui.Row(c).PopoverMenu(menu).Children(pill)
}

// clearPillMenu is a pill's right-click menu: one item to reset it, greyed out while it's
// already there, and Clear all filters.
func (e *Env) clearPillMenu(label string, at bool, reset func()) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item(label).Disabled(at).Chosen() {
			reset()
		}
		if m.Item("Clear all filters").Disabled(!e.canClear()).Chosen() {
			e.clearFilters()
		}
	}
}
