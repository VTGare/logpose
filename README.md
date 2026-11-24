# logpose

A stat tracker for [OPTCG Simulator](https://www.patreon.com/Batsu). Keep it running next to the
sim on Windows and it picks up your games automatically.

## What's in it

**Stats and matchups.** Track win rates by deck, matchup, turn order, and format, with your
current session and streak on the overview and in the tray.

**Matches and replays.** Open any game to see opening hands, life/hand/board turn by turn, and
check your opponent's decklist. Key moments pull out big turns and break the lethal turn down
attack by attack. You can watch the game back in the animated replay viewer, export it as a save
state to load into the sim, or copy either decklist to your clipboard in sim format.

**Decks and hands.** Browse every list version you've played with card diffs, cost curves, and
counter totals, and group lists into builds to compare variants. Check how individual cards do in
your opening hand, on curve, and in play, or deal practice hands in the hand tester with each
card's real opening-hand record.

**Live strip.** While a game is running, logpose shows your opponent, your record against their
leader going first and second, and both players' life and hand counts.

**Card database and alt art.** Search and filter every card, check Bandai's ban list, and view
printings full size. You can also swap card art directly in the sim.

## Install

Download `logpose-setup-<version>-amd64.exe` from
[Releases](https://github.com/VTGare/logpose/releases) and run it.

The installer isn't code signed yet, so Windows SmartScreen will warn on first run. Choose More
info, then Run anyway.

logpose keeps your data in `%APPDATA%\logpose`. Uninstalling leaves it there.

Windows only for now. A Linux build for the sim's native Linux version is planned.

## Data sources

Games are read locally from the sim on your PC. Card text and art come from
[Limitless TCG](https://onepiece.limitlesstcg.com), with extra printings and bans from Bandai's
official card list. Art is downloaded and cached on your PC, never shipped with logpose.

logpose is an unofficial fan project and isn't affiliated with Bandai, OPTCG Simulator, OPBounty, or
Limitless TCG.

## Building

Needs Go (see `go.mod`) and [Task](https://taskfile.dev).

```sh
task build     # out/bin/logpose.exe
task check     # format check, vet, lint, and race tests
task release   # installer and update files (needs NSIS and the update signing key)
```

## License

[MIT](LICENSE)
