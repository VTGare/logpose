# logpose

A stat tracker for [OPTCG Simulator](https://www.patreon.com/Batsu). It runs next to
the sim and records your games from its combat logs. It needs no account and works offline.

- Win rates by deck, matchup, turn order, format and build, with sample sizes.
- Every match with its opening hands, a turn by turn chart, key moments and a replay.
- Card stats for your decks: how cards do in the opening hand, when drawn and when played, and
  how kept hands and mulligans do.
- A live strip during a game with your record against the opponent's leader.
- A card database with every card's text and art, and art you can put in the sim.

## Install

Download `logpose-setup-<version>-amd64.exe` from
[Releases](https://github.com/VTGare/logpose/releases) and run it. It installs for your Windows
user without admin rights, and logpose updates itself from then on.

The installer isn't code signed yet, so Windows SmartScreen warns about it. Choose More info,
then Run anyway.

logpose keeps its database in `%APPDATA%\logpose`. Uninstalling leaves it there.

logpose is Windows only for now. A Linux build for the sim's native Linux version is planned.

## Where the data comes from

Games come from the sim's own logs, read on your PC. Card text and art come from
[Limitless TCG](https://onepiece.limitlesstcg.com), and bans from Bandai's official card list.
Art is downloaded and cached on your PC, never shipped with logpose.

logpose is a fan project. It isn't affiliated with Bandai, OPTCG Simulator, OPBounty or
Limitless TCG.

## Building

Needs Go (see `go.mod`) and [Task](https://taskfile.dev).

```sh
task build     # out/bin/logpose.exe
task check     # format check, vet, lint and tests
task release   # the installer and update files, which need NSIS and the update signing key
```

## License

[MIT](LICENSE)
