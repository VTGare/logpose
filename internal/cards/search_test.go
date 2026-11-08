package cards

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("SearchTargets",
	func(effect string, want []string) {
		Expect(SearchTargets(effect)).To(Equal(want))
	},
	Entry("a type in braces, with a color before it",
		"[On Play] Look at 5 cards from the top of your deck; reveal up to 1 green {Land of Wano} type card other than [Otama] and add it to your hand. Then, place the rest at the bottom of your deck in any order.",
		[]string{"Land of Wano"}),
	Entry("two types",
		"[Main] Look at 4 cards from the top of your deck; reveal up to 1 {Supernovas} or {Straw Hat Crew} type Character card and add it to your hand.",
		[]string{"Supernovas", "Straw Hat Crew"}),
	Entry("a type the card's type has to include",
		`[On Play] If your Leader's type includes "CP", look at 3 cards from the top of your deck; reveal up to 1 card with a type including "CP" other than [Spandam] and add it to your hand. Then, trash the rest.`,
		[]string{"CP"}),
	Entry("an attribute",
		"[On Play] Look at 4 cards from the top of your deck; reveal up to 1 card with the <Wisdom> attribute and add it to your hand.",
		[]string{"Wisdom"}),
	Entry("a trigger",
		"[On Play] If your Leader is [Nico Robin], look at 3 cards from the top of your deck; reveal up to 1 card with a [Trigger] and add it to your hand.",
		[]string{"Trigger"}),
	Entry("an attribute or a color of Event",
		"[On Play] If your Leader has the <Slash> attribute, look at 5 cards from the top of your deck; reveal up to 1 <Slash> attribute card or green Event and add it to your hand.",
		[]string{"Slash", "green Event"}),
	Entry("a cost",
		"[On Play] If your Leader is [Sabo], [Portgas.D.Ace] or [Monkey.D.Luffy], look at 4 cards from the top of your deck; reveal up to 1 card with a cost of 3 or more and add it to your hand.",
		[]string{"Cost 3+"}),
	Entry("a card by name, but not the one it leaves out",
		"[On Play] Look at 5 cards from the top of your deck; reveal up to 1 [Smiley] other than [Caesar Clown] and add it to your hand.",
		[]string{"Smiley"}),
	Entry("an exact cost and a power",
		"[On Play] Look at 5 cards from the top of your deck; reveal up to 1 Character card with a cost of 2 and 6000 power and add it to your hand.",
		[]string{"Cost 2", "6000 power"}),
	Entry("a total of several, of a card type or a type",
		"[On Play] Look at 5 cards from the top of your deck; reveal up to a total of 2 Stage cards or {Water Seven} type cards and add them to your hand.",
		[]string{"Water Seven", "Stage"}),
	Entry("a comma before adding",
		"[On Play] Look at 4 cards from the top of your deck; reveal up to 1 {Navy} type card, add it to your hand and place the rest at the bottom of your deck in any order.",
		[]string{"Navy"}),
	Entry("a card that doesn't search",
		"[Counter] Up to 1 of your Leader or Character cards gains +4000 power during this battle.",
		[]string(nil)),
)
