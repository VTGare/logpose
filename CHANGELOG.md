# Changelog

## 0.5.0

- Export a match to the sim. Export to sim on a match's page, or in any game's right-click menu,
  saves a file you open with LoadState From File in the sim, with the board at the start of every
  turn.
  Files go to a sim-saves folder next to your data, and Settings › Sim saves picks another one.
- The opening shuffle doesn't name every card, so a deck card the log never shows takes its most
  likely code from your other games with that leader. It never makes a fifth copy, and when
  nothing fits the deck is left a card short.
- A match's page shows the bookmark as a star by the date, and Add note as a pencil.
- Replays keep the DON on a card when it rests or stands up, and pick the right copy when two
  can attack, so the board matches the log's own counts in every game we checked. Your games are
  read again on the first start.

## 0.4.0

- Filter the card database by power. Open Power and drag the range, or pick exactly a power, that
  power or less, or that power or more, the way card text puts it.
- Cost is a row of buttons, one for each cost from 0 to 10. A cost no card has under your other
  filters is dimmed.
- Pick two traits, keywords, attributes or sets and click the "or" between them to show only the
  cards that have both.
- Reading Bandai's card list shows quietly next to the card count, with a spinner.

## 0.3.0

- Sort the card database by set, name, cost, power, counter, rarity or color, and click the list
  headers to sort. The sort and the images or list view are kept between starts.
- Right-click a game, card, deck, opposing leader, build or filter pill anywhere for what you can
  do with it, such as opening the match, putting a card's art in the sim, or clearing a filter.
- Pick which format the pages start on, the latest one or all formats, from the format menu or the
  Formats page. Until you pick, they start on all formats.
- The sidebar puts Matches second, and Formats sits at the bottom next to Settings.
- Opening a match from Decks, the Overview or Matchups goes back there, not to Matches.
- Games from the sim's latest update show the time you played them. They showed two hours early,
  some under the wrong day.
- A card's plays count right on the Cards tab. Some plays put out by an effect counted as paid,
  and events used as counters didn't count as counters. Your games are read again on the first
  start.
- A hand you sent back in a mulligan no longer counts as in hand.
- Rates under 5 games read "too few" on every page.
- Settings shows the sim's version from your latest game.
- A damaged card data file is downloaded again instead of leaving the card database without bans
  and printings.

## 0.2.1

- A game you lose by running out of cards shows as a loss by deck out. Since the sim's latest
  update it showed an Unknown result.
- Ranked games the sim's ranked panel joined for you count as ranked. Before, they showed as
  casual, because the sim logs their room code without the room letter.

## 0.2.0

- An art viewer in the card database. Click a card's picture in the panel, or right-click any card
  and pick View art, to see it full size with every art of the card listed beside it.
- In the viewer, the Up and Down keys switch art, and you can put the art you're looking at in
  the sim, save it or copy it.

## 0.1.2

- A game's result shows as soon as it ends. Before, it stayed Unknown until logpose restarted.
- The sim can always save a finished game while logpose is reading it.

## 0.1.1

Fixes for logs from the sim's latest update.

- Games played after the update showed an Unknown result. They now show who won.
- Games no longer end as Unknown when logpose reads them while you're still playing.

## 0.1.0

The first release.

- Imports every game from the sim's combat logs and keeps them in one database.
- Overview, Matches, Matchups and Decks pages with win rates, turn order, builds, card stats,
  hands and mulligans.
- Match pages with opening hands, turn by turn charts, key moments and a replay.
- A live strip during games with your record against the opponent's leader.
- A card database with art from Limitless, bans from Bandai, and art you can put in the sim.
